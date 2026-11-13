package ai

import (
	"fmt"
	"strings"
)

// SchemaContext 一个数据库的表结构快照，由 schema 索引构建并缓存。
//
// @author ixxxxoooo
type SchemaContext struct {
	DatabaseType    string
	DatabaseName    string
	DatabaseVersion string // 服务器版本，如 "8.0.11-TiDB-v7.5.3"
	Tables          []TableSchema
}

// TableSchema 表结构。
type TableSchema struct {
	Name    string
	Comment string
	Columns []ColumnSchema
	Indexes []IndexSchema // 二级索引（含主键索引）；旧版本索引记录里为空
}

// ColumnSchema 列结构。
type ColumnSchema struct {
	Name         string
	Type         string
	Nullable     bool
	Comment      string
	IsPrimary    bool
	DefaultValue string
	ForeignKey   string // 外键引用，如 "other_table.id"
}

// IndexSchema 表上的一个索引。
type IndexSchema struct {
	Name    string
	Columns []string
	Unique  bool
	Primary bool
}

// BuildTablesDDL 把表列表构建为 DDL 风格文本（含注释、外键引用与索引）。
func BuildTablesDDL(tables []TableSchema) string {
	var sb strings.Builder
	for _, t := range tables {
		sb.WriteString("CREATE TABLE " + t.Name + " (")
		if t.Comment != "" {
			sb.WriteString(" -- " + t.Comment)
		}
		sb.WriteString("\n")
		for i, c := range t.Columns {
			sb.WriteString("  " + c.Name + " " + c.Type)
			if c.IsPrimary {
				sb.WriteString(" PRIMARY KEY")
			} else if !c.Nullable {
				sb.WriteString(" NOT NULL")
			}
			if c.DefaultValue != "" {
				sb.WriteString(" DEFAULT " + c.DefaultValue)
			}
			if c.ForeignKey != "" {
				sb.WriteString(" REFERENCES " + c.ForeignKey)
			}
			if c.Comment != "" {
				sb.WriteString(" COMMENT '" + c.Comment + "'")
			}
			if i < len(t.Columns)-1 || len(t.Indexes) > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		for i, idx := range t.nonPrimaryIndexes() {
			kind := "INDEX"
			if idx.Unique {
				kind = "UNIQUE INDEX"
			}
			sb.WriteString(fmt.Sprintf("  %s %s (%s)", kind, idx.Name, strings.Join(idx.Columns, ", ")))
			if i < len(t.nonPrimaryIndexes())-1 {
				sb.WriteString(",")
			}
			sb.WriteString("\n")
		}
		sb.WriteString(");\n\n")
	}
	return sb.String()
}

// nonPrimaryIndexes 返回主键以外的索引（主键已在列定义里体现）。
func (t TableSchema) nonPrimaryIndexes() []IndexSchema {
	var out []IndexSchema
	for _, idx := range t.Indexes {
		if !idx.Primary {
			out = append(out, idx)
		}
	}
	return out
}

// BuildCompactSchema 构建紧凑的表结构总览：每表一行，列出列名、类型、主键与外键。
// 输出只取决于 schema 本身，所以同一个库每轮对话的提示词前缀完全一致，可以命中提示词缓存。
// maxColumns 限制每表展示的列数，超出的用"共 N 列"收尾。
func BuildCompactSchema(tables []TableSchema, maxColumns int) string {
	var sb strings.Builder
	for _, t := range tables {
		sb.WriteString("- " + t.Name)
		if t.Comment != "" {
			sb.WriteString("（" + t.Comment + "）")
		}
		sb.WriteString(": ")
		for i, c := range t.Columns {
			if maxColumns > 0 && i >= maxColumns {
				sb.WriteString(fmt.Sprintf(", …共 %d 列", len(t.Columns)))
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(c.Name + " " + shortType(c.Type))
			if c.IsPrimary {
				sb.WriteString(" PK")
			}
			if c.ForeignKey != "" {
				sb.WriteString("→" + c.ForeignKey)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// BuildTableList 构建只含表名与注释的清单，用于表很多时的总览。
// maxTables 限制展示的表数，超出的用"另有 N 张表"收尾。
func BuildTableList(tables []TableSchema, maxTables int) string {
	var sb strings.Builder
	for i, t := range tables {
		if maxTables > 0 && i >= maxTables {
			sb.WriteString(fmt.Sprintf("- …另有 %d 张表（用 table_list / schema_search 查找）\n", len(tables)-maxTables))
			break
		}
		sb.WriteString("- " + t.Name)
		if t.Comment != "" {
			sb.WriteString("（" + t.Comment + "）")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// shortType 去掉类型里的长度参数之外的冗余（如 "varchar(255) character set utf8" → "varchar(255)"）。
func shortType(t string) string {
	t = strings.TrimSpace(t)
	if i := strings.IndexByte(t, ' '); i > 0 {
		return t[:i]
	}
	return t
}
