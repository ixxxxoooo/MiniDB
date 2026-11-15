package ui

import (
	"strings"
	"testing"
	"time"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
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

// 切库浮层的宽度：库名短时是最小宽度，库名很长时变宽，但不超过最大宽度。
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
	if w := dbPopWidth(huge); w != sizeDbPopMax {
		t.Errorf("超长库名应封顶在 %v，实际 %v", float32(sizeDbPopMax), w)
	}
}

// 点库名弹出切库浮层，再点一次收起；连接胶囊同理。
func TestPopoversToggleOnSecondClick(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	tip := locales.ToolbarSwitchDatabase() + w.sh.keyHint("switcher")
	for i, want := range []bool{true, false, true} {
		if err := e.tt.Click(tip); err != nil {
			t.Fatal(err)
		}
		e.tt.Frame()
		e.tt.Frame()
		if w.sh.sw.open != want {
			t.Fatalf("第 %d 次点库名后 open=%v，应为 %v", i+1, w.sh.sw.open, want)
		}
	}
	w.sh.sw.open = false
	e.tt.Frame()
	chip := w.cfg.Name
	for i, want := range []bool{true, false} {
		if err := e.tt.Click(chip); err != nil {
			t.Fatal(err)
		}
		e.tt.Frame()
		e.tt.Frame()
		if w.sh.wsPop.open != want {
			t.Fatalf("第 %d 次点连接胶囊后 open=%v，应为 %v", i+1, w.sh.wsPop.open, want)
		}
	}
}

// 侧栏开关在活动栏底部：点它收起 / 展开侧栏，标题栏里不再有第二个。
func TestSidebarToggleLivesInRail(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	if w.sidebarHidden {
		t.Fatal("准备：侧栏应是展开的")
	}
	if err := e.tt.Click(locales.ToolbarSidebar() + w.sh.keyHint("sidebar")); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if !w.sidebarHidden {
		t.Fatal("点活动栏底部的侧栏开关应收起侧栏")
	}
}

// 文档页：标题、说明、三列字段表；列的备注缺失时用 AI 草稿并带标记。
func TestDocPageRendersFieldTable(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	tab.mode = 3
	for i := 0; i < 6; i++ {
		e.tt.Frame()
	}
	for _, s := range []string{"users", locales.DocColField(), locales.DocColType(), locales.DocColDesc(), "email", "INTEGER", locales.DocNoDesc()} {
		if !e.tt.HasText(s) {
			t.Errorf("文档页缺少 %q: %q", s, e.tt.Texts())
		}
	}
	tab.doc.drafts = map[string]string{"email": "登录邮箱"}
	tab.doc.draftDesc = "存放用户资料"
	e.tt.Frame()
	e.tt.Frame()
	for _, s := range []string{"登录邮箱", "存放用户资料", locales.DocAIDraft()} {
		if !e.tt.HasText(s) {
			t.Errorf("补全草稿后缺少 %q: %q", s, e.tt.Texts())
		}
	}
	rows := tab.docRows()
	for _, r := range rows {
		if r.name == "email" && !r.ai {
			t.Error("草稿给出的说明应带 AI 标记")
		}
	}
}

func TestFirstParagraphAndDocMarkdown(t *testing.T) {
	md := "# users\n\n> 备注\n\n存放注册用户。\n被 orders 引用。\n\n## 字段\n| a | b |\n"
	if got := firstParagraph(md); got != "存放注册用户。 被 orders 引用。" {
		t.Errorf("firstParagraph = %q", got)
	}
	if firstParagraph("# 只有标题\n\n| a |\n") != "" {
		t.Error("没有正文应返回空串")
	}
	got := docMarkdown("users", "用户表", "一句说明", []docRow{{name: "id", typ: "integer", comment: "主键|自增"}})
	for _, want := range []string{"# users", "> 用户表", "一句说明", "| `id` | INTEGER | 主键\\|自增 |"} {
		if !strings.Contains(got, want) {
			t.Errorf("Markdown 缺少 %q:\n%s", want, got)
		}
	}
}

// 出图看一眼：文档页和数据页顶栏（MINIDB_SHOTS=目录 时才写文件）。
func TestShotsDocPage(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	e.tt.Frame()
	e.shot("fix-data-top")
	tab.mode = 3
	tab.doc.drafts = map[string]string{"email": "登录邮箱，唯一", "age": "年龄（岁），可空"}
	tab.doc.draftDesc = "存放注册用户的基础资料与账户状态。"
	for i := 0; i < 6; i++ {
		e.tt.Frame()
	}
	e.shot("fix-doc")
}

// 查询结果的「耗时 · 行数 · 查找 · 固定 · 导出」并进底部状态栏，结果区不再有自己的底栏：
// 状态栏里能找到固定和导出按钮，而「耗时 · 行数」只出现一次。
func TestQueryStatusMergedIntoStatusBar(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT id, name FROM users")
	e.tt.Frame()
	q.run(true)
	for i := 0; i < 4; i++ {
		e.tt.Frame()
	}
	for _, s := range []string{locales.QueryPin(), locales.QueryExportResult()} {
		if !e.tt.HasText(s) {
			t.Errorf("状态栏里缺少 %q: %q", s, e.tt.Texts())
		}
	}
	rows := locales.TableRows(len(q.results[0].res.Rows))
	n := 0
	for _, s := range e.tt.Texts() {
		if strings.Contains(s, rows) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("「%s」应只出现一次（两条状态栏已合成一条），实际 %d 次: %q", rows, n, e.tt.Texts())
	}
	e.shot("fix-query-status")
}

// 标题栏左边距：系统内缩很大时封顶在红绿灯右缘 + 一个间距（78），不留大片空白；内缩小于它时照用；没有内缩时至少一个间距。
func TestTitleLeftInset(t *testing.T) {
	for _, c := range []struct{ in, want float32 }{{0, spaceMD}, {70, 70}, {78, 78}, {120, 78}} {
		if got := titleLeftInset(c.in); got != c.want {
			t.Errorf("titleLeftInset(%v) = %v，应为 %v", c.in, got, c.want)
		}
	}
}

// 标签很多时：标签条右端有「已打开的标签」按钮，点开列表能看到全部标签，选中的切过去；再点按钮收起。
func TestTabsListPopover(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.newQuery("SELECT 0")
	for i := 0; i < 3; i++ {
		e.tt.Frame()
	}
	if _, ok := e.tt.Find(locales.TabsMore()); ok {
		t.Fatal("标签放得下时不该有「已打开的标签」入口")
	}
	for i := 1; i < 12; i++ {
		w.newQuery("SELECT " + itoa(i))
	}
	w.openTable("users")
	for i := 0; i < 4; i++ {
		e.tt.Frame()
	}
	tip := locales.TabsMore()
	if err := e.tt.Click(tip); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	e.tt.Frame()
	if !e.tt.HasText(locales.TabsOpenedN(len(w.tabs))) {
		t.Fatalf("点开后应有「已打开的标签 · %d」: %q", len(w.tabs), e.tt.Texts())
	}
	e.shot("fix-tabs-list")
	before := w.active
	if err := e.tt.Click("查询 3"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if w.active == before {
		t.Error("点列表里的标签应切过去")
	}
}

// 命令面板：左边列表、右边预览——选中表时预览里有它的字段，历史项在列表里是高亮 SQL。
func TestPalettePreview(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	w.sh.openPalette()
	w.sh.pal.query = "users"
	for i := 0; i < 6; i++ {
		e.tt.Frame()
	}
	for _, s := range []string{"users", "email", "TEXT"} {
		if !e.tt.HasText(s) {
			t.Errorf("面板预览里缺少 %q: %q", s, e.tt.Texts())
		}
	}
	e.shot("fix-palette")
}

// 命令面板：选中项很靠后时，列表要滚到它可见（上下键移动选中项时列表跟着走）。
func TestPaletteListFollowsSelection(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	w.sh.openPalette()
	for i := 0; i < 30; i++ {
		w.sh.pal.history = append(w.sh.pal.history, app.HistoryItem{SQL: "SELECT " + itoa(i) + " FROM users"})
	}
	w.sh.pal.cat = 3 // 历史：条目多、列表放不下
	for i := 0; i < 3; i++ {
		e.tt.Frame()
	}
	w.sh.pal.sel = 25
	for i := 0; i < 4; i++ {
		e.tt.Frame()
	}
	var list mui.Rect
	for _, a := range e.tt.AccessAll() {
		if a.Bounds.W == sizePalList && a.Bounds.H == sizePalBody { // 左边的列表（滚动容器）
			list = a.Bounds
		}
	}
	for _, a := range e.tt.AccessAll() {
		if a.Role == mui.RoleText && a.Name == "SELECT 25 FROM users" && a.Bounds.X < 640 {
			if a.Bounds.Y < list.Y || a.Bounds.Y+a.Bounds.H > list.Y+list.H {
				t.Fatalf("选中的第 26 项应在列表可见范围内: 项 %v 列表 %v", a.Bounds, list)
			}
			return
		}
	}
	t.Fatalf("没找到第 26 项: %q", e.tt.Texts())
}

// 窗口很窄、右边又开着 AI 面板时，表页顶栏只收一部分——视图分段永远在，不会整条收成一个「»」。
func TestTableTopBarNeverCollapsesWhole(t *testing.T) {
	e := newEnv(t)
	tab := openUsers(e)
	for _, width := range []int{1280, 1100, 960} { // 960 是窗口的最小宽度
		e.tt.SetSize(width, 800)
		w := tab.w
		w.panel = 2 // 右边的 AI 面板占去一截
		for i := 0; i < 4; i++ {
			e.tt.Frame()
		}
		for _, s := range []string{locales.TableViewData(), locales.TableViewStructure(), locales.TableViewDoc()} {
			if !e.tt.HasText(s) {
				t.Errorf("窗口宽 %d：顶栏里的「%s」不见了（整条被折叠）", width, s)
			}
		}
		e.shot("fix-topbar-" + itoa(width))
	}
}

// 侧栏收起带过渡：收起后的一小段时间里内容还在（淡出、宽度收窄），播完才卸载；展开同理。
func TestSidebarCollapseAnimates(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	if !e.tt.HasText("orders") {
		t.Fatal("准备：侧栏应显示表")
	}
	w.sidebarHidden = true
	e.tt.Frame()
	e.tt.Advance(60 * time.Millisecond)
	e.tt.Frame()
	if !e.tt.HasText("orders") {
		t.Error("收起动画刚开始，侧栏内容应还在")
	}
	e.tt.Advance(600 * time.Millisecond)
	e.tt.Frame()
	e.tt.Frame()
	if e.tt.HasText("orders") {
		t.Error("动画播完后侧栏内容应已卸载")
	}
	w.sidebarHidden = false
	e.tt.Frame()
	e.tt.Advance(600 * time.Millisecond)
	e.tt.Frame()
	if !e.tt.HasText("orders") {
		t.Error("展开后侧栏内容应回来")
	}
}

// 备份 / 还原是三步向导：下一步 / 上一步在步骤间走；还原没选备份文件时「下一步」是灰的。
func TestBackupRestoreWizardSteps(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	e.tt.Frame()
	e.sh.openBackup()
	e.tt.Frame()
	e.has(locales.DumpStepOptions())
	e.has(locales.DumpStepLocation())
	if err := e.tt.Click(locales.XferNext()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if e.sh.bak.step != 1 {
		t.Fatalf("点下一步应到第二步: %d", e.sh.bak.step)
	}
	e.has(locales.DumpStart())
	e.shot("fix-backup-wizard")
	if err := e.tt.Click(locales.XferPrev()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if e.sh.bak.step != 0 {
		t.Fatalf("点上一步应回第一步: %d", e.sh.bak.step)
	}
	e.sh.bak.open = false

	e.sh.openRestore()
	e.tt.Frame()
	e.tt.Click(locales.XferNext()) // 没选文件：灰着，点不动
	e.tt.Frame()
	if e.sh.rst.step != 0 {
		t.Fatalf("没选备份文件时不该能下一步: %d", e.sh.rst.step)
	}
	e.sh.rst.path = "/tmp/shop-2026-11-08.sql.gz"
	e.tt.Frame()
	if err := e.tt.Click(locales.XferNext()); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if e.sh.rst.step != 1 {
		t.Fatalf("选好文件后下一步应到第二步: %d", e.sh.rst.step)
	}
	e.shot("fix-restore-wizard")
	_ = w
}
