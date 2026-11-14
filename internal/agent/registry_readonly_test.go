package agent

import (
	"context"
	"testing"
)

// 没声明只读的工具即使注册了，助手也调不动。
func TestInvokeRejectsNonReadOnlyTool(t *testing.T) {
	reg := NewToolRegistry()
	called := false
	reg.MustRegister(Tool{Name: "danger", Parameters: Object(nil), Handler: func(context.Context, ToolInput) *ToolResult {
		called = true
		return Text("x", "x")
	}})
	res := reg.Invoke(context.Background(), "danger", ToolInput{})
	if res.OK || res.ErrorCode != "not_read_only" || called {
		t.Fatalf("应拒绝且不执行: %+v called=%v", res, called)
	}
}
