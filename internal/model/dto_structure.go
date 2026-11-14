package model

import (
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
	// 只在新建表时用：这一列是不是主键（可以多列组成联合主键）
	PrimaryKey bool `json:"primaryKey,omitempty"`
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

// StructureFKEdit 结构编辑：外键工作态。UID 是已有外键的原名，新增的是 "new:N"；
// 已有的外键被改过时 Status 为 "modified"，生成「删旧建新」。
//
// @author ixxxxoooo
type StructureFKEdit struct {
	UID        string   `json:"uid"`
	Status     string   `json:"status"` // "", "new", "deleted", "modified"
	Name       string   `json:"name"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"refTable"`
	RefColumns []string `json:"refColumns"`
	OnDelete   string   `json:"onDelete"`
	OnUpdate   string   `json:"onUpdate"`
}

// Blank 报告一行新增的外键是不是刚点了「添加」、什么都没填的空行。
func (f StructureFKEdit) Blank() bool {
	return f.Status == "new" && strings.TrimSpace(f.Name) == "" && len(f.Columns) == 0 && f.RefTable == "" && len(f.RefColumns) == 0
}

// StructureTriggerEdit 结构编辑：触发器工作态。已有的触发器只能改整条语句：
// Definition 是完整的 CREATE TRIGGER，Status 为 "modified" 时生成「删旧、原样执行新语句」；
// 新增的（Status 为 "new"）由名称、时机、事件和 Definition（这时只是触发的内容）拼出语句。
type StructureTriggerEdit struct {
	UID        string `json:"uid"`
	Status     string `json:"status"`
	Name       string `json:"name"`
	Timing     string `json:"timing"`
	Event      string `json:"event"`
	Definition string `json:"definition"`
}

// Blank 报告一行新增的触发器是不是什么都没填的空行。
func (t StructureTriggerEdit) Blank() bool {
	return t.Status == "new" && strings.TrimSpace(t.Name) == "" && strings.TrimSpace(t.Definition) == ""
}

// StructureObjectEdits 是一次提交里外键和触发器的全部编辑。
type StructureObjectEdits struct {
	FKs      []StructureFKEdit      `json:"fks"`
	Triggers []StructureTriggerEdit `json:"triggers"`
	// RenameTo 不为空时，在其他变更都做完之后把表改成这个名字
	RenameTo string `json:"renameTo,omitempty"`
}

// Empty 报告有没有需要执行的外键 / 触发器编辑（还没填内容的空白新行不算）。
func (e StructureObjectEdits) Empty() bool {
	if strings.TrimSpace(e.RenameTo) != "" {
		return false
	}
	for _, f := range e.FKs {
		if f.Status != "" && !f.Blank() {
			return false
		}
	}
	for _, t := range e.Triggers {
		if t.Status != "" && !t.Blank() {
			return false
		}
	}
	return true
}
