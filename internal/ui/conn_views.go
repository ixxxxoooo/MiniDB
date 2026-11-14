package ui

import (
	"fmt"
	"minidb/internal/model"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 连接管理器是一个两级对话框，固定大小，内容在两页之间切换：
//
//	列表页（conn_list.go）：已保存的连接 + 列表下方的"再添加新的连接"。
//	配置页（conn_form.go）：新建或编辑一条连接，数据库类型是带图标的下拉（conn_type.go）。
//
// 流转关系：列表 → 点行连接 / ✎ 编辑 / "再添加" 进入配置页；配置页 → 返回、Esc、
// 保存都回到列表（保存后高亮刚保存的那条），连接则直接打开并关闭对话框。
// 一条连接都没有时直接落在配置页，没有"返回"。

// tagColors are the status colours of the environment tags.
var tagColors = map[string]string{"local": "#34c759", "development": "#0a84ff", "test": "#ff9f0a", "staging": "#af52de", "production": "#ff453a"}

var tagOrder = []string{"local", "development", "test", "staging", "production"}

// 对话框的宽度固定；配置页高度固定，列表页按连接条数自适应（见 connDialogHeight）。
const connDialogW, connDialogH = 600, 520

// connectionDialog 画连接管理器：d.form 为真时是配置页，否则是列表页。
// 参数 c 为界面上下文；无返回值。
func (s *Shell) connectionDialog(c *ui.Context) {
	d := &s.dlg
	if !d.open {
		return
	}
	p := s.tokens(c)
	s.connSync()
	ct := *c.Theme()
	ct.Spacing, ct.FontSize = 2.75, fontBase
	submit, back := false, false
	m := ui.Modal(c, &d.open, func() {
		ui.Themed(c, &ct).Background(ui.Transparent).Width(connDialogW).Height(s.connDialogHeight()).Children(func() {
			ui.Column(c).Fill().Children(func() {
				if d.form {
					s.connForm(c, p, &submit, &back)
				} else {
					s.connList(c, p)
				}
			})
		})
	}).Padding(0).Clip()
	// 配置页里 Esc / 点遮罩是"返回列表"，而不是关掉整个对话框
	if m.Dismissed() && d.form && s.canBackToList() {
		d.open = true
		s.leaveForm()
	}
	if back { // 返回要等这一帧画完：leaveForm 会清掉表单，不能在画到一半时做
		s.leaveForm()
	}
	if d.form && c.Shortcut(ui.Cmd, ui.KeyS) {
		s.saveConnection(false)
	}
	if submit {
		s.saveConnection(true)
	}
}

// connSync 在每帧开头理顺管理器的状态：刚打开时，一条连接都没有就直接进入配置页，
// 否则落在列表页并让光标停在当前（或第一条）连接上；正在编辑的连接被别处删掉时退回列表。
func (s *Shell) connSync() {
	d := &s.dlg
	if !s.connsReady {
		return
	}
	if !d.ready {
		d.ready = true
		switch {
		case len(s.conns) == 0:
			s.startDraft(model.MySQL)
		case s.selectedConnection() == nil:
			s.selected = s.conns[0].ID
		}
		return
	}
	if d.form && d.editing && !d.draft && !s.hasConnection(d.cfg.ID) {
		s.showList()
	}
}

// connDialogHeight 是对话框当前的高度：配置页固定；列表页 = 页头、搜索框、页脚加上每条连接一行，
// 至少够放下空状态，最多不超过配置页，免得一两条连接也撑出一大片空白。
func (s *Shell) connDialogHeight() float32 {
	if s.dlg.form {
		return connDialogH
	}
	rows := max(len(s.filteredConns()), 4) // 空状态大约占四行
	return float32(min(150+40*rows, connDialogH))
}

// hasConnection 判断 id 是否还在已保存的连接里。
func (s *Shell) hasConnection(id string) bool {
	for _, cfg := range s.conns {
		if cfg.ID == id {
			return true
		}
	}
	return false
}

// canBackToList 报告列表页是否有内容可回：没有任何连接时配置页就是唯一的一页。
func (s *Shell) canBackToList() bool { return len(s.conns) > 0 }

// showList 切到列表页：丢掉表单里的内容，保留搜索词，让搜索框重新拿到键盘。
func (s *Shell) showList() {
	s.dlg = connDialog{open: s.dlg.open, ready: true, search: s.dlg.search}
}

// leaveForm 从配置页回到列表页；表单有没保存的修改时先问一句。
func (s *Shell) leaveForm() {
	if s.dlg.dirty() {
		s.askWarn(locales.ConnDiscardTitle(), locales.ConnDiscardMsg(), locales.ConnDiscard(), s.showList)
		return
	}
	s.showList()
}

// startDraft 开始填写一条新连接，类型先按 driver，保留搜索词。
func (s *Shell) startDraft(driver model.DBType) {
	open, search := s.dlg.open, s.dlg.search
	s.beginForm(driver)
	s.dlg.open, s.dlg.search = open, search
}

// sig 是表单当前内容的指纹，用来判断有没有未保存的修改。
func (d *connDialog) sig() string {
	return fmt.Sprintf("%+v|%s|%s|%s|%s|%d|%s|%s", d.cfg, d.port, d.sshPort, d.sshAuth, d.safeSel, d.pwMode, d.password, d.secret)
}

// rebase 把表单当前内容记为"没有修改"的基线。
func (d *connDialog) rebase() { d.base = d.sig() }

// dirty 报告表单自载入以来是否改动过。
func (d *connDialog) dirty() bool { return d.form && d.sig() != d.base }
