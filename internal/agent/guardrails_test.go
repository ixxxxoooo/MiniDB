package agent

import "testing"

func TestGuardrailCheckReadOnlySQL(t *testing.T) {
	g := NewDefaultGuardrailChain()

	cases := []struct {
		sql      string
		denied   bool
		reason   string
	}{
		{"SELECT * FROM users", false, ""},
		{"select id, name from orders where id > 1", false, ""},
		{"WITH t AS (SELECT 1) SELECT * FROM t", false, ""},
		{"SHOW TABLES", false, ""},
		{"EXPLAIN SELECT * FROM users", false, ""},
		{"", true, "empty_sql"},
		{"SELECT 1; DROP TABLE users", true, "multi_sql"},
		{"UPDATE users SET x=1", true, "non_readonly_sql"},
		{"DELETE FROM users", true, "non_readonly_sql"},
		{"INSERT INTO users VALUES (1)", true, "non_readonly_sql"},
		{"CREATE TABLE t (id int)", true, "non_readonly_sql"},
		{"SELECT SLEEP(10)", true, "danger_function"},
		{"SELECT pg_sleep(5)", true, "danger_function"},
		{"SELECT LOAD_FILE('/etc/passwd')", true, "danger_function"},
		{"SELECT * FROM users INTO OUTFILE '/tmp/x.csv'", true, "danger_function"},
		{"TRUNCATE TABLE users", true, "non_readonly_sql"},
	}
	for _, c := range cases {
		denied, reason, verb := g.CheckReadOnlySQL(c.sql)
		if denied != c.denied || (denied && reason != c.reason) {
			t.Fatalf("sql=%q => denied=%v reason=%q verb=%q; want denied=%v reason=%q",
				c.sql, denied, reason, verb, c.denied, c.reason)
		}
	}
}

func TestGuardrailQuoteAwareSemicolon(t *testing.T) {
	g := NewDefaultGuardrailChain()
	// 引号内的分号不应算多语句
	for _, sql := range []string{
		`SELECT * FROM users WHERE name = 'a;b'`,
		`SELECT "a;b" FROM t`,
	} {
		denied, reason, _ := g.CheckReadOnlySQL(sql)
		if denied && reason == "multi_sql" {
			t.Fatalf("sql=%q should not be multi_sql", sql)
		}
	}
}

func TestGuardrailBuildReadOnlyGuard(t *testing.T) {
	g := NewDefaultGuardrailChain()
	guard := g.BuildReadOnlyGuard()

	ok := guard(nil, &RunContext{ToolArgs: map[string]any{"sql": "SELECT 1"}}, StepCall{Name: "sql_readonly_execute"})
	if ok.Denied {
		t.Fatalf("SELECT should pass: %+v", ok)
	}

	denied := guard(nil, &RunContext{ToolArgs: map[string]any{"sql": "DROP TABLE x"}}, StepCall{Name: "sql_readonly_execute"})
	if !denied.Denied {
		t.Fatal("DROP should be denied")
	}

	// 非 SQL 工具不受该护栏约束
	other := guard(nil, &RunContext{}, StepCall{Name: "table_sample"})
	if other.Denied {
		t.Fatal("table_sample should not be guarded by read-only guard")
	}
}