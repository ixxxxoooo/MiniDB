package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// 界面的令牌见 theme_tokens.go（结构）、theme_palettes.go（色值）、theme_metrics.go（尺寸），
// 规范文档是 docs/UI_TOKENS.md。这里负责把令牌装进窗口主题，以及按类型取语义色。

// tokens returns the scheme for the current appearance. 强调色固定，不再跟随连接颜色：
// 连接色只用在连接头像和生产环境的顶部色带上。
func (s *Shell) tokens(c *ui.Context) tokens {
	return tokensFor(activeScheme, c.Theme().Dark)
}

// applyLook 把令牌装进窗口主题，并返回当前令牌。
func (s *Shell) applyLook(c *ui.Context) tokens {
	p := s.tokens(c)
	t := *c.Theme()
	t.Background = p.surface
	t.Surface = p.surfaceSecondary
	t.SurfaceHover = p.hover
	t.SurfacePressed = p.hover
	t.Border = p.border
	t.Text, t.TextMuted = p.fg, p.fgSecondary
	t.Accent, t.AccentHover, t.AccentPressed = p.accent, p.accentHover, p.accentHover
	t.AccentText = p.accentFg
	t.Danger, t.Warning, t.Success = p.danger, p.warning, p.success
	t.Radius = radiusBtn
	t.FontSize = fontBase
	t.Spacing = 3
	// 悬浮提示由 tipOn 自己计时（停留 tipDelay 才显示），交给框架显示时不再叠加框架的延迟，
	// 否则会等两遍；框架自带的 Tooltip 不再使用。
	t.Inverse, t.InverseText = p.surfaceElev, p.fg
	t.TooltipDelay = -1
	c.SetTheme(&t)
	c.SetMenuPresentation(ui.PopoverMenus) // 右键菜单画在窗口里，套用主题的样子（设计稿 A 的菜单：图标 + 右端键帽）
	return p
}

// Mono is the family for tables, SQL and identifiers (设计稿的 JetBrains Mono，没有安装时依次退到系统等宽字体).
const mono = "JetBrains Mono, SF Mono, Menlo, Monaco, Consolas, monospace"

// valueColor is the colour of a cell's text by its column type: numbers blue,
// dates and times teal, the rest the text colour; NULL is drawn muted.
func valueColor(p tokens, typ string) ui.Color {
	t := strings.ToUpper(typ)
	switch {
	case strings.Contains(t, "POINT"), strings.Contains(t, "GEOM"):
		return p.fg
	case isNumericType(typ):
		return p.valNum
	case strings.Contains(t, "DATE"), strings.Contains(t, "TIME"), strings.Contains(t, "YEAR"):
		return p.valTime
	}
	return p.fg
}

// isNumericType 判断列类型是不是数值：整数、小数、浮点、序列、货币。几何类型（POINT 里含 INT）不算。
func isNumericType(typ string) bool {
	t := strings.ToUpper(typ)
	if strings.Contains(t, "POINT") || strings.Contains(t, "GEOM") || strings.HasPrefix(t, "ENUM") { // enum 里带 NUM，不是数字
		return false
	}
	for _, k := range []string{"INT", "DEC", "NUM", "FLOAT", "DOUBLE", "REAL", "SERIAL", "MONEY"} {
		if strings.Contains(t, k) {
			return true
		}
	}
	return false
}

// iconColor is the colour of an icon for a thing: tables blue, views purple,
// queries green.
func iconColor(p tokens, kind ObjectKind) ui.Color {
	switch kind {
	case KindTable:
		return p.iconTable
	case KindView:
		return p.iconView
	case KindQuery:
		return p.iconQuery
	case KindFunction, KindProcedure:
		return p.iconRoutine
	}
	return p.fgSecondary
}
