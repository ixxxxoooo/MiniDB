package database

import (
	"context"
	"fmt"
	"strings"
)

// ClickHouse 的元数据读取：全部来自 system.* 表，限定在当前连接的库（currentDatabase()）。
// 为了不依赖驱动对 Nullable、无符号整数的扫描细节，查询里统一用 toString / toInt64 / ifNull 转成基础类型。

// chViewEngines 是视图类的引擎，界面上按视图对待（不能编辑、不能清空）。
const chViewEngines = `'View', 'MaterializedView', 'LiveView', 'WindowView'`

func (clickhouseDialect) Databases(ctx context.Context, db Executor, withCounts bool) ([]DatabaseInfo, error) {
	var out []DatabaseInfo
	err := queryEach(ctx, db, `SELECT name FROM system.databases
		WHERE name NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA') ORDER BY name`, nil,
		func(rows rowScanner) error {
			var d DatabaseInfo
			if err := rows.Scan(&d.Name); err != nil {
				return err
			}
			out = append(out, d)
			return nil
		})
	if err != nil || !withCounts {
		return out, err
	}
	counts := map[string]int{}
	// 数量只是附带信息，取不到不影响库列表
	_ = queryEach(ctx, db, `SELECT database, toInt64(count()) FROM system.tables WHERE is_temporary = 0 GROUP BY database`, nil,
		func(rows rowScanner) error {
			var name string
			var n int64
			if err := rows.Scan(&name, &n); err != nil {
				return err
			}
			counts[name] = int(n)
			return nil
		})
	for i := range out {
		out[i].TableCount = counts[out[i].Name]
	}
	return out, nil
}

func (clickhouseDialect) Tables(ctx context.Context, db Executor, _ string) ([]TableInfo, error) {
	var out []TableInfo
	err := queryEach(ctx, db, `SELECT name, engine IN (`+chViewEngines+`),
		toInt64(ifNull(total_rows, 0)), toInt64(ifNull(total_bytes, 0)), comment
		FROM system.tables WHERE database = currentDatabase() AND is_temporary = 0 ORDER BY name`, nil,
		func(rows rowScanner) error {
			var t TableInfo
			var view uint8
			if err := rows.Scan(&t.Name, &view, &t.RowCount, &t.Size, &t.Comment); err != nil {
				return err
			}
			t.Type = "table"
			if view != 0 {
				t.Type = "view"
			}
			out = append(out, t)
			return nil
		})
	return out, err
}

func (clickhouseDialect) Columns(ctx context.Context, db Executor, _, table string) ([]ColumnInfo, error) {
	var out []ColumnInfo
	err := queryEach(ctx, db, `SELECT name, type, default_kind, default_expression, comment, toUInt8(is_in_primary_key)
		FROM system.columns WHERE database = currentDatabase() AND table = ? ORDER BY position`, []any{table},
		func(rows rowScanner) error {
			var c ColumnInfo
			var kind, expr string
			var primary uint8
			if err := rows.Scan(&c.Name, &c.Type, &kind, &expr, &c.Comment, &primary); err != nil {
				return err
			}
			c.Nullable = strings.HasPrefix(c.Type, "Nullable(") || strings.HasPrefix(c.Type, "LowCardinality(Nullable(")
			c.IsPrimary = primary != 0
			switch {
			case kind == "DEFAULT" && expr != "":
				d := expr
				c.DefaultValue = &d
			case kind != "":
				// MATERIALIZED / ALIAS / EPHEMERAL：不是普通默认值，放到「额外」里
				c.Extra = strings.TrimSpace(strings.ToLower(kind) + " " + expr)
			}
			out = append(out, c)
			return nil
		})
	return out, err
}

// Indexes 返回主键、排序键和跳数索引。ClickHouse 的主键只用于稀疏索引，不保证唯一，所以不标 IsUnique。
func (clickhouseDialect) Indexes(ctx context.Context, db Executor, _, table string) ([]IndexInfo, error) {
	var primary, sorting string
	if err := db.QueryRowContext(ctx, `SELECT primary_key, sorting_key FROM system.tables
		WHERE database = currentDatabase() AND name = ?`, table).Scan(&primary, &sorting); err != nil {
		return nil, err
	}
	var out []IndexInfo
	if cols := splitKeyExpr(primary); len(cols) > 0 {
		out = append(out, IndexInfo{Name: "PRIMARY KEY", Columns: cols, IsPrimary: true, Type: "primary key"})
	}
	if cols := splitKeyExpr(sorting); len(cols) > 0 && sorting != primary {
		out = append(out, IndexInfo{Name: "ORDER BY", Columns: cols, Type: "sorting key"})
	}
	// 跳数索引只是附带信息，取不到（旧版本没有这张表）不影响其余部分
	_ = queryEach(ctx, db, `SELECT name, type, expr, toInt64(granularity) FROM system.data_skipping_indices
		WHERE database = currentDatabase() AND table = ? ORDER BY name`, []any{table},
		func(rows rowScanner) error {
			var name, typ, expr string
			var gran int64
			if err := rows.Scan(&name, &typ, &expr, &gran); err != nil {
				return err
			}
			out = append(out, IndexInfo{Name: name, Columns: []string{expr}, Type: typ, Comment: fmt.Sprintf("GRANULARITY %d", gran)})
			return nil
		})
	return out, nil
}

// splitKeyExpr 把 "a, b, toDate(c)" 这样的键表达式按顶层逗号拆开。
func splitKeyExpr(s string) []string {
	var out []string
	depth, start := 0, 0
	flush := func(end int) {
		if p := strings.TrimSpace(s[start:end]); p != "" {
			out = append(out, p)
		}
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				flush(i)
				start = i + 1
			}
		}
	}
	flush(len(s))
	return out
}

func (clickhouseDialect) DDL(ctx context.Context, db Executor, _, table string) (string, error) {
	var ddl string
	err := db.QueryRowContext(ctx, `SELECT create_table_query FROM system.tables
		WHERE database = currentDatabase() AND name = ?`, table).Scan(&ddl)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(ddl), nil
}

func (clickhouseDialect) TableStats(ctx context.Context, db Executor, _, table string) (*TableStats, error) {
	stats := &TableStats{}
	// 取不到的项保持零值：统计只是展示用
	_ = db.QueryRowContext(ctx, `SELECT engine, toInt64(ifNull(total_rows, 0)), toInt64(ifNull(total_bytes, 0)),
			toString(metadata_modification_time)
		FROM system.tables WHERE database = currentDatabase() AND name = ?`, table).
		Scan(&stats.Engine, &stats.RowCount, &stats.DataSize, &stats.UpdateTime)
	_ = db.QueryRowContext(ctx, `SELECT toInt64(sum(primary_key_bytes_in_memory + secondary_indices_compressed_bytes))
		FROM system.parts WHERE database = currentDatabase() AND table = ? AND active`, table).Scan(&stats.IndexSize)
	stats.TotalSize = stats.DataSize + stats.IndexSize
	return stats, nil
}

func (clickhouseDialect) TableOverview(ctx context.Context, db Executor, _ string) ([]TableOverview, error) {
	var out []TableOverview
	err := queryEach(ctx, db, `SELECT t.name, t.engine IN (`+chViewEngines+`), t.engine,
		toInt64(ifNull(t.total_rows, -1)), toInt64(ifNull(t.total_bytes, -1)), t.comment,
		toString(t.metadata_modification_time)
		FROM system.tables t WHERE t.database = currentDatabase() AND t.is_temporary = 0 ORDER BY t.name`, nil,
		func(rows rowScanner) error {
			var o TableOverview
			var view uint8
			if err := rows.Scan(&o.Name, &view, &o.Engine, &o.Rows, &o.DataSize, &o.Comment, &o.UpdatedAt); err != nil {
				return err
			}
			o.Type, o.IndexSize = "table", -1
			if view != 0 {
				o.Type = "view"
			}
			out = append(out, o)
			return nil
		})
	return out, err
}




