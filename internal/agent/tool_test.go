package agent

import (
	"context"
	"testing"
	"time"
)

func TestRegistryRegisterAndList(t *testing.T) {
	reg := NewToolRegistry()
	reg.MustRegister(Tool{
		Name:        "table_sample",
		Description: "sample rows",
		Parameters:  map[string]any{"type": "object"},
		Handler:     func(ctx context.Context, in ToolInput) *ToolResult { return TextResult("ok") },
		ReadOnly:    true,
	})
	reg.MustRegister(Tool{
		Name:        "sql_readonly_execute",
		Description: "exec sql",
		Parameters:  map[string]any{"type": "object"},
		Handler:     func(ctx context.Context, in ToolInput) *ToolResult { return TextResult("ok") },
		ReadOnly:    true,
	})
	got := reg.List()
	if len(got) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(got))
	}
	if got[0].Name != "table_sample" {
		t.Fatalf("order should follow registration, got %s first", got[0].Name)
	}
	if _, ok := reg.GetByName("SQL_READONLY_EXECUTE"); !ok {
		t.Fatal("lookup should be case-insensitive")
	}
}

func TestRegistryInvokeValidatesRequired(t *testing.T) {
	reg := NewToolRegistry()
	reg.MustRegister(Tool{
		Name: "needs_sql",
		Parameters: map[string]any{
			"type":     "object",
			"properties": map[string]any{
				"sql": map[string]any{"type": "string"},
			},
			"required": []string{"sql"},
		},
		Handler: func(ctx context.Context, in ToolInput) *ToolResult {
			return TextResult("ran: " + in.String("sql"))
		},
	})
	res := reg.Invoke(context.Background(), "needs_sql", ToolInput{})
	if res.OK {
		t.Fatal("missing required arg should fail")
	}
	if res.ErrorCode != "invalid_arguments" {
		t.Fatalf("expected invalid_arguments, got %s", res.ErrorCode)
	}
	ok := reg.Invoke(context.Background(), "needs_sql", ToolInput{"sql": "SELECT 1"})
	if !ok.OK {
		t.Fatalf("valid call should succeed: %+v", ok)
	}
}

func TestRegistryInvokeTimeout(t *testing.T) {
	reg := NewToolRegistry()
	reg.MustRegister(Tool{
		Name: "slow",
		Parameters: map[string]any{"type": "object"},
		Timeout:    20 * time.Millisecond,
		Handler: func(ctx context.Context, in ToolInput) *ToolResult {
			time.Sleep(200 * time.Millisecond)
			return TextResult("done")
		},
	})
	res := reg.Invoke(context.Background(), "slow", ToolInput{})
	if res.OK {
		t.Fatal("timeout should produce not-ok result")
	}
}

func TestToolResultModelText(t *testing.T) {
	r := RowsResult([]string{"id", "name"}, []map[string]any{
		{"id": 1, "name": "a"},
		{"id": 2, "name": "b"},
	}, false)
	text := r.FormatForModel(5, 120)
	if r.Kind != ResultKindRows || len(r.Rows) != 2 {
		t.Fatalf("rows result malformed: %+v", r)
	}
	for _, want := range []string{"id", "name", "a", "b"} {
		if !contains(text, want) {
			t.Fatalf("model text missing %q: %s", want, text)
		}
	}
}

func TestNormalizeArguments(t *testing.T) {
	got := NormalizeArguments(map[string]any{"tableName": "x", "topK": 5})
	if got["table_name"] != "x" {
		t.Fatalf("alias tableName not canonicalized: %+v", got)
	}
	if got["limit"] != 5 {
		t.Fatalf("alias topK not canonicalized: %+v", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}