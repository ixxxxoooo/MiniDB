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

func TestExtractAutoExecuteMetaBlock(t *testing.T) {
	input := "```minidb-meta\n{\"autoExecute\":{\"enabled\":true,\"mode\":\"first_sql_readonly\",\"reason\":\"user_requested_result\"}}\n```\n\n这里是正文\n```sql\nSELECT 1\n```"
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