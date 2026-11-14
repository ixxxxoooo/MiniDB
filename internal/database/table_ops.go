package database

import (
	"fmt"
	"minidb/internal/model"
	"strings"
)

// 表级操作的 SQL：新建、重命名、清空、删除。清空和删除的可选项（Cascade、Restart identity）
// 只有部分数据库支持，TableOpSupport 说明某个库能给哪些。

// TableOpSupport 报告 dbType 在清空、删除表时支持哪些可选项。
//
// 参数：dbType 库类型。
// 返回值：truncateCascade 清空是否支持 Cascade；restartIdentity 清空是否支持 Restart identity；dropCascade 删除是否支持 Cascade。
func TableOpSupport(dbType DBType) (truncateCascade, restartIdentity, dropCascade bool) {
	c := CapsOf(dbType)
	return c.TruncateCascade, c.RestartIdentity, c.DropCascade
}

// BuildTruncateTableSQL 生成清空表的语句；SQLite 没有 TRUNCATE，由方言改写成 DELETE FROM。
// 选项只有方言声明支持（Caps().RestartIdentity / TruncateCascade）才生效。
func BuildTruncateTableSQL(dbType DBType, table string, o TableOpOptions) string {
	c := CapsOf(dbType)
	o.RestartIdentity = o.RestartIdentity && c.RestartIdentity
	o.Cascade = o.Cascade && c.TruncateCascade
	if t, ok := hookOf[TableTruncater](dbType); ok {
		return t.TruncateTableSQL(table, o)
	}
	return "TRUNCATE TABLE " + QuoteTableName(dbType, table)
}

// BuildDropTableSQL 生成删除表的语句；Cascade 只在方言支持时（Caps().DropCascade）才加上。
func BuildDropTableSQL(dbType DBType, table string, o TableOpOptions) string {
	out := "DROP TABLE " + QuoteTableName(dbType, table)
	if o.Cascade && CapsOf(dbType).DropCascade {
		out += " CASCADE"
	}
	return out
}

// BuildRenameTableSQL 生成重命名表的语句。
func BuildRenameTableSQL(dbType DBType, from, to string) (string, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" || to == "" {
		return "", model.Errf(model.CodeTableNameEmpty, "表名不能为空")
	}
	if from == to {
		return "", fmt.Errorf("新表名和原来的一样")
	}
	if r, ok := hookOf[TableRenamer](dbType); ok {
		return r.RenameTableSQL(from, to), nil
	}
	return fmt.Sprintf("ALTER TABLE %s RENAME TO %s", QuoteTableName(dbType, from), QuoteTableName(dbType, to)), nil
}

// BuildCreateTableStatements 生成新建表的语句：先是 CREATE TABLE（列、主键），
// 然后是索引（和 PostgreSQL 的列注释），按顺序执行。
//
// 参数：dbType 库类型；name 表名；cols 列（可标主键、自增）；idx 索引（名称或列不全的会被跳过）。
// 返回值：按顺序执行的语句；表名、列不合法时返回错误。
func BuildCreateTableStatements(dbType DBType, name string, cols []StructureColumnEdit, idx []StructureIndexEdit) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("请先填写表名")
	}
	if !CapsOf(dbType).CreateTable {
		return nil, fmt.Errorf("%s 的建表需要额外的存储设置（如分桶、副本、引擎），请用 SQL 编辑器", dbType)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("至少要有一列")
	}
	seen := map[string]bool{}
	var pks []string
	for _, c := range cols {
		n := strings.TrimSpace(c.Name)
		if n == "" || strings.TrimSpace(c.Type) == "" {
			return nil, fmt.Errorf("每一列都需要名称和类型")
		}
		if seen[strings.ToLower(n)] {
			return nil, fmt.Errorf("列名重复: %s", n)
		}
		seen[strings.ToLower(n)] = true
		if c.PrimaryKey {
			pks = append(pks, QuoteIdent(dbType, n))
		}
	}
	var defs, comments []string
	tq := QuoteTableName(dbType, name)
	inlinePK := false
	for _, c := range cols {
		c.Name, c.Type = strings.TrimSpace(c.Name), strings.TrimSpace(c.Type)
		qn := QuoteIdent(dbType, c.Name)
		var def string
		var inline bool
		var after []string
		if b, ok := hookOf[CreateColumnBuilder](dbType); ok {
			var err error
			if def, inline, after, err = b.CreateColumnDef(c, qn, tq, len(pks)); err != nil {
				return nil, err
			}
		} else {
			def = buildColumnClause(dbType, c, qn)
		}
		defs = append(defs, def)
		inlinePK = inlinePK || inline
		comments = append(comments, after...)
	}
	if len(pks) > 0 && !inlinePK {
		defs = append(defs, "PRIMARY KEY ("+strings.Join(pks, ", ")+")")
	}
	out := []string{fmt.Sprintf("CREATE TABLE %s (\n  %s\n)", tq, strings.Join(defs, ",\n  "))}
	out = append(out, comments...)
	for _, ix := range idx {
		if !validIndexEdit(ix) || ix.IsPrimary {
			continue
		}
		stmt, err := BuildAddIndexSQL(dbType, name, strings.TrimSpace(ix.Name), ix.Columns, ix.IsUnique)
		if err != nil {
			return nil, err
		}
		out = append(out, stmt)
	}
	return out, nil
}

// serialType 把整数类型换成 PostgreSQL 的自增类型。
func serialType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "bigint", "int8":
		return "BIGSERIAL"
	case "smallint", "int2":
		return "SMALLSERIAL"
	}
	return "SERIAL"
}
