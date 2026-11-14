package ui

import "testing"

// 首次使用 AI 前必须先看到数据外发说明：没确认过时拦住并不执行原操作，确认后记住并继续。
func TestAIDisclosureGate(t *testing.T) {
	e := newEnv(t)
	e.sh.prefs.AIDisclosed = false
	called := 0
	if e.sh.aiDisclosed(func() { called++ }) {
		t.Fatal("未确认时应拦住")
	}
	if called != 0 {
		t.Fatal("未确认前不能执行原操作")
	}
	e.sh.prefs.AIDisclosed = true
	if !e.sh.aiDisclosed(func() { called++ }) {
		t.Fatal("确认后应直接放行")
	}
}
