package ui

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"
)

// TestFrameTime guards the cost of one frame of a full data grid.
func TestFrameTime(t *testing.T) {
	if raceEnabled {
		t.Skip("-race 下每帧慢好几倍，耗时预算没有意义")
	}
	e := newEnv(t)
	openUsers(e)
	e.tt.Frame()
	const n = 40
	start := time.Now()
	for i := 0; i < n; i++ {
		e.tt.Move(float32(300+i*7), float32(200+i*5)) // the pointer moving rebuilds the view
		e.tt.Frame()
	}
	per := time.Since(start) / n
	t.Logf("frame: %v", per)
	if per > 40*time.Millisecond {
		t.Fatalf("a frame of the data grid takes %v, over the 40ms budget", per)
	}
}

// TestWideTableScrollsSideways: columns past the window's width must scroll
// sideways, which needs every container above the table to be allowed to be
// narrower than the table's columns.
func TestWideTableScrollsSideways(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	cols, vals := "", ""
	for i := 0; i < 40; i++ {
		cols += fmt.Sprintf(", c%02d TEXT", i)
		vals += fmt.Sprintf(", 'value-%02d'", i)
	}
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE wide (id INTEGER PRIMARY KEY" + cols + ")"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO wide VALUES (1" + vals + ")"); err != nil {
		t.Fatal(err)
	}
	w.loadTables()
	w.openTable("wide")
	e.tt.Frame()
	before, _ := e.tt.Find("c00")
	e.tt.Scroll(700, 97, 300, 0)
	e.tt.Frame()
	after, _ := e.tt.Find("c00")
	if after.X >= before.X {
		t.Fatalf("a sideways scroll must move the columns: c00 at %v, then %v", before.X, after.X)
	}
	e.shot("61-wide-scrolled")
}

// TestEmptyTableStripesFillTheGrid: below the last row (here: none, then a
// few) the stripes continue to the bottom of the grid, in step with the rows,
// as in the original.
func TestEmptyTableStripesFillTheGrid(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE empty_t (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatal(err)
	}
	w.loadTables()
	w.openTable("empty_t")
	e.tt.Frame()
	e.shot("62-empty")
	boundaries := func() []int {
		img := e.tt.Image()
		var ys []int
		prev := img.At(900, 87)
		for y := 87; y < 750; y++ {
			if c := img.At(900, y); c != prev {
				ys = append(ys, y)
				prev = c
			}
		}
		return ys
	}
	empty := boundaries()
	if len(empty) < 20 {
		t.Fatalf("an empty table must show stripes down the whole grid; found %d color changes", len(empty))
	}
	for i := 1; i <= 3; i++ {
		if _, err := db.Exec("INSERT INTO empty_t(name) VALUES (?)", fmt.Sprint("row", i)); err != nil {
			t.Fatal(err)
		}
	}
	w.tabs[0].(*tableTab).load()
	e.tt.Frame()
	e.shot("63-short")
	short := boundaries()
	if fmt.Sprint(short) != fmt.Sprint(empty) {
		t.Fatalf("the stripes below real rows must continue those of an empty grid:\n empty %v\n short %v", empty, short)
	}
}

// TestPreviewKeepsListPosition: Space opens the row pane beside the grid
// without the rows moving.
func TestPreviewKeepsListPosition(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	e.tt.Scroll(500, 300, 0, 1500)
	e.tt.Frame()
	e.tt.Move(1000, 780) // pointer off the grid
	e.tt.Frame()
	rows := func() map[string]float32 {
		out := map[string]float32{}
		for i := 1; i <= 450; i++ {
			name := fmt.Sprintf("用户%03d", i)
			if r, ok := e.tt.Find(name); ok && r.Y > 70 && r.Y < 760 {
				out[name] = r.Y
			}
		}
		return out
	}
	// choose a row in view, as the user would before pressing Space
	e.tt.ClickAt(450, 300)
	e.tt.Frame()
	e.tt.Move(1000, 780)
	e.tt.Frame()
	before := rows()
	if len(before) == 0 {
		t.Fatal("no rows visible after scrolling")
	}
	first := ""
	for i := 1; i <= 450 && first == ""; i++ {
		if _, ok := before[fmt.Sprintf("用户%03d", i)]; ok {
			first = fmt.Sprintf("用户%03d", i)
		}
	}
	t.Logf("first visible row: %s, chosen=%d", first, tab.g.chosen.Len())
	e.tt.Key(0, mui.KeySpace)
	e.tt.Frame()
	if !tab.showPreview {
		t.Fatal("Space must open the preview")
	}
	e.shot("64-preview-pos")
	after := rows()
	for name, y := range before {
		if ay, ok := after[name]; !ok || ay != y {
			t.Fatalf("row %s moved from y=%v to %v (visible: %v) when the preview opened", name, y, ay, ok)
		}
	}
}

func TestPreviewKeepsSidewaysScroll(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	cols, vals := "", ""
	for i := 0; i < 40; i++ {
		cols += fmt.Sprintf(", c%02d TEXT", i)
		vals += fmt.Sprintf(", 'value-%02d'", i)
	}
	db, _ := sql.Open("sqlite", e.dbPath)
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE wide (id INTEGER PRIMARY KEY" + cols + ")"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60; i++ {
		if _, err := db.Exec("INSERT INTO wide VALUES (NULL" + vals + ")"); err != nil {
			t.Fatal(err)
		}
	}
	w.loadTables()
	w.openTable("wide")
	e.tt.Frame()
	e.tt.Scroll(700, 200, 1200, 600)
	e.tt.Frame()
	e.tt.Move(1000, 780)
	e.tt.Frame()
	before, _ := e.tt.Find("c06")
	e.tt.Key(0, mui.KeySpace)
	e.tt.Frame()
	e.shot("65-wide-preview")
	after, _ := e.tt.Find("c06")
	if before.X != after.X || before.Y != after.Y {
		t.Fatalf("the grid moved when the preview opened: c06 at %v,%v then %v,%v", before.X, before.Y, after.X, after.Y)
	}
}

// TestEditingACellKeepsRowHeight: the input that replaces a cell while it is
// edited must not make its row taller.
func TestEditingACellKeepsRowHeight(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	y := func(name string) float32 {
		r, ok := e.tt.Find(name)
		if !ok {
			t.Fatalf("%s not on screen", name)
		}
		return r.Y
	}
	before := y("用户005")
	tab.beginEdit(2, "name")
	e.tt.Frame()
	e.shot("66-editing-cell")
	after := y("用户005")
	if before != after {
		t.Fatalf("editing a cell moved the rows below it: 用户005 at y=%v, then %v", before, after)
	}
}
