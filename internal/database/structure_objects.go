package database

import (
	"fmt"
	"strings"
)

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

// StructureObjectEdits 是一次提交里外键和触发器的全部编辑。
type StructureObjectEdits struct {
	FKs      []StructureFKEdit      `json:"fks"`
	Triggers []StructureTriggerEdit `json:"triggers"`
}

// Empty 报告有没有需要执行的外键 / 触发器编辑（还没填内容的空白新行不算）。
func (e StructureObjectEdits) Empty() bool {
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

// Blank 报告一行新增的外键是不是刚点了「添加」、什么都没填的空行。
func (f StructureFKEdit) Blank() bool {
	return f.Status == "new" && strings.TrimSpace(f.Name) == "" && len(f.Columns) == 0 && f.RefTable == "" && len(f.RefColumns) == 0
}

// Blank 报告一行新增的触发器是不是什么都没填的空行。
func (t StructureTriggerEdit) Blank() bool {
	return t.Status == "new" && strings.TrimSpace(t.Name) == "" && strings.TrimSpace(t.Definition) == ""
}

// BuildStructureObjectStatements 把外键和触发器的编辑翻译成语句：
// pre 在列、索引变更之前执行（删除旧的，免得挡住删列），post 在之后执行（创建新的，可以引用新加的列）。
//
// 参数：dbType 库类型；table 表名；e 全部编辑。
// 返回值：pre、post 两组语句；任何一条生成失败就返回错误。
func BuildStructureObjectStatements(dbType, table string, e StructureObjectEdits) (pre, post []string, err error) {
	for _, f := range e.FKs {
		if f.Blank() {
			continue
		}
		if f.Status == "deleted" || f.Status == "modified" {
			stmt, derr := BuildDropForeignKeySQL(dbType, table, f.UID)
			if derr != nil {
				return nil, nil, derr
			}
			pre = append(pre, stmt)
		}
		if f.Status == "new" || f.Status == "modified" {
			stmt, aerr := BuildAddForeignKeySQL(dbType, table, ForeignKeyInfo{
				Name: f.Name, Columns: f.Columns, RefTable: f.RefTable, RefColumns: f.RefColumns, OnDelete: f.OnDelete, OnUpdate: f.OnUpdate,
			})
			if aerr != nil {
				return nil, nil, fmt.Errorf("外键 %s: %w", f.Name, aerr)
			}
			post = append(post, stmt)
		}
	}
	for _, t := range e.Triggers {
		if t.Blank() {
			continue
		}
		if t.Status == "deleted" || t.Status == "modified" {
			pre = append(pre, BuildDropTriggerSQL(dbType, table, t.UID))
		}
		switch t.Status {
		case "new":
			stmt, cerr := BuildCreateTriggerSQL(dbType, table, TriggerInfo{Name: t.Name, Timing: t.Timing, Event: t.Event, Definition: t.Definition})
			if cerr != nil {
				return nil, nil, fmt.Errorf("触发器 %s: %w", t.Name, cerr)
			}
			post = append(post, stmt)
		case "modified":
			body := strings.TrimSpace(t.Definition)
			if !strings.HasPrefix(strings.ToUpper(body), "CREATE") {
				return nil, nil, fmt.Errorf("触发器 %s: 需要完整的 CREATE TRIGGER 语句", t.UID)
			}
			post = append(post, body)
		}
	}
	return pre, post, nil
}

// BuildFullStructureStatements 在 BuildStructureAlterDDLStatements 的基础上带上外键和触发器：
// 先删旧的外键 / 触发器，再改列和索引，最后建新的外键 / 触发器。
//
// 参数：同 BuildStructureAlterDDLStatements，另加 objs 外键和触发器的编辑。
// 返回值：按顺序执行的全部语句。
func BuildFullStructureStatements(dbType, serverVersion, tableName string, workingCols, originalCols []StructureColumnEdit,
	workingIndexes []StructureIndexEdit, objs StructureObjectEdits) ([]string, error) {
	pre, post, err := BuildStructureObjectStatements(dbType, tableName, objs)
	if err != nil {
		return nil, err
	}
	mid, err := BuildStructureAlterDDLStatements(dbType, serverVersion, tableName, workingCols, originalCols, workingIndexes)
	if err != nil {
		return nil, err
	}
	out := append(append(append([]string(nil), pre...), mid...), post...)
	return out, nil
}
