package database

import (
	"context"
	"strings"

	"minidb/internal/model"
	"minidb/internal/sqltext"
)

// mysqlDialect 是 MySQL 系的基类。TiDB、StarRocks 内嵌它，只覆盖不同的方法。
// 注意 Go 的内嵌没有虚方法：基类里的方法调用 d.Xxx() 时不会跳到子类的覆盖版本，
// 因此需要随类型变化的部分都通过参数传入（见 databases 的 isSystem、tolerantCounts）。
type mysqlDialect struct{ kind DBType }

func init() {
	register(mysqlDialect{kind: MySQL})
	register(mariadbDialect{mysqlDialect{kind: MariaDB}})
	register(tidbDialect{mysqlDialect{kind: TiDB}})
	register(starrocksDialect{mysqlDialect{kind: StarRocks}})
}

func (d mysqlDialect) Type() DBType { return d.kind }

func (mysqlDialect) Label() string       { return "MySQL" }
func (mysqlDialect) DefaultPort() int    { return 3306 }
func (mysqlDialect) DefaultUser() string { return "root" }

func (mysqlDialect) SSLModes() []string {
	return []string{SSLDisable, SSLPreferred, SSLRequired, SSLVerifyCA, SSLVerifyFull}
}

func (mysqlDialect) Caps() Caps {
	return Caps{
		RowEdit: true, Monitor: true,
		SessionUseDB: true, Network: true,
		Users: true, ProcessList: true, ReadOnlyTx: true,
		ForeignKeys: true, Triggers: true, Routines: true, AddDropForeignKey: true,
		CreateTable: true, AlterTable: true, ColumnReorder: true, RowsAffectedReliable: true, CreateDatabase: true, Charset: true,
		MultiDatabase: true, UserResourceLimits: true, Explain: true, ExplainAnalyze: true,
		BackslashEscape: true, DeleteLimit: true, Dump: model.DumpMySQL,
		ManualTx: true, TxBegin: "START TRANSACTION", ImplicitCommitDDL: true,
	}
}

func (mysqlDialect) QuoteIdent(name string) string {
	if name == "" {
		return ""
	}
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func (mysqlDialect) Driver() string { return "mysql" }

// DSN StarRocks 不支持 COM_STMT_PREPARE，需要 interpolateParams=true 让驱动在客户端插值参数。
func (d mysqlDialect) DSN(cfg *ConnectionConfig) string { return mysqlDSN(cfg, d.kind == StarRocks) }

func (mysqlDialect) QuoteString(s string) string { return sqltext.QuoteStringBackslash(s) }

func (mysqlDialect) ColumnTypes() []string {
	return []string{
		"tinyint", "smallint", "mediumint", "int", "bigint",
		"decimal", "numeric", "float", "double", "bit", "boolean",
		"date", "datetime", "timestamp", "time", "year",
		"char", "varchar", "tinytext", "text", "mediumtext", "longtext",
		"binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob",
		"enum", "set",
		"json",
		"geometry", "point", "linestring", "polygon",
		"multipoint", "multilinestring", "multipolygon", "geometrycollection",
	}
}

func (mysqlDialect) ServerVersion(ctx context.Context, db Executor) (string, error) {
	var version string
	err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	return version, err
}

// mysqlSystemDBs MySQL 系统数据库（大小写不敏感匹配）
var mysqlSystemDBs = map[string]bool{
	"information_schema": true,
	"performance_schema": true,
	"sys":                true,
	"mysql":              true,
}

func isMySQLSystemDB(name string) bool { return mysqlSystemDBs[strings.ToLower(name)] }

func (d mysqlDialect) Databases(ctx context.Context, db Executor, withCounts bool) ([]DatabaseInfo, error) {
	return d.databases(ctx, db, withCounts, isMySQLSystemDB, false)
}

// databases 列出库并按 isSystem 过滤；withCounts 时用一条 GROUP BY 取各库表数量，避免 N+1。
// tolerantCounts 为 true 时统计失败不算错误（StarRocks 的 information_schema 不一定完整）。
func (mysqlDialect) databases(ctx context.Context, db Executor, withCounts bool, isSystem func(string) bool, tolerantCounts bool) ([]DatabaseInfo, error) {
	var databases []DatabaseInfo
	err := queryEach(ctx, db, "SHOW DATABASES", nil, func(rows rowScanner) error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if !isSystem(name) {
			databases = append(databases, DatabaseInfo{Name: name})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !withCounts || len(databases) == 0 {
		return databases, nil
	}
	counts := make(map[string]int)
	err = queryEach(ctx, db, "SELECT table_schema, COUNT(*) FROM information_schema.tables GROUP BY table_schema", nil,
		func(rows rowScanner) error {
			var schema string
			var count int
			if err := rows.Scan(&schema, &count); err != nil {
				return err
			}
			if !isSystem(schema) {
				counts[schema] = count
			}
			return nil
		})
	if err != nil && !tolerantCounts {
		return nil, err
	}
	for i := range databases {
		databases[i].TableCount = counts[databases[i].Name]
	}
	return databases, nil
}

func (mysqlDialect) Tables(ctx context.Context, db Executor, dbName string) ([]TableInfo, error) {
	var tables []TableInfo
	err := queryEach(ctx, db, `SELECT table_name, table_type,
		IFNULL(table_rows, 0), IFNULL(data_length, 0),
		IFNULL(table_comment, '')
		FROM information_schema.tables
		WHERE table_schema = ?
		ORDER BY table_name`, []any{dbName}, func(rows rowScanner) error {
		var t TableInfo
		var tableType string
		if err := rows.Scan(&t.Name, &tableType, &t.RowCount, &t.Size, &t.Comment); err != nil {
			return err
		}
		t.Type = tableTypeOf(tableType)
		tables = append(tables, t)
		return nil
	})
	return tables, err
}

// columnsNoFK 读取 information_schema.columns，不查外键（TiDB、StarRocks 用）。
func (mysqlDialect) columnsNoFK(ctx context.Context, db Executor, dbName, table string) ([]ColumnInfo, error) {
	var columns []ColumnInfo
	err := queryEach(ctx, db, `SELECT c.column_name, c.column_type, c.is_nullable, c.column_default,
		c.column_key, c.extra, c.column_comment, c.character_maximum_length,
		IFNULL(c.character_set_name, ''), IFNULL(c.collation_name, '')
		FROM information_schema.columns c
		WHERE c.table_schema = ? AND c.table_name = ?
		ORDER BY c.ordinal_position`, []any{dbName, table}, func(rows rowScanner) error {
		var c ColumnInfo
		var nullable, key, extra string
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &c.DefaultValue, &key, &extra, &c.Comment, &c.MaxLength,
			&c.CharacterSet, &c.Collation); err != nil {
			return err
		}
		c.Nullable = nullable == "YES"
		c.IsPrimary = key == "PRI"
		c.IsAutoIncrement = extra == "auto_increment"
		c.Extra = extra
		columns = append(columns, c)
		return nil
	})
	return columns, err
}

func (d mysqlDialect) Columns(ctx context.Context, db Executor, dbName, table string) ([]ColumnInfo, error) {
	columns, err := d.columnsNoFK(ctx, db, dbName, table)
	if err != nil {
		return nil, err
	}
	// 外键信息填到对应列；读取失败（权限等）时静默跳过，不影响列本身
	fkMap := make(map[string]string)
	_ = queryEach(ctx, db, `SELECT kcu.column_name,
		CONCAT(kcu.referenced_table_name, '.', kcu.referenced_column_name) AS fk_ref
		FROM information_schema.key_column_usage kcu
		WHERE kcu.table_schema = ? AND kcu.table_name = ?
		AND kcu.referenced_table_name IS NOT NULL`, []any{dbName, table}, func(rows rowScanner) error {
		var colName, fkRef string
		if rows.Scan(&colName, &fkRef) == nil {
			fkMap[colName] = fkRef
		}
		return nil
	})
	for i := range columns {
		if ref, ok := fkMap[columns[i].Name]; ok {
			columns[i].ForeignKey = ref
		}
	}
	return columns, nil
}

// DDL 表与视图都用 SHOW CREATE TABLE；两者返回的列数不同，定义都在第二列。
func (d mysqlDialect) DDL(ctx context.Context, db Executor, dbName, table string) (string, error) {
	return showCreate(ctx, db, "SHOW CREATE TABLE "+d.QuoteIdent(dbName)+"."+d.QuoteIdent(table), 1)
}
