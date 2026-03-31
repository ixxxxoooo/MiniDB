package database

import (
	"database/sql"
	"fmt"
	"strings"
	"tableplus-ai/internal/logger"
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
	Columns  []ColumnMeta             `json:"columns"`
	Rows     []map[string]interface{} `json:"rows"`
	Total    int64                    `json:"total"`
	Duration int64                    `json:"duration"`
	Error    string                   `json:"error,omitempty"`
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

// ExecuteQuery 执行 SQL 查询
func ExecuteQuery(db *sql.DB, sqlStr string) (*QueryResult, error) {
	start := time.Now()
	logger.Debug("执行 SQL: %s", strings.TrimSpace(sqlStr))

	// 判断是否为非查询语句
	trimmed := strings.TrimSpace(strings.ToUpper(sqlStr))
	if strings.HasPrefix(trimmed, "INSERT") ||
		strings.HasPrefix(trimmed, "UPDATE") ||
		strings.HasPrefix(trimmed, "DELETE") ||
		strings.HasPrefix(trimmed, "CREATE") ||
		strings.HasPrefix(trimmed, "ALTER") ||
		strings.HasPrefix(trimmed, "DROP") {
		result, err := db.Exec(sqlStr)
		if err != nil {
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

	rows, err := db.Query(sqlStr)
	if err != nil {
		return &QueryResult{
			Error:    err.Error(),
			Duration: time.Since(start).Milliseconds(),
		}, nil
	}
	defer rows.Close()

	return scanRows(rows, start)
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

	// 构建 WHERE 子句
	where := buildWhereClause(filters)

	// 构建 ORDER BY 子句
	orderBy := buildOrderByClause(sorts)

	// 获取总行数
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", quoteTable(dbType, dbName, table), where)
	var total int64
	db.QueryRow(countSQL).Scan(&total)

	// 构建查询
	offset := (page - 1) * pageSize
	var querySQL string
	switch dbType {
	case "mysql":
		querySQL = fmt.Sprintf("SELECT * FROM %s %s %s LIMIT %d OFFSET %d",
			quoteTable(dbType, dbName, table), where, orderBy, pageSize, offset)
	case "postgres":
		querySQL = fmt.Sprintf("SELECT * FROM %s %s %s LIMIT %d OFFSET %d",
			quoteTable(dbType, dbName, table), where, orderBy, pageSize, offset)
	case "sqlite":
		querySQL = fmt.Sprintf("SELECT * FROM %s %s %s LIMIT %d OFFSET %d",
			quoteTable(dbType, dbName, table), where, orderBy, pageSize, offset)
	}

	rows, err := db.Query(querySQL)
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
			conditions = append(conditions, fmt.Sprintf("%s = $%d", col, i))
		default:
			conditions = append(conditions, fmt.Sprintf("`%s` = ?", col))
		}
		args = append(args, val)
		i++
	}

	limitClause := ""
	if dbType == "mysql" {
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
			setClauses = append(setClauses, fmt.Sprintf("\"%s\" = $%d", col, idx))
		default:
			setClauses = append(setClauses, fmt.Sprintf("`%s` = ?", col))
		}
		args = append(args, val)
		idx++
	}

	for col, val := range primaryKey {
		switch dbType {
		case "postgres":
			whereClauses = append(whereClauses, fmt.Sprintf("\"%s\" = $%d", col, idx))
		default:
			whereClauses = append(whereClauses, fmt.Sprintf("`%s` = ?", col))
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
			setClauses = append(setClauses, fmt.Sprintf("\"%s\" = $%d", col, idx))
		default:
			setClauses = append(setClauses, fmt.Sprintf("`%s` = ?", col))
		}
		args = append(args, val)
		idx++
	}

	for col, val := range primaryKey {
		switch dbType {
		case "postgres":
			whereClauses = append(whereClauses, fmt.Sprintf("\"%s\" = $%d", col, idx))
		default:
			whereClauses = append(whereClauses, fmt.Sprintf("`%s` = ?", col))
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
			cols = append(cols, fmt.Sprintf("\"%s\"", col))
			placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		default:
			cols = append(cols, fmt.Sprintf("`%s`", col))
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

func buildWhereClause(filters []Filter) string {
	if len(filters) == 0 {
		return ""
	}
	var conditions []string
	for _, f := range filters {
		switch f.Operator {
		case "IS NULL":
			conditions = append(conditions, fmt.Sprintf("%s IS NULL", f.Column))
		case "IS NOT NULL":
			conditions = append(conditions, fmt.Sprintf("%s IS NOT NULL", f.Column))
		default:
			conditions = append(conditions, fmt.Sprintf("%s %s '%s'", f.Column, f.Operator, f.Value))
		}
	}
	return "WHERE " + strings.Join(conditions, " AND ")
}

func buildOrderByClause(sorts []Sort) string {
	if len(sorts) == 0 {
		return ""
	}
	var parts []string
	for _, s := range sorts {
		parts = append(parts, fmt.Sprintf("%s %s", s.Column, s.Direction))
	}
	return "ORDER BY " + strings.Join(parts, ", ")
}

func quoteTable(dbType, dbName, table string) string {
	switch dbType {
	case "mysql":
		if dbName != "" {
			return fmt.Sprintf("`%s`.`%s`", dbName, table)
		}
		return fmt.Sprintf("`%s`", table)
	case "postgres":
		return fmt.Sprintf("\"%s\"", table)
	default:
		return table
	}
}

// QuoteTableName 对表名进行引用，不含库名前缀（供外部调用）
func QuoteTableName(dbType, table string) string {
	switch dbType {
	case "mysql":
		return fmt.Sprintf("`%s`", table)
	case "postgres":
		return fmt.Sprintf("\"%s\"", table)
	default:
		return table
	}
}
