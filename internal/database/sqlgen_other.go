// @author ixxxxoooo

package database

import "fmt"

// ClickHouse、SQL Server、StarRocks 的语句写法；其余没列出的沿用通用写法。

// cloneWithInsert 生成「建表语句 + 可选的 INSERT ... SELECT」；createFmt 的两个 %s 依次是新表和原表。
func cloneWithInsert(dbType DBType, src, dst string, withData bool, createFmt string) []string {
	s, d := QuoteTableName(dbType, src), QuoteTableName(dbType, dst)
	out := []string{fmt.Sprintf(createFmt, d, s)}
	if withData {
		out = append(out, fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", d, s))
	}
	return out
}

// pickImportType 按推断出的类型（int、float、其余当文本）选列类型。
func pickImportType(kind, i, f, t string) string {
	switch kind {
	case "int":
		return i
	case "float":
		return f
	}
	return t
}

// ---- ClickHouse ----

func (clickhouseDialect) ImportColumnType(kind string) string {
	return pickImportType(kind, "Int64", "Float64", "String")
}

// ImportTableSuffix：ClickHouse 建表必须指定引擎。
func (clickhouseDialect) ImportTableSuffix() string { return " ENGINE = MergeTree ORDER BY tuple()" }

func (clickhouseDialect) RenameDatabaseSQL(oldName, newName string) string {
	return fmt.Sprintf("RENAME DATABASE %s TO %s", QuoteIdent(ClickHouse, oldName), QuoteIdent(ClickHouse, newName))
}

func (clickhouseDialect) CloneTableSQL(src, dst string, withData bool) []string {
	return cloneWithInsert(ClickHouse, src, dst, withData, "CREATE TABLE %s AS %s")
}

func (clickhouseDialect) RenameTableSQL(from, to string) string {
	return fmt.Sprintf("RENAME TABLE %s TO %s", QuoteTableName(ClickHouse, from), QuoteTableName(ClickHouse, to))
}

// ---- SQL Server ----

func (sqlserverDialect) ImportColumnType(kind string) string {
	return pickImportType(kind, "BIGINT", "FLOAT", "NVARCHAR(MAX)")
}

func (sqlserverDialect) CloneTableSQL(src, dst string, withData bool) []string {
	return sqlserverCloneTable(src, dst, withData)
}

func (sqlserverDialect) RenameTableSQL(from, to string) string { return sqlserverRenameTable(from, to) }

// ---- StarRocks ----

func (starrocksDialect) RenameDatabaseSQL(oldName, newName string) string {
	return fmt.Sprintf("ALTER DATABASE %s RENAME %s", QuoteIdent(StarRocks, oldName), QuoteIdent(StarRocks, newName))
}

func (starrocksDialect) RenameTableSQL(from, to string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME %s", QuoteTableName(StarRocks, from), QuoteTableName(StarRocks, to))
}
