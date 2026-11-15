package ui

import (
	"database/sql"
	"os"
	"testing"

	"minidb/internal/app"
)

// @author ixxxxoooo
//
// README 配图的原始截图：电商库 shop 的演示数据，输出 readme-xxx.png（无窗口、无背景）。
// 只在设置了 MINIDB_SHOTS 时运行；scripts/readme-shots.sh 会先跑它，再用 scripts/mockup 给每张图加上背景。
// 改界面后重跑脚本即可更新 docs/screenshots/。

// readmeShopKeys 给演示库补上外键，让 ER 图里有连线。这几张表都是空的，重建不会丢数据。
func readmeShopKeys(t *testing.T, e *env) {
	t.Helper()
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, s := range []string{
		`DROP TABLE order_items`,
		`DROP TABLE payments`,
		`DROP TABLE products`,
		`CREATE TABLE order_items (id BIGINT PRIMARY KEY, order_id BIGINT REFERENCES orders(id), product_id BIGINT, qty INT, price DECIMAL(10,2))`,
		`CREATE TABLE payments (id BIGINT PRIMARY KEY, order_id BIGINT REFERENCES orders(id), channel VARCHAR(20), amount DECIMAL(10,2))`,
		`CREATE TABLE products (id BIGINT PRIMARY KEY, sku VARCHAR(32), name VARCHAR(120), category_id INT REFERENCES categories(id), price DECIMAL(10,2), stock INT)`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

// TestReadmeShots 逐个场景出图：每个场景一个独立环境，免得前一个的状态串到后一个。
func TestReadmeShots(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	scenes := []struct {
		name string
		dark bool
		run  func(e *env, w *Workspace)
	}{
		{"data-grid", false, func(e *env, w *Workspace) { w.openTable("orders") }},
		{"sql-editor", false, func(e *env, w *Workspace) {
			q := w.newQuery("SELECT status, COUNT(*) AS n, SUM(total) AS revenue\nFROM orders\nGROUP BY status\nORDER BY revenue DESC;")
			e.tt.Frame()
			q.run(true)
		}},
		{"structure", false, func(e *env, w *Workspace) {
			w.openTable("orders")
			e.tt.Frame()
			if tab, ok := w.activeTab().(*tableTab); ok {
				tab.mode = 1
				tab.modeChanged()
			}
		}},
		{"chart", false, func(e *env, w *Workspace) {
			q := w.newQuery("-- minidb barchart x:status, y:n\nSELECT status, COUNT(*) AS n FROM orders GROUP BY status")
			e.tt.Frame()
			q.run(true)
		}},
		{"er-diagram", false, func(e *env, w *Workspace) { w.openER("") }},
		{"ai-assistant", false, readmeAIScene},
		{"dark-edit", true, func(e *env, w *Workspace) {
			w.openTable("orders")
			e.tt.Frame()
			if tab, ok := w.activeTab().(*tableTab); ok {
				tab.g.chosen.Add(2)
				tab.setCell(2, "status", "refunded")
				tab.setCell(4, "note", "客户已确认收货")
			}
		}},
	}
	for _, sc := range scenes {
		t.Run(sc.name, func(t *testing.T) {
			e := newShopEnv(t)
			readmeShopKeys(t, e)
			e.tt.SetDark(sc.dark)
			w := openWorkspace(e)
			sc.run(e, w)
			e.tt.Frame()
			e.tt.Frame()
			e.shot("readme-" + sc.name)
		})
	}
}

// readmeAIScene 摆一段 AI 对话：先查表结构、再跑只读 SQL，回答里的数字和下面 SQL 的结果一致（paid 最多）。
func readmeAIScene(e *env, w *Workspace) {
	e.sh.app.Settings.SaveAIConfig(appAIConfig())
	q := w.newQuery("SELECT status, COUNT(*) AS n, SUM(total) AS revenue\nFROM orders GROUP BY status;")
	w.panel = 2
	e.tt.Frame()
	q.run(true)
	tool := func(id, name string, args map[string]any, sum string) app.AgentPart {
		return app.AgentPart{Tool: &app.AgentToolPart{CallID: id, Name: name, Args: args, Result: &app.AgentToolResult{CallID: id, ToolName: name, OK: true, Kind: "text", Summary: sum}}}
	}
	w.ai.msgs = []*chatMsg{
		{id: 1, user: true, text: "哪个状态的订单金额最高？退款的情况怎么样？"},
		{id: 2, tr: app.AgentTranscript{Done: true, Parts: []app.AgentPart{
			tool("a", "table_describe", map[string]any{"table_names": []any{"orders"}}, "1 张表"),
			tool("b", "sql_readonly_execute", map[string]any{"sql": "SELECT status, SUM(total) FROM orders GROUP BY status"}, "4 行"),
			{Type: app.AgentPartText, Text: "**paid** 的金额最高：23 单，共 6371.7；**refunded** 有 7 单、1982.3，约占总额的 12%。\n\n```sql\nSELECT date(created_at) d, COUNT(*) FROM orders\nWHERE status = 'refunded'\nGROUP BY d ORDER BY d;\n```"},
		}}},
	}
	// 建议按钮的类型在 agent 包里、界面测试不直接引用：造出切片后逐项填字段
	sg := &w.ai.msgs[1].tr
	sg.Suggestions = growSlice(sg.Suggestions, 3)
	for i, l := range []string{"按天画折线图", "找出退款最多的用户", "只看最近一周"} {
		sg.Suggestions[i].Label, sg.Suggestions[i].Prompt = l, l
	}
	w.ai.input = "@orders 里退款最多的用户是谁？"
}
