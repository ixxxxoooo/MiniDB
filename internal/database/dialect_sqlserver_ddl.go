package database

import "fmt"

// SQL Server 里和其他方言写法不同的几条表级语句。

// sqlserverRenameTable 用 sp_rename 改表名；第二个参数只能是新名字本身，不带架构。
func sqlserverRenameTable(from, to string) string {
	return fmt.Sprintf("EXEC sp_rename %s, %s", QuoteString(SQLServer, from), QuoteString(SQLServer, to))
}

// sqlserverCloneTable 用 SELECT INTO 复制表：只要结构时加一个永远为假的条件。
// 它不带主键、索引和默认值，复制出来的只是列。
func sqlserverCloneTable(src, dst string, withData bool) []string {
	s, d := QuoteTableName(SQLServer, src), QuoteTableName(SQLServer, dst)
	if withData {
		return []string{fmt.Sprintf("SELECT * INTO %s FROM %s", d, s)}
	}
	return []string{fmt.Sprintf("SELECT * INTO %s FROM %s WHERE 1 = 0", d, s)}
}
