// @author ixxxxoooo

package database

import (
	"fmt"
	"strings"
)

// PostgreSQL 与 SQLite 的语句写法（两者在冲突处理、索引上写法相近，共用 conflictUpdate 等辅助）。

// conflictUpdate 生成 ON CONFLICT (...) DO UPDATE SET 子句；excluded 是引用新值的关键字（EXCLUDED / excluded）。
func conflictUpdate(dbType DBType, targets, keys []string, excluded string) (string, error) {
	quotedKeys, sets := conflictSets(dbType, targets, keys, "%s = "+excluded+".%s")
	if len(quotedKeys) == 0 {
		return "", fmt.Errorf("“更新”需要表有主键或指定判断重复的列")
	}
	if len(sets) == 0 {
		return " ON CONFLICT DO NOTHING", nil
	}
	return fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s", strings.Join(quotedKeys, ", "), strings.Join(sets, ", ")), nil
}

func createIndexSQL(dbType DBType, table, index string, cols []string, unique bool) string {
	u := ""
	if unique {
		u = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)",
		u, QuoteIdent(dbType, index), QuoteTableName(dbType, table), strings.Join(cols, ", "))
}

func dropIndexIfExistsSQL(dbType DBType, index string) string {
	return "DROP INDEX IF EXISTS " + QuoteIdent(dbType, index)
}

// ---- PostgreSQL ----

func (postgresDialect) ImportColumnType(kind string) string {
	return pickImportType(kind, "BIGINT", "DOUBLE PRECISION", "TEXT")
}

func (d postgresDialect) ImportModifiers(targets, keys []string, mode string) (string, string, error) {
	switch mode {
	case ConflictIgnore:
		return "INSERT INTO", " ON CONFLICT DO NOTHING", nil
	case ConflictReplace:
		return "", "", fmt.Errorf("%s 不支持“替换”，请改用“更新”", Postgres)
	case ConflictUpdate:
		s, err := conflictUpdate(Postgres, targets, keys, "EXCLUDED")
		return "INSERT INTO", s, err
	}
	return "", "", fmt.Errorf("未知的冲突处理方式: %s", mode)
}

func (postgresDialect) RenameDatabaseSQL(oldName, newName string) string {
	return fmt.Sprintf("ALTER DATABASE %s RENAME TO %s", QuoteIdent(Postgres, oldName), QuoteIdent(Postgres, newName))
}

func (postgresDialect) BinaryLiteral(upperHex string) string { return `'\x` + upperHex + `'` }

func (postgresDialect) CreateTriggerSQL(name, timing, event, table, body string) string {
	return fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
		QuoteIdent(Postgres, name), timing, event, QuoteTableName(Postgres, table), strings.TrimSuffix(body, "()"))
}

func (postgresDialect) DropTriggerSQL(table, name string) string {
	return fmt.Sprintf("DROP TRIGGER %s ON %s", QuoteIdent(Postgres, name), QuoteTableName(Postgres, table))
}

func (postgresDialect) DropForeignKeySQL(table, name string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", QuoteTableName(Postgres, table), QuoteIdent(Postgres, name))
}

func (postgresDialect) CloneTableSQL(src, dst string, withData bool) []string {
	return cloneWithInsert(Postgres, src, dst, withData, "CREATE TABLE %s (LIKE %s INCLUDING ALL)")
}

func (postgresDialect) TruncateTableSQL(table string, o TableOpOptions) string {
	out := "TRUNCATE TABLE " + QuoteTableName(Postgres, table)
	if o.RestartIdentity {
		out += " RESTART IDENTITY"
	}
	if o.Cascade {
		out += " CASCADE"
	}
	return out
}

// CreateColumnDef：自增列用 serial 系列类型；列注释要建表之后单独用 COMMENT ON 写。
func (postgresDialect) CreateColumnDef(c StructureColumnEdit, qn, tq string, _ int) (string, bool, []string, error) {
	if c.AutoIncrement {
		c.Type = serialType(c.Type)
	}
	var after []string
	if c.Comment != "" {
		after = append(after, fmt.Sprintf(`COMMENT ON COLUMN %s.%s IS %s`, tq, qn, QuoteString(Postgres, c.Comment)))
	}
	return buildColumnClause(Postgres, c, qn), false, after, nil
}

func (postgresDialect) StructureAlterSQL(_, table string, working, original []StructureColumnEdit, idx []StructureIndexEdit) ([]string, error) {
	return buildPostgresStructureStatements(table, working, original, idx)
}

func (postgresDialect) AddIndexSQL(table, index string, cols []string, unique bool) string {
	return createIndexSQL(Postgres, table, index, cols, unique)
}

func (postgresDialect) DropIndexSQL(_, index string) string {
	return dropIndexIfExistsSQL(Postgres, index)
}

func (postgresDialect) FullStatementPrefixes() []string { return fullStatementPostgresExtra }

// ---- SQLite ----

func (sqliteDialect) ImportColumnType(kind string) string {
	return pickImportType(kind, "INTEGER", "REAL", "TEXT")
}

func (sqliteDialect) ImportModifiers(targets, keys []string, mode string) (string, string, error) {
	switch mode {
	case ConflictIgnore:
		return "INSERT OR IGNORE INTO", "", nil
	case ConflictReplace:
		return "INSERT OR REPLACE INTO", "", nil
	case ConflictUpdate:
		s, err := conflictUpdate(SQLite, targets, keys, "excluded")
		return "INSERT INTO", s, err
	}
	return "", "", fmt.Errorf("未知的冲突处理方式: %s", mode)
}

func (sqliteDialect) CreateTriggerSQL(name, timing, event, table, body string) string {
	return fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW BEGIN %s; END",
		QuoteIdent(SQLite, name), timing, event, QuoteTableName(SQLite, table), strings.TrimSuffix(body, ";"))
}

// TruncateTableSQL：SQLite 没有 TRUNCATE，用 DELETE FROM。
func (sqliteDialect) TruncateTableSQL(table string, _ TableOpOptions) string {
	return "DELETE FROM " + QuoteTableName(SQLite, table)
}

// CreateColumnDef：自增列只能是唯一的主键，写成 INTEGER PRIMARY KEY AUTOINCREMENT。
func (sqliteDialect) CreateColumnDef(c StructureColumnEdit, qn, _ string, pkCount int) (string, bool, []string, error) {
	if !c.AutoIncrement {
		return buildColumnClause(SQLite, c, qn), false, nil, nil
	}
	if pkCount != 1 || !c.PrimaryKey {
		return "", false, nil, fmt.Errorf("SQLite 的自增列必须是唯一的主键")
	}
	return qn + " INTEGER PRIMARY KEY AUTOINCREMENT", true, nil, nil
}

func (sqliteDialect) StructureAlterSQL(serverVersion, table string, working, original []StructureColumnEdit, idx []StructureIndexEdit) ([]string, error) {
	return buildSQLiteStructureStatements(serverVersion, table, working, original, idx)
}

func (sqliteDialect) AddIndexSQL(table, index string, cols []string, unique bool) string {
	return createIndexSQL(SQLite, table, index, cols, unique)
}

func (sqliteDialect) DropIndexSQL(_, index string) string { return dropIndexIfExistsSQL(SQLite, index) }

func (sqliteDialect) FullStatementPrefixes() []string { return fullStatementSQLiteExtra }

// CheckStructureAlter：RENAME COLUMN 要 3.25 以上，DROP COLUMN 要 3.35 以上。
func (sqliteDialect) CheckStructureAlter(serverVersion string) error {
	if serverVersion == "" {
		return nil
	}
	if v := ParseSQLiteNumericVersion(serverVersion); v > 0 && v < 3_025_000 {
		return fmt.Errorf("当前 SQLite 版本过低（需 ≥3.25 才支持 RENAME COLUMN，≥3.35 才支持 DROP COLUMN），请升级或改用 SQL 编辑器执行 DDL")
	}
	return nil
}
