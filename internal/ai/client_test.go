package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseDSMLFunctionCallsAcceptsToolCallsAlias(t *testing.T) {
	content := strings.Join([]string{
		"planning text",
		`< | DSML | tool_calls>`,
		`< | DSML | invoke name="table_stats">`,
		`< | DSML | parameter name="tables" string="false">["tblA","tblB"]</ | DSML | parameter>`,
		`</ | DSML | invoke>`,
		`</ | DSML | tool_calls>`,
	}, "\n")

	calls := parseDSMLFunctionCalls(content)
	if len(calls) != 1 {
		t.Fatalf("expected one call, got %d", len(calls))
	}
	if calls[0].Name != "table_stats" {
		t.Fatalf("expected table_stats, got %q", calls[0].Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatalf("arguments should be json: %v", err)
	}
	tables, ok := args["tables"].([]any)
	if !ok || len(tables) != 2 || tables[0] != "tblA" || tables[1] != "tblB" {
		t.Fatalf("unexpected tables argument: %#v", args["tables"])
	}
}

func TestSanitizeThinkingFallbackRemovesToolCallsAlias(t *testing.T) {
	content := strings.Join([]string{
		"before",
		`< | DSML | tool_calls>`,
		`< | DSML | invoke name="table_stats">`,
		`< | DSML | parameter name="tables" string="false">["tblA","tblB"]</ | DSML | parameter>`,
		`</ | DSML | invoke>`,
		`</ | DSML | tool_calls>`,
		"after",
	}, "\n")

	output := sanitizeThinkingFallback(content)
	if strings.Contains(output, "DSML") || strings.Contains(output, "tool_calls") || strings.Contains(output, "tblA") {
		t.Fatalf("protocol leaked into sanitized output: %q", output)
	}
	if !strings.Contains(output, "before") || !strings.Contains(output, "after") {
		t.Fatalf("sanitized output should keep surrounding text, got %q", output)
	}
}
