package database

import (
	"context"
	"fmt"

	"minidb/internal/sqltext"
)

// DBType 是数据库类型。底层仍是字符串，JSON 与已落盘的连接配置保持兼容；
// 用具名类型后，漏写 case 或传错类型会在编译期暴露，而不是运行时悄悄走进 default。
type DBType string

// 支持的数据库类型。除 dialect_*.go 与本文件外，其他地方一律用这些常量，不写字符串字面量。
const (
	MySQL     DBType = "mysql"
	TiDB      DBType = "tidb"
	StarRocks DBType = "starrocks"
	Postgres  DBType = "postgres"
	SQLite    DBType = "sqlite"
)

// AllDBTypes 按界面展示顺序列出全部受支持的数据库类型。
func AllDBTypes() []DBType {
	return []DBType{MySQL, Postgres, SQLite, TiDB, StarRocks}
}

func (t DBType) String() string { return string(t) }

// Valid 报告 t 是否是受支持的数据库类型。
func (t DBType) Valid() bool {
	_, ok := dialects[t]
	return ok
}

// MySQLCompatible 报告 t 是否使用 MySQL 协议与驱动（mysql、tidb、starrocks）。
func (t DBType) MySQLCompatible() bool {
	switch t {
	case MySQL, TiDB, StarRocks:
		return true
	}
	return false
}

// Caps 描述一种数据库能做什么。调用方查能力，不再按类型名写 if。
type Caps struct {
	// SessionUseDB：用会话内 USE 切换当前库（MySQL 系）。PostgreSQL 的连接绑定单个库，SQLite 只有一个库。
	SessionUseDB bool
	// PerDatabasePool：访问其他库要单独建连接池（PostgreSQL）。
	PerDatabasePool bool
	// Network：通过网络连接，因此支持 SSH 隧道与 TLS；SQLite 是本地文件。
	Network bool
	// SupportsTLS：连接表单提供 SSL/TLS 选项。
	SupportsTLS bool
	// 是否支持用户管理、进程列表、只读事务（只读事务要能真的拒绝写操作）。
	Users bool
	// UserResourceLimits：账号支持每小时查询数等资源限制（仅 MySQL）。
	UserResourceLimits bool
	ProcessList        bool
	ReadOnlyTx         bool
	// 是否有外键、触发器、函数与存储过程。
	ForeignKeys bool
	Triggers    bool
	Routines    bool
	// AddDropForeignKey：能对已有的表添加、删除外键（SQLite 只能重建表）。
	AddDropForeignKey bool
	// CreateTable：能在界面里可视化建表（StarRocks 需要分桶、副本等，必须手写）。
	CreateTable bool
	// 清空/删除表的可选项。
	TruncateCascade bool
	RestartIdentity bool
	DropCascade     bool
	// RowsAffectedReliable：UPDATE/DELETE 的受影响行数可信（StarRocks 不可信）。
	RowsAffectedReliable bool
	// CreateDatabase：能在连接里新建/删除/重命名库（SQLite 的库就是文件）。
	CreateDatabase bool
	// Charset：新建库时可以指定字符集（MySQL、TiDB）。
	Charset bool
	// MultiDatabase：一个连接下有多个库可以浏览。
	MultiDatabase bool
}

// Dialect 把一种数据库的差异收在一处：标识符与字符串转义、元数据读取、能力声明。
// 新增一种数据库就是新增一个 dialect_xxx.go 并在 init 里 register。
//
// 所有读取方法都带 ctx，调用方可以取消或设置超时。
type Dialect interface {
	Type() DBType
	Caps() Caps
	// Label 是界面上显示的名字；DefaultPort、DefaultUser 是新建连接时的默认值（SQLite 没有）。
	Label() string
	DefaultPort() int
	DefaultUser() string
	// SSLModes 是连接表单里 SSL/TLS 选项的可选值；空表示该类型没有这个选项。
	SSLModes() []string

	// QuoteIdent 给标识符加引号并转义内部的引号；空串返回空串。
	QuoteIdent(name string) string
	// QuoteString 生成字符串字面量（含两侧单引号），按该方言转义。
	QuoteString(s string) string
	// ColumnTypes 返回结构编辑器里可选的字段类型。
	ColumnTypes() []string
	// Driver 是 database/sql 的驱动名；DSN 由连接配置生成该驱动的连接串。
	Driver() string
	DSN(cfg *ConnectionConfig) string

	ServerVersion(ctx context.Context, db Executor) (string, error)
	// Databases 列出库；withCounts 为 true 时附带各库的表数量（较慢）。
	Databases(ctx context.Context, db Executor, withCounts bool) ([]DatabaseInfo, error)
	Tables(ctx context.Context, db Executor, dbName string) ([]TableInfo, error)
	Columns(ctx context.Context, db Executor, dbName, table string) ([]ColumnInfo, error)
	Indexes(ctx context.Context, db Executor, dbName, table string) ([]IndexInfo, error)
	DDL(ctx context.Context, db Executor, dbName, table string) (string, error)
	TableStats(ctx context.Context, db Executor, dbName, table string) (*TableStats, error)
	TableOverview(ctx context.Context, db Executor, dbName string) ([]TableOverview, error)
	ForeignKeys(ctx context.Context, db Executor, dbName, table string) ([]ForeignKeyInfo, error)
	Triggers(ctx context.Context, db Executor, dbName, table string) ([]TriggerInfo, error)
	Routines(ctx context.Context, db Executor, dbName string) ([]RoutineInfo, error)
	RoutineDDL(ctx context.Context, db Executor, dbName, kind, name string) (string, error)
}

// UserAdmin 由支持用户管理的方言额外实现（Caps().Users 为 true）。
type UserAdmin interface {
	Users(ctx context.Context, db Executor) ([]UserInfo, error)
	UserGrants(ctx context.Context, db Executor, name, host string) ([]string, error)
	CreateUserSQL(u UserSpec) string
	AlterUserSQL(u UserSpec) string
	DropUserSQL(name, host string) string
	// PrivSQL 生成授权（grant 为 true）或收回权限的语句。
	PrivSQL(grant bool, privs []string, scope PrivScope, name, host string, withGrant bool) (string, error)
	Privileges(scope PrivScope) []string
}

// ProcessAdmin 由支持查看/终止会话的方言额外实现（Caps().ProcessList 为 true）。
type ProcessAdmin interface {
	ProcessList(ctx context.Context, db Executor) ([]ProcessInfo, error)
	KillProcess(ctx context.Context, db Executor, id uint64, queryOnly bool) error
}

var dialects = map[DBType]Dialect{}

func register(d Dialect) { dialects[d.Type()] = d }

// DialectOf 返回类型对应的方言；类型未知时返回错误。
func DialectOf(t DBType) (Dialect, error) {
	if d, ok := dialects[t]; ok {
		return d, nil
	}
	return nil, errUnsupportedType(t)
}

// mustDialect 给只在已校验过类型之后调用的内部代码用；未知类型退化为一个只做通用引号转义的方言，
// 保证纯字符串生成函数（BuildXxxSQL）对未知类型不 panic，行为与迁移前一致（标识符原样返回）。
func mustDialect(t DBType) Dialect {
	if d, ok := dialects[t]; ok {
		return d
	}
	return unknownDialect{t: t}
}

func errUnsupportedType(t DBType) error {
	return fmt.Errorf("不支持的数据库类型: %s", t)
}

// SSLModesOf 返回类型可选的 SSL/TLS 模式；未知类型或没有此选项的类型返回 nil。
func SSLModesOf(t DBType) []string {
	if d, ok := dialects[t]; ok {
		return d.SSLModes()
	}
	return nil
}

// CapsOf 返回类型的能力；未知类型返回全部为 false 的零值。
func CapsOf(t DBType) Caps {
	if d, ok := dialects[t]; ok {
		return d.Caps()
	}
	return Caps{}
}

// unknownDialect 是未知类型的占位：标识符不加引号，其余能力一律没有。
type unknownDialect struct{ t DBType }

func (u unknownDialect) Type() DBType                  { return u.t }
func (u unknownDialect) Caps() Caps                    { return Caps{} }
func (u unknownDialect) Label() string                 { return string(u.t) }
func (u unknownDialect) DefaultPort() int              { return 0 }
func (u unknownDialect) DefaultUser() string           { return "" }
func (u unknownDialect) SSLModes() []string            { return nil }
func (u unknownDialect) QuoteIdent(name string) string { return name }
func (u unknownDialect) QuoteString(s string) string   { return sqltext.QuoteString(s) }
func (u unknownDialect) ColumnTypes() []string         { return nil }
func (u unknownDialect) Driver() string                { return "" }
func (u unknownDialect) DSN(*ConnectionConfig) string  { return "" }
func (u unknownDialect) ServerVersion(context.Context, Executor) (string, error) {
	return "", errUnsupportedType(u.t)
}
func (u unknownDialect) Databases(context.Context, Executor, bool) ([]DatabaseInfo, error) {
	return nil, errUnsupportedType(u.t)
}
func (u unknownDialect) Tables(context.Context, Executor, string) ([]TableInfo, error) {
	return nil, errUnsupportedType(u.t)
}
func (u unknownDialect) Columns(context.Context, Executor, string, string) ([]ColumnInfo, error) {
	return nil, errUnsupportedType(u.t)
}
func (u unknownDialect) Indexes(context.Context, Executor, string, string) ([]IndexInfo, error) {
	return nil, errUnsupportedType(u.t)
}
func (u unknownDialect) DDL(context.Context, Executor, string, string) (string, error) {
	return "", errUnsupportedType(u.t)
}
func (u unknownDialect) TableStats(context.Context, Executor, string, string) (*TableStats, error) {
	return nil, errUnsupportedType(u.t)
}
func (u unknownDialect) TableOverview(context.Context, Executor, string) ([]TableOverview, error) {
	return nil, errUnsupportedType(u.t)
}
func (u unknownDialect) ForeignKeys(context.Context, Executor, string, string) ([]ForeignKeyInfo, error) {
	return nil, nil
}
func (u unknownDialect) Triggers(context.Context, Executor, string, string) ([]TriggerInfo, error) {
	return nil, nil
}
func (u unknownDialect) Routines(context.Context, Executor, string) ([]RoutineInfo, error) {
	return nil, nil
}
func (u unknownDialect) RoutineDDL(context.Context, Executor, string, string, string) (string, error) {
	return "", fmt.Errorf("%s 不支持存储过程与函数", u.t)
}
