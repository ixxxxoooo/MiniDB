package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// again 让同一快捷键能在测试里连续触发：fire 会挡掉 150ms 内重复的同一命令
// （防止菜单和窗口各触发一次），真人按键间隔远大于此。
func again(e *env) { e.sh.lastKey = "" }

// ⌘0 收起侧栏，⌘1 到 ⌘9 按序号切换标签。
func TestSidebarAndTabNumberShortcuts(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	w.openTable("orders")
	e.tt.Frame()
	if w.active != 1 {
		t.Fatalf("active = %d", w.active)
	}
	e.tt.Key(mui.Cmd, mui.Key1)
	e.tt.Frame()
	if w.active != 0 {
		t.Fatalf("⌘1 切到第一个标签，active = %d", w.active)
	}
	e.tt.Key(mui.Cmd, mui.Key9) // 没有第九个标签：保持不动
	e.tt.Frame()
	if w.active != 0 {
		t.Fatalf("⌘9 越界时不应切换，active = %d", w.active)
	}
	e.tt.Key(mui.Cmd, mui.Key0)
	e.tt.Frame()
	if !w.sidebarHidden {
		t.Fatal("⌘0 应收起侧栏")
	}
	again(e)
	e.tt.Key(mui.Cmd, mui.Key0)
	e.tt.Frame()
	if w.sidebarHidden {
		t.Fatal("再按 ⌘0 应展开侧栏")
	}
}

// ⌘⌃← 与 ⌘⌃→ 在访问过的标签间后退、前进。
func TestTabHistoryBackAndForward(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	w.openTable("orders")
	e.tt.Frame()
	w.newQuery("")
	e.tt.Frame()
	if w.active != 2 {
		t.Fatalf("active = %d", w.active)
	}
	e.tt.Key(mui.Cmd|mui.Ctrl, mui.KeyLeft)
	e.tt.Frame()
	if w.active != 1 {
		t.Fatalf("后退应回到上一个访问的标签，active = %d", w.active)
	}
	again(e)
	e.tt.Key(mui.Cmd|mui.Ctrl, mui.KeyLeft)
	e.tt.Frame()
	if w.active != 0 {
		t.Fatalf("再后退，active = %d", w.active)
	}
	again(e)
	e.tt.Key(mui.Cmd|mui.Ctrl, mui.KeyRight)
	e.tt.Frame()
	if w.active != 1 {
		t.Fatalf("前进，active = %d", w.active)
	}
}

// 表加载与查询执行的 SQL 会进入控制台日志；⌃⌘C 开关面板。
func TestConsoleLogsExecutedSQL(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.openTable("users")
	e.tt.Frame()
	e.tt.Frame()
	found := false
	for _, c := range e.sh.console {
		if strings.HasPrefix(c.sql, "SELECT * FROM") && strings.Contains(c.sql, "users") {
			found = true
		}
	}
	if !found {
		t.Fatalf("打开表应记下 SELECT，console = %+v", e.sh.console)
	}

	q := w.newQuery("SELECT COUNT(*) AS n FROM users")
	e.tt.Frame()
	q.run(false)
	e.tt.Frame()
	last := e.sh.console[len(e.sh.console)-1]
	if !strings.Contains(last.sql, "COUNT(*)") || last.err != "" || last.rows != 1 {
		t.Fatalf("查询应记入日志: %+v", last)
	}

	e.tt.Key(mui.Cmd|mui.Ctrl, mui.KeyC)
	e.tt.Frame()
	if !e.sh.consoleOpen {
		t.Fatal("⌃⌘C 应打开控制台")
	}
	e.has("控制台日志")
	e.has("COUNT(*)")
	e.shot("93-console")
}

// ⌘⇧P 预览 SQL（不提交），⌘⇧⌫ 丢弃全部待提交的修改。
func TestPendingPreviewAndDiscardShortcuts(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	tab.beginEdit(0, "name")
	tab.es.buf = "改过了"
	tab.commitEdit()
	if tab.pendingCount() != 1 {
		t.Fatalf("pending = %d", tab.pendingCount())
	}
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyP)
	e.tt.Frame()
	if !tab.sqlOpen {
		t.Fatal("⌘⇧P 应打开 SQL 预览")
	}
	if tab.pendingCount() != 1 {
		t.Fatal("预览不应提交")
	}
	e.shot("94-sql-preview")
	tab.sqlOpen = false
	e.tt.Frame()
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyBackspace)
	e.tt.Frame()
	if tab.pendingCount() != 0 {
		t.Fatalf("⌘⇧⌫ 应丢弃修改，pending = %d", tab.pendingCount())
	}
}

// 没有待提交内容时预览和丢弃什么也不做。
func TestPendingShortcutsIdleWithoutChanges(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyP)
	e.tt.Frame()
	if tab.sqlOpen {
		t.Fatal("没有修改时不应弹出预览")
	}
}

// ⌘⇧\ 打开所有工作区列表并可切换。
func TestWorkspacesPopupSwitches(t *testing.T) {
	e := newEnv(t)
	openWorkspace(e)
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyBackslash)
	e.tt.Frame()
	if !e.sh.wsPop.open {
		t.Fatal("⌘⇧\\ 应打开工作区列表")
	}
	e.tt.Frame()
	e.has("local-sqlite") // 弹窗里列出了这个工作区
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if e.sh.wsPop.open {
		t.Fatal("回车选中后应关闭列表")
	}
}

// ⌘E 新建 SQL 标签，⌘. 打开进程列表，⌘⇧E 导出当前表。
func TestNavigationShortcutsOpenThings(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Key(mui.Cmd, mui.KeyE)
	e.tt.Frame()
	if len(w.tabs) != 1 {
		t.Fatalf("⌘E 应新建 SQL 标签，tabs = %d", len(w.tabs))
	}
	w.openTable("users")
	e.tt.Frame()
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyE)
	e.tt.Frame()
	if !e.sh.exp.open || e.sh.exp.table != "users" {
		t.Fatalf("⌘⇧E 应导出当前表: %+v", e.sh.exp)
	}
}

// ⌘= 与 ⌘- 调整 SQL 编辑器字号，并限制在范围内。
func TestEditorFontZoom(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1")
	e.tt.Frame()
	e.tt.Key(mui.Cmd, mui.KeyEqual)
	e.tt.Frame()
	if e.sh.prefs.EditorFont != editorFontDefault+1 {
		t.Fatalf("⌘= 应放大一号，字号 = %v", e.sh.prefs.EditorFont)
	}
	e.tt.Frame()
	if q.ed.Font.Size != editorFontDefault+1 {
		t.Fatalf("编辑器应采用新字号，字号 = %v", q.ed.Font.Size)
	}
	for i := 0; i < 40; i++ {
		e.sh.zoomEditor(-1)
	}
	if e.sh.prefs.EditorFont != editorFontMin {
		t.Fatalf("字号下限 = %v", e.sh.prefs.EditorFont)
	}
	for i := 0; i < 40; i++ {
		e.sh.zoomEditor(1)
	}
	if e.sh.prefs.EditorFont != editorFontMax {
		t.Fatalf("字号上限 = %v", e.sh.prefs.EditorFont)
	}
}

// 不在查询标签里时 ⌘= 不改动字号。
func TestEditorFontZoomNeedsQueryTab(t *testing.T) {
	e := newEnv(t)
	openUsers(e)
	e.sh.zoomEditor(1)
	if e.sh.prefs.EditorFont != 0 {
		t.Fatalf("表标签里不应改字号: %v", e.sh.prefs.EditorFont)
	}
}

// ⌘⌥R 只刷新当前标签。
func TestRefreshCurrentTabOnly(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	seq := tab.seq
	e.tt.Key(mui.Cmd|mui.Alt, mui.KeyR)
	e.tt.Frame()
	if tab.seq == seq {
		t.Fatal("⌘⌥R 应重新加载当前表")
	}
}

// 关闭其他工作区只留下选中的那一个。
func TestCloseOtherWorkspaces(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.closeOtherSpaces(w)
	e.tt.Frame()
	if len(e.sh.spaces) != 1 || e.sh.spaces[0] != w {
		t.Fatalf("spaces = %d", len(e.sh.spaces))
	}
}
