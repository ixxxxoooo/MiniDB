package database

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SQLBuildContext 描述当前连接的数据库类型与服务器版本字符串，供生成方言正确 SQL 时使用
type SQLBuildContext struct {
	DBType        string // 与 ConnectionConfig.Type 一致：mysql, tidb, starrocks, postgres, sqlite
	ServerVersion string // GetServerVersion 返回值，可能为空表示未知
}

// ParseSQLiteNumericVersion 从 sqlite_version() 或带前缀的字符串解析为 3040500 形式（major*1000000+minor*1000+patch），失败返回 0
func ParseSQLiteNumericVersion(version string) int {
	s := strings.TrimSpace(version)
	s = strings.TrimPrefix(s, "SQLite ")
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(s)
	if len(m) < 4 {
		return 0
	}
	maj, _ := strconv.Atoi(m[1])
	min, _ := strconv.Atoi(m[2])
	pat, _ := strconv.Atoi(m[3])
	return maj*1_000_000 + min*1000 + pat
}

// SQLiteSupportsDropColumn 3.35.0 起支持 DROP COLUMN
func SQLiteSupportsDropColumn(serverVersion string) bool {
	return ParseSQLiteNumericVersion(serverVersion) >= 3_035_000
}

// SQLiteSupportsRenameColumn 3.25.0 起支持 RENAME COLUMN
func SQLiteSupportsRenameColumn(serverVersion string) bool {
	return ParseSQLiteNumericVersion(serverVersion) >= 3_025_000
}

// fullStatementCommonPrefixes 各库共用的「整句 SQL」前缀（大写比较）
var fullStatementCommonPrefixes = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE",
	"ALTER", "DROP", "SHOW", "DESCRIBE", "DESC", "EXPLAIN", "WITH", "MERGE",
	"TRUNCATE", "REPLACE", "GRANT", "REVOKE", "CALL", "EXEC", "EXECUTE",
}

// fullStatementMySQLExtra MySQL 系额外前缀
var fullStatementMySQLExtra = []string{
	"USE", "SET", "BEGIN", "START", "COMMIT", "ROLLBACK", "LOCK", "UNLOCK", "SAVEPOINT", "XA",
	"ANALYZE", "OPTIMIZE", "CHECK", "REPAIR", "BACKUP", "RESTORE",
}

// fullStatementPostgresExtra PostgreSQL 额外前缀
var fullStatementPostgresExtra = []string{
	"COPY", "LISTEN", "NOTIFY", "DO", "VALUES", "REFRESH", "REINDEX", "VACUUM", "CLUSTER",
}

// fullStatementSQLiteExtra SQLite 额外前缀
var fullStatementSQLiteExtra = []string{
	"PRAGMA", "ATTACH", "DETACH", "REINDEX", "VACUUM", "ANALYZE",
}

// IsLikelyFullStatementForDialect 按数据库类型扩展「整句 SQL」识别，避免把 PRAGMA/USE 等误判为 WHERE 片段
func IsLikelyFullStatementForDialect(dbType DBType, rawInput string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(rawInput))
	if trimmed == "" {
		return false
	}
	prefixes := append([]string{}, fullStatementCommonPrefixes...)
	if h, ok := hookOf[FullStatementPrefixer](dbType); ok {
		prefixes = append(prefixes, h.FullStatementPrefixes()...)
	}
	for _, p := range prefixes {
		if strings.HasPrefix(trimmed, p+" ") || trimmed == p {
			return true
		}
	}
	return false
}

// ValidateStructureAlterSupported 若当前方言暂无法安全生成批量结构 DDL，返回错误（中文日志友好）
func ValidateStructureAlterSupported(dbType DBType, serverVersion string) error {
	if !dbType.Valid() {
		return fmt.Errorf("不支持的数据库类型用于结构变更: %s", dbType)
	}
	if !CapsOf(dbType).AlterTable {
		return fmt.Errorf("%s 不支持在界面里修改表结构，请用 SQL 编辑器执行 DDL", dbType)
	}
	if c, ok := hookOf[StructureAlterChecker](dbType); ok {
		return c.CheckStructureAlter(serverVersion)
	}
	return nil
}

// GetColumnTypes 返回指定数据库引擎支持的字段类型列表；未知类型按 MySQL 处理。
func GetColumnTypes(dbType DBType) []string {
	if d, ok := dialects[dbType]; ok {
		return d.ColumnTypes()
	}
	return dialects[DefaultDBType()].ColumnTypes()
}
