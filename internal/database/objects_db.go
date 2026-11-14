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
	if CapsOf(dbType).LocalFile() {
		return "", fmt.Errorf("SQLite 的数据库就是文件，请新建连接")
	}
	if !CapsOf(dbType).Charset { // 不支持字符集的方言只写库名
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
	if CapsOf(dbType).LocalFile() {
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
	if CapsOf(dbType).LocalFile() {
		return "", fmt.Errorf("SQLite 的数据库就是文件，请重命名文件")
	}
	if r, ok := hookOf[DatabaseRenamer](dbType); ok {
		return r.RenameDatabaseSQL(oldName, newName), nil
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
	if CapsOf(dbType).LocalFile() {
		return nil, fmt.Errorf("SQLite 请用 CloneSQLiteTableSQL")
	}
	if c, ok := hookOf[TableCloner](dbType); ok {
		return c.CloneTableSQL(src, dst, withData), nil
	}
	return cloneWithInsert(dbType, src, dst, withData, "CREATE TABLE %s LIKE %s"), nil
}
