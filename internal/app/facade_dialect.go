// @author ixxxxoooo

package app

import "minidb/internal/database"

// 方言里与界面相关的小差异，ui 不直接依赖 database，经这里取用。

// DefaultDBType 转调 database.DefaultDBType。
func DefaultDBType() database.DBType { return database.DefaultDBType() }

// AutoPKType 转调 database.AutoPKType。
func AutoPKType(t database.DBType) string { return database.AutoPKType(t) }

// URLSchemeOf 转调 database.URLSchemeOf。
func URLSchemeOf(t database.DBType) string { return database.URLSchemeOf(t) }

// TypeFromURLScheme 转调 database.TypeFromURLScheme。
func TypeFromURLScheme(scheme string) (database.DBType, bool) {
	return database.TypeFromURLScheme(scheme)
}

// RefineType 转调 database.RefineType。
func RefineType(t database.DBType, version string) (database.DBType, bool) {
	return database.RefineType(t, version)
}
