package ui

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// ---- 按键组合 ----

// chord 是一个快捷键：修饰键加一个主键。
type chord struct {
	mods ui.Modifiers
	key  ui.Key
}

// valid 报告是否真的绑定了按键（未绑定时 key 为 KeyUnknown）。
func (ch chord) valid() bool { return ch.key != ui.KeyUnknown }

// keyNames 是主键和它存进配置的名字。
var keyNames = func() map[ui.Key]string {
	m := map[ui.Key]string{}
	for i := 0; i < 26; i++ {
		m[ui.KeyA+ui.Key(i)] = string(rune('a' + i))
	}
	for i := 0; i < 10; i++ {
		m[ui.Key0+ui.Key(i)] = string(rune('0' + i))
	}
	for i := 0; i < 12; i++ {
		m[ui.KeyF1+ui.Key(i)] = "f" + strconv.Itoa(i+1)
	}
	for k, n := range map[ui.Key]string{
		ui.KeyEnter: "enter", ui.KeyEscape: "escape", ui.KeyBackspace: "backspace", ui.KeyTab: "tab", ui.KeySpace: "space",
		ui.KeyDelete: "delete", ui.KeyInsert: "insert", ui.KeyHome: "home", ui.KeyEnd: "end", ui.KeyPageUp: "pageup",
		ui.KeyPageDown: "pagedown", ui.KeyLeft: "left", ui.KeyRight: "right", ui.KeyUp: "up", ui.KeyDown: "down",
		ui.KeyMinus: "minus", ui.KeyEqual: "equal", ui.KeyComma: "comma", ui.KeyPeriod: "period", ui.KeySlash: "slash",
		ui.KeySemicolon: "semicolon", ui.KeyQuote: "quote", ui.KeyBracketLeft: "bracketleft", ui.KeyBracketRight: "bracketright",
		ui.KeyBackslash: "backslash", ui.KeyBackquote: "backquote",
	} {
		m[k] = n
	}
	return m
}()

var keyByName = func() map[string]ui.Key {
	m := map[string]ui.Key{}
	for k, n := range keyNames {
		m[n] = k
	}
	return m
}()

// String 是存进配置的写法，如 "cmd+shift+f"。cmd 是平台的主修饰键（macOS 的 ⌘，其他系统的 Ctrl）。
func (ch chord) String() string {
	name, ok := keyNames[ch.key]
	if !ok {
		return ""
	}
	var parts []string
	rest := ch.mods
	if rest&ui.Cmd != 0 {
		parts = append(parts, "cmd")
		rest &^= ui.Cmd
	}
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Ctrl, "ctrl"}, {ui.Super, "super"}, {ui.Alt, "alt"}, {ui.Shift, "shift"}} {
		if rest&m.mod != 0 {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, name), "+")
}

// parseChord 读回配置里的写法；认不得的返回 false。
func parseChord(s string) (chord, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	key, ok := keyByName[parts[len(parts)-1]]
	if !ok {
		return chord{}, false
	}
	ch := chord{key: key}
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "cmd":
			ch.mods |= ui.Cmd
		case "ctrl":
			ch.mods |= ui.Ctrl
		case "super":
			ch.mods |= ui.Super
		case "alt":
			ch.mods |= ui.Alt
		case "shift":
			ch.mods |= ui.Shift
		default:
			return chord{}, false
		}
	}
	return ch, true
}

// glyphs 是主键在界面上的写法。
var glyphs = map[ui.Key]string{
	ui.KeyEnter: "↵", ui.KeyEscape: "⎋", ui.KeyBackspace: "⌫", ui.KeyTab: "⇥", ui.KeySpace: "Space", ui.KeyDelete: "⌦",
	ui.KeyInsert: "Ins", ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PgUp", ui.KeyPageDown: "PgDn",
	ui.KeyLeft: "←", ui.KeyRight: "→", ui.KeyUp: "↑", ui.KeyDown: "↓",
	ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".", ui.KeySlash: "/", ui.KeySemicolon: ";",
	ui.KeyQuote: "'", ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]", ui.KeyBackslash: "\\", ui.KeyBackquote: "`",
}

func keyLabel(k ui.Key) string {
	if g, ok := glyphs[k]; ok {
		return g
	}
	return strings.ToUpper(keyNames[k])
}

// Label 是界面上显示的写法：macOS 用 ⌃⌥⇧⌘ 符号，其他系统用 Ctrl+Shift+E。
func (ch chord) Label() string {
	if !ch.valid() {
		return ""
	}
	if runtime.GOOS == "darwin" {
		var b strings.Builder
		for _, m := range []struct {
			mod ui.Modifiers
			sym string
		}{{ui.Ctrl, "⌃"}, {ui.Alt, "⌥"}, {ui.Shift, "⇧"}, {ui.Super, "⌘"}} {
			if ch.mods&m.mod != 0 {
				b.WriteString(m.sym)
			}
		}
		return b.String() + keyLabel(ch.key)
	}
	var parts []string
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Ctrl, "Ctrl"}, {ui.Super, "Win"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}} {
		if ch.mods&m.mod != 0 {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, keyLabel(ch.key)), "+")
}

// accelerator 是菜单栏里用的写法，如 "Super+Shift+f"；没有绑定时为空。
func (ch chord) accelerator() string {
	name, ok := keyNames[ch.key]
	if !ok {
		return ""
	}
	var parts []string
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Super, "Super"}, {ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}} {
		if ch.mods&m.mod != 0 {
			parts = append(parts, m.name)
		}
	}
	switch ch.key {
	case ui.KeyEnter:
		name = "Enter"
	case ui.KeyEscape:
		name = "Escape"
	case ui.KeyBackspace:
		name = "Backspace"
	case ui.KeyTab:
		name = "Tab"
	case ui.KeySpace:
		name = "Space"
	case ui.KeyDelete:
		name = "Delete"
	case ui.KeyInsert:
		name = "Insert"
	case ui.KeyHome:
		name = "Home"
	case ui.KeyEnd:
		name = "End"
	case ui.KeyPageUp:
		name = "PageUp"
	case ui.KeyPageDown:
		name = "PageDown"
	case ui.KeyLeft:
		name = "Left"
	case ui.KeyRight:
		name = "Right"
	case ui.KeyUp:
		name = "Up"
	case ui.KeyDown:
		name = "Down"
	case ui.KeyMinus:
		name = "-"
	case ui.KeyEqual:
		name = "="
	case ui.KeyComma:
		name = ","
	case ui.KeyPeriod:
		name = "."
	case ui.KeySlash:
		name = "/"
	case ui.KeySemicolon:
		name = ";"
	case ui.KeyQuote:
		name = "'"
	case ui.KeyBracketLeft:
		name = "["
	case ui.KeyBracketRight:
		name = "]"
	case ui.KeyBackslash:
		name = "\\"
	case ui.KeyBackquote:
		name = "`"
	default:
		if ch.key >= ui.KeyF1 && ch.key <= ui.KeyF12 {
			name = strings.ToUpper(name)
		}
	}
	return strings.Join(append(parts, name), "+")
}

// bindable 报告一个组合能不能当快捷键：要带 ⌘ 或 Ctrl，功能键可以单独用。
// Esc 留给关闭弹窗，不能绑。
func (ch chord) bindable() bool {
	if !ch.valid() || ch.key == ui.KeyEscape {
		return false
	}
	if ch.key >= ui.KeyF1 && ch.key <= ui.KeyF12 {
		return true
	}
	return ch.mods&(ui.Cmd|ui.Ctrl|ui.Super) != 0
}

// ---- 命令表 ----

// 命令所在的分组，决定设置页里的分段。
const (
	cmdGlobal = iota
	cmdTables
	cmdEditor
)

// command 是一个可以改键的命令。
type command struct {
	id    string
	group int
	name  func() string
	def   chord
}

// commands 是所有可改键的命令，含默认键；快捷键、菜单、提示和设置页都读它。
var commands = []command{
	{"new-conn", cmdGlobal, locales.ScNewConn, chord{ui.Cmd, ui.KeyN}},
	{"open-conn", cmdGlobal, locales.ScOpenConn, chord{ui.Cmd, ui.KeyO}},
	{"new-query", cmdGlobal, locales.ScNewQuery, chord{ui.Cmd, ui.KeyT}},
	{"new-sql", cmdGlobal, locales.ScNewSQL, chord{ui.Cmd, ui.KeyE}},
	{"palette", cmdGlobal, locales.ScQuickSearch, chord{ui.Cmd, ui.KeyP}},
	{"switcher", cmdGlobal, locales.ScSwitchDB, chord{ui.Cmd, ui.KeyK}},
	{"settings", cmdGlobal, locales.ScSettings, chord{ui.Cmd, ui.KeyComma}},
	{"close-tab", cmdGlobal, locales.ScCloseTab, chord{ui.Cmd, ui.KeyW}},
	{"tab-prev", cmdGlobal, locales.MenuPrevTab, chord{ui.Cmd, ui.KeyBracketLeft}},
	{"tab-next", cmdGlobal, locales.MenuNextTab, chord{ui.Cmd, ui.KeyBracketRight}},
	{"ws-prev", cmdGlobal, locales.KmPrevWorkspace, chord{ui.Cmd | ui.Shift, ui.KeyBracketLeft}},
	{"ws-next", cmdGlobal, locales.KmNextWorkspace, chord{ui.Cmd | ui.Shift, ui.KeyBracketRight}},
	{"workspaces", cmdGlobal, locales.WsAllTitle, chord{ui.Cmd | ui.Shift, ui.KeyBackslash}},
	{"ai", cmdGlobal, locales.ScAI, chord{ui.Cmd, ui.KeyI}},
	{"sidebar", cmdGlobal, locales.ToolbarSidebar, chord{ui.Cmd, ui.Key0}},
	{"processes", cmdGlobal, locales.MenuProcesses, chord{ui.Cmd, ui.KeyPeriod}},
	{"export", cmdGlobal, locales.MenuExport, chord{ui.Cmd | ui.Shift, ui.KeyE}},
	{"reconnect", cmdGlobal, locales.ToolbarReconnect, chord{ui.Cmd | ui.Shift, ui.KeyR}},
	{"disconnect", cmdGlobal, locales.ToolbarDisconnect, chord{ui.Cmd | ui.Alt, ui.KeyPeriod}},
	{"back", cmdGlobal, locales.MenuBack, chord{ui.Cmd | ui.Ctrl, ui.KeyLeft}},
	{"forward", cmdGlobal, locales.MenuForward, chord{ui.Cmd | ui.Ctrl, ui.KeyRight}},
	{"console", cmdGlobal, locales.ToolbarConsole, chord{ui.Cmd | ui.Ctrl, ui.KeyC}},
	{"fullscreen", cmdGlobal, locales.KmFullscreen, chord{ui.Cmd | ui.Ctrl, ui.KeyF}},
	{"refresh", cmdTables, locales.ScRefresh, chord{ui.Cmd, ui.KeyR}},
	{"refresh-tab", cmdTables, locales.MenuRefreshTab, chord{ui.Cmd | ui.Alt, ui.KeyR}},
	{"filter", cmdTables, locales.ScFilter, chord{ui.Cmd, ui.KeyF}},
	{"side-filter", cmdTables, locales.KmSideFilter, chord{ui.Cmd | ui.Shift, ui.KeyF}},
	{"columns", cmdTables, locales.ScColumns, chord{ui.Cmd | ui.Alt, ui.KeyF}},
	{"sub-prev", cmdTables, locales.KmPrevSub, chord{ui.Cmd | ui.Alt, ui.KeyBracketLeft}},
	{"sub-next", cmdTables, locales.KmNextSub, chord{ui.Cmd | ui.Alt, ui.KeyBracketRight}},
	{"commit", cmdTables, locales.ScCommit, chord{ui.Cmd, ui.KeyS}},
	{"preview", cmdTables, locales.TablePreviewSQL, chord{ui.Cmd | ui.Shift, ui.KeyP}},
	{"discard", cmdTables, locales.TableDiscard, chord{ui.Cmd | ui.Shift, ui.KeyBackspace}},
	{"run", cmdEditor, locales.MenuRun, chord{ui.Cmd, ui.KeyEnter}},
	{"run-all", cmdEditor, locales.MenuRunAll, chord{ui.Cmd | ui.Shift, ui.KeyEnter}},
	{"format", cmdEditor, locales.MenuFormat, chord{ui.Cmd | ui.Shift, ui.KeyF}},
	{"favorite", cmdEditor, locales.ScFavorite, chord{ui.Cmd, ui.KeyD}},
	{"font-up", cmdEditor, locales.MenuFontUp, chord{ui.Cmd, ui.KeyEqual}},
	{"font-down", cmdEditor, locales.MenuFontDown, chord{ui.Cmd, ui.KeyMinus}},
}

// fixedChords 是不能改、也不能被别的命令占用的键：系统和编辑器自己的，以及写死的别名。
// 设置页的"固定快捷键"一栏列的是同一批。
var fixedChords = func() []chord {
	out := []chord{
		{ui.Cmd, ui.KeyC}, {ui.Cmd, ui.KeyV}, {ui.Cmd, ui.KeyX}, {ui.Cmd, ui.KeyA}, {ui.Cmd, ui.KeyZ},
		{ui.Cmd | ui.Shift, ui.KeyZ}, {ui.Cmd, ui.KeyQ}, {ui.Cmd, ui.KeyH}, {ui.Cmd, ui.KeyM},
		{ui.Cmd | ui.Shift, ui.KeyW}, {ui.Cmd, ui.KeyY}, {ui.Cmd | ui.Shift, ui.KeyI}, {ui.Cmd | ui.Shift, ui.KeyD},
		{ui.Cmd, ui.KeySlash}, {ui.Cmd | ui.Shift, ui.KeyK}, {ui.Cmd | ui.Ctrl, ui.KeyBracketLeft},
		{ui.Cmd | ui.Ctrl, ui.KeyBracketRight}, {ui.Ctrl, ui.KeyBackquote},
	}
	for k := ui.Key1; k <= ui.Key9; k++ {
		out = append(out, chord{ui.Cmd, k})
	}
	return out
}()

func commandByID(id string) (command, bool) {
	for _, c := range commands {
		if c.id == id {
			return c, true
		}
	}
	return command{}, false
}

// ---- 当前的键位 ----

// chordOf 是命令现在的快捷键：用户改过就用改过的，清空了就没有，否则是默认键。
func (s *Shell) chordOf(id string) chord {
	if ch, ok := s.keyCache[id]; ok {
		return ch
	}
	def, ok := commandByID(id)
	if !ok {
		return chord{}
	}
	ch := def.def
	if v, set := s.prefs.Keymap[id]; set {
		if v == keyNone {
			ch = chord{}
		} else if parsed, ok := parseChord(v); ok && parsed.bindable() {
			ch = parsed
		}
	}
	if s.keyCache == nil {
		s.keyCache = map[string]chord{}
	}
	s.keyCache[id] = ch
	return ch
}

// keyNone 在配置里表示"这个命令不绑快捷键"。
const keyNone = "none"

// hit 报告命令的快捷键这一帧是否按下了。
func (s *Shell) hit(c *ui.Context, id string) bool {
	ch := s.chordOf(id)
	return ch.valid() && c.Shortcut(ch.mods, ch.key)
}

// keyHint 是放在按钮提示后面的快捷键，如 " (⌘N)"；没有绑定时为空。
func (s *Shell) keyHint(id string) string {
	if l := s.chordOf(id).Label(); l != "" {
		return " (" + l + ")"
	}
	return ""
}

// cmdKeys 是命令的快捷键写法（用于菜单和命令面板），没有绑定时为空。
func (s *Shell) cmdKeys(id string) string { return s.chordOf(id).Label() }

// conflict 返回已经占用这个组合的命令名（排除 except 自己）；没有冲突返回空串。
func (s *Shell) conflict(ch chord, except string) string {
	for _, cm := range commands {
		if cm.id != except && s.chordOf(cm.id) == ch {
			return cm.name()
		}
	}
	for _, f := range fixedChords {
		if f == ch {
			return locales.KmReserved()
		}
	}
	return ""
}

// bindKey 给命令绑定新的快捷键。组合不合规或已被占用时返回提示，不做修改。
func (s *Shell) bindKey(id string, ch chord) string {
	def, ok := commandByID(id)
	if !ok {
		return ""
	}
	if !ch.bindable() {
		return locales.KmNeedModifier()
	}
	if who := s.conflict(ch, id); who != "" {
		return locales.KmConflict(ch.Label(), who)
	}
	s.setKey(id, ch.String(), def)
	return ""
}

// clearKey 让命令不再有快捷键。
func (s *Shell) clearKey(id string) {
	if def, ok := commandByID(id); ok {
		s.setKey(id, keyNone, def)
	}
}

// resetKey 把命令恢复成默认键。
func (s *Shell) resetKey(id string) {
	delete(s.prefs.Keymap, id)
	s.keymapChanged()
}

// resetKeys 恢复所有命令的默认键。
func (s *Shell) resetKeys() {
	s.prefs.Keymap = nil
	s.keymapChanged()
}

func (s *Shell) setKey(id, val string, def command) {
	if val == def.def.String() { // 和默认一样就不用存
		delete(s.prefs.Keymap, id)
	} else {
		if s.prefs.Keymap == nil {
			s.prefs.Keymap = map[string]string{}
		}
		s.prefs.Keymap[id] = val
	}
	s.keymapChanged()
}

// keymapChanged 在键位变了之后：清缓存、存盘、重建菜单、让编辑器让开新的键。
func (s *Shell) keymapChanged() {
	s.keyCache = nil
	s.savePrefs()
	s.refreshMenu()
	for _, w := range s.spaces {
		for _, t := range w.tabs {
			if q, ok := t.(*queryTab); ok {
				q.ed.Bindings = s.queryBindings()
				if q.pane2 != nil {
					q.pane2.ed.Bindings = s.queryBindings()
				}
			}
		}
	}
}

// refreshMenu 按当前键位重建菜单栏。没有运行中的应用循环时（测试）什么也不做。
func (s *Shell) refreshMenu() {
	if s.win != nil {
		s.menuApply()
	}
}
