package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 筛选浮层、WHERE 预览、列头类型徽标、视图切换的测试。

func TestWhereText(t *testing.T) {
	q := func(s string) string { return `"` + s + `"` }
	lit := func(s string) string { return "'" + s + "'" }
	f := &filterState{filters: []filterRow{
		{on: true, col: "age", op: ">", val: "30"},
		{on: true, col: "status", op: "=", val: "active"},
		{on: true, col: "my col", op: "IS NULL"},
		{on: true, col: "name", op: "="}, // 没填值：不进预览
	}}
	if got, want := f.whereText(q, lit), `WHERE age > 30 AND status = 'active' AND "my col" IS NULL`; got != want {
		t.Fatalf("预览 = %q，想要 %q", got, want)
	}
	f.filterOr = true
	if got := f.whereText(q, lit); !strings.Contains(got, " OR ") {
		t.Fatalf("或：%q", got)
	}
	f = &filterState{filters: []filterRow{{on: true, col: "id", op: "IN", val: "1,2"}, {on: true, col: locales.DatagridRawSQL(), val: "x < 9"}}}
	if got, want := f.whereText(q, lit), "WHERE id IN (1,2) AND (x < 9)"; got != want {
		t.Fatalf("预览 = %q，想要 %q", got, want)
	}
	if got := (&filterState{filters: []filterRow{newFilterRow()}}).whereText(q, lit); got != "" {
		t.Fatalf("没有条件时预览应为空: %q", got)
	}
}

func TestTypeKindOf(t *testing.T) {
	for in, want := range map[string]typeKind{
		"INTEGER": tyInt, "bigint unsigned": tyInt, "serial": tyInt,
		"DECIMAL(10,2)": tyFloat, "double precision": tyFloat, "REAL": tyFloat,
		"datetime": tyTime, "timestamp with time zone": tyTime, "DATE": tyTime,
		"VARCHAR(255)": tyText, "text": tyText, "enum('a','b')": tyText, "": tyText,
	} {
		if got := typeKindOf(in); got != want {
			t.Errorf("typeKindOf(%q) = %v，想要 %v", in, got, want)
		}
	}
}

// 列头：非 plain 的数据网格画「徽标 + 列名」，类型 INTEGER 的 id 列是 123，TEXT 的 name 列是 Aa。
func TestGridHeaderShowsTypeBadges(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	for _, want := range []string{"123", "Aa", "id", "name"} {
		if !e.tt.HasText(want) {
			t.Errorf("列头里应有 %q: %v", want, e.tt.Texts())
		}
	}
	if tab.g.plain {
		t.Fatal("表数据网格不是 plain")
	}
}

// 筛选完整流程：⌘F 弹出浮层 → 填一条 → 应用：浮层收起，工具条里出现条件芯片，网格只剩匹配的行；
// 摘掉芯片后恢复全部行。
func TestFilterPopApplyAndChips(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	all := len(tab.g.rows)
	if all < 3 {
		t.Fatalf("测试库里 users 行数不够: %d", all)
	}
	e.tt.Key(mui.Cmd, mui.KeyF)
	e.tt.Frame()
	e.tt.Frame()
	for _, want := range []string{locales.TableFilterTitle(), locales.TableFilterMatchAll(), locales.TableFilterToQuery()} {
		if !e.tt.HasText(want) {
			t.Fatalf("浮层里应有 %q: %v", want, e.tt.Texts())
		}
	}
	tab.filters[0] = filterRow{on: true, col: "id", op: "<=", val: "2"}
	e.tt.Frame()
	if !e.tt.HasText("WHERE id <= 2") {
		t.Fatalf("应有 WHERE 预览: %v", e.tt.Texts())
	}
	e.tt.Click(locales.TableFilterRun() + " ↵")
	e.tt.Frame()
	e.tt.Frame()
	if tab.showFilter {
		t.Fatal("应用后浮层应收起")
	}
	if got := tab.appliedLabels(); len(got) != 1 || got[0] != "id <= 2" {
		t.Fatalf("已生效的条件 = %v", got)
	}
	if !e.tt.HasText("id <= 2") {
		t.Fatalf("工具条里应有条件芯片: %v", e.tt.Texts())
	}
	if len(tab.g.rows) != 2 {
		t.Fatalf("应只剩 2 行: %d", len(tab.g.rows))
	}
	e.tt.Click(locales.TableFilterRemove()) // 芯片上的 ×
	e.tt.Frame()
	e.tt.Frame()
	if tab.hasApplied() || len(tab.g.rows) != all {
		t.Fatalf("摘掉芯片应恢复全部行: applied=%v rows=%d", tab.appliedLabels(), len(tab.g.rows))
	}
}

// 「取消筛选」清掉条件并收起浮层；「把筛选放进新查询」开一个带 WHERE 的查询标签，网格当前的筛选不变。
func TestFilterPopUnsetAndToQuery(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	w := tab.w
	e.tt.Key(mui.Cmd, mui.KeyF)
	e.tt.Frame()
	e.tt.Frame()
	tab.filters[0] = filterRow{on: true, col: "id", op: ">", val: "1"}
	e.tt.Frame()
	e.tt.Click(locales.TableFilterToQuery())
	e.tt.Frame()
	e.tt.Frame()
	q, ok := w.tabs[w.active].(*queryTab)
	if !ok || !strings.Contains(q.ed.Buffer.String(), "WHERE") || !strings.Contains(q.ed.Buffer.String(), `"id" > `) {
		t.Fatalf("应打开带条件的新查询: %T", w.tabs[w.active])
	}
	if tab.hasApplied() {
		t.Fatal("放进新查询不应改变网格当前的筛选")
	}
	w.active = 0
	tab.toggleFilter()
	e.tt.Frame()
	e.tt.Frame()
	e.tt.Click(locales.TableFilterRun() + " ↵")
	e.tt.Frame()
	e.tt.Frame()
	if !tab.hasApplied() {
		t.Fatal("应已生效")
	}
	tab.toggleFilter()
	e.tt.Frame()
	e.tt.Frame()
	e.tt.Click(locales.TableFilterUnset())
	e.tt.Frame()
	e.tt.Frame()
	if tab.hasApplied() || tab.showFilter {
		t.Fatal("取消筛选应清掉条件并收起浮层")
	}
}

// 视图切换：点「结构」切到结构页。
func TestViewSegSwitchesMode(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Click(locales.TableViewStructure())
	e.tt.Frame()
	if tab.mode != 1 {
		t.Fatalf("点「结构」应切到结构页: mode=%d", tab.mode)
	}
}
