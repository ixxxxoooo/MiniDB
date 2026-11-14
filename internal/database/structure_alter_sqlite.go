package database

import (
	"fmt"

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
