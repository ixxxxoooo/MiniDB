package app

import (
	"testing"

	"minidb/internal/database"
)

// TestCapabilityMatrix 锁定 5 种方言在界面门禁上的能力，防止新增数据库或改动 Caps 时悄悄放开入口。
func TestCapabilityMatrix(t *testing.T) {
	cases := []struct {
		typ                                                             database.DBType
		createTable, alterTable, reorder, rowsReliable, roTx, sslConfig bool
	}{
		{database.MySQL, true, true, true, true, true, true},
		{database.TiDB, true, true, true, true, true, true},
		{database.StarRocks, false, false, false, false, false, true},
		{database.Postgres, true, true, false, true, true, true},
		{database.SQLite, true, true, false, true, false, false},
	}
	for _, c := range cases {
		t.Run(string(c.typ), func(t *testing.T) {
			if got := SupportsVisualCreateTable(c.typ); got != c.createTable {
				t.Errorf("SupportsVisualCreateTable = %v, 期望 %v", got, c.createTable)
			}
			if got := SupportsVisualAlterTable(c.typ); got != c.alterTable {
				t.Errorf("SupportsVisualAlterTable = %v, 期望 %v", got, c.alterTable)
			}
			if got := SupportsColumnReorder(c.typ); got != c.reorder {
				t.Errorf("SupportsColumnReorder = %v, 期望 %v", got, c.reorder)
			}
			if got := RowsAffectedReliable(c.typ); got != c.rowsReliable {
				t.Errorf("RowsAffectedReliable = %v, 期望 %v", got, c.rowsReliable)
			}
			if got := SupportsReadOnlyTx(c.typ); got != c.roTx {
				t.Errorf("SupportsReadOnlyTx = %v, 期望 %v", got, c.roTx)
			}
			if got := SupportsSSLConfig(c.typ); got != c.sslConfig {
				t.Errorf("SupportsSSLConfig = %v, 期望 %v", got, c.sslConfig)
			}
		})
	}
}

// TestUnsupportedStructureOps 不支持可视化建表/改表的库，后端也必须拒绝，而不是生成错误的 DDL。
func TestUnsupportedStructureOps(t *testing.T) {
	if err := database.ValidateStructureAlterSupported(database.StarRocks, ""); err == nil {
		t.Error("StarRocks 不应允许可视化修改表结构")
	}
	if _, err := database.BuildCreateTableStatements(database.StarRocks, "t", nil, nil); err == nil {
		t.Error("StarRocks 不应允许可视化建表")
	}
}

// TestTableOpSupportFollowsCaps 清空/删除表的可选项只由 Caps 决定。
func TestTableOpSupportFollowsCaps(t *testing.T) {
	tc, ri, dc := database.TableOpSupport(database.Postgres)
	if !tc || !ri || !dc {
		t.Errorf("PostgreSQL 应支持全部可选项: %v %v %v", tc, ri, dc)
	}
	tc, ri, dc = database.TableOpSupport(database.MySQL)
	if tc || ri || dc {
		t.Errorf("MySQL 不应支持这些可选项: %v %v %v", tc, ri, dc)
	}
}
