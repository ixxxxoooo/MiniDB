package database

import (
	"fmt"
	"regexp"
	"strings"
)

// StructureColumnEdit 结构编辑：列工作态（与结构视图里的工作副本对应）
type StructureColumnEdit struct {
	UID          string  `json:"uid"`
	Status       string  `json:"status"` // "", "new", "deleted", "modified"
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	DefaultValue *string `json:"defaultValue"`
	Comment      string  `json:"comment"`
	// 下面两项只有 MySQL 系用得上：改列、挪列时要把它们原样带回去，否则 CHANGE COLUMN 会悄悄丢掉
	AutoIncrement bool   `json:"autoIncrement,omitempty"`
	OnUpdate      string `json:"onUpdate,omitempty"` // 如 CURRENT_TIMESTAMP(3)，不含 ON UPDATE 关键字
}

// StructureIndexEdit 结构编辑：索引工作态
type StructureIndexEdit struct {
	UID       string   `json:"uid"`
	Status    string   `json:"status"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	IsUnique  bool     `json:"isUnique"`
	IsPrimary bool     `json:"isPrimary"`
	Columns   []string `json:"columns"`
}

// escapeMySQLStringLiteral MySQL 单引号字符串字面量转义（标准 SQL：单引号加倍）
func escapeMySQLStringLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func escapePostgresStringLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func buildColumnClause(dbType string, col StructureColumnEdit, prefix string) string {
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
	if IsMySQLCompatible(dbType) {
		if col.OnUpdate != "" {
			ddl += " ON UPDATE " + col.OnUpdate
		}
		if col.AutoIncrement {
			ddl += " AUTO_INCREMENT"
		}
		if col.Comment != "" {
			ddl += fmt.Sprintf(" COMMENT '%s'", escapeMySQLStringLiteral(col.Comment))
		}
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
func mysqlColumnPlacement(dbType string, working, original []StructureColumnEdit) (after map[string]string, moved map[string]bool) {
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
func buildMySQLFamilyAlterTableSQL(dbType, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("表名不能为空")
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
func BuildStructureAlterDDLStatements(dbType, serverVersion, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) ([]string, error) {
	if tableName == "" {
		return nil, fmt.Errorf("表名不能为空")
	}
	if err := ValidateStructureAlterSupported(dbType, serverVersion); err != nil {
		return nil, err
	}
	workingIndexes = expandModifiedIndexes(workingIndexes)
	switch {
	case IsMySQLCompatible(dbType):
		s, err := buildMySQLFamilyAlterTableSQL(dbType, tableName, workingCols, originalCols, workingIndexes)
		if err != nil {
			return nil, err
		}
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	case dbType == "postgres":
		return buildPostgresStructureStatements(tableName, workingCols, originalCols, workingIndexes)
	case dbType == "sqlite":
		return buildSQLiteStructureStatements(serverVersion, tableName, workingCols, originalCols, workingIndexes)
	default:
		return nil, fmt.Errorf("不支持的结构变更方言: %s", dbType)
	}
}

func buildPostgresStructureStatements(tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) ([]string, error) {
	const dt = "postgres"
	tq := QuoteTableName(dt, tableName)
	var alterParts []string
	var commentStmts []string

	appendComment := func(colName, comment string) {
		if comment == "" {
			return
		}
		qc := QuoteIdent(dt, colName)
		commentStmts = append(commentStmts, fmt.Sprintf(
			`COMMENT ON COLUMN %s.%s IS '%s'`,
			tq, qc, escapePostgresStringLiteral(comment),
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

func buildSQLiteStructureStatements(serverVersion, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) ([]string, error) {
	const dt = "sqlite"
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
func BuildAddIndexSQL(dbType, tableName, indexName string, columns []string, unique bool) (string, error) {
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
	tq := QuoteTableName(dbType, tableName)
	u := ""
	if unique {
		u = "UNIQUE "
	}
	switch dbType {
	case "postgres", "sqlite":
		return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)",
			u, QuoteIdent(dbType, indexName), tq, strings.Join(cols, ", ")), nil
	default:
		if IsMySQLCompatible(dbType) {
			return fmt.Sprintf("ALTER TABLE %s ADD %sINDEX %s (%s)", tq, u, QuoteIdent(dbType, indexName), strings.Join(cols, ", ")), nil
		}
		return "", fmt.Errorf("BuildAddIndexSQL 不支持的数据库类型: %s", dbType)
	}
}

// BuildDropIndexSQL 按方言删除索引
func BuildDropIndexSQL(dbType, tableName, indexName string) (string, error) {
	if tableName == "" || indexName == "" {
		return "", fmt.Errorf("表名或索引名不能为空")
	}
	switch dbType {
	case "postgres", "sqlite":
		return fmt.Sprintf("DROP INDEX IF EXISTS %s", QuoteIdent(dbType, indexName)), nil
	default:
		if IsMySQLCompatible(dbType) {
			tq := QuoteTableName(dbType, tableName)
			return fmt.Sprintf("ALTER TABLE %s DROP INDEX %s", tq, QuoteIdent(dbType, indexName)), nil
		}
		return "", fmt.Errorf("BuildDropIndexSQL 不支持的数据库类型: %s", dbType)
	}
}
