package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 导出 / 导入向导（设计稿：弹窗向导）：头部是图标块 + 标题，下面是横向步骤条，中间是当前这一步，底部是上一步 / 下一步。
// 画法在 transfer_dialog.go；这里是向导的状态和各步共用的小部件。
//   - 导出：选择对象 → 格式与选项 → 目标位置 → 执行（transfer_export.go）；
//   - 导入：选择文件 → 目标表 → 字段映射 → 导入（transfer_import.go）。
//
// 表单状态仍放在 Shell 的 exp / imp 里（导出查询结果的对话框与它共用 exportDialog），
// 所以同一时刻只有一份向导的数据；在另一个连接里再开向导时从头来过。

// 向导的两种模式。
const (
	xferExport = iota
	xferImport
)

// transferTab 是导出或导入向导的状态（弹窗里的那一份，Shell.xf.t）。
type transferTab struct {
	w        *Workspace
	mode     int  // xferExport / xferImport
	step     int  // 当前是第几步（0 起）
	showCols bool // 导出第一步：展开列挑选
	waiting  bool // 导入：已点「开始」，等任务启动后跳到执行步
}

func (t *transferTab) title() string {
	if t.mode == xferImport {
		return locales.XferImport()
	}
	return locales.XferExport()
}

func (t *transferTab) icon() *ui.SVG {
	if t.mode == xferImport {
		return icUpload
	}
	return icDownload
}

// openTransfer 打开指定模式的向导弹窗，返回它的状态；已经开着同一个连接、同一种模式的就沿用。
func (w *Workspace) openTransfer(mode int) *transferTab {
	d := &w.sh.xf
	if d.open && d.t != nil && d.t.w == w && d.t.mode == mode {
		return d.t
	}
	d.open, d.t = true, &transferTab{w: w, mode: mode}
	return d.t
}

// restart 让向导从第一步重新开始（从侧栏、右键菜单再次进入向导时用，表单数据已由调用方换成新的）。
func (t *transferTab) restart() {
	t.step, t.showCols, t.waiting = 0, false, false
}

// prepare 在每帧开头保证表单状态属于当前工作区，并处理「任务启动后跳到执行步」。
func (t *transferTab) prepare() {
	s := t.w.sh
	if t.mode == xferExport {
		if s.exp.w != t.w || (!s.exp.open && s.exp.query != "") { // 别的连接的数据，或上次导出查询结果留下的
			pf := s.prefs.Export
			s.exp = exportDialog{w: t.w, sel: map[string]bool{}, prefs: pf, format: pf.Format}
		}
		return
	}
	if s.imp.w != t.w {
		s.imp = newImportDialog(t.w, "")
	}
	d := &s.imp
	switch {
	case t.waiting && d.xfer.active:
		t.step, t.waiting = 3, false
	case t.waiting && d.err != "" && !d.busy:
		t.waiting = false
	}
}

// navButtons 画页脚右侧的「上一步 / 下一步」：next 为主按钮文字，off 为真时灰着，onNext 是点下一步的动作。
func (t *transferTab) navButtons(c *ui.Context, back bool, next string, off bool, onNext func()) {
	if back && fieldBtn(c.Key("xf-prev"), nil, locales.XferPrev()).Clicked() {
		t.step--
	}
	if fieldPrimary(c.Key("xf-next"), nil, next).Disabled(off).Clicked() {
		onNext()
	}
}

// ---- 向导里反复用到的小部件 ----

// xferCard 是向导里的白底圆角卡片（清单、预览表都放在里面）。
func xferCard(c *ui.Context, p tokens) ui.Element {
	return ui.Column(c).FillWidth().Radius(radiusPanel).Border(1, p.border).Background(p.surface).Clip()
}

// xferSection 是带小标题的一节：标题在上，内容 fn 在下。
func xferSection(c *ui.Context, p tokens, title string, fn func()) {
	ui.Column(c).Gap(spaceSM).FillWidth().Children(func() {
		ui.Text(c, title).FontSize(fontBody).FontWeight(600)
		fn()
	})
}

// switchRow 画一行「说明 …… 开关」，返回开关是否被拨动。key 在同一父元素里要唯一。
func switchRow(c *ui.Context, p tokens, key, label string, v *bool) bool {
	changed := false
	ui.Row(c.Key(key)).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(fontBody).TextColor(p.fgSecondary).SingleLine()
		ui.Spacer(c)
		changed = ui.Switch(c.Key(key+"-sw"), v).Changed()
	})
	return changed
}

// switchNot 同 switchRow，但开关的「开」对应 *v 为假（比如「写入表头」对应 NoHeader）。
func switchNot(c *ui.Context, p tokens, key, label string, v *bool) {
	on := !*v
	if switchRow(c, p, key, label, &on) {
		*v = !on
	}
}
