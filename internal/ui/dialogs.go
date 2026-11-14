package ui

import "github.com/egoist/mygo/ui"

// modal 是 Shell 上一个对话框的统一接口：Shell 只通过它判断"有没有对话框占着键盘"
// 和"按顺序把各个对话框画出来"，不再各处手写一遍对话框清单。
// 新增对话框：在 dialogs 里加状态字段，再到 Shell.modals 里登记一行。
type modal interface {
	// name 是对话框状态在 dialogs 里的字段名，archtest 式的单元测试用它核对登记是否齐全。
	name() string
	// isOpen 报告对话框当前是否打开。
	isOpen() bool
	// draw 每帧调用一次；对话框没打开时应当什么也不画。
	draw(c *ui.Context)
}

// modalEntry 把"状态里的 open 标志 + 画它的方法"包成 modal。
type modalEntry struct {
	field string
	open  func() bool
	view  func(c *ui.Context)
}

func (m modalEntry) name() string       { return m.field }
func (m modalEntry) isOpen() bool       { return m.open() }
func (m modalEntry) draw(c *ui.Context) { m.view(c) }

// modals 按绘制顺序（后画的盖在先画的上面）返回全部对话框；列表只建一次。
func (s *Shell) modals() []modal {
	if s.modalList != nil {
		return s.modalList
	}
	e := func(field string, open func() bool, view func(c *ui.Context)) modal {
		return modalEntry{field: field, open: open, view: view}
	}
	s.modalList = []modal{
		e("wsPop", func() bool { return s.wsPop.open }, s.workspacesPopup),
		e("dlg", func() bool { return s.dlg.open }, s.connectionDialog),
		e("exp", func() bool { return s.exp.open }, s.exportDialogView),
		e("bak", func() bool { return s.bak.open }, s.backupView),
		e("usr", func() bool { return s.usr.open }, s.usersView),
		e("rst", func() bool { return s.rst.open }, s.restoreView),
		e("imp", func() bool { return s.imp.open }, s.importDialogView),
		e("set", func() bool { return s.set.open }, s.settingsView),
		e("proc", func() bool { return s.proc.open }, s.monitorView),
		e("sw", func() bool { return s.sw.open }, s.switcherView),
		e("pal", func() bool { return s.pal.open }, s.paletteView),
		e("prm", func() bool { return s.prm.open }, s.promptView),
		e("edOpt", func() bool { return s.edOpt }, s.editorOptionsDialog),
		e("srch", func() bool { return s.srch.open }, s.searchView),
		e("confirm", func() bool { return s.confirm.open }, s.confirmView),
		e("pwp", func() bool { return s.pwp.open }, s.passwordView), // 密码框最后画，盖在其他对话框之上
	}
	return s.modalList
}

// anyModalOpen 报告是否有对话框打开。
func (s *Shell) anyModalOpen() bool {
	for _, m := range s.modals() {
		if m.isOpen() {
			return true
		}
	}
	return false
}

// drawModals 按顺序画出全部对话框。
func (s *Shell) drawModals(c *ui.Context) {
	for _, m := range s.modals() {
		m.draw(c)
	}
}

// dialogs 汇总 Shell 上各个对话框的状态。它嵌在 Shell 里，字段名保持不变；
// 对话框的登记（打开状态与绘制）见上面的 Shell.modals。
type dialogs struct {
	modalList []modal // 对话框登记表（Shell.modals 懒建），不是对话框状态

	dlg     connDialog
	confirm confirmDialog
	pwp     pwPrompt
	prm     promptDialog
	edOpt   bool // the editor options dialog
	srch    dbSearch
	imp     importDialog
	set     settingsDialog
	pal     palette
	sw      dbSwitcher
	proc    processDialog
	exp     exportDialog
	bak     dumpDialog
	usr     usersDialog
	rst     restoreDialog
	wsPop   wsPopup
}
