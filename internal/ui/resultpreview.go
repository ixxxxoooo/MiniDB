package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// 查询结果的行预览：和表数据页一样，选中行按空格，在网格右边打开一个窄栏，
// 每个字段一行，只读（查询结果不能直接改）；JSON 自动缩进，可以搜索字段。

// previewRows 是选中的行（升序）；一行都没选时为空。
func (r *queryResult) previewRows() []int { return r.g.chosenRows() }

// togglePreview 显示/隐藏行预览；打开时如果没有选中行，就选中第一行，让栏里总有内容可看。
func (r *queryResult) togglePreview() {
	r.showPreview = !r.showPreview
	if r.previewW == 0 {
		r.previewW = 340
	}
	if r.showPreview && len(r.previewRows()) == 0 && len(r.g.rows) > 0 {
		r.g.chosen.Add(0)
	}
}

// gridKeys 是结果网格的键盘：空格切换行预览，⌘C 复制行，⌘⇧C 复制单元格。
// 只在网格拿着焦点时生效，所以在编辑器里打字不会触发。
func (r *queryResult) gridKeys(c *ui.Context, tbl ui.Element, sh *Shell) {
	switch {
	case tbl.Shortcut(0, ui.KeySpace):
		r.togglePreview()
	case tbl.Shortcut(ui.Cmd|ui.Shift, ui.KeyC):
		if r.g.copyCells(c) {
			sh.toastOK(locales.StatusCopied())
		}
	case tbl.Shortcut(ui.Cmd, ui.KeyC):
		if r.g.copyRows(c) {
			sh.toastOK(locales.StatusCopied())
		}
	}
}

// previewHandle 是网格和行预览之间的分隔线，拖动改变预览栏宽度。
func (r *queryResult) previewHandle(c *ui.Context, p tokens) {
	dx, reset := vHandle(c, p, "qphandle")
	r.previewW = min(max(r.previewW-dx, 220), 720)
	if reset {
		r.previewW = 340
	}
}

// resultPreview 画行预览栏：和表数据页共用一份（rowpane.go），只是只读、没有字段右端的下拉箭头。
func (r *queryResult) resultPreview(c *ui.Context, p tokens, sh *Shell) {
	r.pv.view(c, p, paneSource{g: r.g, ro: true, rawJSON: sh.prefs.RawJSON, onClose: func() { r.showPreview = false }})
}
