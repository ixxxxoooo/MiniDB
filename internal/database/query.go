package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"minidb/internal/logger"
	"time"
)

// ColumnMeta 列元数据
type ColumnMeta struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

// QueryResult 查询结果
type QueryResult struct {
	Columns     []ColumnMeta             `json:"columns"`
	Rows        []map[string]interface{} `json:"rows"`
	Total       int64                    `json:"total"`
	Duration    int64                    `json:"duration"`
	Error       string                   `json:"error,omitempty"`
	AutoLimited bool                     `json:"autoLimited,omitempty"` // 标记是否被自动追加了 LIMIT
}

// Filter 筛选条件
type Filter struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// Sort 排序条件
type Sort struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}

// DefaultPageSize 默认分页大小（用户未写 LIMIT 时自动追加）
const DefaultPageSize = 500

// hasLimitClause 检测 SQL 是否已包含 LIMIT 子句。
// 用词边界匹配，避免列名/别名/字符串字面量中的 "limit" 子串（如 `limited_user`、'LIMITED'）造成误判。
var reLimitClause = regexp.MustCompile(`(?i)\blimit\b`)

func hasLimitClause(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	upper = strings.TrimRight(upper, "; \t\n\r")
	return reLimitClause.MatchString(upper)
}

// buildAutoCountSQL 构造自动分页用的 COUNT 语句。
// 对结构简单的单表查询（无子查询/JOIN/UNION/GROUP BY/DISTINCT/ORDER BY 等）直接生成
// "SELECT COUNT(*) FROM <from 片段>"，避免把整条查询包成子查询让数据库执行两遍；
// 其余情况回退到子查询包装，保持原有正确性。
func buildAutoCountSQL(cleanSQL string) string {
	if from, ok := extractSimpleCountFrom(cleanSQL); ok {
		return "SELECT COUNT(*) FROM " + from
	}
	return fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS __auto_count__", cleanSQL)
}

// extractSimpleCountFrom 仅在查询结构足够简单时提取可直接用于 COUNT 的 FROM 片段。
// 保守规则：无括号（子查询/函数调用）、无引号字面量、无 JOIN/UNION/GROUP BY/HAVING/DISTINCT，
// 且只出现一次 FROM；命中任一风险特征即返回 false 走子查询包装。
func extractSimpleCountFrom(cleanSQL string) (string, bool) {
	cleaned := StripLeadingSQLComments(strings.TrimSpace(cleanSQL))
	if cleaned == "" || len(cleaned) < len("SELECT 1 FROM t") {
		return "", false
	}
	upper := strings.ToUpper(cleaned)
	if !strings.HasPrefix(upper, "SELECT ") {
		return "", false
	}
	for _, risk := range []string{"(", ")", "'", `"`, " JOIN ", " UNION ", " GROUP BY ", " HAVING ", " DISTINCT "} {
		if strings.Contains(upper, risk) {
			return "", false
		}
	}
	fromIdx := strings.Index(upper, " FROM ")
	if fromIdx < 0 {
		return "", false
	}
	// 只允许一个 FROM（多表连接已通过 JOIN 关键词排除，这里防御逗号连接等写法）
	if strings.Index(upper[fromIdx+len(" FROM "):], " FROM ") >= 0 {
		return "", false
	}
	// 从 FROM 之后截断 ORDER BY / LIMIT / OFFSET（词边界匹配，列名恰好为 order/limit 时不受影响）
	rest := cleaned[fromIdx+len(" FROM "):]
	restUpper := upper[fromIdx+len(" FROM "):]
	for _, kw := range []string{" ORDER BY ", " LIMIT ", " OFFSET "} {
		if k := strings.Index(restUpper, kw); k >= 0 {
			rest = rest[:k]
			restUpper = restUpper[:k]
		}
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "", false
	}
	return rest, true
}

// IsSelectQuery 判断是否为返回结果集的查询（SELECT 或 WITH/CTE）。
// 用词边界匹配，避免误把 SELECTX、WITHIN 等识别为查询动词；前导注释不影响判断。
func IsSelectQuery(sqlStr string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(StripLeadingSQLComments(sqlStr)))
	if trimmed == "" {
		return false
	}
	return strings.HasPrefix(trimmed, "SELECT ") ||
		trimmed == "SELECT" ||
		strings.HasPrefix(trimmed, "WITH ") ||
		trimmed == "WITH"
}

// ExecuteQuery 执行 SQL 查询（对无 LIMIT 的 SELECT 自动追加分页）
func ExecuteQuery(db *sql.DB, sqlStr string) (*QueryResult, error) {
	return ExecuteQueryPagedContext(context.Background(), db, sqlStr, 1, DefaultPageSize, true)
}

// ExecuteQueryRaw 执行 SQL 查询（不自动追加 LIMIT，用于导出等场景）
func ExecuteQueryRaw(db *sql.DB, sqlStr string) (*QueryResult, error) {
	return ExecuteQueryPagedContext(context.Background(), db, sqlStr, 0, 0, false)
}

// QueryRawWithArgs 执行带参数的原始查询并返回结果（供导出游标分页等场景使用）。
func QueryRawWithArgs(db *sql.DB, sqlStr string, args ...interface{}) (*QueryResult, error) {
	start := time.Now()
	rows, err := db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows, start)
}

// ExecuteQueryPaged 执行 SQL 查询，支持分页
// page=0 表示不分页；autoLimit=true 且 SELECT 未写 LIMIT 时，先 COUNT(*) 再 LIMIT/OFFSET
func ExecuteQueryPaged(db *sql.DB, sqlStr string, page, pageSize int, autoLimit bool) (*QueryResult, error) {
	return ExecuteQueryPagedContext(context.Background(), db, sqlStr, page, pageSize, autoLimit)
}

// ExecuteQueryPagedContext 执行 SQL 查询，支持取消。
// page=0 表示不分页；autoLimit=true 且 SELECT 未写 LIMIT 时，先 COUNT(*) 再 LIMIT/OFFSET
func ExecuteQueryPagedContext(ctx context.Context, db *sql.DB, sqlStr string, page, pageSize int, autoLimit bool) (*QueryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	start := time.Now()
	logger.Debug("执行 SQL: verb=%s len=%d", SQLLeadingVerb(sqlStr), len(strings.TrimSpace(sqlStr)))

	// 非查询语句直接执行（先剥离前导注释再判断动词，
	// 避免 "/* 说明 */ INSERT ..."、"CREATE OR REPLACE VIEW ..." 等被误判为查询）
	switch SQLLeadingVerb(sqlStr) {
	case "insert", "update", "delete", "create", "alter", "drop", "truncate", "replace":
		result, err := db.ExecContext(ctx, sqlStr)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return &QueryResult{
				Error:    err.Error(),
				Duration: time.Since(start).Milliseconds(),
			}, nil
		}
		affected, _ := result.RowsAffected()
		return &QueryResult{
			Total:    affected,
			Duration: time.Since(start).Milliseconds(),
			Rows:     []map[string]interface{}{},
			Columns:  []ColumnMeta{},
		}, nil
	}

	wasAutoLimited := false
	actualSQL := sqlStr
	var totalCount int64 = -1

	// 对 SELECT 且未写 LIMIT 的查询：先 COUNT 获取总数，再分页查询
	if autoLimit && IsSelectQuery(sqlStr) && !hasLimitClause(sqlStr) {
		cleanSQL := strings.TrimRight(strings.TrimSpace(sqlStr), ";")

		countSQL := buildAutoCountSQL(cleanSQL)
		err := db.QueryRowContext(ctx, countSQL).Scan(&totalCount)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			logger.Warn("自动分页 COUNT 失败，降级为全量查询: %v", err)
			totalCount = -1
		} else {
			logger.Info("自动分页: 总行数=%d", totalCount)
		}

		if totalCount >= 0 && page > 0 && pageSize > 0 {
			offset := (page - 1) * pageSize
			actualSQL = fmt.Sprintf("%s LIMIT %d OFFSET %d", cleanSQL, pageSize, offset)
			wasAutoLimited = true
			logger.Info("自动分页: page=%d pageSize=%d offset=%d", page, pageSize, offset)
		}
	}

	rows, err := db.QueryContext(ctx, actualSQL)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return &QueryResult{
			Error:    err.Error(),
			Duration: time.Since(start).Milliseconds(),
		}, nil
	}
	defer rows.Close()

	result, err := scanRows(rows, start)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}

	result.AutoLimited = wasAutoLimited
	if totalCount >= 0 {
		result.Total = totalCount
	}
	return result, nil
}

// QueryTableData 分页查询表数据
func QueryTableData(db *sql.DB, dbType, dbName, table string, page, pageSize int, filters []Filter, sorts []Sort) (*QueryResult, error) {
	start := time.Now()

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}

	// 构建 WHERE 子句（含 __any 时需先探测表列名）
	var tableColumns []string
	var err error
	if hasAnyColumnFilter(filters) {
		tableColumns, err = getTableColumnNames(db, dbType, dbName, table)
		if err != nil {
			return &QueryResult{Error: err.Error(), Duration: time.Since(start).Milliseconds()}, nil
		}
	}
	where, args, err := buildWhereClause(dbType, filters, tableColumns)
	if err != nil {
		return &QueryResult{Error: err.Error(), Duration: time.Since(start).Milliseconds()}, nil
	}

	// 构建 ORDER BY 子句
	orderBy, err := buildOrderByClause(dbType, sorts)
	if err != nil {
		return &QueryResult{Error: err.Error(), Duration: time.Since(start).Milliseconds()}, nil
	}

	// 获取总行数
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", quoteTable(dbType, dbName, table), where)
	var total int64
	if err := db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return &QueryResult{
			Error:    err.Error(),
			Duration: time.Since(start).Milliseconds(),
		}, nil
	}

	// 构建查询（所有支持的数据库类型均使用 LIMIT/OFFSET 分页语法）
	offset := (page - 1) * pageSize
	querySQL := fmt.Sprintf("SELECT * FROM %s %s %s LIMIT %d OFFSET %d",
		quoteTable(dbType, dbName, table), where, orderBy, pageSize, offset)

	rows, err := db.Query(querySQL, args...)
	if err != nil {
		return &QueryResult{
			Error:    err.Error(),
			Duration: time.Since(start).Milliseconds(),
		}, nil
	}
	defer rows.Close()

	result, err := scanRows(rows, start)
	if err != nil {
		return nil, err
	}
	result.Total = total
	return result, nil
}

// DeleteRow 删除行
func DeleteRow(db *sql.DB, dbType, dbName, table string, primaryKey map[string]interface{}) error {
	if len(primaryKey) == 0 {
		return fmt.Errorf("主键不能为空")
	}

	var conditions []string
	var args []interface{}
	i := 1
	for col, val := range primaryKey {
		switch dbType {
		case "postgres":
			conditions = append(conditions, fmt.Sprintf("%s = $%d", QuoteIdent(dbType, col), i))
		default:
			conditions = append(conditions, fmt.Sprintf("%s = ?", QuoteIdent(dbType, col)))
		}
		args = append(args, val)
		i++
	}

	// MySQL 和 TiDB 支持 DELETE ... LIMIT 1，StarRocks 不支持
	limitClause := ""
	if dbType == "mysql" || dbType == "tidb" {
		limitClause = " LIMIT 1"
	}
	sqlStr := fmt.Sprintf("DELETE FROM %s WHERE %s%s",
		quoteTable(dbType, dbName, table),
		strings.Join(conditions, " AND "),
		limitClause)

	_, err := db.Exec(sqlStr, args...)
	return err
}

// UpdateRow 根据主键更新行
func UpdateRow(db *sql.DB, dbType, dbName, table string, primaryKey map[string]interface{}, changes map[string]interface{}) error {
	if len(primaryKey) == 0 {
		return fmt.Errorf("主键不能为空")
	}
	if len(changes) == 0 {
		return fmt.Errorf("没有要更新的字段")
	}

	var setClauses []string
	var whereClauses []string
	var args []interface{}
	idx := 1

	for col, val := range changes {
		switch dbType {
		case "postgres":
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", QuoteIdent(dbType, col), idx))
		default:
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", QuoteIdent(dbType, col)))
		}
		args = append(args, val)
		idx++
	}

	for col, val := range primaryKey {
		switch dbType {
		case "postgres":
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", QuoteIdent(dbType, col), idx))
		default:
			whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", QuoteIdent(dbType, col)))
		}
		args = append(args, val)
		idx++
	}

	sqlStr := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		quoteTable(dbType, dbName, table),
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "))

	logger.Info("更新行: %s", sqlStr)
	_, err := db.Exec(sqlStr, args...)
	return err
}

// UpdateRowTx 在事务内更新行
func UpdateRowTx(tx *sql.Tx, dbType, dbName, table string, primaryKey map[string]interface{}, changes map[string]interface{}) error {
	if len(primaryKey) == 0 {
		return fmt.Errorf("主键不能为空")
	}
	if len(changes) == 0 {
		return fmt.Errorf("没有要更新的字段")
	}

	var setClauses []string
	var whereClauses []string
	var args []interface{}
	idx := 1

	for col, val := range changes {
		switch dbType {
		case "postgres":
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", QuoteIdent(dbType, col), idx))
		default:
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", QuoteIdent(dbType, col)))
		}
		args = append(args, val)
		idx++
	}

	for col, val := range primaryKey {
		switch dbType {
		case "postgres":
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", QuoteIdent(dbType, col), idx))
		default:
			whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", QuoteIdent(dbType, col)))
		}
		args = append(args, val)
		idx++
	}

	sqlStr := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		quoteTable(dbType, dbName, table),
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "))

	logger.Info("事务更新行: %s", sqlStr)
	_, err := tx.Exec(sqlStr, args...)
	return err
}

// InsertRow 插入新行
func InsertRow(db *sql.DB, dbType, dbName, table string, row map[string]interface{}) error {
	if len(row) == 0 {
		return fmt.Errorf("没有要插入的数据")
	}

	var cols []string
	var placeholders []string
	var args []interface{}
	idx := 1

	for col, val := range row {
		switch dbType {
		case "postgres":
			cols = append(cols, QuoteIdent(dbType, col))
			placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		default:
			cols = append(cols, QuoteIdent(dbType, col))
			placeholders = append(placeholders, "?")
		}
		args = append(args, val)
		idx++
	}

	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteTable(dbType, dbName, table),
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "))

	logger.Info("插入行: %s", sqlStr)
	_, err := db.Exec(sqlStr, args...)
	return err
}

// DeleteRowTx 在事务内删除行
func DeleteRowTx(tx *sql.Tx, dbType, dbName, table string, primaryKey map[string]interface{}) error {
	if len(primaryKey) == 0 {
		return fmt.Errorf("主键不能为空")
	}
	var conditions []string
	var args []interface{}
	i := 1
	for col, val := range primaryKey {
		switch dbType {
		case "postgres":
			conditions = append(conditions, fmt.Sprintf("%s = $%d", QuoteIdent(dbType, col), i))
		default:
			conditions = append(conditions, fmt.Sprintf("%s = ?", QuoteIdent(dbType, col)))
		}
		args = append(args, val)
		i++
	}
	limitClause := ""
	if dbType == "mysql" || dbType == "tidb" {
		limitClause = " LIMIT 1"
	}
	sqlStr := fmt.Sprintf("DELETE FROM %s WHERE %s%s",
		quoteTable(dbType, dbName, table),
		strings.Join(conditions, " AND "),
		limitClause)
	logger.Info("事务删除行: %s", sqlStr)
	_, err := tx.Exec(sqlStr, args...)
	return err
}

// InsertRowTx 在事务内插入行
func InsertRowTx(tx *sql.Tx, dbType, dbName, table string, row map[string]interface{}) error {
	if len(row) == 0 {
		return fmt.Errorf("没有要插入的数据")
	}
	var cols []string
	var placeholders []string
	var args []interface{}
	idx := 1
	for col, val := range row {
		switch dbType {
		case "postgres":
			cols = append(cols, QuoteIdent(dbType, col))
			placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		default:
			cols = append(cols, QuoteIdent(dbType, col))
			placeholders = append(placeholders, "?")
		}
		args = append(args, val)
		idx++
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteTable(dbType, dbName, table),
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "))
	logger.Info("事务插入行: %s", sqlStr)
	_, err := tx.Exec(sqlStr, args...)
	return err
}

// GenerateInsertSQL 生成 INSERT 语句
func GenerateInsertSQL(table string, row map[string]interface{}) string {
	var columns []string
	var values []string

	for col, val := range row {
		columns = append(columns, col)
		switch v := val.(type) {
		case nil:
			values = append(values, "NULL")
		case string:
			values = append(values, fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''")))
		case float64:
			values = append(values, fmt.Sprintf("%v", v))
		case bool:
			if v {
				values = append(values, "1")
			} else {
				values = append(values, "0")
			}
		default:
			values = append(values, fmt.Sprintf("'%v'", v))
		}
	}

	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);",
		table,
		strings.Join(columns, ", "),
		strings.Join(values, ", "))
}

func scanRows(rows *sql.Rows, start time.Time) (*QueryResult, error) {
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	var columns []ColumnMeta
	for _, ct := range colTypes {
		nullable, _ := ct.Nullable()
		columns = append(columns, ColumnMeta{
			Name:     ct.Name(),
			Type:     ct.DatabaseTypeName(),
			Nullable: nullable,
		})
	}

	var resultRows []map[string]interface{}
	scanCols := make([]interface{}, len(columns))
	scanVals := make([]interface{}, len(columns))
	for i := range scanCols {
		scanCols[i] = &scanVals[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanCols...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := scanVals[i]
			switch v := val.(type) {
			case []byte:
				row[col.Name] = string(v)
			default:
				row[col.Name] = v
			}
		}
		resultRows = append(resultRows, row)
	}

	if resultRows == nil {
		resultRows = []map[string]interface{}{}
	}

	return &QueryResult{
		Columns:  columns,
		Rows:     resultRows,
		Total:    int64(len(resultRows)),
		Duration: time.Since(start).Milliseconds(),
	}, nil
}

func buildWhereClause(dbType string, filters []Filter, tableColumns []string) (string, []interface{}, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	var conditions []string
	var args []interface{}
	for _, f := range filters {
		if f.Column == "" {
			return "", nil, fmt.Errorf("不支持的筛选列: %s", f.Column)
		}
		if f.Column == "__any" {
			cond, anyArgs, err := buildAnyColumnCondition(dbType, f, tableColumns)
			if err != nil {
				return "", nil, err
			}
			conditions = append(conditions, cond)
			args = append(args, anyArgs...)
			continue
		}
		op := strings.ToUpper(strings.TrimSpace(f.Operator))
		col := QuoteIdent(dbType, f.Column)
		switch op {
		case "IS NULL":
			conditions = append(conditions, fmt.Sprintf("%s IS NULL", col))
		case "IS NOT NULL":
			conditions = append(conditions, fmt.Sprintf("%s IS NOT NULL", col))
		case "=", "!=", "<>", ">", "<", ">=", "<=", "LIKE", "NOT LIKE":
			placeholder := "?"
			if dbType == "postgres" {
				placeholder = fmt.Sprintf("$%d", len(args)+1)
			}
			conditions = append(conditions, fmt.Sprintf("%s %s %s", col, op, placeholder))
			args = append(args, f.Value)
		case "IN":
			values := splitFilterListValue(f.Value)
			if len(values) == 0 {
				return "", nil, fmt.Errorf("IN 筛选值不能为空")
			}
			placeholders := make([]string, 0, len(values))
			for _, value := range values {
				if dbType == "postgres" {
					placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)+1))
				} else {
					placeholders = append(placeholders, "?")
				}
				args = append(args, value)
			}
			conditions = append(conditions, fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")))
		default:
			return "", nil, fmt.Errorf("不支持的筛选操作符: %s", f.Operator)
		}
	}
	return "WHERE " + strings.Join(conditions, " AND "), args, nil
}

func buildOrderByClause(dbType string, sorts []Sort) (string, error) {
	if len(sorts) == 0 {
		return "", nil
	}
	var parts []string
	for _, s := range sorts {
		if s.Column == "" || s.Column == "__any" {
			return "", fmt.Errorf("不支持的排序列: %s", s.Column)
		}
		dir := strings.ToUpper(strings.TrimSpace(s.Direction))
		if dir != "ASC" && dir != "DESC" {
			return "", fmt.Errorf("不支持的排序方向: %s", s.Direction)
		}
		parts = append(parts, fmt.Sprintf("%s %s", QuoteIdent(dbType, s.Column), dir))
	}
	return "ORDER BY " + strings.Join(parts, ", "), nil
}

func quoteTable(dbType, dbName, table string) string {
	if IsMySQLCompatible(dbType) {
		if dbName != "" {
			return fmt.Sprintf("%s.%s", QuoteIdent(dbType, dbName), QuoteIdent(dbType, table))
		}
		return QuoteIdent(dbType, table)
	}
	switch dbType {
	case "postgres", "sqlite":
		return QuoteIdent(dbType, table)
	default:
		return table
	}
}

// QuoteTableName 对表名进行引用，不含库名前缀（供外部调用）
func QuoteTableName(dbType, table string) string {
	if IsMySQLCompatible(dbType) {
		return QuoteIdent(dbType, table)
	}
	switch dbType {
	case "postgres", "sqlite":
		return QuoteIdent(dbType, table)
	default:
		return table
	}
}

func splitFilterListValue(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `"'`)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// hasAnyColumnFilter 判断筛选条件是否包含任意列搜索
func hasAnyColumnFilter(filters []Filter) bool {
	for _, f := range filters {
		if f.Column == "__any" {
			return true
		}
	}
	return false
}

// getTableColumnNames 探测表的列名列表
func getTableColumnNames(db *sql.DB, dbType, dbName, table string) ([]string, error) {
	probeSQL := fmt.Sprintf("SELECT * FROM %s LIMIT 0", quoteTable(dbType, dbName, table))
	rows, err := db.Query(probeSQL)
	if err != nil {
		return nil, fmt.Errorf("获取表列信息失败: %w", err)
	}
	defer rows.Close()
	return rows.Columns()
}

// castColumnAsText 按方言将列转为文本以便跨类型模糊匹配
func castColumnAsText(dbType, column string) string {
	quoted := QuoteIdent(dbType, column)
	switch dbType {
	case "postgres", "sqlite":
		return fmt.Sprintf("CAST(%s AS TEXT)", quoted)
	default:
		return fmt.Sprintf("CAST(%s AS CHAR)", quoted)
	}
}

// buildAnyColumnCondition 将 __any 展开为跨列 OR 的 LIKE 条件
func buildAnyColumnCondition(dbType string, filter Filter, tableColumns []string) (string, []interface{}, error) {
	value := strings.TrimSpace(filter.Value)
	if value == "" {
		return "", nil, fmt.Errorf("任意列搜索关键字不能为空")
	}
	if len(tableColumns) == 0 {
		return "", nil, fmt.Errorf("表无可用列，无法执行任意列搜索")
	}
	likeValue := "%" + value + "%"
	var parts []string
	var args []interface{}
	for _, col := range tableColumns {
		placeholder := "?"
		if dbType == "postgres" {
			placeholder = fmt.Sprintf("$%d", len(args)+1)
		}
		parts = append(parts, fmt.Sprintf("%s LIKE %s", castColumnAsText(dbType, col), placeholder))
		args = append(args, likeValue)
	}
	return "(" + strings.Join(parts, " OR ") + ")", args, nil
}

// BuildNewRowDefaults 根据列结构为新增行生成默认值
// 自增字段返回 nil；时间型且默认值为 CURRENT_TIMESTAMP/NOW() 的返回当前时间；其余为 nil
func BuildNewRowDefaults(cols []ColumnInfo) map[string]interface{} {
	now := time.Now()
	row := make(map[string]interface{}, len(cols))
	for _, col := range cols {
		if col.IsAutoIncrement {
			row[col.Name] = nil
			continue
		}
		kind := resolveTemporalKind(col.Type)
		if kind != "" && (isCurrentTemporalDefault(col.DefaultValue) || (!col.Nullable && col.DefaultValue == nil)) {
			row[col.Name] = formatTemporalNow(kind, now)
		} else {
			row[col.Name] = nil
		}
	}
	return row
}

func resolveTemporalKind(colType string) string {
	t := strings.ToLower(strings.TrimSpace(colType))
	if strings.Contains(t, "datetime") || strings.Contains(t, "timestamp") {
		return "datetime"
	}
	if strings.Contains(t, "date") {
		return "date"
	}
	if strings.Contains(t, "time") {
		return "time"
	}
	return ""
}

func isCurrentTemporalDefault(defaultValue *string) bool {
	if defaultValue == nil {
		return false
	}
	v := strings.TrimSpace(*defaultValue)
	v = strings.Trim(v, "'\"")
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	matched, _ := regexp.MatchString(`(?i)^(current_(timestamp|date|time)(\(\d*\))?|local(timestamp|time)(\(\d*\))?|now\(\))$`, v)
	return matched
}

func formatTemporalNow(kind string, now time.Time) string {
	date := now.Format("2006-01-02")
	timeStr := now.Format("15:04:05")
	switch kind {
	case "date":
		return date
	case "time":
		return timeStr
	default:
		return date + " " + timeStr
	}
}
