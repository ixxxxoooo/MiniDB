package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// previewRow is the first chosen row, or -1.
func (t *tableTab) previewRow() int {
	row := -1
	for r := range t.g.chosen.All() {
		if row < 0 || r < row {
			row = r
		}
	}
	if row >= len(t.g.rows) {
		return -1
	}
	return row
}

// prettyValue indents a value that is JSON; other text is returned as is.
func prettyValue(text string) (string, bool) {
	s := strings.TrimSpace(text)
	if len(s) < 2 || (s[0] != '{' && s[0] != '[') {
		return text, false
	}
	var out bytes.Buffer
	if json.Indent(&out, []byte(s), "", "  ") != nil {
		return text, false
	}
	return out.String(), true
}

// chosenRows are the valid chosen rows, in order.
func (t *tableTab) chosenRows() []int {
	var rows []int
	for r := range t.g.chosen.All() {
		if r >= 0 && r < len(t.g.rows) {
			rows = append(rows, r)
		}
	}
	sort.Ints(rows)
	return rows
}

// fieldValue is a field's text across rows: its value when they agree, else mixed.
func (t *tableTab) fieldValue(rows []int, col string) (text string, null, mixed bool) {
	for i, r := range rows {
		tx, n := cellText(t.g.rows[r][col])
		if i == 0 {
			text, null = tx, n
			continue
		}
		if tx != text || n != null {
			return "", false, true
		}
	}
	return text, null, false
}

// minifyJSON is the value on one line, if it is JSON.
func minifyJSON(text string) (string, bool) {
	var out bytes.Buffer
	if json.Compact(&out, []byte(strings.TrimSpace(text))) != nil || out.Len() < 2 || (out.Bytes()[0] != '{' && out.Bytes()[0] != '[') {
		return text, false
	}
	return out.String(), true
}

// rowPreview is the row pane Space opens beside the grid, TablePlus's right
// sidebar: a field per line that can be edited in place (what is typed joins
// the pending changes), with a menu of quick values, JSON pretty/minify, a
// search for fields, and editing of one field across several chosen rows.
func (t *tableTab) rowPreview(c *ui.Context, p tokens) {
	rows := t.chosenRows()
	ro := t.w.cfg.ReadOnly
	if t.pv.buf == nil {
		t.pv.buf, t.pv.sig = map[string]string{}, map[string]string{}
	}
	ui.Column(c).Grow(1).FillHeight().Background(p.surface).Children(func() {
		ui.Row(c).Gap(6).Padding(0, 8, 0, 12).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 2).Shrink(0).Children(func() {
			ui.Text(c, locales.PreviewTitle()).FontSize(font2XS).FontWeight(500)
			if len(rows) > 1 {
				ui.Text(c, locales.TableRows(len(rows))).FontSize(font2XS - 1).TextColor(p.fgMuted)
			}
			ui.Spacer(c)
			if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				t.showPreview = false
			}
		})
		hairline(c, p.borderSubtle)
		if len(rows) == 0 {
			ui.Text(c, locales.PreviewNone()).FontSize(font2XS).TextColor(p.fgMuted).Padding(14)
			return
		}
		ui.Row(c).Padding(8, 10, 2, 10).FillWidth().Shrink(0).Children(func() {
			ui.SearchField(c.Key("fieldq"), &t.pv.q).Placeholder(locales.TableFieldSearch()).Height(sizeBtnSm).FontSize(font2XS).FillWidth()
		})
		q := strings.ToLower(strings.TrimSpace(t.pv.q))
		scrollbars(c, ui.Scroll(c.Key("rowpreview")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Gap(10).Padding(8, 12, 12, 12).FillWidth().Children(func() {
				for _, i := range t.g.vis {
					col := t.g.cols[i]
					if q != "" && !strings.Contains(strings.ToLower(col.Name), q) {
						continue
					}
					t.fieldEditor(c, p, rows, col, ro)
				}
			})
		})
	})
}

// fieldEditor is one field of the row pane.
func (t *tableTab) fieldEditor(c *ui.Context, p tokens, rows []int, col database.ColumnMeta, ro bool) {
	text, null, mixed := t.fieldValue(rows, col.Name)
	show := text
	if null || mixed {
		show = ""
	}
	if pretty, ok := prettyValue(show); ok && !t.w.sh.prefs.RawJSON && len(rows) == 1 {
		show = pretty // JSON reads better indented; it changes the data only once edited
	}
	// the buffer follows the data, except while the user is typing in it
	sig := fmt.Sprint(rows, "\x00", show, null, mixed)
	if t.pv.sig[col.Name] != sig {
		t.pv.sig[col.Name], t.pv.buf[col.Name] = sig, show
	}
	buf := t.pv.buf[col.Name]
	if t.pv.menu == nil {
		t.pv.menu = map[string]*bool{}
	}
	if t.pv.menu[col.Name] == nil {
		t.pv.menu[col.Name] = new(bool)
	}
	menuOpen := t.pv.menu[col.Name]
	field := ui.Column(c.Key(col.Name)).Gap(3).FillWidth()
	// ⋯ 只在鼠标停在这个字段上（或菜单正开着）时出现，不再每个字段都摆一个
	showMenu := field.Hovered() || *menuOpen
	field.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Height(20).Children(func() {
			ui.Text(c, col.Name).FontSize(font2XS - 1).FontWeight(500).TextColor(p.fgSecondary).SingleLine()
			ui.Text(c, col.Type).FontSize(10).TextColor(p.fgMuted).SingleLine()
			ui.Spacer(c)
			if !showMenu {
				return
			}
			ui.PopoverMenuButton(c.Key("menu"), "⋯", func(m *ui.Menu) {
				for _, v := range quickValues(col.Type) {
					if m.Item(v).Disabled(ro).Chosen() {
						for _, r := range rows {
							t.setQuickValue(r, col.Name, v)
						}
					}
				}
				if pretty, ok := prettyValue(text); ok && len(rows) == 1 {
					m.Separator()
					if m.Item("PRETTY").Disabled(ro).Chosen() {
						t.setCell(rows[0], col.Name, pretty)
					}
					if mini, ok := minifyJSON(text); ok && m.Item("MINIFY").Disabled(ro).Chosen() {
						t.setCell(rows[0], col.Name, mini)
					}
				}
				m.Separator()
				// 整个面板只留这一个 ⋯：面板级的选项也放在字段菜单里
				if m.Item(locales.PreviewAutoJSON()).Checked(!t.w.sh.prefs.RawJSON).Chosen() {
					defer func() {
						t.w.sh.prefs.RawJSON = !t.w.sh.prefs.RawJSON
						t.w.sh.savePrefs()
						t.pv.sig = map[string]string{}
					}()
				}
				if m.Item(locales.TableQuickLook() + " (⌘↵)").Chosen() {
					t.openQuickLook(rows[0], col.Name)
				}
				if m.Item(locales.PreviewCopy()).Chosen() {
					c.Services().WriteClipboard(text)
					t.w.sh.toastOK(locales.StatusCopied())
				}
			}, ui.PopoverMenuButtonOptions{Open: menuOpen}).Height(20).FontSize(font2XS)
		})
		ph := ""
		switch {
		case mixed:
			ph = locales.TableMultiValue()
		case null:
			ph = locales.TableNull()
		}
		// one line of text is a field; long or multi-line values get a text area
		var in ui.Element
		if strings.Contains(text, "\n") || len([]rune(text)) > 60 || strings.Contains(buf, "\n") {
			in = ui.TextArea(c.Key("val-a"), &buf).Lines(2, 8).Placeholder(ph).FillWidth().Font(mono).FontSize(font2XS).ReadOnly(ro)
		} else {
			in = ui.TextInput(c.Key("val-i"), &buf).Placeholder(ph).FillWidth().Height(sizeBtnSm).Font(mono).FontSize(font2XS).ReadOnly(ro)
		}
		if in.Changed() && !ro && t.writable(rows[0]) {
			t.pv.buf[col.Name] = buf
			for _, r := range rows {
				t.setCell(r, col.Name, buf)
			}
		}
	})
}
