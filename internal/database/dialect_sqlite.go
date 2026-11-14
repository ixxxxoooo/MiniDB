package database

import (
	"context"
	"database/sql"
	"fmt"
	"minidb/internal/model"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"minidb/internal/sqltext"
)

// sqliteDialect 是 SQLite 方言：库就是一个文件，只有 main；没有存储过程；不能给已有的表加外键。
type sqliteDialect struct{}

func init() { register(sqliteDialect{}) }

func (sqliteDialect) Type() DBType { return SQLite }

func (sqliteDialect) Label() string       { return "SQLite" }
func (sqliteDialect) DefaultPort() int    { return 0 }
func (sqliteDialect) DefaultUser() string { return "" }
func (sqliteDialect) SSLModes() []string  { return nil }

func (sqliteDialect) Caps() Caps {
	return Caps{
		ForeignKeys: true, Triggers: true, CreateTable: true, RowsAffectedReliable: true,
	}
}

func (sqliteDialect) Driver() string { return "sqlite" }

// DSN 给 path 加 5 秒 busy_timeout。旧的 cgo 驱动默认就有；modernc.org/sqlite 完全不等待，
// 并发的池内连接（读还没结束就来了 DDL）会以 SQLITE_BUSY 失败。
func (sqliteDialect) DSN(cfg *ConnectionConfig) string {
	path := cfg.Database
	if strings.HasPrefix(path, "file:") || path == ":memory:" {
		return path
	}
	return "file:" + (&url.URL{Path: path}).EscapedPath() + "?_pragma=busy_timeout(5000)"
}

func (sqliteDialect) QuoteIdent(name string) string {
	if name == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (sqliteDialect) QuoteString(s string) string { return sqltext.QuoteString(s) }

func (sqliteDialect) ColumnTypes() []string {
	return []string{
		"INTEGER", "REAL", "TEXT", "BLOB", "NUMERIC",
		"INT", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT",
		"UNSIGNED BIG INT", "INT2", "INT8",
		"CHARACTER(20)", "VARCHAR(255)", "VARYING CHARACTER(255)",
		"NCHAR(55)", "NATIVE CHARACTER(70)", "NVARCHAR(100)", "CLOB",
		"DOUBLE", "DOUBLE PRECISION", "FLOAT",
		"DECIMAL(10,5)", "BOOLEAN", "DATE", "DATETIME",
	}
}

// reSQLitePragmaIdentifier 限制 PRAGMA 标识符只允许 SQLite 安全字符集，
// 避免 PRAGMA table_info(<name>) 等内联拼接时被注入（PRAGMA 不支持引号标识符，也不支持占位符）。
var reSQLitePragmaIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

// safeSQLitePragmaName 校验 PRAGMA 标识符；不合法时返回错误，调用方应跳过该查询。
func safeSQLitePragmaName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("SQLite 标识符为空")
	}
	if !reSQLitePragmaIdentifier.MatchString(name) {
		return "", fmt.Errorf("SQLite 标识符包含不支持的字符: %s", name)
	}
	return name, nil
}

func (sqliteDialect) ServerVersion(ctx context.Context, db Executor) (string, error) {
	var version string
	err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
	if err == nil {
		version = "SQLite " + version
	}
	return version, err
}

func (sqliteDialect) Databases(context.Context, Executor, bool) ([]DatabaseInfo, error) {
	return []DatabaseInfo{{Name: "main"}}, nil
}

func (sqliteDialect) Tables(ctx context.Context, db Executor, _ string) ([]TableInfo, error) {
	var tables []TableInfo
	err := queryEach(ctx, db, `SELECT name, type FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
		ORDER BY name`, nil, func(rows rowScanner) error {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return err
		}
		tables = append(tables, t)
		return nil
	})
	return tables, err
}

func (d sqliteDialect) Columns(ctx context.Context, db Executor, _, table string) ([]ColumnInfo, error) {
	safeName, err := safeSQLitePragmaName(table)
	if err != nil {
		return nil, err
	}
	var columns []ColumnInfo
	err = queryEach(ctx, db, fmt.Sprintf("PRAGMA table_info(%s)", safeName), nil, func(rows rowScanner) error {
		var cid, notNull, pk int
		var c ColumnInfo
		if err := rows.Scan(&cid, &c.Name, &c.Type, &notNull, &c.DefaultValue, &pk); err != nil {
			return err
		}
		c.Nullable = notNull == 0
		c.IsPrimary = pk == 1
		columns = append(columns, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 外键引用填到列上；读取失败时静默跳过
	refs := map[string]string{}
	_ = queryEach(ctx, db, fmt.Sprintf("PRAGMA foreign_key_list(%s)", safeName), nil, func(rows rowScanner) error {
		var id, seq int
		var refTable, from string
		var to, onUpdate, onDelete, match *string
		if rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match) != nil {
			return nil
		}
		ref := refTable
		if to != nil && *to != "" {
			ref += "." + *to
		}
		refs[from] = ref
		return nil
	})
	for i := range columns {
		if ref, ok := refs[columns[i].Name]; ok {
			columns[i].ForeignKey = ref
		}
	}
	return columns, nil
}

// Indexes 读取 SQLite 表的索引。先读完索引清单并关闭游标，再逐个查询索引包含的列：
// 在游标还开着时嵌套发起查询，会各占一条连接，多张表并发读取时连接池被占满，嵌套查询永远拿不到连接，
// 读锁迟迟不放，后面的写入就会撞上 SQLITE_BUSY。
func (sqliteDialect) Indexes(ctx context.Context, db Executor, _, table string) ([]IndexInfo, error) {
	safeName, err := safeSQLitePragmaName(table)
	if err != nil {
		return nil, err
	}
	var indexes []IndexInfo
	err = queryEach(ctx, db, fmt.Sprintf("PRAGMA index_list(%s)", safeName), nil, func(rows rowScanner) error {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return err
		}
		indexes = append(indexes, IndexInfo{Name: name, IsUnique: unique == 1, IsPrimary: origin == "pk", Type: "BTREE"})
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i := range indexes {
		// 索引名来自数据库结果，仍需校验后再拼入 PRAGMA
		safeIndexName, nameErr := safeSQLitePragmaName(indexes[i].Name)
		if nameErr != nil {
			continue
		}
		_ = queryEach(ctx, db, fmt.Sprintf("PRAGMA index_info(%s)", safeIndexName), nil, func(rows rowScanner) error {
			var seqno, cid int
			var colName string
			if err := rows.Scan(&seqno, &cid, &colName); err == nil {
				indexes[i].Columns = append(indexes[i].Columns, colName)
			}
			return nil
		})
	}
	return indexes, nil
}

func (sqliteDialect) DDL(ctx context.Context, db Executor, _, table string) (string, error) {
	var ddl string
	err := db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE name = ?", table).Scan(&ddl)
	return ddl, err
}

func (d sqliteDialect) TableStats(ctx context.Context, db Executor, _, table string) (*TableStats, error) {
	stats := &TableStats{Engine: "SQLite"}
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+d.QuoteIdent(table)).Scan(&stats.RowCount)
	return stats, nil
}

func (d sqliteDialect) TableOverview(ctx context.Context, db Executor, _ string) ([]TableOverview, error) {
	var out []TableOverview
	err := queryEach(ctx, db, `SELECT name, type FROM sqlite_master
		WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' ORDER BY name`, nil, func(rows rowScanner) error {
		t := TableOverview{Rows: -1, DataSize: -1, IndexSize: -1}
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return err
		}
		out = append(out, t)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 游标已关闭，再逐表补行数与大小（原因同 Indexes）
	for i := range out {
		if out[i].Type != "table" {
			continue
		}
		var n int64
		if db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+d.QuoteIdent(out[i].Name)).Scan(&n) == nil {
			out[i].Rows = n
		}
		// dbstat 不是每个构建都有；取不到就不显示大小
		var size sql.NullInt64
		if db.QueryRowContext(ctx, "SELECT SUM(pgsize) FROM dbstat WHERE name = ?", out[i].Name).Scan(&size) == nil && size.Valid {
			out[i].DataSize = size.Int64
		}
	}
	return out, nil
}

func (sqliteDialect) ForeignKeys(ctx context.Context, db Executor, _, table string) ([]ForeignKeyInfo, error) {
	safe, err := safeSQLitePragmaName(table)
	if err != nil {
		return nil, err
	}
	byID := map[int]*ForeignKeyInfo{}
	var ids []int
	err = queryEach(ctx, db, fmt.Sprintf("PRAGMA foreign_key_list(%s)", safe), nil, func(rows rowScanner) error {
		var id, seq int
		var refTable, from string
		var to, upd, del, match *string
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &upd, &del, &match); err != nil {
			return err
		}
		fk, ok := byID[id]
		if !ok {
			fk = &ForeignKeyInfo{RefTable: refTable, OnUpdate: deref(upd, "NO ACTION"), OnDelete: deref(del, "NO ACTION")}
			byID[id] = fk
			ids = append(ids, id)
		}
		fk.Columns = append(fk.Columns, from)
		if to != nil {
			fk.RefColumns = append(fk.RefColumns, *to)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Ints(ids)
	out := make([]ForeignKeyInfo, 0, len(ids))
	for _, id := range ids {
		fk := byID[id]
		fk.Name = "fk_" + table + "_" + strings.Join(fk.Columns, "_") // SQLite 的外键没有名字
		out = append(out, *fk)
	}
	return out, nil
}

func deref(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

func (sqliteDialect) Triggers(ctx context.Context, db Executor, _, table string) ([]TriggerInfo, error) {
	var out []TriggerInfo
	err := queryEach(ctx, db, `SELECT name, sql FROM sqlite_master WHERE type = 'trigger' AND tbl_name = ? ORDER BY name`,
		[]any{table}, func(rows rowScanner) error {
			var t TriggerInfo
			if err := rows.Scan(&t.Name, &t.Definition); err != nil {
				return err
			}
			t.Timing, t.Event = parseTriggerHead(t.Definition)
			out = append(out, t)
			return nil
		})
	return out, err
}

func (sqliteDialect) Routines(context.Context, Executor, string) ([]RoutineInfo, error) {
	return nil, nil
}

func (sqliteDialect) RoutineDDL(context.Context, Executor, string, string, string) (string, error) {
	return "", model.Errf(model.CodeUnsupportedRoutines, "%s 不支持存储过程与函数", SQLite)
}
