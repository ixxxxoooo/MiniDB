package database

import (
	"fmt"
	"strings"
)

// 表级操作的 SQL：新建、重命名、清空、删除。清空和删除的可选项（Cascade、Restart identity）
// 只有部分数据库支持，TableOpSupport 说明某个库能给哪些。

// TableOpOptions 是清空 / 删除表时的可选项。
//
// @author ixxxxoooo
type TableOpOptions struct {
	Cascade         bool `json:"cascade"`         // 连带清空 / 删除引用它的表（PostgreSQL）
	RestartIdentity bool `json:"restartIdentity"` // 清空后自增序列从头开始（PostgreSQL）
}

// TableOpSupport 报告 dbType 在清空、删除表时支持哪些可选项。
//
// 参数：dbType 库类型。
// 返回值：truncateCascade 清空是否支持 Cascade；restartIdentity 清空是否支持 Restart identity；dropCascade 删除是否支持 Cascade。
func TableOpSupport(dbType DBType) (truncateCascade, restartIdentity, dropCascade bool) {
	if dbType == Postgres {
		return true, true, true
	}
	return false, false, false
}

// BuildTruncateTableSQL 生成清空表的语句；SQLite 没有 TRUNCATE，用 DELETE FROM。
func BuildTruncateTableSQL(dbType DBType, table string, o TableOpOptions) string {
	tq := QuoteTableName(dbType, table)
	if dbType == SQLite {
		return "DELETE FROM " + tq
	}
	out := "TRUNCATE TABLE " + tq
	if dbType == Postgres {
		if o.RestartIdentity {
			out += " RESTART IDENTITY"
		}
		if o.Cascade {
			out += " CASCADE"
		}
	}
	return out
}

// BuildDropTableSQL 生成删除表的语句。
func BuildDropTableSQL(dbType DBType, table string, o TableOpOptions) string {
	out := "DROP TABLE " + QuoteTableName(dbType, table)
	if dbType == Postgres && o.Cascade {
		out += " CASCADE"
	}
	return out
}

// BuildRenameTableSQL 生成重命名表的语句。
func BuildRenameTableSQL(dbType DBType, from, to string) (string, error) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if from == "" || to == "" {
		return "", fmt.Errorf("表名不能为空")
	}
	if from == to {
		return "", fmt.Errorf("新表名和原来的一样")
	}
	f, t := QuoteTableName(dbType, from), QuoteTableName(dbType, to)
	switch {
	case IsMySQLCompatible(dbType) && dbType != StarRocks:
		return fmt.Sprintf("RENAME TABLE %s TO %s", f, t), nil
	case dbType == StarRocks:
		return fmt.Sprintf("ALTER TABLE %s RENAME %s", f, t), nil
	}
	return fmt.Sprintf("ALTER TABLE %s RENAME TO %s", f, t), nil
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
	if dbType == StarRocks {
		return nil, fmt.Errorf("StarRocks 的建表需要分桶和副本等设置，请用 SQL 编辑器")
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
		switch {
		case dbType == SQLite && c.AutoIncrement:
			if len(pks) != 1 || !c.PrimaryKey {
				return nil, fmt.Errorf("SQLite 的自增列必须是唯一的主键")
			}
			defs = append(defs, qn+" INTEGER PRIMARY KEY AUTOINCREMENT")
			inlinePK = true
			continue
		case dbType == Postgres && c.AutoIncrement:
			c.Type = serialType(c.Type)
		}
		defs = append(defs, buildColumnClause(dbType, c, qn))
		if dbType == Postgres && c.Comment != "" {
			comments = append(comments, fmt.Sprintf(`COMMENT ON COLUMN %s.%s IS %s`, tq, qn, QuoteString(Postgres, c.Comment)))
		}
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
