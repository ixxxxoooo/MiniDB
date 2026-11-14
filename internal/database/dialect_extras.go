// @author ixxxxoooo

package database

import "strings"

// 方言里与界面相关、但不属于能力开关的几项小差异：默认类型、主键类型、连接地址协议名、按服务端版本纠正类型。

// DefaultDBType 是新建连接时预选的数据库类型。
func DefaultDBType() DBType { return MySQL }

// AutoPKType 返回新建表时预填的自增主键列类型。
func AutoPKType(t DBType) string {
	if v := CapsOf(t).AutoPKType; v != "" {
		return v
	}
	return "bigint"
}

// URLSchemer 由连接地址协议名不止是类型名的方言实现；第一项是生成地址时用的规范写法，其余是解析时认的别名。
type URLSchemer interface{ URLSchemes() []string }

func (postgresDialect) URLSchemes() []string   { return []string{"postgresql", "postgres", "pg"} }
func (clickhouseDialect) URLSchemes() []string { return []string{"clickhouse", "ch"} }
func (sqlserverDialect) URLSchemes() []string  { return []string{"sqlserver", "mssql"} }
func (sqliteDialect) URLSchemes() []string     { return []string{"sqlite", "sqlite3", "file"} }

// urlSchemesOf 返回类型认的全部协议名，没有声明的只认类型名。
func urlSchemesOf(d Dialect) []string {
	if s, ok := d.(URLSchemer); ok {
		return s.URLSchemes()
	}
	return []string{string(d.Type())}
}

// URLSchemeOf 返回类型生成连接地址（如 postgresql://…）时用的协议名。
func URLSchemeOf(t DBType) string {
	if d, ok := dialects[t]; ok {
		return urlSchemesOf(d)[0]
	}
	return string(t)
}

// TypeFromURLScheme 按连接地址的协议名找出数据库类型；不认识时 ok 为 false。
func TypeFromURLScheme(scheme string) (DBType, bool) {
	scheme = strings.ToLower(scheme)
	for _, t := range AllDBTypes() {
		d, ok := dialects[t]
		if !ok {
			continue
		}
		for _, s := range urlSchemesOf(d) {
			if s == scheme {
				return t, true
			}
		}
	}
	return "", false
}

// TypeRefiner 由能从服务端版本串看出「其实是另一种数据库」的方言实现。
type TypeRefiner interface {
	RefineType(version string) (DBType, bool)
}

// RefineType 在连上服务器之后，按版本串判断连接类型是否选得不对（如把 MariaDB 当成 MySQL 连）。
// 返回更合适的类型和 true；没有更合适的类型时返回 false。
func RefineType(t DBType, version string) (DBType, bool) {
	if d, ok := dialects[t]; ok {
		if r, ok := d.(TypeRefiner); ok {
			return r.RefineType(version)
		}
	}
	return "", false
}

// RefineType 只对 MySQL 类型生效：版本串里带 mariadb 就是 MariaDB。TiDB、StarRocks 也内嵌了本类型，所以要看 kind。
func (d mysqlDialect) RefineType(version string) (DBType, bool) {
	if d.kind == MySQL && strings.Contains(strings.ToLower(version), "mariadb") {
		return MariaDB, true
	}
	return "", false
}
