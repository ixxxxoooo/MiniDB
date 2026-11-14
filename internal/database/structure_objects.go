package database

import (
	"fmt"
	"strings"
)

// BuildStructureObjectStatements 把外键和触发器的编辑翻译成语句：
// pre 在列、索引变更之前执行（删除旧的，免得挡住删列），post 在之后执行（创建新的，可以引用新加的列）。
//
// 参数：dbType 库类型；table 表名；e 全部编辑。
// 返回值：pre、post 两组语句；任何一条生成失败就返回错误。
func BuildStructureObjectStatements(dbType DBType, table string, e StructureObjectEdits) (pre, post []string, err error) {
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
func BuildFullStructureStatements(dbType DBType, serverVersion, tableName string, workingCols, originalCols []StructureColumnEdit,
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
	if to := strings.TrimSpace(objs.RenameTo); to != "" && to != tableName {
		stmt, err := BuildRenameTableSQL(dbType, tableName, to)
		if err != nil {
			return nil, err
		}
		out = append(out, stmt)
	}
	return out, nil
}
