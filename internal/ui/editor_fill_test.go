package ui

import (
	"os"
	"testing"
)

// 空查询页：截图看编辑区实际占多大。
func TestShotsEmptyQuery(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newEnv(t)
	w := openWorkspace(e)
	w.newQuery("")
	e.shot("q1-empty")
	w.newQuery("SELECT 1")
	e.shot("q2-one-line")
}

// 点编辑区空白处（最后一行下面）应当聚焦编辑器、光标落到文末，随后能直接输入。
func TestQueryEditorBlankAreaIsWritable(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1\nFROM users")
	e.tt.Frame()
	// 编辑区在标签页内容的上半部分：(600, 250) 远在最后一行之下
	e.tt.ClickAt(600, 250)
	e.tt.Frame()
	if got := q.ed.Cursor(); got != len(q.ed.Buffer.String()) {
		t.Fatalf("点空白处光标应落到文末(%d)，实际 %d", len(q.ed.Buffer.String()), got)
	}
	e.tt.Type(";\nSELECT 2")
	e.tt.Frame()
	if want := "SELECT 1\nFROM users;\nSELECT 2"; q.ed.Buffer.String() != want {
		t.Fatalf("空白处点击后应能输入，实际 %q", q.ed.Buffer.String())
	}
}

// 空查询页：不点编辑器，打开就能直接打字（自动聚焦）。
func TestQueryEditorTypesImmediately(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("")
	e.tt.Frame()
	e.tt.Type("SELECT 1")
	e.tt.Frame()
	if got := q.ed.Buffer.String(); got != "SELECT 1" {
		t.Fatalf("新查询页应自动聚焦，实际 %q", got)
	}
}

func TestShotsQueryAfterRun(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newEnv(t)
	e.tt.SetSize(1440, 900)
	w := openWorkspace(e)
	q := w.newQuery("SELECT * FROM users")
	e.tt.Frame()
	q.run(true)
	e.shot("q3-after-run")
}

// 编辑区里各处空白（行号栏下方、文本区右侧、最后一行下方）点一下都应聚焦并把光标放到文末。
func TestQueryEditorBlankPoints(t *testing.T) {
	for _, pt := range [][2]float32{{800, 128}, {700, 400}, {1200, 300}, {400, 405}, {600, 250}} {
		e := newEnv(t)
		w := openWorkspace(e)
		q := w.newQuery("SELECT 1\nFROM users")
		e.tt.Frame()
		// 先把焦点挪到侧栏，确保是这一下点击聚焦了编辑器
		e.tt.ClickAt(100, 500) // 侧栏的空白处
		e.tt.Frame()
		if q.ed.Focused() {
			t.Fatal("点侧栏后编辑器应当失焦")
		}
		e.tt.ClickAt(pt[0], pt[1])
		e.tt.Frame()
		e.tt.Type("!")
		e.tt.Frame()
		if got, want := q.ed.Buffer.String(), "SELECT 1\nFROM users!"; got != want {
			t.Errorf("点 (%v,%v) 后输入，期望 %q，实际 %q", pt[0], pt[1], want, got)
		}
	}
}

func TestShotsQueryRetina(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newEnv(t)
	e.tt.SetScale(2)
	w := openWorkspace(e)
	w.newQuery("")
	e.shot("q4-retina")
}
