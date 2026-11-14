package ui

import (
	"github.com/egoist/mygo/ui"
	"runtime"
	"strconv"
	"strings"
)

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
