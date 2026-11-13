package ui

import (
	"testing"
)

// 没有标签时，标签栏右端的「＋」和空状态里的按钮都能新建一个 SQL 查询。
func TestNewQueryButtons(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	if len(w.tabs) != 0 {
		t.Fatalf("准备: tabs = %d", len(w.tabs))
	}
	e.has("新建查询")
	e.shot("tabs-empty")
	if err := e.tt.Click("新建查询 (⌘T)"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if len(w.tabs) != 1 {
		t.Fatalf("点空状态的按钮应新建查询标签, tabs = %d", len(w.tabs))
	}
	if _, ok := w.tabs[0].(*queryTab); !ok {
		t.Fatalf("新标签应是查询: %T", w.tabs[0])
	}
}

// 多条语句的结果用标签排开，点哪个标签就看哪个结果。
func TestResultTabsSwitch(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1 AS a; SELECT 2 AS b; SELECT 3 AS c")
	e.tt.Frame()
	q.run(true)
	e.tt.Frame()
	if len(q.results) != 3 {
		t.Fatalf("results = %d", len(q.results))
	}
	e.shot("tabs-results")
	if err := e.tt.Click(q.results[2].label(2)); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if q.shown != 2 {
		t.Fatalf("点第三个结果标签后 shown = %d", q.shown)
	}
}
