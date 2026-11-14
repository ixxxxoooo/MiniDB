package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// The original's keyboard shortcuts (Settings → Shortcuts), handled here for
// the whole window so they work wherever the focus is. The menu bar has the
// same accelerators; fire keeps a key that reaches both from running twice.

// modalOpen reports whether a dialog holds the keyboard.
func (s *Shell) modalOpen() bool {
	return s.anyModalOpen() || s.notesOpen // 通知中心是弹出层，不在对话框清单里，但同样占着键盘
}

// fire runs fn unless the same command ran a moment ago.
func (s *Shell) fire(name string, fn func()) {
	now := time.Now()
	if s.lastKey == name && now.Sub(s.lastKeyAt) < 150*time.Millisecond {
		return
	}
	s.lastKey, s.lastKeyAt = name, now
	fn()
}

func (s *Shell) activeTable() *tableTab {
	if w := s.activeWorkspace(); w != nil && w.active < len(w.tabs) {
		t, _ := w.tabs[w.active].(*tableTab)
		return t
	}
	return nil
}

func (s *Shell) shortcuts(c *ui.Context) {
	if s.modalOpen() {
		return
	}
	// 可以在设置里改键的命令：键由 keymap 决定，动作见 command_run.go。
	// 编辑器里的 run / format 等由编辑器自己处理按键，commandRuns 里的同名动作只给菜单与命令面板用。
	// fixed 是改不了的别名和单键，和 fixedChords 对应。
	type fixedSC struct {
		mods ui.Modifiers
		key  ui.Key
		name string
		fn   func()
	}
	w := s.activeWorkspace()
	subView := func(d int) {
		if t := s.activeTable(); t != nil {
			t.mode = (t.mode + d + 4) % 4
			t.modeChanged()
		}
	}
	runs := s.commandRuns()
	fixed := []fixedSC{
		{ui.Cmd | ui.Shift, ui.KeyK, "open-conn2", func() { s.newConnectionDialog() }},
		{ui.Cmd | ui.Ctrl, ui.KeyBracketLeft, "sub-prev2", func() { subView(-1) }},
		{ui.Cmd | ui.Ctrl, ui.KeyBracketRight, "sub-next2", func() { subView(1) }},
		{0, ui.KeyEscape, "deselect", func() {
			// TablePlus: Esc deselects all rows (the filter bar, a cell being
			// edited and the dialogs take Esc first)
			if t := s.activeTable(); t != nil && t.mode == 0 && !t.showFilter && t.es.editing == nil && !t.qlOpen && t.quick == nil {
				t.g.chosen.Clear()
			}
		}},
		{0, ui.KeyInsert, "insert", func() {
			if t := s.activeTable(); t != nil && !t.w.cfg.ReadOnly {
				switch t.mode {
				case 0:
					t.addRow()
				case 1:
					t.st.addColumn()
				}
			}
		}},
	}
	// ⌘1 到 ⌘9 按序号切换标签
	for i, key := range []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5, ui.Key6, ui.Key7, ui.Key8, ui.Key9} {
		i := i
		fixed = append(fixed, fixedSC{ui.Cmd, key, fmt.Sprint("tab", i), func() {
			if w != nil && i < len(w.tabs) {
				w.active = i
			}
		}})
	}
	for _, cmd := range commands {
		if editorHandled[cmd.id] {
			continue
		}
		if fn := runs[cmd.id]; fn != nil && s.hit(c, cmd.id) {
			s.fire(cmd.id, fn)
			s.redraw()
		}
	}
	for _, k := range fixed {
		if c.Shortcut(k.mods, k.key) {
			s.fire(k.name, k.fn)
			s.redraw()
		}
	}
}
