package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// iconButton is a flat button showing only an icon.
func iconButton(c *ui.Context, ic *ui.SVG, tip string) ui.Element {
	return tipOn(c, ui.Button(c, "").Label(tip), tip).Padding(0).BorderWidth(0).Background(ui.Color{}).Children(func() { ui.Icon(c, ic) })
}

// labeledButton is a button with an icon before its label.
func labeledButton(c *ui.Context, ic *ui.SVG, label string, primary bool) ui.Element {
	body := func() {
		ui.Icon(c, ic)
		ui.Text(c, label).SingleLine()
	}
	if primary {
		return ui.PrimaryButton(c, "").Label(label).Children(body)
	}
	return ui.Button(c, "").Label(label).Children(body)
}

func tagColor(c *ui.Context, tag string) ui.Color {
	t := c.Theme()
	switch tag {
	case "production":
		return t.Danger
	case "test", "staging":
		return t.Warning
	case "development":
		return t.Accent
	}
	return t.Success
}

// connColor is a connection's own color, or a neutral one.
func connColor(c *ui.Context, color string) ui.Color {
	if strings.HasPrefix(color, "#") && (len(color) == 7 || len(color) == 4) {
		return ui.Hex(color)
	}
	return c.Theme().TextMuted
}

// cellText renders a database value for a grid cell.
func cellText(v any) (text string, null bool) {
	switch x := v.(type) {
	case nil:
		return "NULL", true
	case string:
		return x, false
	case []byte:
		return string(x), false
	case time.Time:
		return x.Format("2006-01-02 15:04:05"), false
	case float64:
		if x == float64(int64(x)) && x < 1e15 && x > -1e15 {
			return fmt.Sprintf("%d", int64(x)), false
		}
		return fmt.Sprintf("%v", x), false
	}
	return fmt.Sprintf("%v", v), false
}

// oneLine collapses a value for display in a single-line cell.
func oneLine(s string) string {
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return strings.NewReplacer("\r\n", " ⏎ ", "\n", " ⏎ ", "\r", " ⏎ ", "\t", " ").Replace(s)
}

// clipText 截断悬浮提示里的长文本：最多 maxRunes 个字符、maxLines 行，超出用省略号收尾。
func clipText(s string, maxRunes, maxLines int) string {
	s = strings.TrimSpace(s)
	cut := false
	if lines := strings.Split(s, "\n"); len(lines) > maxLines {
		s, cut = strings.Join(lines[:maxLines], "\n"), true
	}
	if rs := []rune(s); len(rs) > maxRunes {
		s, cut = string(rs[:maxRunes]), true
	}
	if cut {
		s += "…"
	}
	return s
}

// bigTip 在 anchor 旁边显示单元格里的完整内容：等宽字体，字号、延迟与其它提示一致（见 tipOnFont）。
func bigTip(c *ui.Context, anchor ui.Element, text string) {
	tipOnFont(c, anchor, text, mono)
}
