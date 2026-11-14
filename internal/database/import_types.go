package database

// importColumnType 把推断出的类型（int、float、text）写成各方言的列类型。
func importColumnType(dbType DBType, kind string) string {
	pick := func(i, f, t string) string {
		switch kind {
		case "int":
			return i
		case "float":
			return f
		}
		return t
	}
	switch dbType {
	case SQLite:
		return pick("INTEGER", "REAL", "TEXT")
	case Postgres:
		return pick("BIGINT", "DOUBLE PRECISION", "TEXT")
	case SQLServer:
		return pick("BIGINT", "FLOAT", "NVARCHAR(MAX)")
	}
	return pick("BIGINT", "DOUBLE", "TEXT") // MySQL 系
}

// importParamLimit 是单条语句的占位符上限：MySQL/PG 为 65535，SQLite 默认 32766，取保守值；
// SQL Server 只有 2100，留一点余量。
func importParamLimit(dbType DBType) int {
	if l, ok := SyntaxOf(dbType).(interface{ MaxParams() int }); ok {
		return l.MaxParams()
	}
	return maxImportParams
}
