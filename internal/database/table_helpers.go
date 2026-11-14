// @author ixxxxoooo

package database

import (
	"context"
	"strings"
)

// DefaultColumnTypes 是没有连接可参考时结构编辑器里提供的字段类型（按 MySQL）。
func DefaultColumnTypes() []string { return GetColumnTypes(MySQL) }

// CanReplaceInPlace 判断修改对象定义时能否直接执行新定义，而不用先 DROP 再 CREATE。
// PostgreSQL 的函数与存储过程、以及带 OR REPLACE 的视图可以原地替换。
// 参数：dbType 连接类型；kind 对象种类（view / function / procedure）；ddl 新定义。
func CanReplaceInPlace(dbType DBType, kind, ddl string) bool {
	if dbType != Postgres {
		return false
	}
	return kind != "view" || strings.Contains(strings.ToUpper(ddl), "OR REPLACE")
}

// CloneTableStatements 生成复制表（可连数据）要依次执行的语句。
// SQLite 没有 CREATE TABLE ... LIKE，要先读出原表的建表语句再改名，所以需要连接；其余方言是纯文本生成。
// 参数：db 用于读取原表定义的执行器；dbType 连接类型；dbName 库名；src、dst 原表与新表名；withData 是否复制数据。
func CloneTableStatements(ctx context.Context, db Executor, dbType DBType, dbName, src, dst string, withData bool) ([]string, error) {
	if dbType != SQLite {
		return BuildCloneTableSQL(dbType, src, dst, withData)
	}
	ddl, err := GetDDL(ctx, db, dbType, dbName, src)
	if err != nil {
		return nil, err
	}
	return CloneSQLiteTableSQL(ddl, src, dst, withData)
}
