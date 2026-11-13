package app

import (
	"fmt"
	"strings"
	"testing"

	"minidb/internal/ai"
)

func tablesN(n int) []ai.TableSchema {
	out := make([]ai.TableSchema, n)
	for i := range out {
		out[i] = ai.TableSchema{Name: fmt.Sprintf("t%03d", i), Comment: "表", Columns: []ai.ColumnSchema{{Name: "id", Type: "bigint", IsPrimary: true}, {Name: "name", Type: "varchar(64) utf8"}}}
	}
	return out
}

func TestOverviewIsDeterministicAndScalesWithDatabaseSize(t *testing.T) {
	small := &ai.SchemaContext{Tables: tablesN(3)}
	a, b := buildSchemaOverview(small), buildSchemaOverview(small)
	if a != b || !strings.Contains(a, "t001（表）: id bigint PK, name varchar(64)") {
		t.Fatalf("a small database lists columns, identically every turn (prompt-cache friendly):\n%s", a)
	}
	large := buildSchemaOverview(&ai.SchemaContext{Tables: tablesN(350)})
	if strings.Contains(large, "bigint") || !strings.Contains(large, "另有 50 张表") {
		t.Fatalf("a large database lists names only and says what was left out:\n%.200s", large)
	}
	if buildSchemaOverview(&ai.SchemaContext{}) != "" || buildSchemaOverview(nil) != "" {
		t.Fatal("no tables, no overview")
	}
}

func TestSystemPromptStructure(t *testing.T) {
	p := buildSystemPrompt(promptInfo{dbName: "shop", dbType: "tidb", dbVersion: "v7.5", overview: "- orders: id bigint PK\n"})
	for _, want := range []string{"只读权限", "ERROR", "minidb-next-steps", "库名: shop", "tidb", "GROUP_CONCAT", "库结构总览", "- orders: id bigint PK"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if strings.Index(p, "库结构总览") < strings.Index(p, "工作方式") || strings.Index(p, "当前数据库") < strings.Index(p, "回答") {
		t.Error("stable rules come first, per-database content last, so the prefix caches")
	}
	for _, stale := range []string{"table_fuzzy_match", "column_fuzzy_match", "table_ddl", "table_relationships", "[SQL_ERROR]", "minidb-meta"} {
		if strings.Contains(p, stale) {
			t.Errorf("prompt still mentions removed %q", stale)
		}
	}
	if none := buildSystemPrompt(promptInfo{}); !strings.Contains(none, "不能调用工具") {
		t.Error("without a schema the model is told it has no tools")
	}
	if empty := buildSystemPrompt(promptInfo{dbType: "sqlite", dbName: "x"}); !strings.Contains(empty, "没有读到任何表") {
		t.Error("empty database is stated")
	}
}

func TestTrimHistory(t *testing.T) {
	var msgs []ai.ChatMessage
	for i := 0; i < 20; i++ {
		msgs = append(msgs, ai.ChatMessage{Role: "user", Content: fmt.Sprint("q", i)}, ai.ChatMessage{Role: "assistant", Content: fmt.Sprint("a", i)})
	}
	got := trimHistory(msgs)
	if len(got) != historyMaxMessages || got[len(got)-1].Content != "a19" || got[0].Content != "q14" {
		t.Fatalf("keeps the latest %d in order: first=%q last=%q n=%d", historyMaxMessages, got[0].Content, got[len(got)-1].Content, len(got))
	}

	long := strings.Repeat("长", 10000)
	got = trimHistory([]ai.ChatMessage{{Role: "user", Content: "q"}, {Role: "assistant", Content: long}, {Role: "user", Content: "再问"}})
	if len([]rune(got[1].Content)) > historyOldAnswerChars+20 || got[2].Content != "再问" {
		t.Fatalf("an old long answer is shortened, the new question is not: %d", len([]rune(got[1].Content)))
	}

	huge := strings.Repeat("问", historyMaxChars+500)
	got = trimHistory([]ai.ChatMessage{{Role: "user", Content: "old"}, {Role: "assistant", Content: "ok"}, {Role: "user", Content: huge}})
	if len(got) != 1 || len([]rune(got[0].Content)) > historyMaxChars+10 {
		t.Fatalf("the latest question is always kept, cut to budget: n=%d", len(got))
	}
}
