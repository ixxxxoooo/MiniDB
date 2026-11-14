package database

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	mssql "github.com/microsoft/go-mssqldb"

	"minidb/internal/model"
	"minidb/internal/sqltext"
)

// sqlserverDialect 是 Microsoft SQL Server 方言（驱动 go-mssqldb，纯 Go）。
// 标识符用 [ ]，占位符是 @p1，分页用 OFFSET ... FETCH（见 syntax_sqlserver.go）。
// 连接绑定单个库，USE 在连接池里不可靠，所以访问其他库像 PostgreSQL 一样用独立连接池；
// 元数据按登录用户的默认架构读取（通常是 dbo），其余架构里的表暂不显示。
// 第一版不做用户管理、可视化建表与改表、触发器与例程的编辑，这些入口由 Caps 关闭。
type sqlserverDialect struct{}

func init() { register(sqlserverDialect{}) }

func (sqlserverDialect) Type() DBType { return SQLServer }

func (sqlserverDialect) Label() string       { return "SQL Server" }
func (sqlserverDialect) DefaultPort() int    { return 1433 }
func (sqlserverDialect) DefaultUser() string { return "sa" }

// SSLModes 对应连接串里的 encrypt 与 TrustServerCertificate。
func (sqlserverDialect) SSLModes() []string {
	return []string{SSLDisable, SSLRequired, SSLVerifyFull}
}

func (sqlserverDialect) Caps() Caps {
	return Caps{
		RowEdit:         true,
		PerDatabasePool: true, Network: true,
		ProcessList: true, ForeignKeys: true,
		RowsAffectedReliable: true, MultiDatabase: true,
	}
}

func (sqlserverDialect) QuoteIdent(name string) string {
	if name == "" {
		return ""
	}
	return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
}

func (sqlserverDialect) QuoteString(s string) string { return sqltext.QuoteString(s) }

func (sqlserverDialect) ColumnTypes() []string {
	return []string{
		"bit", "tinyint", "smallint", "int", "bigint",
		"decimal", "numeric", "money", "smallmoney", "float", "real",
		"char", "varchar", "varchar(max)", "nchar", "nvarchar", "nvarchar(max)", "text", "ntext",
		"binary", "varbinary", "varbinary(max)", "image",
		"date", "time", "datetime", "datetime2", "smalldatetime", "datetimeoffset",
		"uniqueidentifier", "xml", "sql_variant", "rowversion",
	}
}

func (sqlserverDialect) Driver() string { return "sqlserver" }

// DSN 生成 sqlserver:// 连接串。SSL：disable 不加密；required 加密但不校验证书；verify-full 加密并校验证书。
func (sqlserverDialect) DSN(cfg *ConnectionConfig) string {
	q := url.Values{}
	if cfg.Database != "" {
		q.Set("database", cfg.Database)
	}
	switch cfg.SSLMode {
	case SSLRequired:
		q.Set("encrypt", "true")
		q.Set("TrustServerCertificate", "true")
	case SSLVerifyFull:
		q.Set("encrypt", "true")
		q.Set("TrustServerCertificate", "false")
	default:
		q.Set("encrypt", "disable")
	}
	q.Set("connection timeout", strconv.Itoa(int(defaultConnectTimeout/time.Second)))
	q.Set("app name", "MiniDB")
	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		RawQuery: q.Encode(),
	}
	return u.String()
}

func (sqlserverDialect) ServerVersion(ctx context.Context, db Executor) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT CAST(SERVERPROPERTY('ProductVersion') AS nvarchar(64)) + ' ' + CAST(SERVERPROPERTY('Edition') AS nvarchar(128))`).Scan(&v)
	return v, err
}

// guidString 把 uniqueidentifier 列扫出的 16 字节转成常见的 GUID 文本；不是 16 字节返回 false。
func guidString(b []byte) (string, bool) {
	if len(b) != 16 {
		return "", false
	}
	var u mssql.UniqueIdentifier
	if u.Scan(b) != nil {
		return "", false
	}
	return u.String(), true
}

// 取不到时给出清楚的错误，而不是返回空结果。
func errSQLServerRoutines() error {
	return model.Errf(model.CodeUnsupportedRoutines, "%s 暂不支持在这里查看存储过程与函数", "SQL Server")
}
