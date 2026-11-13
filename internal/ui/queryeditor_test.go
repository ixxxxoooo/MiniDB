package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 查询参数默认关闭：占位符原样发给数据库；开启后先弹变量窗口。
func TestQueryParamsAskBeforeRunning(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT :a + :b AS s")
	e.tt.Frame()
	q.run(true)
	if q.vars.open {
		t.Fatal("参数默认关闭，不应弹窗")
	}

	e.sh.prefs.Params = true
	q.run(true)
	e.tt.Frame()
	if !q.vars.open || len(q.vars.names) != 2 || q.vars.names[0] != "a" {
		t.Fatalf("应弹出变量窗口: %+v", q.vars)
	}
	e.has("查询参数")
	e.shot("23-query-params")
	q.vars.vals["a"], q.vars.vals["b"] = "1", "2"
	e.tt.Key(mui.Cmd, mui.KeyEnter) // ⌘↵ 应用
	e.tt.Frame()
	if q.vars.open {
		t.Fatal("⌘↵ 应关闭变量窗口并运行")
	}
	if !ranOK(q) || q.results[0].res.Rows[0]["s"] != int64(3) {
		t.Fatalf("应算出 3: %+v failed=%q", q.results, q.failed)
	}
	if e.sh.prefs.ParamValues["a"] != "1" {
		t.Fatalf("应记住参数值: %v", e.sh.prefs.ParamValues)
	}
	// 下次再跑，窗口里带着上次的值
	q.run(true)
	if q.vars.vals["b"] != "2" {
		t.Fatalf("应带出上次的值: %v", q.vars.vals)
	}
}

// 没有占位符时不打扰。
func TestQueryParamsSilentWithoutPlaceholders(t *testing.T) {
	e := newEnv(t)
	e.sh.prefs.Params = true
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1 AS one")
	e.tt.Frame()
	q.run(true)
	if q.vars.open || !ranOK(q) {
		t.Fatal("没有占位符应直接运行")
	}
}

// 结果标签用 -- name: 命名；固定的结果在下次运行后仍在最前。
func TestResultNamesAndPinning(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("-- name: 第一\nSELECT 1 AS a;\n-- name: 第二\nSELECT 2 AS b")
	e.tt.Frame()
	q.run(true)
	if len(q.results) != 2 || q.results[0].label(0) != "第一" || q.results[1].label(1) != "第二" {
		t.Fatalf("labels: %q %q", q.results[0].label(0), q.results[1].label(1))
	}
	e.has("第二")
	q.results[0].pinned = true
	if q.results[0].label(0) != "◆ 第一" {
		t.Fatalf("pinned label: %q", q.results[0].label(0))
	}
	q.ed.Buffer.Set("SELECT 3 AS c")
	q.run(true)
	if len(q.results) != 2 || !q.results[0].pinned || q.results[0].res.Rows[0]["a"] != int64(1) || q.results[1].res.Rows[0]["c"] != int64(3) {
		t.Fatalf("固定的结果应留在最前: %+v", q.results)
	}
}

func TestFindInResultAndMessages(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT * FROM users")
	e.tt.Frame()
	q.run(true)
	r := q.results[0]
	all := len(r.g.rows)
	r.find = "用户007"
	r.narrow()
	if len(r.g.rows) != 1 || len(r.all) != all {
		t.Fatalf("查找后应只剩 1 行，实际 %d（全部 %d）", len(r.g.rows), len(r.all))
	}
	e.has("1 / ")
	r.find = ""
	r.narrow()
	if len(r.g.rows) != all {
		t.Fatal("清空后应恢复全部行")
	}
	q.page = pageMessages
	e.has("SELECT * FROM users")
	e.shot("24-query-messages")
}

func TestRowLimitAndRunDefault(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.prefs.RowLimit = 5
	q := w.newQuery("SELECT * FROM users;\nSELECT 1 AS one")
	e.tt.Frame()
	q.ed.Select(3, 3)
	q.runDefault(false)
	if len(q.results) != 1 || len(q.results[0].res.Rows) != 5 {
		t.Fatalf("行数上限 5: %d 个结果, %d 行", len(q.results), len(q.results[0].res.Rows))
	}
	e.sh.prefs.RunAll = true
	q.runDefault(false)
	if len(q.results) != 2 {
		t.Fatalf("RunAll 时 ⌘↵ 跑全部: %d", len(q.results))
	}
	q.runDefault(true) // ⌘⇧↵ 反过来
	if len(q.results) != 1 {
		t.Fatalf("⌘⇧↵ 跑当前: %d", len(q.results))
	}
}

func TestUglifyButtonAndShortcut(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT  id,\n  name\nFROM users -- 注释\nWHERE id = 1")
	e.tt.Frame()
	q.uglify()
	if got := q.ed.Buffer.String(); got != "SELECT id, name FROM users WHERE id = 1" {
		t.Fatalf("%q", got)
	}
}

// 分栏：⌘⇧D 开第二个编辑器，运行的是有键盘的那一栏；⌘⌥] 在两栏间切换。
func TestSplitPanes(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1 AS one")
	e.tt.Frame()
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyD)
	e.tt.Frame()
	if q.pane2 == nil || !q.second {
		t.Fatal("⌘⇧D 应开出第二栏并把键盘给它")
	}
	if e.sh.activeQuery() != q.pane2 {
		t.Fatal("全局命令应作用于有键盘的那一栏")
	}
	e.shot("25-split")
	q.pane2.ed.Buffer.Set("SELECT 2 AS two")
	e.sh.activeQuery().run(true)
	if len(q.pane2.results) != 1 || len(q.results) != 0 {
		t.Fatalf("结果应出现在第二栏: %d / %d", len(q.pane2.results), len(q.results))
	}
	e.tt.Key(mui.Cmd|mui.Alt, mui.KeyBracketRight)
	e.tt.Frame()
	if q.second || e.sh.activeQuery() != q {
		t.Fatal("⌘⌥] 应切回第一栏")
	}
	q.toggleSplit()
	if q.pane2 != nil || e.sh.activeQuery() != q {
		t.Fatal("再次分栏应收起第二栏")
	}
}

func TestToggleResultsPane(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1 AS one")
	e.tt.Frame()
	q.run(true)
	e.has("1 行")
	e.tt.Key(mui.Ctrl, mui.KeyBackquote)
	e.tt.Frame()
	if !q.hideResults {
		t.Fatal("⌃` 应收起结果区")
	}
	e.hasNot("1 行")
}

// 收藏查询绑定关键字后，补全里能输入关键字展开整条语句。
func TestFavoriteKeywordCompletion(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.sh.app.History.Add(w.cfg.ID, w.db, "SELECT COUNT(*) FROM users")
	items, _ := e.sh.app.History.List(w.cfg.ID, "", 0)
	w.setKeyword(items[0].ID, "cnt")
	if w.keywords["cnt"] != "SELECT COUNT(*) FROM users" {
		t.Fatalf("keywords=%v", w.keywords)
	}
	q := w.newQuery("")
	got := q.keywordCompletions("cn")
	if len(got) != 1 || got[0].Label != "cnt" || got[0].Insert != "SELECT COUNT(*) FROM users" {
		t.Fatalf("%+v", got)
	}
	if len(q.keywordCompletions("zz")) != 0 {
		t.Fatal("不匹配的前缀不应出现")
	}
	w.setKeyword(items[0].ID, "")
	if len(w.keywords) != 0 {
		t.Fatal("解除后应不再补全")
	}
	// 重复的关键字被拒绝并提示
	e.sh.app.History.Add(w.cfg.ID, w.db, "SELECT 2")
	items, _ = e.sh.app.History.List(w.cfg.ID, "", 0)
	w.setKeyword(items[0].ID, "k")
	w.setKeyword(items[1].ID, "k")
	var seen []string
	for _, m := range e.sh.toasts {
		seen = append(seen, m.msg)
	}
	if !strings.Contains(strings.Join(seen, " "), "已被") {
		t.Fatalf("应提示关键字重复: %v", seen)
	}
}

func TestEditorOptionsDialog(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.newQuery("SELECT 1")
	e.tt.Frame()
	e.sh.edOpt = true
	e.has("编辑器设置")
	e.has("启用查询参数")
	e.shot("26-editor-options")
	e.sh.prefs.Params = true
	e.has("占位符写法")
}

// 设置弹窗按分组列出全部选项，每项都带说明。
func TestEditorOptionsDialogIsGrouped(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.newQuery("SELECT 1")
	e.tt.Frame()
	e.sh.edOpt = true
	for _, s := range []string{"运行", "外观", "缩进", "输入与补全", "字号", "自动换行", "显示行号", "高亮当前行", "折叠标记", "缩进参考线", "显示空白字符", "缩进宽度", "用空格缩进", "恢复默认"} {
		e.has(s)
	}
	e.has("长语句在窗口边缘折行") // 说明文字
}

// 偏好里的"不……"开关为零值时，编辑器保持原有行为；设了就逐项生效。
func TestApplyEditorPrefs(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1")
	sh := e.sh
	sh.prefs = uiPrefs{}
	sh.applyEditorPrefs(q.ed)
	if !q.ed.SoftWrap || q.ed.LineNumbers != editor.LineNumbersOn || q.ed.NoActiveLine || q.ed.NoFolding || q.ed.NoIndentGuides ||
		q.ed.Whitespace != editor.WhitespaceSelection || q.ed.TabSize != 2 || q.ed.Indent != editor.IndentSpaces {
		t.Fatalf("零值应是默认外观: %+v", q.ed)
	}
	sh.prefs = uiPrefs{NoWrap: true, NoLineNumbers: true, NoActiveLine: true, NoFolding: true, NoIndentGuides: true,
		ShowWhitespace: true, TabSize: 4, IndentTabs: true, EditorFont: 16}
	sh.applyEditorPrefs(q.ed)
	if q.ed.SoftWrap || q.ed.LineNumbers != editor.LineNumbersOff || !q.ed.NoActiveLine || !q.ed.NoFolding || !q.ed.NoIndentGuides ||
		q.ed.Whitespace != editor.WhitespaceAll || q.ed.TabSize != 4 || q.ed.Indent != editor.IndentTabs || q.ed.Font.Size != 16 {
		t.Fatalf("偏好没有逐项生效: %+v", q.ed)
	}
	sh.prefs.TabSize = 3 // 不在可选范围内，回到默认
	if sh.prefs.tabSize() != 2 {
		t.Fatal("非法缩进宽度应回到 2")
	}
}

// "恢复默认"清掉编辑器选项，但保留已记住的参数值。
func TestResetEditorPrefsKeepsParamValues(t *testing.T) {
	e := newEnv(t)
	sh := e.sh
	sh.prefs = uiPrefs{NoWrap: true, TabSize: 8, RunAll: true, RowLimit: 100, EditorFont: 20, ParamValues: map[string]string{"id": "1"}}
	sh.resetEditorPrefs()
	if sh.prefs.NoWrap || sh.prefs.TabSize != 0 || sh.prefs.RunAll || sh.prefs.RowLimit != 0 || sh.prefs.EditorFont != 0 {
		t.Fatalf("没有恢复默认: %+v", sh.prefs)
	}
	if sh.prefs.ParamValues["id"] != "1" {
		t.Fatal("记住的参数值不该被清掉")
	}
}

// 键盘在左栏时按 ⌘⇧D 收起分栏：左栏画完时处理快捷键并清空 q.pane2，
// 右栏在同一帧里不能再去用它（曾经空指针崩溃）。键盘在右栏时没有这个问题，
// 因为右栏是最后画的。
func TestSplitToggleOffInsideFrame(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1 AS one")
	e.tt.Frame()
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyD)
	e.tt.Frame()
	if q.pane2 == nil {
		t.Fatal("⌘⇧D 应开出第二栏")
	}
	e.tt.Key(mui.Cmd|mui.Alt, mui.KeyBracketRight) // 键盘回到左栏
	e.tt.Frame()
	if q.second {
		t.Fatal("键盘应在左栏")
	}
	e.tt.Key(mui.Cmd|mui.Shift, mui.KeyD)
	e.tt.Frame()
	e.tt.Frame()
	if q.pane2 != nil {
		t.Fatal("再按 ⌘⇧D 应收起第二栏")
	}
}
