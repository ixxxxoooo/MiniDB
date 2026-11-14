package database

import (
	"fmt"
	"regexp"
	"strings"
)

func buildSQLiteStructureStatements(serverVersion, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) ([]string, error) {
	const dt = SQLite
	tq := QuoteTableName(dt, tableName)
	var stmts []string

	for _, w := range workingCols {
		switch w.Status {
		case "new":
			if strings.TrimSpace(w.Name) == "" || strings.TrimSpace(w.Type) == "" {
				continue
			}
			tmp := w
			tmp.Comment = ""
			prefix := "ADD COLUMN " + QuoteIdent(dt, strings.TrimSpace(w.Name))
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s %s", tq, buildColumnClause(dt, tmp, prefix)))
		case "deleted":
			if !SQLiteSupportsDropColumn(serverVersion) {
				return nil, fmt.Errorf("当前 SQLite 版本不支持 DROP COLUMN（需 ≥3.35），请改用 SQL 编辑器")
			}
			orig := findColByUID(originalCols, w.UID)
			if orig != nil {
				stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tq, QuoteIdent(dt, orig.Name)))
			}
		default:
			orig := findColByUID(originalCols, w.UID)
			if orig == nil {
				continue
			}
			changed := w.Name != orig.Name || w.Type != orig.Type || w.Nullable != orig.Nullable ||
				structureDefaultStr(w.DefaultValue) != structureDefaultStr(orig.DefaultValue) || w.Comment != orig.Comment
			if !changed {
				continue
			}
			if orig.Type != w.Type || orig.Nullable != w.Nullable ||
				structureDefaultStr(orig.DefaultValue) != structureDefaultStr(w.DefaultValue) || orig.Comment != w.Comment {
				return nil, fmt.Errorf("SQLite 不支持在此直接修改列类型/默认值/注释，请使用 SQL 编辑器重建表")
			}
			if orig.Name != strings.TrimSpace(w.Name) {
				if !SQLiteSupportsRenameColumn(serverVersion) {
					return nil, fmt.Errorf("当前 SQLite 版本不支持 RENAME COLUMN（需 ≥3.25），请改用 SQL 编辑器")
				}
				stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
					tq, QuoteIdent(dt, orig.Name), QuoteIdent(dt, strings.TrimSpace(w.Name))))
			}
		}
	}

	for _, o := range originalCols {
		if findColByUID(workingCols, o.UID) == nil {
			if !SQLiteSupportsDropColumn(serverVersion) {
				return nil, fmt.Errorf("当前 SQLite 版本不支持 DROP COLUMN（需 ≥3.35），请改用 SQL 编辑器")
			}
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", tq, QuoteIdent(dt, o.Name)))
		}
	}

	for _, idx := range workingIndexes {
		switch idx.Status {
		case "deleted":
			if !idx.IsPrimary && strings.TrimSpace(idx.Name) != "" {
				stmts = append(stmts, fmt.Sprintf("DROP INDEX IF EXISTS %s", QuoteIdent(dt, strings.TrimSpace(idx.Name))))
			}
		case "new":
			indexName := strings.TrimSpace(idx.Name)
			var cols []string
			for _, c := range idx.Columns {
				t := strings.TrimSpace(c)
				if t != "" {
					cols = append(cols, QuoteIdent(dt, t))
				}
			}
			if indexName == "" || len(cols) == 0 {
				continue
			}
			u := ""
			if idx.IsUnique {
				u = "UNIQUE "
			}
			stmts = append(stmts, fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)",
				u, QuoteIdent(dt, indexName), tq, strings.Join(cols, ", ")))
		}
	}

	if len(stmts) == 0 {
		return nil, nil
	}
	return stmts, nil
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

// CloneTableFromDDL 让 SQLite 经 DDLCloner 接口复制表。
func (sqliteDialect) CloneTableFromDDL(ddl, src, dst string, withData bool) ([]string, error) {
	return CloneSQLiteTableSQL(ddl, src, dst, withData)
}
