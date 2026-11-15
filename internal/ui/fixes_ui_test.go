package ui

import (
	"strings"
	"testing"

	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 界面修正的回归测试：标题栏不被标签撑成「»」、查询结果列头与表数据一致、行预览的 JSON 视图带高亮、切库浮层宽度随库名变化。

// 标签多到放不下时，标题栏里的连接胶囊、搜索等按钮仍在，不能整行折叠成一个「»」溢出按钮。
func TestTitleBarKeepsControlsWithManyTabs(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	for i := 0; i < 40; i++ {
		w.newQuery("SELECT " + itoa(i))
	}
	e.tt.Frame()
	e.tt.Frame()
	if !e.tt.HasText("local-sqlite") {
		t.Fatalf("标签很多时标题栏的连接胶囊不见了（整行被折进溢出菜单）: %q", e.tt.Texts())
	}
}

// 查询结果的列头和表数据一样：有类型的列带类型徽标（整数 123），表达式列只有列名。
func TestQueryHeadHasTypeBadge(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT id, name FROM users")
	e.tt.Frame()
	q.run(true)
	e.tt.Frame()
	g := q.results[0].g
	if !g.plain {
		t.Fatal("查询结果应是 plain 网格（列宽撑满整行）")
	}
	typed := model.ColumnMeta{Name: "id", Type: "INTEGER"}
	if !g.hasBadge(typed) {
		t.Error("有类型的列应画类型徽标")
	}
	if g.hasBadge(model.ColumnMeta{Name: "1+1"}) {
		t.Error("没有类型的列（表达式）不应画徽标")
	}
	if !e.tt.HasText("123") {
		t.Errorf("查询结果列头里应有整数徽标 123: %q", e.tt.Texts())
	}
	for k := range g.vis {
		if g.uiCols[k+1].Title != "" { // 第 0 列是行号 #
			t.Errorf("列头由 headerCell 自己画，Title 应为空: %q", g.uiCols[k+1].Title)
		}
	}
}

// 行预览切到 JSON 后用带语法高亮的只读编辑器，不再是一段纯文本。
func TestRowPaneJSONIsHighlighted(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	clickRow(e, 115)
	tab.togglePreview()
	e.tt.Frame()
	if err := e.tt.Click("JSON"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	e.tt.Frame()
	ed := tab.pv.jsonEd
	if ed == nil {
		t.Fatal("JSON 视图应使用编辑器（带语法高亮）")
	}
	if !ed.ReadOnly || !strings.Contains(ed.Buffer.String(), "\"id\"") {
		t.Errorf("JSON 编辑器应只读且含字段名: ro=%v %q", ed.ReadOnly, ed.Buffer.String())
	}
}

// 切库浮层的宽度：库名短时是最小宽度，库名很长时变宽，但不超过最小宽度的 1.5 倍。
func TestDbPopWidthAdapts(t *testing.T) {
	short := []model.DatabaseInfo{{Name: "a"}}
	long := []model.DatabaseInfo{{Name: strings.Repeat("x", 40)}}
	huge := []model.DatabaseInfo{{Name: strings.Repeat("x", 400)}}
	if w := dbPopWidth(short); w != sizeDbPop {
		t.Errorf("短库名应取最小宽度 %v，实际 %v", float32(sizeDbPop), w)
	}
	if w := dbPopWidth(long); w <= sizeDbPop {
		t.Errorf("长库名应比最小宽度更宽，实际 %v", w)
	}
	if w := dbPopWidth(huge); w != sizeDbPop*3/2 {
		t.Errorf("超长库名应封顶在 %v，实际 %v", float32(sizeDbPop*3/2), w)
	}
}
