package database

import (
	"fmt"
	"strings"
)

// 触发器、函数与存储过程、视图的 SQL 生成。

// TriggerTimings 与 TriggerEvents 是新建触发器时的可选项。
var (
	TriggerTimings = []string{"BEFORE", "AFTER"}
	TriggerEvents  = []string{"INSERT", "UPDATE", "DELETE"}
)

// BuildCreateTriggerSQL 生成创建触发器的语句。body 在 MySQL 里是触发的语句（可用 BEGIN…END），
// 在 SQLite 里是 BEGIN 与 END 之间的语句，在 PostgreSQL 里是要执行的触发器函数名。
func BuildCreateTriggerSQL(dbType DBType, table string, tr TriggerInfo) (string, error) {
	if !CapsOf(dbType).Triggers {
		return "", fmt.Errorf("%s 不支持触发器", dbType)
	}
	name, body := strings.TrimSpace(tr.Name), strings.TrimSpace(tr.Definition)
	timing, event := strings.ToUpper(strings.TrimSpace(tr.Timing)), strings.ToUpper(strings.TrimSpace(tr.Event))
	if name == "" || body == "" {
		return "", fmt.Errorf("触发器需要名称和内容")
	}
	if !inList(TriggerTimings, timing) || !inList(TriggerEvents, event) {
		return "", fmt.Errorf("不支持的触发时机或事件: %s %s", timing, event)
	}
	if w, ok := hookOf[CreateTriggerWriter](dbType); ok {
		return w.CreateTriggerSQL(name, timing, event, table, body), nil
	}
	return fmt.Sprintf("CREATE TRIGGER %s %s %s ON %s FOR EACH ROW %s", QuoteIdent(dbType, name), timing, event, QuoteTableName(dbType, table), body), nil
}

// BuildDropTriggerSQL 生成删除触发器的语句。
func BuildDropTriggerSQL(dbType DBType, table, name string) string {
	if w, ok := hookOf[DropTriggerWriter](dbType); ok {
		return w.DropTriggerSQL(table, name)
	}
	return "DROP TRIGGER " + QuoteIdent(dbType, name)
}

func inList(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// BuildDropRoutineSQL 生成删除函数或存储过程的语句。
func BuildDropRoutineSQL(dbType DBType, kind, name string) (string, error) {
	if kind != "function" && kind != "procedure" {
		return "", fmt.Errorf("未知的例程类型: %s", kind)
	}
	return fmt.Sprintf("DROP %s %s", strings.ToUpper(kind), QuoteIdent(dbType, name)), nil
}

// BuildDropViewSQL 生成删除视图的语句。
func BuildDropViewSQL(dbType DBType, name string) string {
	return "DROP VIEW " + QuoteTableName(dbType, name)
}
