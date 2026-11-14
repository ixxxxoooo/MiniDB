package database

import (
	"context"
	"minidb/internal/logger"
	"strings"
	"time"
)

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

// ExecuteQueryRaw 执行 SQL 查询（不自动追加 LIMIT，用于导出等场景）
func ExecuteQueryRaw(db Executor, sqlStr string) (*QueryResult, error) {
	return ExecuteQueryPagedContext(context.Background(), db, sqlStr, 0, 0, false)
}

// QueryRawWithArgs 执行带参数的原始查询并返回结果（供导出游标分页等场景使用）。
func QueryRawWithArgs(db Executor, sqlStr string, args ...interface{}) (*QueryResult, error) {
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
func ExecuteQueryPaged(db Executor, sqlStr string, page, pageSize int, autoLimit bool) (*QueryResult, error) {
	return ExecuteQueryPagedContext(context.Background(), db, sqlStr, page, pageSize, autoLimit)
}

// ExecuteQueryPagedContext 执行 SQL 查询，支持取消。
// page=0 表示不分页；autoLimit=true 且 SELECT 未写 LIMIT 时，先 COUNT(*) 再 LIMIT/OFFSET
func ExecuteQueryPagedContext(ctx context.Context, db Executor, sqlStr string, page, pageSize int, autoLimit bool) (*QueryResult, error) {
	return ExecuteQueryPagedFor(ctx, "", db, sqlStr, page, pageSize, autoLimit)
}

// ExecuteQueryPagedFor 与 ExecuteQueryPagedContext 相同，但自动分页按 dbType 的写法生成（空串为通用的 LIMIT/OFFSET）。
func ExecuteQueryPagedFor(ctx context.Context, dbType DBType, db Executor, sqlStr string, page, pageSize int, autoLimit bool) (*QueryResult, error) {
	syn := SyntaxOf(dbType)
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
				Duration: time.Since(start).Milliseconds(), DurationUs: time.Since(start).Microseconds(),
			}, nil
		}
		affected, _ := result.RowsAffected()
		return &QueryResult{
			Total:    affected,
			Duration: time.Since(start).Milliseconds(), DurationUs: time.Since(start).Microseconds(),
			Rows:    []map[string]interface{}{},
			Columns: []ColumnMeta{},
		}, nil
	}

	wasAutoLimited := false
	actualSQL := sqlStr
	var totalCount int64 = -1

	// 对 SELECT 且未写 LIMIT 的查询：先 COUNT 获取总数，再分页查询。
	// COUNT 失败（大表超时最常见）时不能退回全量查询：仍然只取一页，
	// 多取 1 行用来判断后面还有没有数据，总数标记为未知。
	probeExtra := false
	if autoLimit && IsSelectQuery(sqlStr) && !syn.HasRowLimit(sqlStr) {
		cleanSQL := strings.TrimRight(strings.TrimSpace(sqlStr), ";")

		countSQL := syn.CountOf(cleanSQL)
		err := db.QueryRowContext(ctx, countSQL).Scan(&totalCount)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			logger.Warn("自动分页 COUNT 失败，总数未知，仍按页限制行数: %v", err)
			totalCount = -1
		} else {
			logger.Info("自动分页: 总行数=%d", totalCount)
		}

		if page > 0 && pageSize > 0 {
			offset := (page - 1) * pageSize
			limit := pageSize
			if totalCount < 0 {
				limit++
				probeExtra = true
			}
			actualSQL = syn.AppendPage(cleanSQL, limit, offset)
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
			Duration: time.Since(start).Milliseconds(), DurationUs: time.Since(start).Microseconds(),
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
	switch {
	case totalCount >= 0:
		result.Total = totalCount
	case probeExtra:
		offset := int64((page - 1) * pageSize)
		result.TotalUnknown = true
		if len(result.Rows) > pageSize {
			result.Rows = result.Rows[:pageSize]
			result.Total = offset + int64(pageSize) + 1 // 至少还有 1 行：界面据此显示下一页
		} else {
			result.Total = offset + int64(len(result.Rows)) // 已经是最后一页，总数其实是准的
			result.TotalUnknown = false
		}
	}
	return result, nil
}
