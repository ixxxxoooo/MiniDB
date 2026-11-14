package database

// importColumnType 把推断出的类型（int、float、text）写成各方言的列类型。
func importColumnType(dbType DBType, kind string) string {
	if h, ok := hookOf[ImportColumnTyper](dbType); ok {
		return h.ImportColumnType(kind)
	}
	return pickImportType(kind, "BIGINT", "DOUBLE", "TEXT") // MySQL 系
}

// importParamLimit 是单条语句的占位符上限：MySQL/PG 为 65535，SQLite 默认 32766，取保守值；
// SQL Server 只有 2100，留一点余量。
func importParamLimit(dbType DBType) int {
	if l, ok := SyntaxOf(dbType).(interface{ MaxParams() int }); ok {
		return l.MaxParams()
	}
	return maxImportParams
}

// importTableSuffix 是建表语句末尾的固定部分：ClickHouse 建表必须指定引擎。
func importTableSuffix(dbType DBType) string {
	if h, ok := hookOf[ImportTableSuffixer](dbType); ok {
		return h.ImportTableSuffix()
	}
	return ""
}
