package model

// Caps 描述一种数据库能做什么。调用方查能力，不再按类型名写 if。
type Caps struct {
	// SessionUseDB：用会话内 USE 切换当前库（MySQL 系）。PostgreSQL 的连接绑定单个库，SQLite 只有一个库。
	SessionUseDB bool
	// PerDatabasePool：访问其他库要单独建连接池（PostgreSQL）。
	PerDatabasePool bool
	// Network：通过网络连接，因此支持 SSH 隧道与 TLS；SQLite 是本地文件。
	Network bool
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
	// AlterTable：能在界面里可视化修改已有表的结构（列、索引）；为 false 时结构页只读，要改请用 SQL 编辑器。
	AlterTable bool
	// ColumnReorder：能调整已有表的列顺序（MySQL 的 AFTER / FIRST）。
	ColumnReorder bool
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
	// Explain：能生成执行计划；ExplainAnalyze：还能带 ANALYZE 真实执行并给出实际耗时。
	Explain        bool
	ExplainAnalyze bool
	// Monitor：能读服务端指标（连接数、QPS 等）和慢查询，监控面板才有「概览」与「慢查询」页签。
	Monitor bool
	// RowEdit：数据网格里可以直接改、增、删行（ClickHouse 的 UPDATE/DELETE 是异步 mutation，网格只读）。
	RowEdit bool
	// MultiDatabase：一个连接下有多个库可以浏览。
	MultiDatabase bool
	// BackslashEscape：字符串字面量里反斜杠是转义符（MySQL 系），导出 SQL 时要把反斜杠也转义。
	BackslashEscape bool
	// QueryOnlyPragma：只读执行靠 PRAGMA query_only（SQLite），而不是只读事务。
	QueryOnlyPragma bool
	// Dump：能用哪一类外部命令行工具备份与还原；DumpNone 表示不能。
	Dump DumpTool
	// QueriesAreTransactions：监控里「查询数」这一项统计的其实是事务数（PostgreSQL）。
	QueriesAreTransactions bool
	// RoleUsers：账号是角色（PostgreSQL）：有「可登录」开关，授权按 schema 而不是按主机。
	RoleUsers bool
	// DefaultSchema：连接默认使用的 schema 名；没有 schema 概念的为空。
	DefaultSchema string
	// AutoPKType：新建表时预填的自增主键列类型；空表示用通用的 bigint。
	AutoPKType string
}

// DumpTool 是备份、还原所用外部工具的系列。
type DumpTool string

const (
	DumpNone     DumpTool = ""
	DumpMySQL    DumpTool = "mysql"    // mysqldump / mariadb-dump，还原用 mysql / mariadb
	DumpPostgres DumpTool = "postgres" // pg_dump，还原用 psql
)

// LocalFile 报告连接是不是本地文件（SQLite）：没有主机、端口、密码，也不能走 SSH 隧道。
func (c Caps) LocalFile() bool { return !c.Network }
