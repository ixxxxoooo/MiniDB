package database

import (
	"fmt"

	"strings"
)

func buildPostgresStructureStatements(tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) ([]string, error) {
	const dt = Postgres
	tq := QuoteTableName(dt, tableName)
	var alterParts []string
	var commentStmts []string

	appendComment := func(colName, comment string) {
		if comment == "" {
			return
		}
		qc := QuoteIdent(dt, colName)
		commentStmts = append(commentStmts, fmt.Sprintf(
			`COMMENT ON COLUMN %s.%s IS %s`,
			tq, qc, QuoteString(Postgres, comment),
		))
	}

	for _, w := range workingCols {
		switch w.Status {
		case "new":
			if strings.TrimSpace(w.Name) == "" || strings.TrimSpace(w.Type) == "" {
				continue
			}
			prefix := "ADD COLUMN " + QuoteIdent(dt, strings.TrimSpace(w.Name))
			// PostgreSQL ADD 不使用 MySQL 的 COMMENT 内联
			tmp := w
			tmp.Comment = ""
			alterParts = append(alterParts, buildColumnClause(dt, tmp, prefix))
			if w.Comment != "" {
				appendComment(strings.TrimSpace(w.Name), w.Comment)
			}
		case "deleted":
			orig := findColByUID(originalCols, w.UID)
			if orig != nil {
				alterParts = append(alterParts, "DROP COLUMN "+QuoteIdent(dt, orig.Name))
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
			finalName := strings.TrimSpace(w.Name)
			if orig.Name != finalName {
				alterParts = append(alterParts, fmt.Sprintf("RENAME COLUMN %s TO %s",
					QuoteIdent(dt, orig.Name), QuoteIdent(dt, finalName)))
			}
			if orig.Type != w.Type {
				alterParts = append(alterParts, fmt.Sprintf("ALTER COLUMN %s TYPE %s",
					QuoteIdent(dt, finalName), w.Type))
			}
			if orig.Nullable != w.Nullable {
				if w.Nullable {
					alterParts = append(alterParts, fmt.Sprintf("ALTER COLUMN %s DROP NOT NULL", QuoteIdent(dt, finalName)))
				} else {
					alterParts = append(alterParts, fmt.Sprintf("ALTER COLUMN %s SET NOT NULL", QuoteIdent(dt, finalName)))
				}
			}
			if structureDefaultStr(orig.DefaultValue) != structureDefaultStr(w.DefaultValue) {
				dv := ""
				if w.DefaultValue != nil {
					dv = strings.TrimSpace(*w.DefaultValue)
				}
				if dv == "" || dv == "NULL" {
					alterParts = append(alterParts, fmt.Sprintf("ALTER COLUMN %s DROP DEFAULT", QuoteIdent(dt, finalName)))
				} else {
					alterParts = append(alterParts, fmt.Sprintf("ALTER COLUMN %s SET DEFAULT %s", QuoteIdent(dt, finalName), dv))
				}
			}
			if orig.Comment != w.Comment {
				if w.Comment != "" {
					appendComment(finalName, w.Comment)
				} else {
					commentStmts = append(commentStmts, fmt.Sprintf("COMMENT ON COLUMN %s.%s IS NULL", tq, QuoteIdent(dt, finalName)))
				}
			}
		}
	}

	for _, o := range originalCols {
		if findColByUID(workingCols, o.UID) == nil {
			alterParts = append(alterParts, "DROP COLUMN "+QuoteIdent(dt, o.Name))
		}
	}

	var dropIdx []string
	var createIdx []string
	for _, idx := range workingIndexes {
		switch idx.Status {
		case "deleted":
			if !idx.IsPrimary && strings.TrimSpace(idx.Name) != "" {
				dropIdx = append(dropIdx, fmt.Sprintf("DROP INDEX IF EXISTS %s", QuoteIdent(dt, strings.TrimSpace(idx.Name))))
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
			createIdx = append(createIdx, fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
				u, QuoteIdent(dt, indexName), tq, strings.Join(cols, ", ")))
		}
	}

	var out []string
	if len(alterParts) > 0 {
		out = append(out, fmt.Sprintf("ALTER TABLE %s %s", tq, strings.Join(alterParts, ", ")))
	}
	out = append(out, commentStmts...)
	out = append(out, dropIdx...)
	out = append(out, createIdx...)
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
