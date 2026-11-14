package database

import (
	"context"
	"minidb/internal/model"

	"minidb/internal/sqltext"
)

// Dialect 把一种数据库的差异收在一处：标识符与字符串转义、连接、元数据读取、能力声明。
// 新增一种数据库就是新增一个 dialect_xxx.go 并在 init 里 register。
//
// 它由四个小接口组合而成，调用方只需要其中一部分时应当只依赖那一部分；
// 外键、触发器、函数与存储过程不是每种数据库都有，放在后面的可选接口里，按 Caps 声明再取用。
// 所有读取方法都带 ctx，调用方可以取消或设置超时。
type Dialect interface {
	Identity
	Quoter
	Connector
	MetadataReader
}

// Identity 描述一种数据库的身份：类型、名字、新建连接时的默认值和能力声明。
type Identity interface {
	Type() DBType
	Caps() Caps
	// Label 是界面上显示的名字；DefaultPort、DefaultUser 是新建连接时的默认值（SQLite 没有）。
	Label() string
	DefaultPort() int
	DefaultUser() string
	// SSLModes 是连接表单里 SSL/TLS 选项的可选值；空表示该类型没有这个选项。
	SSLModes() []string
	// ColumnTypes 返回结构编辑器里可选的字段类型。
	ColumnTypes() []string
}

// Quoter 负责按该方言的规则生成标识符与字符串字面量。
type Quoter interface {
	// QuoteIdent 给标识符加引号并转义内部的引号；空串返回空串。
	QuoteIdent(name string) string
	// QuoteString 生成字符串字面量（含两侧单引号），按该方言转义。
	QuoteString(s string) string
}

// Connector 负责告诉连接管理器怎么连：database/sql 的驱动名，以及由连接配置生成的连接串。
type Connector interface {
	Driver() string
	DSN(cfg *ConnectionConfig) string
}

// MetadataReader 读取库、表、列、索引等基础元数据，每种数据库都必须有。
type MetadataReader interface {
	ServerVersion(ctx context.Context, db Executor) (string, error)
	// Databases 列出库；withCounts 为 true 时附带各库的表数量（较慢）。
	Databases(ctx context.Context, db Executor, withCounts bool) ([]DatabaseInfo, error)
	Tables(ctx context.Context, db Executor, dbName string) ([]TableInfo, error)
	Columns(ctx context.Context, db Executor, dbName, table string) ([]ColumnInfo, error)
	Indexes(ctx context.Context, db Executor, dbName, table string) ([]IndexInfo, error)
	DDL(ctx context.Context, db Executor, dbName, table string) (string, error)
	TableStats(ctx context.Context, db Executor, dbName, table string) (*TableStats, error)
	TableOverview(ctx context.Context, db Executor, dbName string) ([]TableOverview, error)
}

// ForeignKeyReader 由有外键的方言实现（Caps().ForeignKeys 为 true）。
type ForeignKeyReader interface {
	ForeignKeys(ctx context.Context, db Executor, dbName, table string) ([]ForeignKeyInfo, error)
}

// TriggerReader 由有触发器的方言实现（Caps().Triggers 为 true）。
type TriggerReader interface {
	Triggers(ctx context.Context, db Executor, dbName, table string) ([]TriggerInfo, error)
}

// RoutineReader 由有函数与存储过程的方言实现（Caps().Routines 为 true）。
type RoutineReader interface {
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
	return model.Errf(model.CodeUnsupportedType, "不支持的数据库类型: %s", t)
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
