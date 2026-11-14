package database

import (
	"fmt"
	"minidb/internal/model"
	"regexp"
	"strings"
)

func buildColumnClause(dbType DBType, col StructureColumnEdit, prefix string) string {
	ddl := prefix + " " + col.Type
	dv := ""
	if col.DefaultValue != nil {
		dv = strings.TrimSpace(*col.DefaultValue)
	}
	if !col.Nullable {
		ddl += " NOT NULL"
		if dv != "" && dv != "NULL" {
			ddl += " DEFAULT " + dv
		}
	} else {
		ddl += " NULL"
		if dv != "" && dv != "NULL" {
			ddl += " DEFAULT " + dv
		}
	}
	if e, ok := hookOf[ColumnClauseExtender](dbType); ok {
		ddl += e.ColumnClauseExtras(col)
	}
	return ddl
}

var onUpdateRe = regexp.MustCompile(`(?i)on update\s+(\w+(?:\(\d*\))?)`)

// ParseOnUpdate 从 MySQL information_schema 的 EXTRA 里取出 ON UPDATE 的表达式，没有则返回空串。
func ParseOnUpdate(extra string) string {
	if m := onUpdateRe.FindStringSubmatch(extra); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

// mysqlColumnPlacement 算出列顺序变化需要的 AFTER / FIRST 子句。
// 已有列里取原顺序的最长递增子序列当作"没动"，其余的才挪；新增列只在后面还有已有列、
// 或本次有列要挪的时候才带位置，否则照旧追加在末尾（生成的 SQL 保持原样）。
// 返回值：after 是列 UID 到位置子句的映射，moved 是需要 CHANGE 挪位的已有列。
func mysqlColumnPlacement(dbType DBType, working, original []StructureColumnEdit) (after map[string]string, moved map[string]bool) {
	after, moved = map[string]string{}, map[string]bool{}
	var kept []StructureColumnEdit // 最终留在表里的列，按目标顺序
	for _, w := range working {
		if w.Status == "deleted" {
			continue
		}
		if w.Status == "new" && (strings.TrimSpace(w.Name) == "" || strings.TrimSpace(w.Type) == "") {
			continue
		}
		kept = append(kept, w)
	}
	origPos := map[string]int{}
	for i, o := range original {
		origPos[o.UID] = i
	}
	var existing []int // kept 里已有列的下标
	for i, k := range kept {
		if _, ok := origPos[k.UID]; ok && k.Status != "new" {
			existing = append(existing, i)
		}
	}
	// 最长递增子序列：best[i] 是以第 i 个已有列结尾的最长长度
	n := len(existing)
	best, prev := make([]int, n), make([]int, n)
	end := -1
	for i := 0; i < n; i++ {
		best[i], prev[i] = 1, -1
		for j := 0; j < i; j++ {
			if origPos[kept[existing[j]].UID] < origPos[kept[existing[i]].UID] && best[j]+1 > best[i] {
				best[i], prev[i] = best[j]+1, j
			}
		}
		if end < 0 || best[i] > best[end] {
			end = i
		}
	}
	stay := map[string]bool{}
	for i := end; i >= 0; i = prev[i] {
		stay[kept[existing[i]].UID] = true
	}
	for _, i := range existing {
		if !stay[kept[i].UID] {
			moved[kept[i].UID] = true
		}
	}
	lastExisting := -1
	if n > 0 {
		lastExisting = existing[n-1]
	}
	for i, k := range kept {
		isNew := k.Status == "new"
		if !moved[k.UID] && !(isNew && (len(moved) > 0 || i < lastExisting)) {
			continue
		}
		if i == 0 {
			after[k.UID] = " FIRST"
		} else {
			after[k.UID] = " AFTER " + QuoteIdent(dbType, strings.TrimSpace(kept[i-1].Name))
		}
	}
	return after, moved
}

// buildMySQLFamilyAlterTableSQL MySQL/TiDB/StarRocks 单条 ALTER 多子句
func buildMySQLFamilyAlterTableSQL(dbType DBType, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) (string, error) {
	if tableName == "" {
		return "", model.Errf(model.CodeTableNameEmpty, "表名不能为空")
	}
	tq := QuoteTableName(dbType, tableName)
	var sqlParts []string
	after, moved := mysqlColumnPlacement(dbType, workingCols, originalCols)

	for _, w := range workingCols {
		switch w.Status {
		case "new":
			if strings.TrimSpace(w.Name) == "" || strings.TrimSpace(w.Type) == "" {
				continue
			}
			prefix := "ADD COLUMN " + QuoteIdent(dbType, strings.TrimSpace(w.Name))
			sqlParts = append(sqlParts, buildColumnClause(dbType, w, prefix)+after[w.UID])
		case "deleted":
			orig := findColByUID(originalCols, w.UID)
			if orig != nil {
				sqlParts = append(sqlParts, "DROP COLUMN "+QuoteIdent(dbType, orig.Name))
			}
		default:
			orig := findColByUID(originalCols, w.UID)
			if orig == nil {
				continue
			}
			changed := w.Name != orig.Name || w.Type != orig.Type || w.Nullable != orig.Nullable ||
				structureDefaultStr(w.DefaultValue) != structureDefaultStr(orig.DefaultValue) || w.Comment != orig.Comment
			if !changed && !moved[w.UID] {
				continue
			}
			prefix := fmt.Sprintf("CHANGE COLUMN %s %s", QuoteIdent(dbType, orig.Name), QuoteIdent(dbType, strings.TrimSpace(w.Name)))
			sqlParts = append(sqlParts, buildColumnClause(dbType, w, prefix)+after[w.UID])
		}
	}

	for _, idx := range workingIndexes {
		switch idx.Status {
		case "new":
			indexName := strings.TrimSpace(idx.Name)
			var cols []string
			for _, c := range idx.Columns {
				t := strings.TrimSpace(c)
				if t != "" {
					cols = append(cols, QuoteIdent(dbType, t))
				}
			}
			if indexName == "" || len(cols) == 0 {
				continue
			}
			uniqueStr := ""
			if idx.IsUnique {
				uniqueStr = "UNIQUE "
			}
			part := fmt.Sprintf("ADD %sINDEX %s (%s)", uniqueStr, QuoteIdent(dbType, indexName), strings.Join(cols, ", "))
			sqlParts = append(sqlParts, part)
		case "deleted":
			if !idx.IsPrimary && strings.TrimSpace(idx.Name) != "" {
				sqlParts = append(sqlParts, "DROP INDEX "+QuoteIdent(dbType, strings.TrimSpace(idx.Name)))
			}
		}
	}

	for _, o := range originalCols {
		if findColByUID(workingCols, o.UID) == nil {
			sqlParts = append(sqlParts, "DROP COLUMN "+QuoteIdent(dbType, o.Name))
		}
	}

	if len(sqlParts) == 0 {
		return "", nil
	}
	return fmt.Sprintf("ALTER TABLE %s %s", tq, strings.Join(sqlParts, ", ")), nil
}

// expandModifiedIndexes 把被改过的已有索引（Status 为 "modified"，UID 是原名）
// 拆成「删掉原名的旧索引」加「按新名字新建」，各方言的生成逻辑只需处理 new / deleted。
func expandModifiedIndexes(in []StructureIndexEdit) []StructureIndexEdit {
	out := make([]StructureIndexEdit, 0, len(in))
	for _, ix := range in {
		if ix.Status != "modified" {
			out = append(out, ix)
			continue
		}
		if !validIndexEdit(ix) {
			continue // 名称或列被清空了：保持原样，不能因为改坏了就把旧索引删掉
		}
		old := ix
		old.Status, old.Name = "deleted", ix.UID
		fresh := ix
		fresh.Status = "new"
		out = append(out, old, fresh)
	}
	return out
}

// validIndexEdit 报告索引编辑有没有名称和至少一个列，够不够建出一个索引。
func validIndexEdit(ix StructureIndexEdit) bool {
	if strings.TrimSpace(ix.Name) == "" {
		return false
	}
	for _, c := range ix.Columns {
		if strings.TrimSpace(c) != "" {
			return true
		}
	}
	return false
}

// BuildStructureAlterDDLStatements 按数据库类型与版本生成 1 条或多条 DDL，调用方按顺序执行
func BuildStructureAlterDDLStatements(dbType DBType, serverVersion, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) ([]string, error) {
	if tableName == "" {
		return nil, model.Errf(model.CodeTableNameEmpty, "表名不能为空")
	}
	if err := ValidateStructureAlterSupported(dbType, serverVersion); err != nil {
		return nil, err
	}
	workingIndexes = expandModifiedIndexes(workingIndexes)
	a, ok := hookOf[StructureAlterer](dbType)
	if !ok {
		return nil, fmt.Errorf("不支持的结构变更方言: %s", dbType)
	}
	return a.StructureAlterSQL(serverVersion, tableName, workingCols, originalCols, workingIndexes)
}

func findColByUID(cols []StructureColumnEdit, uid string) *StructureColumnEdit {
	for i := range cols {
		if cols[i].UID == uid {
			return &cols[i]
		}
	}
	return nil
}

func structureDefaultStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// BuildAddIndexSQL 按方言生成创建索引语句（PostgreSQL/SQLite 使用 CREATE INDEX，MySQL 系使用 ALTER TABLE ADD INDEX）
func BuildAddIndexSQL(dbType DBType, tableName, indexName string, columns []string, unique bool) (string, error) {
	if tableName == "" || indexName == "" {
		return "", fmt.Errorf("表名或索引名不能为空")
	}
	var cols []string
	for _, c := range columns {
		t := strings.TrimSpace(c)
		if t != "" {
			cols = append(cols, QuoteIdent(dbType, t))
		}
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("索引列不能为空")
	}
	w, ok := hookOf[IndexSQLWriter](dbType)
	if !ok {
		return "", fmt.Errorf("BuildAddIndexSQL 不支持的数据库类型: %s", dbType)
	}
	return w.AddIndexSQL(tableName, indexName, cols, unique), nil
}

// BuildDropIndexSQL 按方言删除索引
func BuildDropIndexSQL(dbType DBType, tableName, indexName string) (string, error) {
	if tableName == "" || indexName == "" {
		return "", fmt.Errorf("表名或索引名不能为空")
	}
	w, ok := hookOf[IndexSQLWriter](dbType)
	if !ok {
		return "", fmt.Errorf("BuildDropIndexSQL 不支持的数据库类型: %s", dbType)
	}
	return w.DropIndexSQL(tableName, indexName), nil
}
