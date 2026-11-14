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
	// RowEdit：数据网格里可以直接改、增、删行（ClickHouse 的 UPDATE/DELETE 是异步 mutation，网格只读）。
	RowEdit bool
	// MultiDatabase：一个连接下有多个库可以浏览。
	MultiDatabase bool
}
