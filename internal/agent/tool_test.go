package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func echoTool(name string, handler ToolHandler) Tool {
	return Tool{
		Name:        name,
		Description: "test tool",
		Parameters: Object(map[string]Prop{
			"table_names": StringList("要查看的表", 5),
			"limit":       Optional(Integer("返回行数", 1, 50)),
			"mode":        Optional(Enum("格式", "brief", "full")),
		}),
		Handler:  handler,
		ReadOnly: true,
	}
}

func TestSchemaIsStrictCompatible(t *testing.T) {
	s := echoTool("t", nil).Parameters
	if s["additionalProperties"] != false {
		t.Fatal("strict mode needs additionalProperties=false")
	}
	if req := requiredNames(s); len(req) != 3 {
		t.Fatalf("strict mode needs every property in required (optional ones are nullable): %v", req)
	}
	limit := s["properties"].(map[string]any)["limit"].(map[string]any)
	if !allowsNull(limit) || typeOf(limit) != "integer" {
		t.Fatalf("optional limit must be nullable integer: %v", limit["type"])
	}
}

func TestRegistryRegisterAndOrder(t *testing.T) {
	r := NewToolRegistry()
	ok := func(context.Context, ToolInput) *ToolResult { return Text("", "ok") }
	r.MustRegister(echoTool("b_tool", ok))
	r.MustRegister(echoTool("a_tool", ok))
	r.MustRegister(echoTool("b_tool", ok)) // 同名覆盖，不改变顺序
	list := r.List()
	if len(list) != 2 || list[0].Name != "b_tool" || list[1].Name != "a_tool" {
		t.Fatalf("registration order must be kept: %+v", list)
	}
	if _, ok := r.Get("  A_TOOL "); !ok {
		t.Fatal("lookup ignores case and spaces")
	}
	if r.Register(Tool{Name: ""}) == nil || r.Register(Tool{Name: "x"}) == nil {
		t.Fatal("empty name and nil handler must be rejected")
	}
}

func TestInvokeValidationMessagesAreActionable(t *testing.T) {
	r := NewToolRegistry()
	called := 0
	r.MustRegister(echoTool("t", func(context.Context, ToolInput) *ToolResult { called++; return Text("", "ok") }))
	cases := []struct {
		name  string
		input ToolInput
		want  string
	}{
		{"missing required", ToolInput{}, `缺少必填参数 "table_names"`},
		{"unknown key names the allowed ones", ToolInput{"table_names": []any{"a"}, "where": "x"}, "只接受: limit, mode, table_names"},
		{"wrong type", ToolInput{"table_names": "users"}, "必须是字符串数组"},
		{"bad element", ToolInput{"table_names": []any{1}}, "元素必须是字符串"},
		{"bad enum lists choices", ToolInput{"table_names": []any{"a"}, "mode": "huge"}, "brief / full"},
		{"fractional integer", ToolInput{"table_names": []any{"a"}, "limit": 2.5}, "必须是整数"},
	}
	for _, c := range cases {
		res := r.Invoke(context.Background(), "t", c.input)
		if res.OK || res.ErrorCode != "invalid_arguments" || !strings.Contains(res.Text, c.want) {
			t.Errorf("%s: %+v", c.name, res)
		}
	}
	if called != 0 {
		t.Fatal("invalid calls must never reach the handler")
	}
	res := r.Invoke(context.Background(), "t", ToolInput{"table_names": []any{"a"}, "limit": 10.0})
	if !res.OK || res.ToolName != "t" || called != 1 {
		t.Fatalf("valid call: %+v", res)
	}
}

func TestInvokeUnknownToolListsAvailable(t *testing.T) {
	r := NewToolRegistry()
	r.MustRegister(echoTool("real", func(context.Context, ToolInput) *ToolResult { return Text("", "") }))
	res := r.Invoke(context.Background(), "ghost", nil)
	if res.OK || res.ErrorCode != "unknown_tool" || !strings.Contains(res.Hint, "real") {
		t.Fatalf("%+v", res)
	}
}

func TestInvokeTimeoutPanicAndCancel(t *testing.T) {
	r := NewToolRegistry()
	slow := echoTool("slow", func(ctx context.Context, _ ToolInput) *ToolResult {
		<-ctx.Done()
		return Text("", "late")
	})
	slow.Timeout = 20 * time.Millisecond
	r.MustRegister(slow)
	r.MustRegister(echoTool("boom", func(context.Context, ToolInput) *ToolResult { panic("bad") }))
	in := ToolInput{"table_names": []any{"a"}}

	if res := r.Invoke(context.Background(), "slow", in); res.ErrorCode != "timeout" || res.Hint == "" {
		t.Fatalf("timeout: %+v", res)
	}
	if res := r.Invoke(context.Background(), "boom", in); res.ErrorCode != "internal" {
		t.Fatalf("a panicking tool must not crash the run: %+v", res)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res := r.Invoke(ctx, "slow", in); res.OK || res.ErrorCode != "cancelled" {
		t.Fatalf("cancel: %+v", res)
	}
}

func TestNormalizeArguments(t *testing.T) {
	got := NormalizeArguments(map[string]any{"tableNames": []any{"a"}, "limit": nil, " sql ": "x"})
	if _, ok := got["table_names"]; !ok || got["sql"] != "x" {
		t.Fatalf("camelCase keys become snake_case and keys are trimmed: %v", got)
	}
	if _, ok := got["limit"]; ok {
		t.Fatal("null means not provided")
	}
}

func TestFormatForModel(t *testing.T) {
	rows := Rows([]string{"id", "note"}, []map[string]any{{"id": 1, "note": "a|b\nc"}, {"id": 2, "note": nil}, {"id": 3, "note": ""}}, 3, "")
	text := rows.FormatForModel(50, 200)
	for _, want := range []string{"共 3 行", "| id | note |", `a\|b c`, "NULL", "''"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}

	partial := Rows([]string{"n"}, []map[string]any{{"n": 1}, {"n": 2}}, 900, "")
	if !partial.Truncated || !strings.Contains(partial.FormatForModel(1, 200), "共 900 行，仅展示前 1 行") {
		t.Fatalf("a truncated result must say how much the model did not see:\n%s", partial.FormatForModel(1, 200))
	}
	if got := FormatCell("你好世界你好", 4); got != "你好世界…" {
		t.Fatalf("cells are cut by characters, not bytes: %q", got)
	}

	fail := Fail("guarded", "只读", "改用 SELECT")
	if got := fail.FormatForModel(50, 200); got != "ERROR: 只读\n建议: 改用 SELECT" {
		t.Fatalf("error text = %q", got)
	}
	custom := &ToolResult{OK: true, Kind: ResultKindRows, Rows: []map[string]any{{"a": 1}}, ModelText: "紧凑文本"}
	if custom.FormatForModel(50, 200) != "紧凑文本" {
		t.Fatal("ModelText wins over the table rendering")
	}
}
