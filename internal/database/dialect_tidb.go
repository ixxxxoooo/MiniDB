package database

import "context"

// tidbDialect 与 MySQL 基本一致：不查外键信息填列（TiDB 的 information_schema 对此支持不完整），
// 账号没有资源限制子句。
type tidbDialect struct{ mysqlDialect }

func (tidbDialect) Label() string    { return "TiDB" }
func (tidbDialect) DefaultPort() int { return 4000 }

func (tidbDialect) Caps() Caps {
	c := mysqlDialect{}.Caps()
	c.UserResourceLimits = false
	c.Monitor = false // 状态变量与 MySQL 差异大，第一版只提供进程列表
	return c
}

func (tidbDialect) ColumnTypes() []string {
	return []string{
		"tinyint", "smallint", "mediumint", "int", "bigint",
		"decimal", "numeric", "float", "double", "bit", "boolean",
		"date", "datetime", "timestamp", "time", "year",
		"char", "varchar", "tinytext", "text", "mediumtext", "longtext",
		"binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob",
		"enum", "set",
		"json",
	}
}

func (d tidbDialect) Columns(ctx context.Context, db Executor, dbName, table string) ([]ColumnInfo, error) {
	return d.columnsNoFK(ctx, db, dbName, table)
}
