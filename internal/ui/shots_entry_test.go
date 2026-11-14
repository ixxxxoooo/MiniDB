package ui

import (
	"os"
	"testing"

	"minidb/internal/app"
	"minidb/internal/model"
)

// TestShotsEntries 渲染这次新增的入口和高亮：顶栏监控按钮 + 索引芯片、慢查询的 SQL 高亮、快捷键设置页。
// 设置 MINIDB_SHOTS 后输出 e1-… 系列截图（浅色 / 深色各一份）。
func TestShotsEntries(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	for _, dark := range []bool{false, true} {
		suffix := map[bool]string{false: "light", true: "dark"}[dark]
		e := newEnv(t)
		e.tt.SetDark(dark)
		w := openWorkspace(e)
		w.cfg.Type = model.MySQL // 让顶栏认为这是有监控能力的连接
		e.tt.Frame()
		w.idx.st = app.SchemaStatus{Exists: true, TableCount: 7}
		e.shot("e1-toolbar-" + suffix)
		w.idx.st.Refreshing = true
		e.shot("e2-toolbar-refreshing-" + suffix)
		w.idx.st.Refreshing = false

		e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{}, mon: monitorPanel{kind: monSlow, slow: slowState{loaded: true, sel: 0, items: []app.SlowQuery{
			{SQL: "SELECT o.id, c.name FROM orders o JOIN customers c ON c.id = o.customer_id WHERE o.status = 'paid' AND o.total > 100 ORDER BY o.created_at DESC -- 最近订单", Database: "shop", Calls: 1520, TotalMs: 912000, AvgMs: 600, MaxMs: 5400, Rows: 480000, LastSeen: "2026-11-14 19:20:11", Source: "performance_schema"},
			{SQL: "UPDATE stock SET qty = qty - 1 WHERE sku = 'A-1001'", Database: "shop", Calls: 90000, TotalMs: 45000, AvgMs: 0.5, MaxMs: 120, Rows: -1, LastSeen: "2026-11-14 19:21:00", Source: "performance_schema"},
		}}}}
		e.shot("e3-monitor-slow-" + suffix)
		e.sh.proc.open = false
		e.tt.Frame()

		e.sh.openSettings(setShortcuts)
		e.shot("e4-keymap-" + suffix)
	}
}
