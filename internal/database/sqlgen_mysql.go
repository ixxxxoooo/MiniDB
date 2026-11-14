// @author ixxxxoooo

package database

import (
	"fmt"
	"strings"
)

// MySQL 系（MySQL、MariaDB、TiDB、StarRocks）的语句写法。TiDB、StarRocks 内嵌 mysqlDialect，
// 与它不同的方法（如 StarRocks 的重命名）在各自的文件里覆盖。

func (d mysqlDialect) RenameTableSQL(from, to string) string {
	return fmt.Sprintf("RENAME TABLE %s TO %s", QuoteTableName(d.kind, from), QuoteTableName(d.kind, to))
}

// ImportModifiers：忽略 → INSERT IGNORE；替换 → REPLACE；更新 → ON DUPLICATE KEY UPDATE。
func (d mysqlDialect) ImportModifiers(targets, keys []string, mode string) (string, string, error) {
	switch mode {
	case ConflictIgnore:
		return "INSERT IGNORE INTO", "", nil
	case ConflictReplace:
		return "REPLACE INTO", "", nil
	case ConflictUpdate:
		_, sets := conflictSets(d.kind, targets, keys, "%s = VALUES(%s)")
		if len(sets) == 0 {
			return "INSERT IGNORE INTO", "", nil
		}
		return "INSERT INTO", " ON DUPLICATE KEY UPDATE " + strings.Join(sets, ", "), nil
	}
	return "", "", fmt.Errorf("未知的冲突处理方式: %s", mode)
}

func (mysqlDialect) FullStatementPrefixes() []string { return fullStatementMySQLExtra }

// ColumnClauseExtras 给列定义追加 ON UPDATE、AUTO_INCREMENT 和列注释。
func (mysqlDialect) ColumnClauseExtras(col StructureColumnEdit) string {
	ddl := ""
	if col.OnUpdate != "" {
		ddl += " ON UPDATE " + col.OnUpdate
	}
	if col.AutoIncrement {
		ddl += " AUTO_INCREMENT"
	}
	if col.Comment != "" {
		ddl += " COMMENT " + QuoteString(MySQL, col.Comment)
	}
	return ddl
}

func (d mysqlDialect) StructureAlterSQL(_, table string, working, original []StructureColumnEdit, idx []StructureIndexEdit) ([]string, error) {
	s, err := buildMySQLFamilyAlterTableSQL(d.kind, table, working, original, idx)
	if err != nil || s == "" {
		return nil, err
	}
	return []string{s}, nil
}

func (d mysqlDialect) AddIndexSQL(table, index string, cols []string, unique bool) string {
	u := ""
	if unique {
		u = "UNIQUE "
	}
	return fmt.Sprintf("ALTER TABLE %s ADD %sINDEX %s (%s)",
		QuoteTableName(d.kind, table), u, QuoteIdent(d.kind, index), strings.Join(cols, ", "))
}

func (d mysqlDialect) DropIndexSQL(table, index string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP INDEX %s", QuoteTableName(d.kind, table), QuoteIdent(d.kind, index))
}

// conflictSets 返回判断重复的键（已引用）和要更新的列赋值；setFmt 是一个列的赋值写法，两个 %s 都是列名。
// 作为判断重复的键的列不参与更新。
func conflictSets(dbType DBType, targets, keys []string, setFmt string) (quotedKeys, sets []string) {
	isKey := map[string]bool{}
	for _, k := range keys {
		isKey[strings.ToLower(k)] = true
		quotedKeys = append(quotedKeys, QuoteIdent(dbType, k))
	}
	for _, col := range targets {
		if isKey[strings.ToLower(strings.Trim(col, "`\""))] {
			continue
		}
		sets = append(sets, fmt.Sprintf(setFmt, col, col))
	}
	return quotedKeys, sets
}
