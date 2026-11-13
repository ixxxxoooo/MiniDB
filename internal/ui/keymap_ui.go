package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// keyRecorder 是设置页里"正在录制某个命令的新快捷键"的状态。
type keyRecorder struct {
	id  string // 录制中的命令，空为没有在录
	msg string // 上一次没绑成功的原因
}

func (r keyRecorder) active() bool { return r.id != "" }

// startRecording 开始为命令录制新键。录制期间菜单栏的快捷键先摘掉，免得按下的键被菜单抢去。
func (s *Shell) startRecording(id string) {
	s.rec = keyRecorder{id: id}
	s.refreshMenu()
}

func (s *Shell) stopRecording() {
	s.rec = keyRecorder{}
	s.refreshMenu()
}

// recordKeys 读这一帧按下的键：能绑就绑上并结束录制，不能绑就提示原因、继续等。
func (s *Shell) recordKeys(c *ui.Context) {
	if !s.rec.active() {
		return
	}
	for _, k := range c.KeysDown() {
		if k == ui.KeyEscape {
			s.stopRecording()
			return
		}
		id := s.rec.id
		if msg := s.bindKey(id, chord{mods: c.Modifiers(), key: k}); msg != "" {
			s.rec.msg = msg
			return
		}
		s.stopRecording()
		return
	}
}

// fixedShortcuts 是改不了的快捷键，设置页最下面列出来。
func fixedShortcuts() []struct {
	title string
	rows  [][2]string
} {
	return []struct {
		title string
		rows  [][2]string
	}{
		{locales.ScGlobal(), [][2]string{{"⌘1 – ⌘9", locales.ScTabNumber()}}},
		{locales.ScTables(), [][2]string{
			{"↑ / ↓", locales.ScMoveRow()}, {"Space", locales.ScPreview()}, {"Insert", locales.ScInsert()},
			{"⌫", locales.ScDelete()}, {"⌘C", locales.ScCopy()},
		}},
		{locales.ScEditor(), [][2]string{{"⌘Y", locales.ScHistory()}, {"⌘/", locales.ScComment()}, {"⌘F", locales.ScFind()}}},
		{locales.ScDialogs(), [][2]string{{"Esc", locales.ScEscape()}, {"↵", locales.ScEnter()}}},
	}
}

// keyChip 是一个快捷键的小标签。
func keyChip(c *ui.Context, p tokens, key, text string) {
	ui.Text(c.Key(key), text).Font(mono).FontSize(font2XS).TextColor(p.fgSecondary).Padding(1, 6).Radius(4).Background(p.surfaceSecondary).Border(1, p.borderSubtle)
}

func (s *Shell) shortcutSettings(c *ui.Context, p tokens) {
	s.recordKeys(c)
	settingsHead(c, p, locales.SettingsShortcuts(), locales.SettingsShortcutsDesc())
	ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, locales.KmHint()).FontSize(font2XS).TextColor(p.fgMuted).Grow(1)
		if ui.Button(c, locales.KmResetAll()).Height(sizeBtn).Disabled(len(s.prefs.Keymap) == 0).Clicked() {
			s.resetKeys()
		}
	})
	if s.rec.msg != "" && s.rec.active() {
		ui.Text(c, s.rec.msg).FontSize(font2XS).TextColor(p.danger).Padding(6, 10).FillWidth().Radius(radiusBtn).Background(p.danger.Alpha(0.10))
	}
	groups := []struct {
		id    int
		title string
	}{{cmdGlobal, locales.ScGlobal()}, {cmdTables, locales.ScTables()}, {cmdEditor, locales.ScEditor()}}
	for _, g := range groups {
		g := g
		ui.Column(c.Key(g.title)).Gap(0).FillWidth().Children(func() {
			ui.Text(c, g.title).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary).Padding(0, 0, 4, 0)
			first := true
			for _, cm := range commands {
				if cm.group != g.id {
					continue
				}
				cm := cm
				if !first {
					hairline(c, p.borderSubtle)
				}
				first = false
				s.commandRow(c, p, cm)
			}
		})
	}
	ui.Text(c, locales.KmFixed()).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary).Margin(8, 0, 0, 0)
	for _, g := range fixedShortcuts() {
		ui.Column(c.Key("fixed-" + g.title)).Gap(0).FillWidth().Children(func() {
			ui.Text(c, g.title).FontSize(font2XS).TextColor(p.fgMuted).Padding(0, 0, 4, 0)
			for i, r := range g.rows {
				ui.Row(c.Key(r[1])).Gap(10).AlignItems(ui.Center).Height(28).FillWidth().Children(func() {
					ui.Text(c, r[1]).Grow(1).SingleLine()
					keyChip(c, p, r[0], r[0])
				})
				if i < len(g.rows)-1 {
					hairline(c, p.borderSubtle)
				}
			}
		})
	}
}

// commandRow 是一个命令的一行：名称、当前快捷键（点一下录制新的）、清空和恢复默认。
func (s *Shell) commandRow(c *ui.Context, p tokens, cm command) {
	cur := s.chordOf(cm.id)
	_, changed := s.prefs.Keymap[cm.id]
	recording := s.rec.id == cm.id
	ui.Row(c.Key("cmd-" + cm.id)).Gap(8).AlignItems(ui.Center).Height(30).FillWidth().Children(func() {
		ui.Text(c, cm.name()).Grow(1).SingleLine()
		label, col := cur.Label(), p.fgSecondary
		switch {
		case recording:
			label, col = locales.KmRecording(), p.accent
		case label == "":
			label, col = locales.KmNone(), p.fgMuted
		}
		chip := ui.Row(c.Key("chip-"+cm.id)).Width(150).Height(sizeBtnSm).Justify(ui.Center).AlignItems(ui.Center).Radius(4).Border(1, p.borderSubtle)
		switch {
		case recording:
			chip.Background(p.accent.Alpha(0.08)).BorderColor(p.accent)
		case chip.Hovered():
			chip.Background(p.hover)
		default:
			chip.Background(p.surfaceSecondary)
		}
		chip.Children(func() {
			ui.Text(c, label).Font(mono).FontSize(font2XS).TextColor(col).SingleLine()
		})
		if chip.Clicked() {
			if recording {
				s.stopRecording()
			} else {
				s.startRecording(cm.id)
			}
		}
		if tipBtn(c, icX, locales.KmClear()).Size(sizeBtnSm, sizeBtnSm).Disabled(!cur.valid()).Clicked() {
			s.clearKey(cm.id)
		}
		if tipBtn(c, icUndo, locales.KmRestore()).Size(sizeBtnSm, sizeBtnSm).Disabled(!changed).Clicked() {
			s.resetKey(cm.id)
		}
	})
}
