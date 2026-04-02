package database

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SQLBuildContext 描述当前连接的数据库类型与服务器版本字符串，供生成方言正确 SQL 时使用
type SQLBuildContext struct {
	DBType         string // 与 ConnectionConfig.Type 一致：mysql, tidb, starrocks, postgres, sqlite
	ServerVersion  string // GetServerVersion 返回值，可能为空表示未知
}

// ParseMySQLMajorVersion 从 VERSION() 字符串解析主版本号，失败返回 0
func ParseMySQLMajorVersion(version string) int {
	// 8.0.33, 5.7.44-log, 10.5.18-MariaDB-...
	s := strings.TrimSpace(version)
	if s == "" {
		return 0
	}
	// MariaDB 常以 10.x 开头，按 10+ 视为「非 5.7 老语义」分支
	re := regexp.MustCompile(`^(\d+)`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// ParsePostgresMajorVersion 从 SELECT version() 文本解析主版本，失败返回 0
func ParsePostgresMajorVersion(version string) int {
	// PostgreSQL 14.5 on x86_64...
	re := regexp.MustCompile(`PostgreSQL\s+(\d+)`)
	m := re.FindStringSubmatch(version)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
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
func IsLikelyFullStatementForDialect(dbType, rawInput string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(rawInput))
	if trimmed == "" {
		return false
	}
	prefixes := append([]string{}, fullStatementCommonPrefixes...)
	switch {
	case dbType == "postgres":
		prefixes = append(prefixes, fullStatementPostgresExtra...)
	case dbType == "sqlite":
		prefixes = append(prefixes, fullStatementSQLiteExtra...)
	case IsMySQLCompatible(dbType):
		prefixes = append(prefixes, fullStatementMySQLExtra...)
	default:
		// 未知类型：仅使用通用前缀，降低误判风险
	}
	for _, p := range prefixes {
		if strings.HasPrefix(trimmed, p+" ") || trimmed == p {
			return true
		}
	}
	return false
}

// IsLikelyFullStatementInput 保留兼容：未指定类型时按最保守通用前缀判断
func IsLikelyFullStatementInput(rawInput string) bool {
	return IsLikelyFullStatementForDialect("", rawInput)
}

// ValidateStructureAlterSupported 若当前方言暂无法安全生成批量结构 DDL，返回错误（中文日志友好）
func ValidateStructureAlterSupported(dbType, serverVersion string) error {
	switch dbType {
	case "postgres", "sqlite", "mysql", "tidb", "starrocks":
		if dbType == "sqlite" && serverVersion != "" {
			v := ParseSQLiteNumericVersion(serverVersion)
			if v > 0 && v < 3_025_000 {
				return fmt.Errorf("当前 SQLite 版本过低（需 ≥3.25 才支持 RENAME COLUMN，≥3.35 才支持 DROP COLUMN），请升级或改用 SQL 编辑器执行 DDL")
			}
		}
		return nil
	default:
		return fmt.Errorf("不支持的数据库类型用于结构变更: %s", dbType)
	}
}
