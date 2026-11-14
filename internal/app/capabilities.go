package app

import "minidb/internal/database"

// 界面按「能力」决定显示哪些入口，统一从这里取，不要在界面里写类型判断。
// 每个函数只是对 Caps 某个字段的语义化封装，真正的取值在各方言的 Caps() 里。

// SupportsVisualCreateTable 该库能否在界面里可视化建表（StarRocks、ClickHouse 需要额外的存储设置，只能手写）。
func SupportsVisualCreateTable(t database.DBType) bool { return database.CapsOf(t).CreateTable }

// SupportsVisualAlterTable 该库能否在界面里可视化修改已有表的结构。
func SupportsVisualAlterTable(t database.DBType) bool { return database.CapsOf(t).AlterTable }

// SupportsColumnReorder 该库能否调整已有表的列顺序。
func SupportsColumnReorder(t database.DBType) bool {
	c := database.CapsOf(t)
	return c.AlterTable && c.ColumnReorder
}

// RowsAffectedReliable UPDATE/DELETE 返回的受影响行数是否可信。
func RowsAffectedReliable(t database.DBType) bool { return database.CapsOf(t).RowsAffectedReliable }

// SupportsReadOnlyTx 只读事务是否真的会拒绝写操作。
func SupportsReadOnlyTx(t database.DBType) bool { return database.CapsOf(t).ReadOnlyTx }

// SupportsSSLConfig 连接表单是否提供 SSL/TLS 选项。
func SupportsSSLConfig(t database.DBType) bool { return len(database.SSLModesOf(t)) > 0 }

// SupportsMonitor 报告连接类型是否有监控概览与慢查询。
func SupportsMonitor(t database.DBType) bool { return database.SupportsMonitor(t) }
