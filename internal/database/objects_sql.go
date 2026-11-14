package database

import (
	"fmt"
	"strings"
)

// 外键与触发器的 SQL 生成。

func validFKAction(a string) (string, error) {
	a = strings.ToUpper(strings.TrimSpace(a))
	if a == "" {
		return "NO ACTION", nil
	}
	for _, v := range FKActions {
		if v == a {
			return a, nil
		}
	}
	return "", fmt.Errorf("不支持的外键动作: %s", a)
}

func quoteList(dbType DBType, names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = QuoteIdent(dbType, n)
	}
	return strings.Join(q, ", ")
}

// BuildAddForeignKeySQL 生成添加外键的语句。SQLite 不能给已有的表加外键。
func BuildAddForeignKeySQL(dbType DBType, table string, fk ForeignKeyInfo) (string, error) {
	if dbType == SQLite {
		return "", fmt.Errorf("SQLite 不支持给已有的表添加外键，需要重建表")
	}
	if dbType == StarRocks {
		return "", fmt.Errorf("StarRocks 不支持外键")
	}
	if strings.TrimSpace(fk.Name) == "" || len(fk.Columns) == 0 || fk.RefTable == "" || len(fk.RefColumns) != len(fk.Columns) {
		return "", fmt.Errorf("外键需要名称、列、被引用的表和与列数相同的被引用列")
	}
	upd, err := validFKAction(fk.OnUpdate)
	if err != nil {
		return "", err
	}
	del, err := validFKAction(fk.OnDelete)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s) ON UPDATE %s ON DELETE %s",
		QuoteTableName(dbType, table), QuoteIdent(dbType, fk.Name), quoteList(dbType, fk.Columns),
		QuoteTableName(dbType, fk.RefTable), quoteList(dbType, fk.RefColumns), upd, del), nil
}

// BuildDropForeignKeySQL 生成删除外键的语句。
func BuildDropForeignKeySQL(dbType DBType, table, name string) (string, error) {
	switch {
	case dbType == SQLite:
		return "", fmt.Errorf("SQLite 不支持删除外键，需要重建表")
	case dbType == StarRocks:
		return "", fmt.Errorf("StarRocks 不支持外键")
	case dbType == Postgres:
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", QuoteTableName(dbType, table), QuoteIdent(dbType, name)), nil
	}
	return fmt.Sprintf("ALTER TABLE %s DROP FOREIGN KEY %s", QuoteTableName(dbType, table), QuoteIdent(dbType, name)), nil
}
