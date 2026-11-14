package database

import (
	"context"
	"fmt"
	"time"
)

// QueryTableData 分页查询表数据
func QueryTableData(ctx context.Context, db Executor, dbType DBType, dbName, table string, page, pageSize int, filters []Filter, sorts []Sort) (*QueryResult, error) {
	return queryTableData(ctx, db, dbType, dbName, table, page, pageSize, filters, sorts, true)
}

// QueryTableRows 与 QueryTableData 相同但不统计总行数（Total 为 -1）：
// 大表上 COUNT(*) 可能很慢，界面先显示数据，再用 CountTableData 补上总数。
func QueryTableRows(ctx context.Context, db Executor, dbType DBType, dbName, table string, page, pageSize int, filters []Filter, sorts []Sort) (*QueryResult, error) {
	return queryTableData(ctx, db, dbType, dbName, table, page, pageSize, filters, sorts, false)
}

// CountTableData 统计满足筛选条件的总行数。
func CountTableData(ctx context.Context, db Executor, dbType DBType, dbName, table string, filters []Filter) (int64, error) {
	var tableColumns []string
	var err error
	if hasAnyColumnFilter(filters) {
		if tableColumns, err = getTableColumnNames(ctx, db, dbType, dbName, table); err != nil {
			return 0, err
		}
	}
	where, args, err := buildWhereClause(dbType, filters, tableColumns)
	if err != nil {
		return 0, err
	}
	var total int64
	err = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s %s", quoteTable(dbType, dbName, table), where), args...).Scan(&total)
	return total, err
}

func queryTableData(ctx context.Context, db Executor, dbType DBType, dbName, table string, page, pageSize int, filters []Filter, sorts []Sort, withCount bool) (*QueryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
		tableColumns, err = getTableColumnNames(ctx, db, dbType, dbName, table)
		if err != nil {
			return &QueryResult{Error: err.Error(), Duration: time.Since(start).Milliseconds(), DurationUs: time.Since(start).Microseconds()}, nil
		}
	}
	where, args, err := buildWhereClause(dbType, filters, tableColumns)
	if err != nil {
		return &QueryResult{Error: err.Error(), Duration: time.Since(start).Milliseconds(), DurationUs: time.Since(start).Microseconds()}, nil
	}

	// 构建 ORDER BY 子句
	orderBy, err := buildOrderByClause(dbType, sorts)
	if err != nil {
		return &QueryResult{Error: err.Error(), Duration: time.Since(start).Milliseconds(), DurationUs: time.Since(start).Microseconds()}, nil
	}

	// 获取总行数
	total := int64(-1)
	if withCount {
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", quoteTable(dbType, dbName, table), where)
		if err := db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return &QueryResult{
				Error:    err.Error(),
				Duration: time.Since(start).Milliseconds(), DurationUs: time.Since(start).Microseconds(),
			}, nil
		}
	}

	// 构建查询（所有支持的数据库类型均使用 LIMIT/OFFSET 分页语法）
	offset := (page - 1) * pageSize
	querySQL := fmt.Sprintf("SELECT * FROM %s %s %s LIMIT %d OFFSET %d",
		quoteTable(dbType, dbName, table), where, orderBy, pageSize, offset)

	rows, err := db.QueryContext(ctx, querySQL, args...)
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
		return nil, err
	}
	result.Total = total
	return result, nil
}

// getTableColumnNames 探测表的列名列表
func getTableColumnNames(ctx context.Context, db Executor, dbType DBType, dbName, table string) ([]string, error) {
	probeSQL := fmt.Sprintf("SELECT * FROM %s LIMIT 0", quoteTable(dbType, dbName, table))
	rows, err := db.QueryContext(ctx, probeSQL)
	if err != nil {
		return nil, fmt.Errorf("获取表列信息失败: %w", err)
	}
	defer rows.Close()
	return rows.Columns()
}
