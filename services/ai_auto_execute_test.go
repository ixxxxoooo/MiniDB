package services

import (
	"strings"
	"testing"
	"tableplus-ai/internal/database"
)

func TestBuildChatAutoExecuteDirective(t *testing.T) {
	assistant := "这里是查询\n```sql\nSELECT * FROM users\n```"
	d := buildChatAutoExecuteDirective("帮我直接查一下 users 的结果", assistant, database.AutoExecuteIntentMetaBlock{}, false)
	if !d.Enabled || d.Mode != "first_sql_readonly" || d.Reason != "user_requested_result" {
		t.Fatalf("unexpected directive: %+v", d)
	}

	d = buildChatAutoExecuteDirective("帮我生成 users 查询 SQL", assistant, database.AutoExecuteIntentMetaBlock{}, false)
	if d.Enabled {
		t.Fatalf("only ask sql should not auto execute: %+v", d)
	}

	d = buildChatAutoExecuteDirective("帮我直接查一下 users 的结果", "没有 SQL 代码块", database.AutoExecuteIntentMetaBlock{}, false)
	if d.Enabled {
		t.Fatalf("no sql fence should not auto execute: %+v", d)
	}
}

func TestBuildChatAutoExecuteDirective_UseMetaOverride(t *testing.T) {
	assistant := "这里是查询\n```sql\nSELECT * FROM users\n```"
	var meta database.AutoExecuteIntentMetaBlock
	meta.AutoExecute.Enabled = false
	meta.AutoExecute.Mode = "first_sql_readonly"
	meta.AutoExecute.Reason = "user_explicit_no_execute"
	d := buildChatAutoExecuteDirective("帮我直接查一下 users 的结果", assistant, meta, true)
	if d.Enabled {
		t.Fatalf("meta should override fallback inference: %+v", d)
	}

	meta.AutoExecute.Enabled = true
	meta.AutoExecute.Reason = "user_requested_result"
	d = buildChatAutoExecuteDirective("帮我解释这条 SQL", assistant, meta, true)
	if !d.Enabled || d.Reason != "user_requested_result" {
		t.Fatalf("meta enabled should be respected: %+v", d)
	}
}

func TestFormatAutoExecuteSkippedSuffix(t *testing.T) {
	msg := formatAutoExecuteSkippedSuffix(database.AutoExecutableCheckResult{ReasonCode: "risky_sql", Verb: "DELETE"})
	if msg == "" {
		t.Fatal("suffix should not be empty")
	}
}

// TestFormatQuerySuccessMarkdownSuffix_SortsMapKeys 无 Columns 元数据时从首行 map 取列名须稳定排序，避免表头随机
func TestFormatQuerySuccessMarkdownSuffix_SortsMapKeys(t *testing.T) {
	result := &database.QueryResult{
		Rows: []map[string]interface{}{
			{"z": 1, "a": 2},
		},
		Total:    1,
		Duration: 10,
	}
	s := formatQuerySuccessMarkdownSuffix(result, false)
	if !strings.Contains(s, "| a |") || !strings.Contains(s, "| z |") {
		t.Fatalf("expected sorted columns in table header: %s", s)
	}
	aPos := strings.Index(s, "| a |")
	zPos := strings.Index(s, "| z |")
	if aPos < 0 || zPos < 0 || aPos >= zPos {
		t.Fatalf("column a should appear before z in header: %s", s)
	}
}
