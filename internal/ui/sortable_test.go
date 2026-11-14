package ui

import (
	"testing"

	"minidb/internal/app"
)

func TestSortSpecCycle(t *testing.T) {
	var s sortSpec
	s.click("avg", true)
	if s.col != "avg" || !s.desc {
		t.Fatalf("数值列第一次点击应降序: %+v", s)
	}
	s.click("avg", true)
	if s.col != "avg" || s.desc {
		t.Fatalf("第二次点击应翻转为升序: %+v", s)
	}
	s.click("avg", true)
	if s.col != "" {
		t.Fatalf("第三次点击应回到原顺序: %+v", s)
	}
	s.click("db", false)
	if s.col != "db" || s.desc {
		t.Fatalf("文字列第一次点击应升序: %+v", s)
	}
	s.click("calls", true)
	if s.col != "calls" || !s.desc {
		t.Fatalf("换列应从该列的默认方向开始: %+v", s)
	}
}

func slowSample() []app.SlowQuery {
	return []app.SlowQuery{
		{SQL: "SELECT a FROM t1", Database: "shop", Calls: 10, AvgMs: 5, MaxMs: 50, Rows: 3, LastSeen: "2026-11-14 10:00:00"},
		{SQL: "SELECT b FROM t2", Database: "blog", Calls: 900, AvgMs: 1, MaxMs: 9, Rows: -1, LastSeen: "2026-11-14 12:00:00"},
		{SQL: "SELECT c FROM t3", Database: "app", Calls: 90, AvgMs: 300, MaxMs: 400, Rows: 70, LastSeen: "2026-11-14 11:00:00"},
	}
}

func slowOrder(st *slowState) string {
	out := ""
	for _, it := range st.items {
		out += it.SQL[7:8]
	}
	return out
}

// 点慢查询的列头：数值按大小而不是按字符串排；选中的行跟着语句走；再点翻转，第三次回到原顺序。
func TestSlowSortByColumn(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{}, mon: monitorPanel{kind: monSlow, slow: slowState{loaded: true, sel: 0, items: slowSample()}}}
	st := &e.sh.proc.mon.slow
	e.tt.Frame()
	if err := e.tt.Click("次数"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if got := slowOrder(st); got != "bca" {
		t.Fatalf("按次数降序应为 b(900) c(90) a(10)，实际 %s", got)
	}
	if it, ok := st.selectedSlow(); !ok || it.SQL != "SELECT a FROM t1" {
		t.Fatalf("选中的应仍是原来那条语句: %+v ok=%v", it, ok)
	}
	e.tt.Click("次数")
	e.tt.Frame()
	if got := slowOrder(st); got != "acb" {
		t.Fatalf("再点一次升序应为 acb，实际 %s", got)
	}
	e.tt.Click("次数")
	e.tt.Frame()
	if got := slowOrder(st); got != "abc" {
		t.Fatalf("第三次点击应回到原顺序 abc，实际 %s", got)
	}
	e.tt.Click("库")
	e.tt.Frame()
	if got := slowOrder(st); got != "cba" { // app < blog < shop
		t.Fatalf("按库升序应为 cba，实际 %s", got)
	}
	e.tt.Click("平均")
	e.tt.Frame()
	if got := slowOrder(st); got != "cab" {
		t.Fatalf("按平均耗时降序应为 cab，实际 %s", got)
	}
	e.shot("62-monitor-slow-sorted")
}

// 进程列表的列头：耗时按数值排，ID 按数值排（9 在 10 前面）。
func TestProcessSortByColumn(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.proc = processDialog{open: true, w: w, sel: map[string]bool{}, items: []app.ProcessInfo{
		{ID: "10", User: "root", Database: "shop", TimeSec: 3, Info: "SELECT * FROM orders"},
		{ID: "9", User: "app", Database: "shop", TimeSec: 41, Info: "UPDATE users SET age = age + 1"},
		{ID: "100", User: "app", Database: "blog", TimeSec: 0, Info: "SELECT 1"},
	}}
	d := &e.sh.proc
	ids := func() string {
		out := ""
		for _, it := range d.visible() {
			out += it.ID + ","
		}
		return out
	}
	e.tt.Frame()
	if err := e.tt.Click("ID"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if got := ids(); got != "9,10,100," {
		t.Fatalf("ID 应按数值升序，实际 %s", got)
	}
	e.tt.Click("耗时")
	e.tt.Frame()
	if got := ids(); got != "9,10,100," {
		t.Fatalf("耗时降序应为 41、3、0，实际 %s", got)
	}
	e.tt.Click("耗时")
	e.tt.Frame()
	if got := ids(); got != "100,10,9," {
		t.Fatalf("耗时升序应为 0、3、41，实际 %s", got)
	}
	e.shot("51-process-sorted")
}
