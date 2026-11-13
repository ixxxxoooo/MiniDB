package ui

import (
	"strings"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"
)

func TestMultipleFiltersApplyTogether(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.filters = []filterRow{
		{on: true, col: "age", op: ">=", val: "55"},
		{on: true, col: "name", op: "LIKE", val: "%9"},
		{on: false, col: "id", op: "=", val: "1"}, // unticked: ignored
	}
	tab.applyFilter()
	if len(tab.applied) != 2 {
		t.Fatalf("two ticked conditions apply, got %d", len(tab.applied))
	}
	// age = 20 + i%40 >= 55 means i%40 >= 35; names ending in 9
	for _, r := range tab.g.rows {
		age, _ := r["age"].(int64)
		if age < 55 {
			t.Fatalf("row %v does not satisfy age >= 55", r)
		}
	}
	if len(tab.g.rows) == 0 {
		t.Fatal("expected rows to match both conditions")
	}
	all := len(tab.g.rows)

	tab.filters[1].on = false
	tab.applyFilter()
	if len(tab.g.rows) <= all {
		t.Fatalf("unticking a condition widens the result: %d then %d", all, len(tab.g.rows))
	}
	tab.clearFilter()
	if len(tab.applied) != 0 || len(tab.filters) != 1 || tab.filters[0].val != "" {
		t.Fatalf("Unset resets the filter bar: %+v", tab.filters)
	}
}

func TestFilterBarAddRemoveAndApplyShortcut(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Key(mui.Cmd, mui.KeyF)
	e.tt.Frame()
	e.tt.Frame()
	tab.filters = append(tab.filters, newFilterRow())
	tab.filters[0] = filterRow{on: true, col: "id", op: "=", val: "7"}
	e.tt.Frame()
	e.tt.Key(mui.Cmd, mui.KeyEnter)
	e.tt.Frame()
	if len(tab.g.rows) != 1 {
		t.Fatalf("⌘↵ applies the filters: %d rows", len(tab.g.rows))
	}
}

func TestHiddenColumnsLeaveTheGrid(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	e.has("email")
	tab.g.setHidden("email", true)
	e.tt.Frame()
	e.hasNot("email")
	e.has("name")
	// a copy leaves hidden columns out
	b := tab.g.selected()
	for _, c := range b.cols {
		if c == "email" {
			t.Fatal("a hidden column must not be in the selection")
		}
	}
	tab.g.setHidden("email", false)
	e.tt.Frame()
	e.has("email")
}

func clickRow(e *env, y float32) {
	e.tt.ClickAt(450, y) // the name column of a grid row: focuses the grid and chooses the row
	e.tt.Frame()
}

func TestGridKeyboardCopyPasteDuplicate(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	clickRow(e, 100) // row 1
	if tab.g.chosen.Len() != 1 {
		t.Fatal("click chooses a row")
	}

	// ⌘C copies the row, every column shown, tab separated
	e.tt.Key(mui.Cmd, mui.KeyC)
	e.tt.Frame()
	got := e.tt.Clipboard()
	if got == "" || !strings.Contains(got, "\t") || !strings.Contains(got, "用户001") {
		t.Fatalf("⌘C must copy the row as tab-separated text, got %q", got)
	}

	// ⌘V pastes it as a new row
	before := len(tab.g.rows)
	e.tt.Key(mui.Cmd, mui.KeyV)
	e.tt.Frame()
	if len(tab.g.rows) != before+1 || tab.es.added != 1 {
		t.Fatalf("⌘V adds the clipboard's row: rows %d -> %d", before, len(tab.g.rows))
	}
	last := tab.g.rows[len(tab.g.rows)-1]
	if last["name"] != "用户001" {
		t.Fatalf("pasted row = %v", last)
	}

	// ⌘D duplicates the chosen row; the primary key is left to the database
	tab.g.chosen.Clear()
	tab.g.chosen.Add(0)
	e.tt.Key(mui.Cmd, mui.KeyD)
	e.tt.Frame()
	dup := tab.g.rows[len(tab.g.rows)-1]
	if tab.es.added != 2 || dup["name"] != "用户001" || dup["id"] != nil {
		t.Fatalf("⌘D duplicates the row without its key: added=%d %v", tab.es.added, dup)
	}
}

func TestGridKeyboardNavigation(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	clickRow(e, 100)
	e.tt.Key(mui.Cmd, mui.KeyDown)
	e.tt.Frame()
	if r := tab.previewRow(); r != len(tab.g.rows)-1 {
		t.Fatalf("⌘↓ chooses the last row, got %d", r)
	}
	e.tt.Key(mui.Cmd, mui.KeyUp)
	e.tt.Frame()
	if r := tab.previewRow(); r != 0 {
		t.Fatalf("⌘↑ chooses the first row, got %d", r)
	}
	e.tt.Key(mui.Cmd, mui.KeyRight)
	e.tt.Frame()
	if tab.page != 2 {
		t.Fatalf("⌘→ turns to the next page, page=%d", tab.page)
	}
	e.tt.Key(mui.Cmd, mui.KeyLeft)
	e.tt.Frame()
	if tab.page != 1 {
		t.Fatalf("⌘← turns back, page=%d", tab.page)
	}
}

func TestReturnEditsAndSetValue(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	clickRow(e, 124)
	e.tt.Key(0, mui.KeyTab)
	e.tt.Frame()
	if tab.es.editing == nil {
		t.Fatal("Tab edits the cell")
	}
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()

	row, col, ok := tab.cursorCell()
	if !ok {
		t.Fatal("a chosen row has a cursor cell")
	}
	tab.setQuickValue(row, "email", "NULL")
	if tab.pendingCount() != 1 || tab.g.rows[row]["email"] != nil {
		t.Fatalf("Set Value NULL: pending=%d (%s)", tab.pendingCount(), col)
	}
	tab.setQuickValue(row, "bio", "EMPTY")
	if tab.g.rows[row]["bio"] != "" {
		t.Fatalf("Set Value EMPTY: %v", tab.g.rows[row]["bio"])
	}
	if got := quickValues("DATETIME"); got[len(got)-1] != "NOW()" {
		t.Fatalf("a date column offers NOW(): %v", got)
	}
}

func TestRowPaneEditsInPlaceAndAcrossRows(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	clickRow(e, 100)
	tab.showPreview, tab.previewW = true, 340
	e.tt.Frame()
	e.tt.Frame()
	e.shot("92-row-pane")
	// edit a field in the pane
	e.tt.ClickAt(1200, 289) // the email field of the pane
	e.tt.Type("X")
	e.tt.Frame()
	if tab.pendingCount() != 1 {
		t.Fatalf("typing in a field of the row pane is a pending edit, got %d", tab.pendingCount())
	}
	tab.commitNow()

	// several rows: one field set across them
	tab.g.chosen.Clear()
	tab.g.chosen.Add(0)
	tab.g.chosen.Add(1)
	tab.g.chosen.Add(2)
	e.tt.Frame()
	rows := tab.chosenRows()
	if _, _, mixed := tab.fieldValue(rows, "name"); !mixed {
		t.Fatal("different names across rows are 'multiple values'")
	}
	for _, r := range rows {
		tab.setCell(r, "bio", "same")
	}
	if text, _, mixed := tab.fieldValue(rows, "bio"); mixed || text != "same" {
		t.Fatalf("one value across rows: %q mixed=%v", text, mixed)
	}
}

func TestQuickLookEditsLongTextAndShowsJSONTree(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.setCell(0, "bio", `{"a":{"b":[1,2,{"c":true}]},"n":null}`)
	e.tt.Frame()
	tab.openQuickLook(0, "bio")
	e.tt.Frame()
	if !tab.qlOpen || !tab.ql.isJSON {
		t.Fatal("JSON opens in Quick Look as JSON")
	}
	e.has("树")
	e.shot("91-quicklook-raw0") // 默认是缩进后的原文，带语法高亮
	tab.ql.tree = true
	e.tt.Frame()
	e.shot("91-quicklook-tree")
	tab.ql.tree = false
	e.tt.Frame()
	e.shot("91-quicklook-raw") // 原文模式带 JSON 语法高亮
	tab.ql.setText("plain edited")
	e.tt.Frame()
	e.tt.Click("应用")
	e.tt.Frame()
	if tab.g.rows[0]["bio"] != "plain edited" || tab.qlOpen {
		t.Fatalf("Apply stores the text and closes: %v open=%v", tab.g.rows[0]["bio"], tab.qlOpen)
	}
}

// 快速查看认出时间戳后，列出本地时间、UTC、相对时间等可读写法。
func TestQuickLookShowsReadableTimestamp(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.setCell(0, "bio", "1700000000")
	e.tt.Frame()
	tab.openQuickLook(0, "bio")
	e.tt.Frame()
	e.has("Unix 时间戳（秒）")
	e.has("可读格式")
	e.has("2023-11-14 22:13:20")
	e.shot("92-quicklook-timestamp")
}

// 单元格的三种状态截图：选中、选中一块、正在编辑。
func TestCellStatesShot(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	tab.setCell(1, "name", "已改动")
	e.tt.Frame()
	e.tt.Click("user3@example.com")
	e.tt.Frame()
	e.shot("93-cell-selected")
	tab.beginEdit(2, "name")
	e.tt.Frame()
	e.shot("93-cell-editing")
}

// 滚动条默认只在滚动时出现，停下一会儿就淡出；"始终显示"则一直在。
func TestScrollbarsFadeWhenIdle(t *testing.T) {
	e := newEnv(t)
	openUsers(e)
	e.tt.Frame()
	right := func() int { // 表格右缘一条窄带里与背景不同的像素数
		img := e.tt.Image()
		n := 0
		for y := 120; y < 700; y++ {
			for x := 1270; x < 1280; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r>>8 < 0xe8 || g>>8 < 0xe8 || b>>8 < 0xe8 {
					n++
				}
			}
		}
		return n
	}
	barsMode = barsOnScroll
	t.Cleanup(func() { barsMode = barsOnScroll })
	e.tt.Wheel(700, 400, 0, 240)
	e.tt.Advance(400 * time.Millisecond)
	e.tt.Frame()
	e.shot("94-scrollbar-scrolling")
	moving := right()
	time.Sleep(1600 * time.Millisecond)
	e.tt.Frame()
	e.tt.Frame()
	e.shot("94-scrollbar-idle")
	idle := right()
	if moving <= idle {
		t.Fatalf("scrolling shows the bar (%d px), idle hides it (%d px)", moving, idle)
	}
	barsMode = barsAlways
	e.tt.Frame()
	if always := right(); always <= idle {
		t.Fatalf("always shows the bar: %d px vs idle %d px", always, idle)
	}
}
