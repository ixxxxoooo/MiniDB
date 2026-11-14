package database

import (
	"context"
	"minidb/internal/model"
	"regexp"
	"strings"
)

// 表之外的数据库对象：外键、触发器、函数与存储过程。读取在各方言（dialect_*.go）里，
// 生成 SQL 的函数都是纯函数，便于预览与测试（objects_sql.go、objects_db.go）。
// SQLite 没有函数与存储过程；StarRocks 没有触发器和外键，读取返回空。

// FKActions 是外键的 ON UPDATE / ON DELETE 可选动作。
var FKActions = []string{"NO ACTION", "RESTRICT", "CASCADE", "SET NULL", "SET DEFAULT"}

// ---- 读取 ----

// GetForeignKeys 读取表上的外键。
func GetForeignKeys(ctx context.Context, db Executor, dbType DBType, dbName, table string) ([]ForeignKeyInfo, error) {
	d, err := DialectOf(dbType)
	if err != nil || !d.Caps().ForeignKeys {
		return nil, nil
	}
	return d.ForeignKeys(ctx, db, dbName, table)
}

// GetTriggers 读取表上的触发器。
func GetTriggers(ctx context.Context, db Executor, dbType DBType, dbName, table string) ([]TriggerInfo, error) {
	d, err := DialectOf(dbType)
	if err != nil || !d.Caps().Triggers {
		return nil, nil
	}
	return d.Triggers(ctx, db, dbName, table)
}

// GetRoutines 列出库里的函数与存储过程。
func GetRoutines(ctx context.Context, db Executor, dbType DBType, dbName string) ([]RoutineInfo, error) {
	d, err := DialectOf(dbType)
	if err != nil || !d.Caps().Routines {
		return nil, nil
	}
	return d.Routines(ctx, db, dbName)
}

// GetRoutineDDL 读取函数或存储过程的完整定义。
func GetRoutineDDL(ctx context.Context, db Executor, dbType DBType, dbName, kind, name string) (string, error) {
	d, err := DialectOf(dbType)
	if err != nil {
		return "", err
	}
	if !d.Caps().Routines {
		return "", model.Errf(model.CodeUnsupportedRoutines, "%s 不支持存储过程与函数", dbType)
	}
	return d.RoutineDDL(ctx, db, dbName, kind, name)
}

var triggerHead = regexp.MustCompile(`(?i)\b(BEFORE|AFTER|INSTEAD\s+OF)\s+(INSERT|UPDATE|DELETE|TRUNCATE)`)

// parseTriggerHead 从 CREATE TRIGGER 语句里读出触发时机与事件。
func parseTriggerHead(def string) (timing, event string) {
	m := triggerHead.FindStringSubmatch(def)
	if m == nil {
		return "", ""
	}
	return strings.ToUpper(strings.Join(strings.Fields(m[1]), " ")), strings.ToUpper(m[2])
}
