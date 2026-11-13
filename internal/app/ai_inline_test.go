package app

import (
	"strings"
	"testing"
)

func TestExtractSQL(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"围栏里的 SQL", "```sql\nSELECT 1\n```", "SELECT 1"},
		{"围栏外有废话", "好的，如下：\n```sql\nSELECT 1;\n```\n希望有帮助", "SELECT 1;"},
		{"没有围栏就取全文", "  SELECT 2  ", "SELECT 2"},
		{"流式：语言标记还没收完", "```s", ""},
		{"流式：围栏没闭合", "```sql\nSELECT id,\n  name", "SELECT id,\n  name"},
		{"流式：结尾围栏只到了一半", "```sql\nSELECT 1\n``", "SELECT 1"},
		{"带注释的多行", "```sql\n-- 假设：近 7 天\nSELECT 1\n```", "-- 假设：近 7 天\nSELECT 1"},
		{"没有语言标记", "```\nSELECT 3\n```", "SELECT 3"},
	}
	for _, c := range cases {
		if got := ExtractSQL(c.in); got != c.want {
			t.Errorf("%s: ExtractSQL(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestBuildInlineMessage(t *testing.T) {
	gen := buildInlineMessage(InlineSQLRequest{Instruction: "统计每个状态的订单数"})
	if gen != "要求：统计每个状态的订单数" {
		t.Fatalf("生成新语句只带要求: %q", gen)
	}
	fix := buildInlineMessage(InlineSQLRequest{Instruction: "修好它", SQL: "SELECT * FORM t", Error: "syntax error near FORM"})
	for _, want := range []string{"要求：修好它", "```sql\nSELECT * FORM t\n```", "syntax error near FORM"} {
		if !strings.Contains(fix, want) {
			t.Fatalf("缺少 %q: %s", want, fix)
		}
	}
	if opt := buildInlineMessage(InlineSQLRequest{SQL: "SELECT 1"}); !strings.Contains(opt, "优化这条语句") {
		t.Fatalf("改写没写要求时默认优化: %s", opt)
	}
}

func TestRelevantTablesPrefersMentionedThenSqlThenSearch(t *testing.T) {
	h := newAIHarness(t)
	got := relevantTables(h.env.schema, h.env.search, InlineSQLRequest{
		Instruction: "orders status", SQL: "SELECT * FROM order_items", Tables: []string{"USERS", "nope"},
	}, 3)
	var names []string
	for _, t := range got {
		names = append(names, t.Name)
	}
	if len(names) != 3 || names[0] != "users" || names[1] != "order_items" || names[2] != "orders" {
		t.Fatalf("顺序应是点名 → 语句里的 → 检索到的，且不重复、不存在的忽略: %v", names)
	}
	if len(relevantTables(h.env.schema, h.env.search, InlineSQLRequest{Tables: []string{"users", "orders", "order_items"}}, 2)) != 2 {
		t.Fatal("要受 limit 限制")
	}
}

func TestGenerateSQLStreamsAndSendsRelevantSchema(t *testing.T) {
	h := newAIHarness(t)
	srv, bodies := gateway(t, func(n int, body string, s *sse) {
		s.text("```sql\n-- 假设：按状态分组\n")
		s.text("SELECT status, COUNT(*) FROM orders GROUP BY status\n```")
		s.done("r1")
	})
	h.useGateway(t, srv.URL)

	var streamed strings.Builder
	sql, err := h.a.AI.GenerateSQL(h.connID, h.db, InlineSQLRequest{Instruction: "统计每个状态的订单数", Tables: []string{"orders"}}, "inline-1",
		func(d string) { streamed.WriteString(d) })
	if err != nil {
		t.Fatal(err)
	}
	if sql != "-- 假设：按状态分组\nSELECT status, COUNT(*) FROM orders GROUP BY status" {
		t.Fatalf("sql: %q", sql)
	}
	if !strings.Contains(streamed.String(), "GROUP BY status") {
		t.Fatalf("增量应该回调给调用方: %q", streamed.String())
	}
	if len(*bodies) != 1 {
		t.Fatalf("内联生成只请求一次模型，不带工具: %d", len(*bodies))
	}
	body := (*bodies)[0]
	for _, want := range []string{"只输出一个", "CREATE TABLE orders", "统计每个状态的订单数"} {
		if !strings.Contains(body, want) {
			t.Fatalf("请求里缺少 %q", want)
		}
	}
	if strings.Contains(body, `"tools"`) {
		t.Fatal("内联生成不该带工具")
	}
}

func TestGenerateSQLRejectsEmptyAndEmptyAnswers(t *testing.T) {
	h := newAIHarness(t)
	if _, err := h.a.AI.GenerateSQL(h.connID, h.db, InlineSQLRequest{Instruction: "  "}, "x", nil); err == nil {
		t.Fatal("没有要求也没有语句应报错")
	}
	srv, _ := gateway(t, func(n int, body string, s *sse) { s.text("```sql\n```"); s.done("r") })
	h.useGateway(t, srv.URL)
	if _, err := h.a.AI.GenerateSQL(h.connID, h.db, InlineSQLRequest{Instruction: "随便"}, "x", nil); err == nil {
		t.Fatal("模型给了空 SQL 应报错，而不是插入空内容")
	}
}
