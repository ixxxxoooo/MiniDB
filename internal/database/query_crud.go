package database

import (
	"database/sql"
	"fmt"
	"minidb/internal/logger"
	"minidb/internal/model"
	"regexp"
	"strings"
	"time"
)

// placeholderFor 返回第 idx 个（从 1 开始）参数的占位符。
func placeholderFor(dbType DBType, idx int) string { return Placeholder(dbType, idx) }

// buildEqualityConditions 把列值映射展开成 "col = ?" 条件列表，参数序号从 startIdx 开始。
func buildEqualityConditions(dbType DBType, values map[string]interface{}, startIdx int) ([]string, []interface{}) {
	conditions := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values))
	idx := startIdx
	for col, val := range values {
		conditions = append(conditions, fmt.Sprintf("%s = %s", QuoteIdent(dbType, col), placeholderFor(dbType, idx)))
		args = append(args, val)
		idx++
	}
	return conditions, args
}

// rowsAffectedReliable 判断该方言返回的影响行数是否可用于校验。
// StarRocks 的 DML 影响行数不可靠，跳过校验。
func rowsAffectedReliable(dbType DBType) bool { return CapsOf(dbType).RowsAffectedReliable }

// rowExists 判断主键条件是否能匹配到行。
func rowExists(db Executor, dbType DBType, dbName, table string, primaryKey map[string]interface{}) (bool, error) {
	conditions, args := buildEqualityConditions(dbType, primaryKey, 1)
	sqlStr := AppendPage(dbType, fmt.Sprintf("SELECT 1 FROM %s WHERE %s",
		quoteTable(dbType, dbName, table), strings.Join(conditions, " AND ")), 1, -1)
	rows, err := db.Query(sqlStr, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	exists := rows.Next()
	return exists, rows.Err()
}

// DeleteRow 根据主键删除一行。未匹配到任何行时返回错误，避免"提示成功但什么都没删"。
func DeleteRow(db Executor, dbType DBType, dbName, table string, primaryKey map[string]interface{}) error {
	if len(primaryKey) == 0 {
		return model.Errf(model.CodePKRequired, "主键不能为空")
	}

	conditions, args := buildEqualityConditions(dbType, primaryKey, 1)

	// MySQL、MariaDB 和 TiDB 支持 DELETE ... LIMIT 1，StarRocks 不支持
	limitClause := ""
	if dbType.MySQLCompatible() && dbType != StarRocks {
		limitClause = " LIMIT 1"
	}
	sqlStr := fmt.Sprintf("DELETE FROM %s WHERE %s%s",
		quoteTable(dbType, dbName, table),
		strings.Join(conditions, " AND "),
		limitClause)

	logger.Info("删除行: %s", sqlStr)
	res, err := db.Exec(sqlStr, args...)
	if err != nil {
		return err
	}
	if !rowsAffectedReliable(dbType) {
		return nil
	}
	if n, raErr := res.RowsAffected(); raErr == nil && n == 0 {
		return fmt.Errorf("未匹配到要删除的行（数据可能已被修改，或主键值不匹配）")
	}
	return nil
}

// UpdateRow 根据主键更新一行。未匹配到任何行、或匹配到多行时返回错误。
func UpdateRow(db Executor, dbType DBType, dbName, table string, primaryKey map[string]interface{}, changes map[string]interface{}) error {
	if len(primaryKey) == 0 {
		return model.Errf(model.CodePKRequired, "主键不能为空")
	}
	if len(changes) == 0 {
		return fmt.Errorf("没有要更新的字段")
	}

	setClauses, args := buildEqualityConditions(dbType, changes, 1)
	whereClauses, whereArgs := buildEqualityConditions(dbType, primaryKey, len(args)+1)
	args = append(args, whereArgs...)

	sqlStr := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		quoteTable(dbType, dbName, table),
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "))

	logger.Info("更新行: %s", sqlStr)
	res, err := db.Exec(sqlStr, args...)
	if err != nil {
		return err
	}
	if !rowsAffectedReliable(dbType) {
		return nil
	}
	n, raErr := res.RowsAffected()
	if raErr != nil {
		return nil
	}
	if n > 1 {
		return fmt.Errorf("主键条件匹配到 %d 行，已拒绝更新", n)
	}
	if n == 0 {
		// MySQL 协议下"新值与旧值相同"也会返回 0，需要再确认行是否真的不存在。
		exists, existErr := rowExists(db, dbType, dbName, table, primaryKey)
		if existErr != nil {
			return fmt.Errorf("校验更新结果失败: %w", existErr)
		}
		if !exists {
			return fmt.Errorf("未匹配到要更新的行（数据可能已被修改，或主键值不匹配）")
		}
	}
	return nil
}

// UpdateRowTx 在事务内更新行
func UpdateRowTx(tx *sql.Tx, dbType DBType, dbName, table string, primaryKey map[string]interface{}, changes map[string]interface{}) error {
	return UpdateRow(tx, dbType, dbName, table, primaryKey, changes)
}

// InsertRow 插入新行
func InsertRow(db Executor, dbType DBType, dbName, table string, row map[string]interface{}) error {
	if len(row) == 0 {
		return fmt.Errorf("没有要插入的数据")
	}

	cols := make([]string, 0, len(row))
	placeholders := make([]string, 0, len(row))
	args := make([]interface{}, 0, len(row))
	idx := 1
	for col, val := range row {
		cols = append(cols, QuoteIdent(dbType, col))
		placeholders = append(placeholders, placeholderFor(dbType, idx))
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
func DeleteRowTx(tx *sql.Tx, dbType DBType, dbName, table string, primaryKey map[string]interface{}) error {
	return DeleteRow(tx, dbType, dbName, table, primaryKey)
}

// InsertRowTx 在事务内插入行
func InsertRowTx(tx *sql.Tx, dbType DBType, dbName, table string, row map[string]interface{}) error {
	return InsertRow(tx, dbType, dbName, table, row)
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
