package database

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"minidb/internal/model"
	"minidb/internal/sqltext"
)

// postgresDialect 是 PostgreSQL 方言。连接绑定单个库，访问其他库要单独建连接池；元数据按 public 模式读取。
type postgresDialect struct{}

func init() { register(postgresDialect{}) }

func (postgresDialect) Type() DBType { return Postgres }

func (postgresDialect) Label() string       { return "PostgreSQL" }
func (postgresDialect) DefaultPort() int    { return 5432 }
func (postgresDialect) DefaultUser() string { return "postgres" }

func (postgresDialect) SSLModes() []string {
	return []string{"disable", "require", "verify-ca", "verify-full"}
}

func (postgresDialect) Caps() Caps {
	return Caps{
		RowEdit: true, Monitor: true,
		PerDatabasePool: true, Network: true,
		Users: true, ProcessList: true, ReadOnlyTx: true,
		ForeignKeys: true, Triggers: true, Routines: true, AddDropForeignKey: true,
		CreateTable: true, AlterTable: true, TruncateCascade: true, RestartIdentity: true, DropCascade: true,
		RowsAffectedReliable: true, CreateDatabase: true, MultiDatabase: true, Explain: true, ExplainAnalyze: true,
		Dump: model.DumpPostgres, QueriesAreTransactions: true, CreateOrReplace: true, RoleUsers: true, DefaultSchema: "public",
		ManualTx: true, TxBegin: "BEGIN",
	}
}

func (postgresDialect) Driver() string { return "postgres" }

func (postgresDialect) DSN(cfg *ConnectionConfig) string {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   cfg.Database,
	}
	q := u.Query()
	q.Set("sslmode", sslMode)
	q.Set("connect_timeout", fmt.Sprintf("%d", int(defaultConnectTimeout/time.Second)))
	u.RawQuery = q.Encode()
	return u.String()
}

func (postgresDialect) QuoteIdent(name string) string {
	if name == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (postgresDialect) QuoteString(s string) string { return sqltext.QuoteString(s) }

func (postgresDialect) ColumnTypes() []string {
	return []string{
		"smallint", "integer", "bigint", "decimal", "numeric",
		"real", "double precision", "smallserial", "serial", "bigserial",
		"money",
		"character varying", "varchar", "character", "char", "text",
		"bytea",
		"date", "time", "time with time zone",
		"timestamp", "timestamp with time zone", "interval",
		"boolean",
		"enum",
		"bit", "bit varying",
		"cidr", "inet", "macaddr", "macaddr8",
		"box", "circle", "line", "lseg", "path", "point", "polygon",
		"json", "jsonb",
		"uuid",
		"xml",
		"tsquery", "tsvector",
		"int4range", "int8range", "numrange", "tsrange", "tstzrange", "daterange",
		"integer[]", "text[]", "boolean[]", "jsonb[]",
	}
}

func (postgresDialect) ServerVersion(ctx context.Context, db Executor) (string, error) {
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	return version, err
}

func (postgresDialect) Databases(ctx context.Context, db Executor, _ bool) ([]DatabaseInfo, error) {
	var databases []DatabaseInfo
	err := queryEach(ctx, db, "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname", nil,
		func(rows rowScanner) error {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			databases = append(databases, DatabaseInfo{Name: name})
			return nil
		})
	return databases, err
}

func (postgresDialect) Tables(ctx context.Context, db Executor, _ string) ([]TableInfo, error) {
	var tables []TableInfo
	err := queryEach(ctx, db, `SELECT tablename, 'table' as type FROM pg_tables
		WHERE schemaname = 'public'
		UNION ALL
		SELECT viewname, 'view' as type FROM pg_views
		WHERE schemaname = 'public'
		ORDER BY 1`, nil, func(rows rowScanner) error {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return err
		}
		tables = append(tables, t)
		return nil
	})
	return tables, err
}

func (d postgresDialect) Columns(ctx context.Context, db Executor, _, table string) ([]ColumnInfo, error) {
	var columns []ColumnInfo
	err := queryEach(ctx, db, `SELECT c.column_name, c.data_type, c.is_nullable, c.column_default,
		CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary,
		c.character_maximum_length
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			WHERE tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.column_name = pk.column_name
		WHERE c.table_name = $1 AND c.table_schema = 'public'
		ORDER BY c.ordinal_position`, []any{table}, func(rows rowScanner) error {
		var c ColumnInfo
		var nullable string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &c.IsPrimary, &c.MaxLength); err != nil {
			return err
		}
		c.Nullable = nullable == "YES"
		columns = append(columns, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	d.fillForeignKeys(ctx, db, table, columns)
	return columns, nil
}

// fillForeignKeys 读取当前 schema 下该表的外键并填到列上。读取失败时静默跳过。
func (postgresDialect) fillForeignKeys(ctx context.Context, db Executor, table string, columns []ColumnInfo) {
	refs := map[string]string{}
	_ = queryEach(ctx, db, `SELECT kcu.column_name, ccu.table_name, ccu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
		  ON ccu.constraint_name = tc.constraint_name AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_name = $1 AND tc.table_schema = current_schema()`,
		[]any{table}, func(rows rowScanner) error {
			var col, refTable, refCol string
			if rows.Scan(&col, &refTable, &refCol) == nil {
				refs[col] = refTable + "." + refCol
			}
			return nil
		})
	for i := range columns {
		if ref, ok := refs[columns[i].Name]; ok {
			columns[i].ForeignKey = ref
		}
	}
}

func (d postgresDialect) DDL(ctx context.Context, db Executor, dbName, table string) (string, error) {
	var def string
	if db.QueryRowContext(ctx, `SELECT definition FROM pg_views WHERE schemaname = current_schema() AND viewname = $1`, table).Scan(&def) == nil {
		return fmt.Sprintf("CREATE OR REPLACE VIEW %s AS\n%s", d.QuoteIdent(table), strings.TrimSpace(def)), nil
	}
	// 简化版：由列定义拼出 CREATE TABLE
	columns, err := d.Columns(ctx, db, dbName, table)
	if err != nil {
		return "", err
	}
	ddl := fmt.Sprintf("CREATE TABLE %s (\n", table)
	for i, c := range columns {
		nullable := ""
		if !c.Nullable {
			nullable = " NOT NULL"
		}
		def := ""
		if c.DefaultValue != nil {
			def = fmt.Sprintf(" DEFAULT %s", *c.DefaultValue)
		}
		ddl += fmt.Sprintf("  %s %s%s%s", c.Name, c.Type, nullable, def)
		if i < len(columns)-1 {
			ddl += ","
		}
		ddl += "\n"
	}
	return ddl + ");", nil
}

func (postgresDialect) TableStats(ctx context.Context, db Executor, _, table string) (*TableStats, error) {
	stats := &TableStats{Engine: "PostgreSQL"}
	// 取不到的项保持 0：统计只是展示用，不因权限等原因让整个页面失败
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(n_live_tup, 0) FROM pg_stat_user_tables WHERE relname = $1`, table).Scan(&stats.RowCount)
	_ = db.QueryRowContext(ctx, `SELECT pg_total_relation_size($1::regclass)`, table).Scan(&stats.TotalSize)
	_ = db.QueryRowContext(ctx, `SELECT pg_relation_size($1::regclass)`, table).Scan(&stats.DataSize)
	stats.IndexSize = stats.TotalSize - stats.DataSize
	return stats, nil
}

func (postgresDialect) TableOverview(ctx context.Context, db Executor, _ string) ([]TableOverview, error) {
	var out []TableOverview
	err := queryEach(ctx, db, `SELECT c.relname,
		CASE WHEN c.relkind IN ('v','m') THEN 'view' ELSE 'table' END,
		CASE WHEN c.relkind IN ('r','p','m') THEN GREATEST(c.reltuples, 0)::bigint ELSE -1 END,
		CASE WHEN c.relkind IN ('r','p','m') THEN pg_table_size(c.oid) ELSE -1 END,
		CASE WHEN c.relkind IN ('r','p','m') THEN pg_indexes_size(c.oid) ELSE -1 END,
		COALESCE(obj_description(c.oid, 'pg_class'), '')
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind IN ('r','p','v','m') ORDER BY c.relname`, nil, func(rows rowScanner) error {
		var t TableOverview
		if err := rows.Scan(&t.Name, &t.Type, &t.Rows, &t.DataSize, &t.IndexSize, &t.Comment); err != nil {
			return err
		}
		out = append(out, t)
		return nil
	})
	return out, err
}
