package app

import (
	"strings"

	"minidb/internal/database"
)

// SortedSQL 把一条 SELECT 包成"按某一列排序"的外层查询，用于分页结果的服务端排序：
// 分页结果每次只取一页，在内存里只能排当前这一页，得到的顺序是错的；让数据库排才对整个结果有效。
//
// 参数：stmt 原语句（不带 LIMIT）；column 排序列名，为空时原样返回 stmt；desc 是否降序。
// 返回：新语句。列名按方言引用，不会被当成 SQL 拼进去。
func SortedSQL(dbType database.DBType, stmt, column string, desc bool) string {
	stmt = strings.TrimRight(strings.TrimSpace(stmt), "; \n\t")
	if column == "" {
		return stmt
	}
	dir := ""
	if desc {
		dir = " DESC"
	}
	return "SELECT * FROM (" + stmt + ") AS _minidb_sorted ORDER BY " + database.QuoteIdent(dbType, column) + dir
}
