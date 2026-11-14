package ui

// dialogs 汇总 Shell 上各个对话框的状态。它嵌在 Shell 里，字段名保持不变，
// 画哪个对话框、什么时候打开仍由各自的 xxxView 方法负责；
// 把它们逐个迁出成独立的 dialog 类型之前，先让 Shell 本身只剩下全局状态。
type dialogs struct {
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
