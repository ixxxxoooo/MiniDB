package ui

import (
	"os"
	"testing"
	"time"

	"minidb/internal/app"
	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 设计稿对照截图（第二批）：浮层、对话框和独立页面。每页一个独立环境，输出 d-xxx.png。

func TestShotsDesignOverlays(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	pages := map[string]func(e *env, w *Workspace){
		"ai": func(e *env, w *Workspace) {
			e.sh.app.Settings.SaveAIConfig(appAIConfig())
			w.newQuery("SELECT status, COUNT(*) AS orders FROM orders GROUP BY status")
			w.panel = 2
			e.tt.Frame()
			tool := func(id, name string, args map[string]any, sum string) app.AgentPart {
				return app.AgentPart{Tool: &app.AgentToolPart{CallID: id, Name: name, Args: args, Result: &app.AgentToolResult{CallID: id, ToolName: name, OK: true, Kind: "text", Summary: sum}}}
			}
			w.ai.msgs = []*chatMsg{
				{id: 1, user: true, text: "最近 7 天哪个状态的订单金额最高？顺便看看有没有异常的退款。"},
				{id: 2, tr: app.AgentTranscript{Done: true, Parts: []app.AgentPart{
					tool("a", "table_describe", map[string]any{"table_names": []any{"orders", "payments"}}, "2 张表"),
					tool("b", "sql_readonly_execute", map[string]any{"sql": "SELECT status, SUM(total) FROM orders GROUP BY status"}, "4 行"),
					{Type: app.AgentPartText, Text: "最近 7 天 **paid** 的金额最高，约 128.5 万；**refunded** 有 618 单、9.8 万，占比 7.6%，略高于上月的 5.9%。"},
				}}},
			}
		},
		"monitor": func(e *env, w *Workspace) {
			w.cfg.Type = model.MySQL
			sess := fakeMonitorSession(40)
			for i := 0; i < 300 && len(sess.View().Points) < 40; i++ {
				time.Sleep(5 * time.Millisecond)
			}
			sess.SetInterval(time.Hour)
			e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{}, mon: monitorPanel{kind: monOverview, interval: 1, sess: sess}}
			w.showMonitorTab()
		},
		"export":  func(e *env, w *Workspace) { e.sh.exportTable(w, "orders") },
		"import":  func(e *env, w *Workspace) { e.sh.importInto(w, "orders") },
		"palette": func(e *env, w *Workspace) { e.sh.openPalette() },
		"settings": func(e *env, w *Workspace) {
			e.sh.openSettings(0)
		},
		"settingslook": func(e *env, w *Workspace) { e.sh.openSettings(setAppearance) },
		"connpop": func(e *env, w *Workspace) {
			cfg2 := e.cfg
			cfg2.ID, cfg2.Name, cfg2.Tag = "c2", "shop-prod", "production"
			if err := e.sh.app.Connections.SaveConnection(cfg2); err != nil {
				e.t.Fatal(err)
			}
			e.sh.loadConnections()
			e.sh.openConnection(cfg2)
			e.tt.Frame()
			e.sh.active = 0
			e.tt.Frame()
			e.tt.ClickAt(150, 70) // 连接卡
		},
		"filter": func(e *env, w *Workspace) {
			w.openTableFiltered("orders", "status", "paid")
			e.tt.Frame()
			if tb, ok := w.activeTab().(*tableTab); ok {
				tb.showFilter = false
			}
		},
		"confirm": func(e *env, w *Workspace) { w.dropTable("orders") },
		"sqlprev": func(e *env, w *Workspace) {
			w.openTable("orders")
			e.tt.Frame()
			if tb, ok := w.activeTab().(*tableTab); ok {
				tb.es.preview.stmts = []string{
					"UPDATE `orders` SET `total` = 128.00 WHERE `id` = 10225;",
					"UPDATE `orders` SET `status` = 'shipped' WHERE `id` = 10230;",
					"UPDATE `orders` SET `note` = '礼品包装' WHERE `id` = 10228;",
				}
				tb.sqlOpen = true
			}
		},
		"bell": func(e *env, w *Workspace) {
			now := time.Now()
			e.sh.notes = []note{
				{at: now.Add(-24 * time.Hour), text: "shop-prod 慢查询增多\n平均耗时 1.8 s，较昨天 +40%", kind: kindWarning},
				{at: now.Add(-time.Hour), text: "有新版本 0.3.0\n更新内容：界面全面重设计", kind: kindInfo},
				{at: now.Add(-2 * time.Minute), text: "导出完成\norders.sql · 21 MB", kind: kindSuccess},
			}
			e.sh.notesOpen = true
		},
		"rowmenu": func(e *env, w *Workspace) {
			w.openTable("orders")
			e.tt.Frame()
			e.tt.Frame()
			if err := e.tt.RightClick("shipped"); err != nil {
				e.t.Fatal(err)
			}
		},
		"newconn": func(e *env, w *Workspace) { e.sh.newConnection() },
		"connlist": func(e *env, w *Workspace) {
			e.sh.openConnList()
		},
		"switcher": func(e *env, w *Workspace) { e.sh.openSwitcher() },
	}
	for name, fn := range pages {
		t.Run(name, func(t *testing.T) {
			e := newShopEnv(t)
			w := openWorkspace(e)
			e.tt.Frame()
			fn(e, w)
			e.tt.Frame()
			e.tt.Frame()
			shotD(e, name)
		})
	}
}

// TestShotsDesignWelcome 欢迎页（连接管理页）：九条连接，分「生产 / 测试 / 本地」三个文件夹。
func TestShotsDesignWelcome(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newShopEnv(t)
	rows := []struct {
		name, tag string
		typ       model.DBType
		host      string
		port      int
	}{
		{"shop-prod", "production", model.MySQL, "rds-prod.cn-east-1", 3306},
		{"analytics", "production", model.Postgres, "pg-ro.internal", 5432},
		{"ledger", "production", model.TiDB, "tidb-01.internal", 4000},
		{"shop-staging", "test", model.MySQL, "stg-db.internal", 3306},
		{"warehouse", "test", model.ClickHouse, "ch-01.internal", 8123},
		{"crm", "test", model.MariaDB, "crm-test.internal", 3306},
		{"local-sqlite", "local", model.SQLite, "~/Data/app.db", 0},
		{"starrocks-lab", "development", model.StarRocks, "127.0.0.1", 9030},
	}
	for i, r := range rows {
		cfg := e.cfg
		cfg.ID, cfg.Name, cfg.Tag, cfg.Type, cfg.Host, cfg.Port = "w"+string(rune('a'+i)), r.name, r.tag, r.typ, r.host, r.port
		if err := e.sh.app.Connections.SaveConnection(cfg); err != nil {
			t.Fatal(err)
		}
	}
	e.sh.loadConnections()
	e.tt.Frame()
	e.tt.Frame()
	shotD(e, "welcome")
}
