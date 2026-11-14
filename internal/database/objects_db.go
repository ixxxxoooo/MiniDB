package database

import (
	"fmt"
	"regexp"
	"strings"
)

// ---- 库级操作 ----

// BuildCreateDatabaseSQL 生成创建数据库的语句。charset 只用于 MySQL 系，空则用 utf8mb4。
func BuildCreateDatabaseSQL(dbType DBType, name, charset string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("数据库名不能为空")
	}
	switch {
	case dbType == SQLite:
		return "", fmt.Errorf("SQLite 的数据库就是文件，请新建连接")
	case dbType == Postgres:
		return "CREATE DATABASE " + QuoteIdent(dbType, name), nil
	case dbType == StarRocks:
		return "CREATE DATABASE " + QuoteIdent(dbType, name), nil
	}
	if charset == "" {
		charset = "utf8mb4"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(charset) {
		return "", fmt.Errorf("字符集名不合法: %s", charset)
	}
	return fmt.Sprintf("CREATE DATABASE %s CHARACTER SET %s", QuoteIdent(dbType, name), charset), nil
}

// BuildDropDatabaseSQL 生成删除数据库的语句。
func BuildDropDatabaseSQL(dbType DBType, name string) (string, error) {
	if dbType == SQLite {
		return "", fmt.Errorf("SQLite 的数据库就是文件，请删除文件")
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("数据库名不能为空")
	}
	return "DROP DATABASE " + QuoteIdent(dbType, name), nil
}

// BuildRenameDatabaseSQL 生成重命名数据库的语句。MySQL 与 TiDB 没有这个语句，返回的是
// 空，由调用方把表逐个搬到新库。
func BuildRenameDatabaseSQL(dbType DBType, oldName, newName string) (string, error) {
	if strings.TrimSpace(newName) == "" || oldName == newName {
		return "", fmt.Errorf("新的数据库名不能为空，也不能与原名相同")
	}
	switch dbType {
	case Postgres:
		return fmt.Sprintf("ALTER DATABASE %s RENAME TO %s", QuoteIdent(dbType, oldName), QuoteIdent(dbType, newName)), nil
	case StarRocks:
		return fmt.Sprintf("ALTER DATABASE %s RENAME %s", QuoteIdent(dbType, oldName), QuoteIdent(dbType, newName)), nil
	case SQLite:
		return "", fmt.Errorf("SQLite 的数据库就是文件，请重命名文件")
	}
	return "", nil
}

// BuildMoveTablesSQL 生成把表从旧库搬到新库的 RENAME TABLE（MySQL 与 TiDB）。
func BuildMoveTablesSQL(dbType DBType, oldDB, newDB string, tables []string) string {
	if len(tables) == 0 {
		return ""
	}
	parts := make([]string, len(tables))
	for i, t := range tables {
		parts[i] = fmt.Sprintf("%s.%s TO %s.%s", QuoteIdent(dbType, oldDB), QuoteIdent(dbType, t), QuoteIdent(dbType, newDB), QuoteIdent(dbType, t))
	}
	return "RENAME TABLE " + strings.Join(parts, ", ")
}

// ---- 表克隆 ----

// BuildCloneTableSQL 生成复制表结构（可连数据）的语句。SQLite 要从 DDL 改名，走 CloneSQLiteTableSQL。
func BuildCloneTableSQL(dbType DBType, src, dst string, withData bool) ([]string, error) {
	if strings.TrimSpace(dst) == "" || src == dst {
		return nil, fmt.Errorf("新表名不能为空，也不能与原表同名")
	}
	s, d := QuoteTableName(dbType, src), QuoteTableName(dbType, dst)
	var out []string
	switch {
	case dbType == SQLite:
		return nil, fmt.Errorf("SQLite 请用 CloneSQLiteTableSQL")
	case dbType == Postgres:
		out = append(out, fmt.Sprintf("CREATE TABLE %s (LIKE %s INCLUDING ALL)", d, s))
	default:
		out = append(out, fmt.Sprintf("CREATE TABLE %s LIKE %s", d, s))
	}
	if withData {
		out = append(out, fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", d, s))
	}
	return out, nil
}

// CloneSQLiteTableSQL 把 SQLite 的 CREATE TABLE 语句改成新表名，并按需附上复制数据的语句。
func CloneSQLiteTableSQL(ddl, src, dst string, withData bool) ([]string, error) {
	if strings.TrimSpace(dst) == "" || src == dst {
		return nil, fmt.Errorf("新表名不能为空，也不能与原表同名")
	}
	re := regexp.MustCompile("(?is)^\\s*CREATE\\s+TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?(?:\"" + regexp.QuoteMeta(strings.ReplaceAll(src, `"`, `""`)) + "\"|`" + regexp.QuoteMeta(src) + "`|\\[" + regexp.QuoteMeta(src) + "\\]|'" + regexp.QuoteMeta(src) + "'|" + regexp.QuoteMeta(src) + ")")
	loc := re.FindStringIndex(ddl)
	if loc == nil {
		return nil, fmt.Errorf("无法从建表语句里识别表名")
	}
	out := []string{"CREATE TABLE " + QuoteIdent(SQLite, dst) + ddl[loc[1]:]}
	if withData {
		out = append(out, fmt.Sprintf("INSERT INTO %s SELECT * FROM %s", QuoteIdent(SQLite, dst), QuoteIdent(SQLite, src)))
	}
	return out, nil
}
