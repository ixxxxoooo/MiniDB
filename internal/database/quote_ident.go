package database

import "strings"

// QuoteIdent 为标识符（列名、索引名等）加引用，按数据库方言处理反义
func QuoteIdent(dbType, name string) string {
	if name == "" {
		return ""
	}
	if IsMySQLCompatible(dbType) {
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	switch dbType {
	case "postgres", "sqlite":
		// PostgreSQL / SQLite 使用双引号标识符，需转义内部双引号
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	default:
		return name
	}
}
