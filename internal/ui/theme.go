package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The look is the original MiniDB's: a compact, TablePlus-like desktop tool.
// These are its design tokens (frontend/src/globals.css), light and dark.
// The accent follows the current connection's color, as it did there.

type tokens struct {
	accent, accentHover                    ui.Color
	surface, surfaceSecondary, surfaceElev ui.Color
	fg, fgSecondary, fgMuted               ui.Color
	border, borderSubtle                   ui.Color
	hover                                  ui.Color // --sidebar-hover
	success, warning, danger, info         ui.Color
	rowNew, rowDelete, cellEdit            ui.Color
	toolbar                                ui.Color
	// TablePlus-like semantic colours: what an icon is, and what a value is
	iconTable, iconView, iconQuery ui.Color
	valNum, valTime                ui.Color
}

var (
	lightTokens = tokens{
		accent: ui.Hex("#007aff"), accentHover: ui.Hex("#0066d6"),
		surface: ui.Hex("#ffffff"), surfaceSecondary: ui.Hex("#f5f5f7"), surfaceElev: ui.Hex("#ffffff"),
		fg: ui.Hex("#1d1d1f"), fgSecondary: ui.Hex("#6e6e73"), fgMuted: ui.Hex("#aeaeb2"),
		border: ui.Hex("#d2d2d7"), borderSubtle: ui.Hex("#e5e5ea"),
		hover:   ui.RGBA(0, 0, 0, 0.04),
		success: ui.Hex("#34c759"), warning: ui.Hex("#ff9500"), danger: ui.Hex("#ff3b30"), info: ui.Hex("#5ac8fa"),
		rowNew: ui.Hex("#dbe9c9"), rowDelete: ui.Hex("#efb7c1"), cellEdit: ui.Hex("#fbefbd"),
		toolbar:   ui.Hex("#f6f6f8"),
		iconTable: ui.Hex("#4a89d6"), iconView: ui.Hex("#9a6fd0"), iconQuery: ui.Hex("#2da46a"),
		valNum: ui.Hex("#1f67c9"), valTime: ui.Hex("#0b8a8c"),
	}
	darkTokens = tokens{
		accent: ui.Hex("#0a84ff"), accentHover: ui.Hex("#409cff"),
		surface: ui.Hex("#252626"), surfaceSecondary: ui.Hex("#363636"), surfaceElev: ui.Hex("#464646"),
		fg: ui.Hex("#f5f5f7"), fgSecondary: ui.Hex("#b8b8be"), fgMuted: ui.Hex("#8a8a90"),
		border: ui.Hex("#5a5a5a"), borderSubtle: ui.Hex("#4a4a4a"),
		hover:   ui.Hex("#363636"),
		success: ui.Hex("#30d158"), warning: ui.Hex("#ff9f0a"), danger: ui.Hex("#ff453a"), info: ui.Hex("#64d2ff"),
		rowNew: ui.Hex("#314129"), rowDelete: ui.Hex("#4a2f35"), cellEdit: ui.Hex("#453f22"),
		toolbar:   ui.Hex("#252626"),
		iconTable: ui.Hex("#6aa8f0"), iconView: ui.Hex("#bb92ec"), iconQuery: ui.Hex("#52c98d"),
		valNum: ui.Hex("#6cb0ff"), valTime: ui.Hex("#4fd1c5"),
	}
)

// Metrics of the original's default (non-compact) mode, in DIPs.
const (
	sizeToolbar = 36
	sizeBtn     = 28
	sizeBtnSm   = 24
	sizeTab     = 28
	sizeInput   = 32
	fontBase    = 13 // controls and body text (the original's --size-font-xs)
	fontXS      = 13
	font2XS     = 12 // dense information: tables, tabs, sidebar rows
	radiusBtn   = 7
	radiusPanel = 12
)

// tokens returns the palette for the current appearance with the accent of the
// active connection.
func (s *Shell) tokens(c *ui.Context) tokens {
	p := lightTokens
	if c.Theme().Dark {
		p = darkTokens
	}
	if w := s.activeWorkspace(); w != nil && w.cfg.Color != "" {
		if col := connColor(c, w.cfg.Color); col != c.Theme().TextMuted {
			p.accent, p.accentHover = col, col
		}
	}
	return p
}

// applyTheme installs the original's colors and metrics as the window theme.
func (s *Shell) applyLook(c *ui.Context) tokens {
	p := s.tokens(c)
	t := *c.Theme()
	t.Background = p.surface
	t.Surface = p.surfaceSecondary
	t.SurfaceHover = p.hover
	if !t.Dark {
		t.SurfaceHover = ui.Hex("#ececef")
	}
	t.SurfacePressed = t.SurfaceHover
	t.Border = p.border
	t.Text, t.TextMuted = p.fg, p.fgSecondary
	t.Accent, t.AccentHover, t.AccentPressed = p.accent, p.accentHover, p.accentHover
	t.AccentText = ui.Hex("#ffffff")
	t.Danger, t.Warning, t.Success = p.danger, p.warning, p.success
	t.Radius = radiusBtn
	t.FontSize = fontBase
	t.Spacing = 3
	c.SetTheme(&t)
	return p
}

// Mono is the family for tables, SQL and identifiers (the original's --font-mono).
const mono = "SF Mono, Menlo, Monaco, Consolas, monospace"

// valueColor is the colour of a cell's text by its column type: numbers blue,
// dates and times teal, the rest the text colour; NULL is drawn muted.
func valueColor(p tokens, typ string) ui.Color {
	t := strings.ToUpper(typ)
	switch {
	case strings.Contains(t, "POINT"), strings.Contains(t, "GEOM"):
		return p.fg
	case strings.Contains(t, "INT"), strings.Contains(t, "DEC"), strings.Contains(t, "NUM"), strings.Contains(t, "FLOAT"),
		strings.Contains(t, "DOUBLE"), strings.Contains(t, "REAL"), strings.Contains(t, "SERIAL"), strings.Contains(t, "MONEY"):
		return p.valNum
	case strings.Contains(t, "DATE"), strings.Contains(t, "TIME"), strings.Contains(t, "YEAR"):
		return p.valTime
	}
	return p.fg
}

// iconColor is the colour of an icon for a thing: tables blue, views purple,
// queries green.
func iconColor(p tokens, kind string) ui.Color {
	switch kind {
	case "table":
		return p.iconTable
	case "view":
		return p.iconView
	case "query":
		return p.iconQuery
	}
	return p.fgSecondary
}
