package database

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	// 注册 database/sql 驱动 "clickhouse"（纯 Go，原生协议）
	_ "github.com/ClickHouse/clickhouse-go/v2"

)

// clickhouseDialect 是 ClickHouse 方言（驱动 clickhouse-go v2，原生协议，默认端口 9000，TLS 为 9440）。
//
// ClickHouse 是列式分析库：没有外键、触发器、事务，UPDATE/DELETE 是异步的 mutation，
// 主键也不保证唯一。所以数据网格只读（Caps.RowEdit 为 false），写数据请用 SQL 编辑器；
// 浏览、查询、导出、导入、查看进程与终止查询都可用。
// 连接绑定单个库，访问其他库像 PostgreSQL 一样用独立连接池；元数据读 system.* 表。
type clickhouseDialect struct{}

func init() { register(clickhouseDialect{}) }

func (clickhouseDialect) Type() DBType { return ClickHouse }

func (clickhouseDialect) Label() string       { return "ClickHouse" }
func (clickhouseDialect) DefaultPort() int    { return 9000 }
func (clickhouseDialect) DefaultUser() string { return "default" }

// SSLModes：required 走 TLS 但不校验证书；verify-full 走 TLS 并校验证书。TLS 端口通常是 9440。
func (clickhouseDialect) SSLModes() []string {
	return []string{SSLDisable, SSLRequired, SSLVerifyFull}
}

func (clickhouseDialect) Caps() Caps {
	return Caps{
		PerDatabasePool: true, Network: true, ProcessList: true, Monitor: true,
		CreateDatabase: true, MultiDatabase: true,
	}
}

// QuoteIdent 用反引号，内部的反引号和反斜杠用反斜杠转义。
func (clickhouseDialect) QuoteIdent(name string) string {
	if name == "" {
		return ""
	}
	r := strings.NewReplacer(`\`, `\\`, "`", "\\`")
	return "`" + r.Replace(name) + "`"
}

// QuoteString 生成字符串字面量：ClickHouse 里反斜杠是转义符，必须连同单引号一起转义。
func (clickhouseDialect) QuoteString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(s) + "'"
}

func (clickhouseDialect) ColumnTypes() []string {
	return []string{
		"Int8", "Int16", "Int32", "Int64", "Int128", "Int256",
		"UInt8", "UInt16", "UInt32", "UInt64", "UInt128", "UInt256",
		"Float32", "Float64", "Decimal(18, 4)", "Bool",
		"String", "FixedString(16)", "UUID", "IPv4", "IPv6",
		"Date", "Date32", "DateTime", "DateTime64(3)",
		"Array(String)", "Map(String, String)", "JSON",
		"Nullable(String)", "LowCardinality(String)",
	}
}

func (clickhouseDialect) Driver() string { return "clickhouse" }

// DSN 生成 clickhouse:// 连接串。库名省略时用 default。
func (clickhouseDialect) DSN(cfg *ConnectionConfig) string {
	q := url.Values{}
	q.Set("dial_timeout", (defaultConnectTimeout / time.Second * time.Second).String())
	switch cfg.SSLMode {
	case SSLRequired:
		q.Set("secure", "true")
		q.Set("skip_verify", "true")
	case SSLVerifyFull:
		q.Set("secure", "true")
	}
	db := cfg.Database
	if db == "" {
		db = "default"
	}
	u := &url.URL{
		Scheme:   "clickhouse",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:     "/" + db,
		RawQuery: q.Encode(),
	}
	return u.String()
}

func (clickhouseDialect) ServerVersion(ctx context.Context, db Executor) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT version()`).Scan(&v)
	return v, err
}

// Syntax：? 占位符（驱动在客户端绑定），转文本用 toString。
func (clickhouseDialect) Syntax() Syntax { return clickhouseSyntax{} }

type clickhouseSyntax struct{ stdSyntax }

func (clickhouseSyntax) CastText(col string) string { return "toString(" + col + ")" }

// clickhouseTxFree 报告该类型的事务是空操作：提交与回滚都不会撤销已写入的数据。
func clickhouseTxFree(t DBType) bool { return t == ClickHouse }
