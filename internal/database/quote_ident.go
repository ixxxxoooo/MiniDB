package database

// QuoteIdent 为标识符（列名、索引名等）加引用，按数据库方言转义内部的引号。
// 未知类型原样返回标识符。
func QuoteIdent(dbType DBType, name string) string {
	return mustDialect(dbType).QuoteIdent(name)
}

// QuoteString 生成字符串字面量（含两侧单引号），按数据库方言转义；
// 未知类型按标准 SQL 处理。需要拼进 SQL 的字符串一律走它，不要自己 ReplaceAll。
func QuoteString(dbType DBType, s string) string {
	return mustDialect(dbType).QuoteString(s)
}
