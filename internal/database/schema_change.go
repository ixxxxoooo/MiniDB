package database

// IsSchemaChangeSQL 判断 SQL 是否属于需要刷新 schema 索引的结构变更语句。
func IsSchemaChangeSQL(sql string) bool {
	switch SQLLeadingVerb(sql) {
	case "create", "alter", "drop", "rename":
		return true
	default:
		return false
	}
}
