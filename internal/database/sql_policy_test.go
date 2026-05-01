package database

import "testing"

func TestCheckAutoExecutableSelectSQL(t *testing.T) {
	if r := CheckAutoExecutableSelectSQL("SELECT 1"); !r.Allowed {
		t.Fatalf("select should be allowed: %+v", r)
	}
	if r := CheckAutoExecutableSelectSQL("WITH x AS (SELECT 1) SELECT * FROM x"); !r.Allowed {
		t.Fatalf("with should be allowed: %+v", r)
	}
	if r := CheckAutoExecutableSelectSQL("DELETE FROM a"); r.Allowed || r.ReasonCode != "risky_sql" {
		t.Fatalf("delete should be risky: %+v", r)
	}
	if r := CheckAutoExecutableSelectSQL(""); r.Allowed || r.ReasonCode != "empty_sql" {
		t.Fatalf("empty: %+v", r)
	}
	if r := CheckAutoExecutableSelectSQL("UNKNOWNOP x"); r.Allowed || r.ReasonCode != "unknown_sql" {
		t.Fatalf("unknown: %+v", r)
	}
}

func TestCheckAutoExecutableReadOnlySingleSQL(t *testing.T) {
	if r := CheckAutoExecutableReadOnlySingleSQL("SELECT ';' AS semi;"); !r.Allowed {
		t.Fatalf("single select with trailing semicolon should be allowed: %+v", r)
	}
	if r := CheckAutoExecutableReadOnlySingleSQL("SELECT 1; SELECT 2"); r.Allowed || r.ReasonCode != "multi_sql" {
		t.Fatalf("multiple statements should be rejected: %+v", r)
	}
	if r := CheckAutoExecutableReadOnlySingleSQL("UPDATE users SET name='x'"); r.Allowed || r.ReasonCode != "risky_sql" {
		t.Fatalf("risky statement should be rejected: %+v", r)
	}
}

func TestExtractFirstSQLFenceFromMarkdown(t *testing.T) {
	md := "text\n```sql\nSELECT 1\n```\n"
	sql, ok := ExtractFirstSQLFenceFromMarkdown(md)
	if !ok || sql != "SELECT 1" {
		t.Fatalf("got ok=%v sql=%q", ok, sql)
	}
}

func TestExtractAutoExecuteMetaBlock(t *testing.T) {
	input := "```tableplus-ai-meta\n{\"autoExecute\":{\"enabled\":true,\"mode\":\"first_sql_readonly\",\"reason\":\"user_requested_result\"}}\n```\n\n这里是正文\n```sql\nSELECT 1\n```"
	meta, cleaned, ok := ExtractAutoExecuteMetaBlock(input)
	if !ok {
		t.Fatal("should parse meta block")
	}
	if !meta.AutoExecute.Enabled || meta.AutoExecute.Mode != "first_sql_readonly" || meta.AutoExecute.Reason != "user_requested_result" {
		t.Fatalf("unexpected meta: %+v", meta)
	}
	if cleaned == "" || cleaned == input || cleaned[:3] == "```" {
		t.Fatalf("meta should be stripped from cleaned content: %q", cleaned)
	}
}

func TestDetectAutoExecuteIntent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want AutoExecuteIntentType
	}{
		{"要结果", "帮我直接查一下 users 的结果", AutoExecuteIntentNeedResults},
		{"只生成SQL", "帮我生成 users 查询 SQL", AutoExecuteIntentGenerateOnly},
		{"解释SQL", "帮我解释这个 SQL 为什么慢", AutoExecuteIntentExplainOnly},
		{"不要执行", "给我 SQL 但不要执行", AutoExecuteIntentNoExecute},
		{"未知", "users 表有哪些字段", AutoExecuteIntentUnknown},
	}
	for _, c := range cases {
		if got := DetectAutoExecuteIntent(c.in); got != c.want {
			t.Fatalf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestWantsAutoExecuteFromConversation(t *testing.T) {
	md := "这里是查询语句\n```sql\nSELECT * FROM users\n```"
	if !WantsAutoExecuteFromConversation("帮我直接查一下 users 的结果", md) {
		t.Fatal("明确要结果时应自动执行")
	}
	if WantsAutoExecuteFromConversation("帮我生成 users 查询 SQL", md) {
		t.Fatal("仅要 SQL 时不应自动执行")
	}
	if WantsAutoExecuteFromConversation("帮我解释这条 SQL", md) {
		t.Fatal("解释 SQL 时不应自动执行")
	}
	if WantsAutoExecuteFromConversation("给我 SQL 但不要执行", md) {
		t.Fatal("明确不要执行时不应自动执行")
	}
	if !WantsAutoExecuteFromConversation("帮我返回 users 的数据", md) {
		t.Fatal("明确要数据时应自动执行")
	}
	if WantsAutoExecuteFromConversation("帮我查一下 users", "没有 SQL 代码块") {
		t.Fatal("没有 SQL 代码块时不应自动执行")
	}
}
