package ui

import (
	"time"

	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

// @author ixxxxoooo

// 列名一律靠左：数值列的值靠右，但表头对齐要是 Start，否则点排序出现标记时列名会被挤着动、列也会变宽。
func TestHeadersAlwaysLeftAligned(t *testing.T) {
	g := newResultGrid(true)
	g.set([]model.ColumnMeta{{Name: "id", Type: "BIGINT"}, {Name: "name", Type: "text"}},
		[]map[string]any{{"id": int64(1), "name": "a"}})
	if !g.numEnd(1) {
		t.Fatal("前置：数值列的值应靠右")
	}
	for i, col := range g.uiCols {
		if !col.RowHeader && col.Align != mui.Start {
			t.Fatalf("第 %d 列表头应靠左，实际 %v", i, col.Align)
		}
	}
}

// 列很少时最后一列不撑满屏幕，按内容定宽：数据页与查询结果一致。
func TestFewColumnsDoNotStretchToFullWidth(t *testing.T) {
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("coupons") // 只有 id、code 两列
	e.tt.Frame()
	e.tt.Advance(time.Second)
	e.tt.Frame()
	tab, ok := w.tabs[w.active].(*tableTab)
	if !ok {
		t.Fatal("当前标签应是表标签")
	}
	if sum := gridWidth(tab.g); sum > 500 {
		t.Fatalf("两列的表各列加起来应按内容定宽，不该撑满窗口：%v", sum)
	}

	q := w.newQuery("SELECT id, code FROM coupons")
	q.run(false)
	e.tt.Frame()
	e.tt.Advance(time.Second)
	e.tt.Frame()
	if len(q.results) == 0 {
		t.Fatal("查询应有结果")
	}
	if sum := gridWidth(q.results[0].g); sum > 500 {
		t.Fatalf("查询结果两列也不该撑满窗口：%v", sum)
	}
}

// gridWidth 是这一帧所有列的宽度之和。
func gridWidth(g *resultGrid) float32 {
	var sum float32
	for i := range g.uiCols {
		sum += g.colWidth(i)
	}
	return sum
}

// nameColX 是测试库 users 表第一行 name 列里的一个横坐标（表头带类型标签后列比以前宽）。
const nameColX = 500

// inkCenterX 返回矩形区域里「有墨」的像素的水平中心（相对区域背景色，取右下角那一点）；没有墨返回 -1。
func inkCenterX(tt *mui.Tester, x0, y0, x1, y1 int) float32 {
	img := tt.Image()
	bg := img.RGBAAt(x1-1, y1-1)
	lo, hi := -1, -1
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			c := img.RGBAAt(x, y)
			if abs(int(c.R)-int(bg.R))+abs(int(c.G)-int(bg.G))+abs(int(c.B)-int(bg.B)) > 60 {
				if lo < 0 {
					lo = x
				}
				hi = x
				break
			}
		}
	}
	if lo < 0 {
		return -1
	}
	return float32(lo+hi) / 2
}

// 结构页：「主键 / 自增 / 可空」三列表头居中，下面的勾选框也要在列的正中，而不是偏左。
func TestStructureCheckboxesCenteredUnderHeaders(t *testing.T) {
	e := newShopEnv(t)
	w := openWorkspace(e)
	w.openTable("orders")
	e.tt.Frame()
	e.click("结构")
	settle(e.tt)
	// 列宽被用户拖宽：勾选框这时才会暴露出「靠左」（64px 的窄列里它刚好显得居中）
	tab := w.tabs[w.active].(*tableTab)
	tab.st.colList.Columns.Widths = map[string]float32{"pk": 140, "ai": 140, "null": 140}
	settle(e.tt)
	for _, name := range []string{"主键", "自增", "可空"} {
		h, ok := e.tt.Find(name)
		if !ok {
			t.Fatalf("找不到表头 %q: %q", name, e.tt.Texts())
		}
		mid := h.X + h.W/2
		// 表头下面第一行：高 y 取表头底边下 4..24 的带，水平取表头中心左右各 30
		got := inkCenterX(e.tt, int(mid)-30, int(h.Y+h.H)+4, int(mid)+30, int(h.Y+h.H)+24)
		if got < 0 {
			t.Fatalf("%s 列下面应有勾选框", name)
		}
		if d := got - mid; d < -3 || d > 3 {
			t.Fatalf("%s 列的勾选框应在表头正下方：表头中心 %v，勾选框中心 %v", name, mid, got)
		}
	}
}
