package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// dlgOpts describes a dialog: its size and header.
type dlgOpts struct {
	title, sub string
	width      float32
	height     float32 // 0 sizes the dialog to its content
	lead       func()  // before the title, e.g. a database tile
	extra      func()  // in the header, before the close button
}

// dialog is the one look of the app's dialogs: a header with title, optional
// subtitle and a close button; the body; and an optional footer band with
// the actions at the right. Sizes follow the original's controls, 28px tall
// at 13px, so every dialog reads the same.
func (s *Shell) dialog(c *ui.Context, open *bool, o dlgOpts, body func(), footer func()) {
	p := s.tokens(c)
	ct := *c.Theme()
	ct.Spacing, ct.FontSize = 2.75, fontBase
	ui.Modal(c, open, func() {
		card := ui.Themed(c, &ct).Background(ui.Transparent).Width(o.width)
		if o.height > 0 {
			card.Height(o.height)
		}
		card.Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(14, 12, 10, 18).FillWidth().Shrink(0).Children(func() {
				if o.lead != nil {
					o.lead()
				}
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, o.title).FontWeight(600).FontSize(15).SingleLine()
					if o.sub != "" {
						ui.Text(c, o.sub).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
					}
				})
				if o.extra != nil {
					o.extra()
				}
				if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					*open = false
				}
			})
			b := ui.Column(c).Gap(10).Padding(2, 18, 16, 18).FillWidth().MinHeight(0)
			if o.height > 0 {
				b.Grow(1)
			}
			b.Children(body)
			if footer != nil {
				hairline(c, p.borderSubtle)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.End).Padding(10, 14).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(footer)
			}
		})
	}).Padding(0).Clip()
}

// dlgBox is the recessed well dialogs show code, lists and previews in.
func dlgBox(c *ui.Context, p tokens) ui.Element {
	return ui.Column(c).Background(p.surfaceSecondary).Radius(radiusBtn+1).Border(1, p.borderSubtle)
}

// dlgField is a labelled row of a dialog form: label above, control below.
func dlgField(c *ui.Context, p tokens, label string, fn func()) {
	ui.Column(c).Gap(4).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary)
		fn()
	})
}

// popup is the quick-pick look of the palette and the database switcher: a
// search row with no title above a list, which the caller builds.
// onArrow moves the highlight by +1 or -1: the input would otherwise keep the
// arrow keys for its caret.
func (s *Shell) popup(c *ui.Context, open *bool, width float32, query *string, placeholder string, onChange func(), onSubmit func(), onArrow func(delta int), list func()) {
	p := s.tokens(c)
	ct := *c.Theme()
	ct.Spacing, ct.FontSize = 2.75, fontBase
	ui.Modal(c, open, func() {
		ui.Themed(c, &ct).Background(ui.Transparent).Width(width).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(0, 16).Height(46).FillWidth().Shrink(0).Children(func() {
				ui.Icon(c, icSearch).FontSize(15).TextColor(p.fgMuted)
				in := ui.TextInput(c.Key("popup-q"), query).Placeholder(placeholder).FontSize(fontBase+1).Grow(1).AutoFocus().Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
					OnTextKey(func(k ui.TextKey) bool {
						if k.Mods != 0 || onArrow == nil {
							return false
						}
						switch k.Key {
						case ui.KeyDown:
							onArrow(1)
						case ui.KeyUp:
							onArrow(-1)
						default:
							return false
						}
						s.redraw()
						return true
					})
				if in.Changed() && onChange != nil {
					onChange()
				}
				if in.Submitted() && onSubmit != nil {
					onSubmit()
				}
			})
			hairline(c, p.borderSubtle)
			ui.Column(c).Padding(6).FillWidth().Children(list)
		})
	}).Padding(0).Clip()
}
