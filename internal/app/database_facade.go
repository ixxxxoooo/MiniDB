// 界面层（ui）不直接依赖 database 包，需要的类型、常量和纯函数都从这里取。
// 这里只做转调，不加逻辑；新增的界面需求先在这里登记，再由界面使用。
package app

import "minidb/internal/database"

type (
	Filter               = database.Filter
	ImportColumn         = database.ImportColumn
	ImportOptions        = database.ImportOptions
	PGRoleAttrs          = database.PGRoleAttrs
	PrivScope            = database.PrivScope
	ProcessInfo          = database.ProcessInfo
	QueryResult          = database.QueryResult
	RowChange            = database.RowChange
	Sort                 = database.Sort
	StructureColumnEdit  = database.StructureColumnEdit
	StructureFKEdit      = database.StructureFKEdit
	StructureIndexEdit   = database.StructureIndexEdit
	StructureObjectEdits = database.StructureObjectEdits
	StructureTriggerEdit = database.StructureTriggerEdit
	TableOpOptions       = database.TableOpOptions
	TableOverview        = database.TableOverview
	UserInfo             = database.UserInfo
	UserLimits           = database.UserLimits
	UserSpec             = database.UserSpec
)

const (
	ConflictIgnore  = database.ConflictIgnore
	ConflictReplace = database.ConflictReplace
	ConflictStop    = database.ConflictStop
	ConflictUpdate  = database.ConflictUpdate
	DefaultPageSize = database.DefaultPageSize
)

var (
	FKActions        = database.FKActions
	MySQLGlobalPrivs = database.MySQLGlobalPrivs
	TriggerEvents    = database.TriggerEvents
	TriggerTimings   = database.TriggerTimings
)

// BuildAddForeignKeySQL 转调 database.BuildAddForeignKeySQL。
func BuildAddForeignKeySQL(dbType database.DBType, table string, fk database.ForeignKeyInfo) (string, error) {
	return database.BuildAddForeignKeySQL(dbType, table, fk)
}

// BuildAlterUserSQL 转调 database.BuildAlterUserSQL。
func BuildAlterUserSQL(dbType database.DBType, u database.UserSpec) (string, error) {
	return database.BuildAlterUserSQL(dbType, u)
}

// BuildCreateDatabaseSQL 转调 database.BuildCreateDatabaseSQL。
func BuildCreateDatabaseSQL(dbType database.DBType, name string, charset string) (string, error) {
	return database.BuildCreateDatabaseSQL(dbType, name, charset)
}

// BuildCreateUserSQL 转调 database.BuildCreateUserSQL。
func BuildCreateUserSQL(dbType database.DBType, u database.UserSpec) (string, error) {
	return database.BuildCreateUserSQL(dbType, u)
}

// BuildDropTableSQL 转调 database.BuildDropTableSQL。
func BuildDropTableSQL(dbType database.DBType, table string, o database.TableOpOptions) string {
	return database.BuildDropTableSQL(dbType, table, o)
}

// BuildDropUserSQL 转调 database.BuildDropUserSQL。
func BuildDropUserSQL(dbType database.DBType, name string, host string) (string, error) {
	return database.BuildDropUserSQL(dbType, name, host)
}

// BuildGrantSQL 转调 database.BuildGrantSQL。
func BuildGrantSQL(dbType database.DBType, privs []string, scope database.PrivScope, name string, host string, withGrant bool) (string, error) {
	return database.BuildGrantSQL(dbType, privs, scope, name, host, withGrant)
}

// BuildRenameDatabaseSQL 转调 database.BuildRenameDatabaseSQL。
func BuildRenameDatabaseSQL(dbType database.DBType, oldName string, newName string) (string, error) {
	return database.BuildRenameDatabaseSQL(dbType, oldName, newName)
}

// BuildRevokeSQL 转调 database.BuildRevokeSQL。
func BuildRevokeSQL(dbType database.DBType, privs []string, scope database.PrivScope, name string, host string) (string, error) {
	return database.BuildRevokeSQL(dbType, privs, scope, name, host)
}

// BuildTruncateTableSQL 转调 database.BuildTruncateTableSQL。
func BuildTruncateTableSQL(dbType database.DBType, table string, o database.TableOpOptions) string {
	return database.BuildTruncateTableSQL(dbType, table, o)
}

// CapsOf 转调 database.CapsOf。
func CapsOf(t database.DBType) database.Caps { return database.CapsOf(t) }

// DialectOf 转调 database.DialectOf。
func DialectOf(t database.DBType) (database.Dialect, error) { return database.DialectOf(t) }

// IsMySQLCompatible 转调 database.IsMySQLCompatible。
func IsMySQLCompatible(dbType database.DBType) bool { return database.IsMySQLCompatible(dbType) }

// ParseOnUpdate 转调 database.ParseOnUpdate。
func ParseOnUpdate(extra string) string { return database.ParseOnUpdate(extra) }

// PreviewChanges 转调 database.PreviewChanges。
func PreviewChanges(dbType database.DBType, dbName string, table string, deletePKs []map[string]interface{}, inserts []map[string]interface{}, updates []database.RowChange) []string {
	return database.PreviewChanges(dbType, dbName, table, deletePKs, inserts, updates)
}

// PrivilegesFor 转调 database.PrivilegesFor。
func PrivilegesFor(dbType database.DBType, scope database.PrivScope) []string {
	return database.PrivilegesFor(dbType, scope)
}

// QuoteIdent 转调 database.QuoteIdent。
func QuoteIdent(dbType database.DBType, name string) string { return database.QuoteIdent(dbType, name) }

// QuoteString 转调 database.QuoteString。
func QuoteString(dbType database.DBType, s string) string { return database.QuoteString(dbType, s) }

// SQLLeadingVerb 转调 database.SQLLeadingVerb。
func SQLLeadingVerb(sql string) string { return database.SQLLeadingVerb(sql) }

// SSLModesOf 转调 database.SSLModesOf。
func SSLModesOf(t database.DBType) []string { return database.SSLModesOf(t) }

// SafeModeRequirement 转调 database.SafeModeRequirement。
func SafeModeRequirement(mode database.SafeMode, hasPassword bool, stmts []string) database.SafeAction {
	return database.SafeModeRequirement(mode, hasPassword, stmts)
}

// SplitForeignKey 转调 database.SplitForeignKey。
func SplitForeignKey(ref string) (string, string) { return database.SplitForeignKey(ref) }

// SupportsProcessList 转调 database.SupportsProcessList。
func SupportsProcessList(dbType database.DBType) bool { return database.SupportsProcessList(dbType) }

// SupportsUsers 转调 database.SupportsUsers。
func SupportsUsers(dbType database.DBType) bool { return database.SupportsUsers(dbType) }

// TableOpSupport 转调 database.TableOpSupport。
func TableOpSupport(dbType database.DBType) (bool, bool, bool) {
	return database.TableOpSupport(dbType)
}
