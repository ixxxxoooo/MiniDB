package database

import (
	"fmt"
	"strings"
)

// StructureColumnEdit 结构编辑：列工作态（与前端 Structure 视图对齐，JSON 由 Wails 绑定）
type StructureColumnEdit struct {
	UID          string  `json:"uid"`
	Status       string  `json:"status"` // "", "new", "deleted", "modified"
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Nullable     bool    `json:"nullable"`
	DefaultValue *string `json:"defaultValue"`
	Comment      string  `json:"comment"`
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
	if col.Comment != "" {
		if IsMySQLCompatible(dbType) {
			ddl += fmt.Sprintf(" COMMENT '%s'", escapeMySQLStringLiteral(col.Comment))
		}
	}
	return ddl
}

// buildMySQLFamilyAlterTableSQL MySQL/TiDB/StarRocks 单条 ALTER 多子句
func buildMySQLFamilyAlterTableSQL(dbType, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) (string, error) {
	if tableName == "" {
		return "", fmt.Errorf("表名不能为空")
	}
	tq := QuoteTableName(dbType, tableName)
	var sqlParts []string

	for _, w := range workingCols {
		switch w.Status {
		case "new":
			if strings.TrimSpace(w.Name) == "" || strings.TrimSpace(w.Type) == "" {
				continue
			}
			prefix := "ADD COLUMN " + QuoteIdent(dbType, strings.TrimSpace(w.Name))
			sqlParts = append(sqlParts, buildColumnClause(dbType, w, prefix))
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
			if !changed {
				continue
			}
			prefix := fmt.Sprintf("CHANGE COLUMN %s %s", QuoteIdent(dbType, orig.Name), QuoteIdent(dbType, strings.TrimSpace(w.Name)))
			sqlParts = append(sqlParts, buildColumnClause(dbType, w, prefix))
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

// BuildStructureAlterDDLStatements 按数据库类型与版本生成 1 条或多条 DDL，调用方按顺序执行
func BuildStructureAlterDDLStatements(dbType, serverVersion, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) ([]string, error) {
	if tableName == "" {
		return nil, fmt.Errorf("表名不能为空")
	}
	if err := ValidateStructureAlterSupported(dbType, serverVersion); err != nil {
		return nil, err
	}
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

// BuildAlterTableFromStructureDiff 兼容旧接口：仅适用于生成单条语句的方言；PostgreSQL/SQLite 请用 BuildStructureAlterDDLStatements
func BuildAlterTableFromStructureDiff(dbType, tableName string, workingCols, originalCols []StructureColumnEdit, workingIndexes []StructureIndexEdit) (string, error) {
	stmts, err := BuildStructureAlterDDLStatements(dbType, "", tableName, workingCols, originalCols, workingIndexes)
	if err != nil {
		return "", err
	}
	if len(stmts) == 0 {
		return "", nil
	}
	if len(stmts) > 1 {
		return "", fmt.Errorf("方言 %s 需要按顺序执行多条 DDL，请在服务层使用 BuildStructureAlterDDLStatements", dbType)
	}
	return stmts[0], nil
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
