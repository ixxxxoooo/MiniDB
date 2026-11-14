// @author ixxxxoooo

package database

// 生成 DDL / 写入语句时，各方言写法不同的地方都做成这里的可选接口：
// 方言只实现自己和 MySQL 通用写法不同的那几个，构造函数（BuildXxxSQL）用 hookOf 取，取不到就用通用写法。
// 这样构造函数里不再出现「if 类型 == X」，新增一种数据库也不用去改它们。

// hookOf 取类型对应方言实现的某个可选接口。
func hookOf[T any](t DBType) (T, bool) {
	var zero T
	d, ok := dialects[t]
	if !ok {
		return zero, false
	}
	h, ok := d.(T)
	return h, ok
}

// DatabaseRenamer 由有「重命名库」语句的方言实现；没有的（MySQL、TiDB）由调用方把表逐个搬走。
type DatabaseRenamer interface {
	RenameDatabaseSQL(oldName, newName string) string
}

// TableCloner 由复制表的写法不是 CREATE TABLE ... LIKE 的方言实现。
type TableCloner interface {
	CloneTableSQL(src, dst string, withData bool) []string
}

// DDLCloner 由只能从现有建表语句改名来复制表的方言实现（SQLite）。
type DDLCloner interface {
	CloneTableFromDDL(ddl, src, dst string, withData bool) ([]string, error)
}

// TableRenamer 由重命名表的写法不是 ALTER TABLE ... RENAME TO 的方言实现。
type TableRenamer interface {
	RenameTableSQL(from, to string) string
}

// TableTruncater 由清空表的写法不是 TRUNCATE TABLE 的方言实现。
type TableTruncater interface {
	TruncateTableSQL(table string, o TableOpOptions) string
}

// ImportColumnTyper 由导入时建表的列类型和 MySQL 系（BIGINT、DOUBLE、TEXT）不同的方言实现。
type ImportColumnTyper interface {
	// ImportColumnType 把推断出的类型（int、float、text）写成该方言的列类型。
	ImportColumnType(kind string) string
}

// ImportTableSuffixer 由建表语句末尾必须带固定内容的方言实现（ClickHouse 要指定引擎）。
type ImportTableSuffixer interface{ ImportTableSuffix() string }

// ImportConflictWriter 由支持导入时处理重复键的方言实现；没有实现的只能「报错并停止」。
type ImportConflictWriter interface {
	// ImportModifiers 给出 INSERT 的关键字和语句末尾的冲突子句。
	// mode 取 ConflictIgnore / ConflictReplace / ConflictUpdate；targets 是已引用的目标列，keys 是判断重复的列。
	ImportModifiers(targets, keys []string, mode string) (verb, suffix string, err error)
}

// BinaryLiteraler 由二进制字面量写法不是 X'HEX' 的方言实现。
type BinaryLiteraler interface{ BinaryLiteral(upperHex string) string }

// CreateTriggerWriter 由创建触发器的写法不是「CREATE TRIGGER … FOR EACH ROW 正文」的方言实现。
type CreateTriggerWriter interface {
	CreateTriggerSQL(name, timing, event, table, body string) string
}

// DropTriggerWriter 由删除触发器要带表名的方言实现。
type DropTriggerWriter interface {
	DropTriggerSQL(table, name string) string
}

// ForeignKeyDropper 由删除外键的写法不是 DROP FOREIGN KEY 的方言实现。
type ForeignKeyDropper interface {
	DropForeignKeySQL(table, name string) string
}

// CreateColumnBuilder 由新建表时列的写法有特别之处的方言实现（自增、列注释）。
type CreateColumnBuilder interface {
	// CreateColumnDef 生成一列的定义。pkCount 是表里主键列数，tq 是已引用的表名。
	// inlinePK 为真表示主键已写在列定义里，不再追加 PRIMARY KEY (...)；after 是建表之后要执行的语句（如列注释）。
	CreateColumnDef(c StructureColumnEdit, qn, tq string, pkCount int) (def string, inlinePK bool, after []string, err error)
}

// ColumnClauseExtender 由列定义末尾还有额外子句的方言实现（MySQL 系的 ON UPDATE、AUTO_INCREMENT、COMMENT）。
type ColumnClauseExtender interface {
	ColumnClauseExtras(col StructureColumnEdit) string
}

// StructureAlterer 由支持在界面里改表结构的方言实现（Caps().AlterTable 为 true）。
type StructureAlterer interface {
	StructureAlterSQL(serverVersion, table string, working, original []StructureColumnEdit, idx []StructureIndexEdit) ([]string, error)
}

// StructureAlterChecker 由对服务端版本有要求的方言实现（SQLite 要 3.25 以上）。
type StructureAlterChecker interface {
	CheckStructureAlter(serverVersion string) error
}

// IndexSQLWriter 由支持在界面里增删索引的方言实现。
type IndexSQLWriter interface {
	AddIndexSQL(table, index string, cols []string, unique bool) string
	DropIndexSQL(table, index string) string
}

// FullStatementPrefixer 由有自己独有的整句开头（PRAGMA、USE 等）的方言实现。
type FullStatementPrefixer interface{ FullStatementPrefixes() []string }
