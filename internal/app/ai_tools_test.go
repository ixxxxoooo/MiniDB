package app

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"minidb/internal/agent"
	"minidb/internal/database"
	"minidb/internal/storage"
)

// aiHarness 一个真实的 SQLite 库 + 完整装配的 App，用来端到端地测工具与对话。
type aiHarness struct {
	a      *App
	connID string
	db     string
	env    *toolEnv
	reg    *agent.ToolRegistry
}

func newAIHarness(t *testing.T) *aiHarness {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	store, err := storage.OpenStore(filepath.Join(t.TempDir(), "s.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	a := New(store, nil)
	t.Cleanup(func() { a.manager.CloseAll(); store.Close() })

	path := filepath.Join(t.TempDir(), "shop.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, email TEXT, status TEXT)`,
		`CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id), total REAL, status TEXT)`,
		`CREATE TABLE order_items (order_id INTEGER, sku TEXT, qty INTEGER)`,
		`CREATE INDEX idx_users_email ON users(email)`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 300; i++ {
		status := []string{"paid", "paid", "refunded", "new"}[i%4]
		raw.Exec(`INSERT INTO users(name,email,status) VALUES(?,?,?)`, fmt.Sprintf("u%03d", i), fmt.Sprintf("u%d@x.com", i), status)
		raw.Exec(`INSERT INTO orders(user_id,total,status) VALUES(?,?,?)`, i, float64(i)*1.5, status)
	}
	raw.Close()

	cfg := &database.ConnectionConfig{ID: "c1", Name: "shop", Type: "sqlite", Database: path}
	if err := a.manager.Connect(cfg); err != nil {
		t.Fatal(err)
	}
	schema, search, err := a.schema.Index(context.Background(), "c1", path)
	if err != nil {
		t.Fatal(err)
	}
	env := &toolEnv{s: a.AI, connID: "c1", dbName: path, schema: schema, search: search}
	return &aiHarness{a: a, connID: "c1", db: path, env: env, reg: env.registry()}
}

func (h *aiHarness) call(name string, args map[string]any) *agent.ToolResult {
	return h.reg.Invoke(context.Background(), name, agent.ToolInput(agent.NormalizeArguments(args)))
}

func mustContain(t *testing.T, what, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("%s lacks %q:\n%s", what, w, text)
		}
	}
}

func TestToolSetIsConsolidatedAndStrictCompatible(t *testing.T) {
	h := newAIHarness(t)
	tools := h.reg.List()
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Name)
		props, _ := tool.Parameters["properties"].(map[string]any)
		req, _ := tool.Parameters["required"].([]string)
		if len(req) != len(props) || tool.Parameters["additionalProperties"] != false {
			t.Errorf("%s: strict mode needs every property required and no extras: props=%d required=%d", tool.Name, len(props), len(req))
		}
		if !tool.ReadOnly || len([]rune(tool.Description)) < 40 {
			t.Errorf("%s: read-only and described well enough to choose it (when to use, limits)", tool.Name)
		}
		for name, p := range props {
			if d, _ := p.(map[string]any)["description"].(string); d == "" {
				t.Errorf("%s.%s has no description", tool.Name, name)
			}
		}
	}
	want := "table_list,schema_search,table_describe,table_stats,table_sample,column_values,table_profile,sql_readonly_execute,sql_explain_plan"
	if strings.Join(names, ",") != want {
		t.Fatalf("tools = %v", names)
	}
	defs := toolDefinitions(h.reg)
	if len(defs) != len(tools) || defs[0].Name != "table_list" {
		t.Fatal("LLM definitions derive from the registry")
	}
}

func TestSchemaToolsFindAndDescribe(t *testing.T) {
	h := newAIHarness(t)

	list := h.call("table_list", nil)
	mustContain(t, "table_list", list.FormatForModel(50, 200), "共 3 张表", "- orders · 4 列", "- users · 4 列")
	kw := h.call("table_list", map[string]any{"keyword": "order"})
	mustContain(t, "table_list order", kw.FormatForModel(50, 200), "共 2 张", "order_items", "orders")
	none := h.call("table_list", map[string]any{"keyword": "zzzz"})
	mustContain(t, "no match", none.FormatForModel(50, 200), "没有与", "schema_search")

	search := h.call("schema_search", map[string]any{"keywords": []any{"email"}})
	mustContain(t, "schema_search", search.FormatForModel(50, 200), "匹配的列:", "users.email TEXT")
	if len(search.Rows) == 0 || search.Rows[0]["种类"] != "表" && search.Rows[0]["种类"] != "列" {
		t.Fatalf("rows for the UI: %+v", search.Rows)
	}
	if res := h.call("schema_search", map[string]any{"keywords": []any{}}); res.OK || res.ErrorCode != "invalid_arguments" {
		t.Fatalf("empty keywords: %+v", res)
	}

	desc := h.call("table_describe", map[string]any{"table_names": []any{"Users", "orders", "ghost"}})
	text := desc.FormatForModel(50, 200)
	mustContain(t, "table_describe", text, "## users · 4 列", "- id INTEGER PK", "- name TEXT NOT NULL",
		"索引: idx_users_email (email)", "关联: orders.user_id → users.id（外键，反向）", "关联: orders.user_id → users.id（外键）", "不存在，已忽略: ghost")
	if !strings.Contains(desc.Notes, "idx_users_email") || len(desc.Rows) != 8 {
		t.Fatalf("the UI gets one row per column and the index/relation notes: rows=%d notes=%q", len(desc.Rows), desc.Notes)
	}
	items := h.call("table_describe", map[string]any{"table_names": []any{"order_items"}}).FormatForModel(50, 200)
	mustContain(t, "inferred relation", items, "order_items.order_id → orders.id（推断：仅依据命名，需确认）")

	miss := h.call("table_describe", map[string]any{"table_names": []any{"usres"}})
	if miss.OK || miss.ErrorCode != "table_not_found" || !strings.Contains(miss.Text, "users") || miss.Hint == "" {
		t.Fatalf("a typo must come back with the likely table: %+v", miss)
	}
}

func TestDataToolsReturnStructuredRowsAndActionableErrors(t *testing.T) {
	h := newAIHarness(t)

	res := h.call("sql_readonly_execute", map[string]any{"sql": "SELECT status, COUNT(*) AS n FROM users GROUP BY status ORDER BY n DESC", "limit": 2})
	if !res.OK || len(res.Rows) != 2 || res.SQL == "" || res.Summary != "2 / 3 行" || !res.Truncated {
		t.Fatalf("%+v", res)
	}
	big := h.call("sql_readonly_execute", map[string]any{"sql": "SELECT * FROM users", "limit": 10})
	mustContain(t, "truncated result", big.FormatForModel(50, 200), "共 300 行，仅展示前 10 行")
	if !big.Truncated || big.Summary != "10 / 300 行" {
		t.Fatalf("truncated=%v summary=%q", big.Truncated, big.Summary)
	}

	for sqlText, want := range map[string]string{
		"DELETE FROM users":          "只读权限",
		"SELECT 1; SELECT 2":         "一条",
		"SELECT * FROM users INTO x": "只读权限",
	} {
		denied := h.call("sql_readonly_execute", map[string]any{"sql": sqlText})
		if denied.OK || denied.ErrorCode != "guarded" || !strings.Contains(denied.Text, want) || denied.Hint == "" || denied.SQL != sqlText {
			t.Errorf("%q: %+v", sqlText, denied)
		}
	}
	bad := h.call("sql_readonly_execute", map[string]any{"sql": "SELECT nope FROM users"})
	if bad.OK || bad.ErrorCode != "sql_error" || !strings.Contains(bad.Hint, "table_describe") {
		t.Fatalf("a failing query tells the model how to recover: %+v", bad)
	}
	if got := bad.FormatForModel(50, 200); !strings.HasPrefix(got, "ERROR: ") || !strings.Contains(got, "建议:") {
		t.Fatalf("model text: %s", got)
	}

	sample := h.call("table_sample", map[string]any{"table_name": "orders", "limit": 3})
	if !sample.OK || len(sample.Rows) != 3 || sample.Summary != "orders · 3 行" {
		t.Fatalf("%+v", sample)
	}
	if extra := h.call("table_sample", map[string]any{"table_name": "orders", "where": "id=1"}); extra.OK || !strings.Contains(extra.Text, `未知参数 "where"`) {
		t.Fatalf("an unsupported filter is rejected loudly instead of silently ignored: %+v", extra)
	}

	vals := h.call("column_values", map[string]any{"table_name": "users", "column": "STATUS"})
	if !vals.OK || vals.Rows[0]["value"] != "paid" || toInt64(vals.Rows[0]["cnt"]) != 150 {
		t.Fatalf("most common value first: %+v", vals.Rows)
	}
	noCol := h.call("column_values", map[string]any{"table_name": "users", "column": "state"})
	if noCol.OK || noCol.ErrorCode != "column_not_found" || !strings.Contains(noCol.Hint, "status") {
		t.Fatalf("an unknown column lists the real ones: %+v", noCol)
	}
}

func TestStatsProfileAndExplain(t *testing.T) {
	h := newAIHarness(t)
	stats := h.call("table_stats", map[string]any{"table_names": []any{"users", "orders"}})
	if !stats.OK || len(stats.Rows) != 2 {
		t.Fatalf("%+v", stats)
	}
	prof := h.call("table_profile", map[string]any{"table_names": []any{"users"}, "columns": []any{"email", "status"}})
	mustContain(t, "profile", prof.FormatForModel(50, 200), "## users（共 300 行）", "- status TEXT: 空值 0, 不同值 3", "- email TEXT:")
	if len(prof.Rows) != 2 || prof.SQL == "" {
		t.Fatalf("rows=%d", len(prof.Rows))
	}
	numeric := h.call("table_profile", map[string]any{"table_names": []any{"orders"}, "columns": []any{"total"}}).FormatForModel(50, 200)
	mustContain(t, "numeric profile", numeric, "范围 1.5 ~ 450")

	plan := h.call("sql_explain_plan", map[string]any{"sql": "SELECT * FROM users WHERE email = 'a'"})
	if !plan.OK || len(plan.Rows) == 0 || !strings.HasPrefix(plan.SQL, "EXPLAIN ") {
		t.Fatalf("%+v", plan)
	}
	if denied := h.call("sql_explain_plan", map[string]any{"sql": "EXPLAIN ANALYZE SELECT 1"}); denied.OK || denied.ErrorCode != "guarded" {
		t.Fatalf("EXPLAIN ANALYZE would run the query: %+v", denied)
	}
}
