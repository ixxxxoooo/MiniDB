package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// @author ixxxxoooo
//
// 导出 / 导入向导的设计稿对照截图：每一步一张，输出 d-export1..4.png、d-import1..4.png。

func TestShotsDesignTransfer(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newShopEnv(t)
	w := openWorkspace(e)
	e.sh.exportTables(w)
	e.sh.exp.sel = map[string]bool{"orders": true, "order_items": true, "payments": true, "users": true}
	e.tt.Frame()
	tab := w.activeTab().(*transferTab)
	for i := 0; i < 3; i++ {
		tab.step = i
		e.tt.Frame()
		e.tt.Frame()
		e.shot("d-export" + string(rune('1'+i)))
	}

	// 执行步：进行中与完成两种状态
	tab.step = 3
	e.sh.exp.xfer.begin("t-ex")
	e.sh.exp.xfer.note(xferEvent{id: "t-ex", status: "progress", cur: 4200, total: 10000})
	e.tt.Frame()
	e.tt.Frame()
	e.shot("d-export4")
	e.sh.exp.xfer.note(xferEvent{id: "t-ex", status: "done", cur: 10000, total: 10000, path: "/tmp/export"})
	e.tt.Frame()
	e.shot("d-export5")

	csv := filepath.Join(t.TempDir(), "orders_2026-11.csv")
	if err := os.WriteFile(csv, []byte("order_id,uid,status,total,created_at,note\n10232,1042,paid,128.00,2026-11-15 10:30:00,\n10231,1043,pending,56.50,2026-11-15 10:31:00,\n10230,1044,paid,9.90,2026-11-15 10:32:00,\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.sh.importInto(w, "orders")
	e.sh.imp.opts.HasHeader = true
	e.tt.Frame()
	itab := w.activeTab().(*transferTab)
	itab.step = 0
	e.tt.Frame()
	e.shot("d-import0")
	e.sh.imp.path = csv
	e.sh.refreshImportPreview()
	for i := 0; i < 3; i++ {
		itab.step = i
		e.tt.Frame()
		e.tt.Frame()
		e.shot("d-import" + string(rune('1'+i)))
	}
}

func TestShotsDesignTransferRun(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newShopEnv(t)
	w := openWorkspace(e)
	e.sh.importInto(w, "orders")
	e.tt.Frame()
	tab := w.activeTab().(*transferTab)
	tab.step = 3
	e.sh.imp.xfer.begin("t-im")
	e.sh.imp.xfer.note(xferEvent{id: "t-im", status: "progress", cur: 300, total: 1200})
	e.tt.Frame()
	e.tt.Frame()
	e.shot("d-import4")
	e.sh.imp.xfer.note(xferEvent{id: "t-im", status: "done", cur: 1200, total: 1200})
	e.tt.Frame()
	e.shot("d-import5")
}
