package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	errNoDefinition    = errors.New("没有返回定义")
	errEmptyDefinition = errors.New("定义为空（可能没有权限）")
)

// rowScanner 是 *sql.Rows 在迭代回调里暴露的最小能力。
type rowScanner interface{ Scan(dest ...any) error }

// queryEach 执行查询并对每一行调用 fn；负责关闭游标并返回迭代过程中的错误。
// 元数据读取都走它，避免每处各写一遍 Close / rows.Err()。
func queryEach(ctx context.Context, db Executor, query string, args []any, fn func(rows rowScanner) error) error {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// tableTypeOf 把 information_schema 的 table_type 归一成 table / view。
func tableTypeOf(infoSchemaType string) string {
	if strings.EqualFold(infoSchemaType, "VIEW") {
		return "view"
	}
	return "table"
}

// nullOr 取 NullInt64 的值，无效时返回 def。
func nullOr(n sql.NullInt64, def int64) int64 {
	if n.Valid {
		return n.Int64
	}
	return def
}

// showCreate 执行 SHOW CREATE …，取第 col 列（从 0 起）的定义；不同对象返回的列数不同。
func showCreate(ctx context.Context, db Executor, stmt string, col int) (string, error) {
	rows, err := db.QueryContext(ctx, stmt)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	if !rows.Next() {
		return "", errNoDefinition
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return "", err
	}
	if col >= len(vals) {
		col = len(vals) - 1
	}
	switch v := vals[col].(type) {
	case []byte:
		return string(v), nil
	case string:
		return v, nil
	case nil:
		return "", errEmptyDefinition
	default:
		return fmt.Sprint(v), nil
	}
}
