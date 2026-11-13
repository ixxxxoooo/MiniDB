package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

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

// isJSONText 报告一段文字是不是 JSON 对象或数组（决定字段用带高亮的编辑器）。
func isJSONText(text string) bool {
	s := strings.TrimSpace(text)
	return len(s) >= 2 && (s[0] == '{' || s[0] == '[') && json.Valid([]byte(s))
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
	if t.pv.json == nil {
		t.pv.json, t.pv.ed, t.pv.own = map[string]bool{}, map[string]*editor.State{}, map[string]string{}
	}
	// the buffer follows the data, except while the user is typing in it.
	// 用哪种输入控件（单行、多行、JSON 编辑器）也在这时定下来，输入过程中不换，免得中途丢了焦点
	sig := fmt.Sprint(rows, "\x00", show, null, mixed)
	if t.pv.sig[col.Name] != sig {
		t.pv.sig[col.Name] = sig
		if own, ok := t.pv.own[col.Name]; ok && own == text {
			// 数据是随用户刚输入的内容更新的：输入框里已经是它，不要再改写（比如把 JSON 重新缩进）
		} else {
			t.pv.buf[col.Name] = show
			t.pv.json[col.Name] = isJSONText(show)
			delete(t.pv.own, col.Name)
			if ed := t.pv.ed[col.Name]; ed != nil {
				ed.Buffer.Set(show)
			}
		}
	}
	buf := t.pv.buf[col.Name]
	if t.pv.menu == nil {
		t.pv.menu = map[string]*bool{}
	}
	if t.pv.menu[col.Name] == nil {
		t.pv.menu[col.Name] = new(bool)
	}
	menuOpen := t.pv.menu[col.Name]
	ph := ""
	switch {
	case mixed:
		ph = locales.TableMultiValue()
	case null:
		ph = locales.TableNull()
	}
	commit := func(v string) {
		if ro || !t.writable(rows[0]) {
			return
		}
		t.pv.buf[col.Name], t.pv.own[col.Name] = v, v
		for _, r := range rows {
			t.setCell(r, col.Name, v)
		}
	}
	ui.Column(c.Key(col.Name)).Gap(3).FillWidth().Children(func() {
		// 字段名在左，类型在右
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Height(20).Children(func() {
			ui.Text(c, col.Name).FontSize(font2XS - 1).FontWeight(500).TextColor(p.fgSecondary).SingleLine().Shrink(1).MinWidth(0)
			ui.Spacer(c)
			ui.Text(c, col.Type).FontSize(10).TextColor(p.fgMuted).SingleLine().Shrink(0)
		})
		// 输入框和它右端的下拉箭头叠在一起：箭头点开就是字段菜单
		ui.Column(c.Key("box")).FillWidth().Children(func() {
			switch {
			case t.pv.json[col.Name]:
				// JSON 值：和 Quick Look 一样用带语法高亮的编辑器
				ed := t.pv.ed[col.Name]
				if ed == nil {
					b := editor.NewBuffer(buf)
					b.SetLanguage(editor.LanguageNamed("JSON"))
					ed = &editor.State{Buffer: b, SoftWrap: true, LineNumbers: editor.LineNumbersOff, NoFolding: true, NoActiveLine: true}
					ed.Font.Size = font2XS
					t.pv.ed[col.Name] = ed
				}
				ed.ReadOnly = ro
				editorBars(c, ed)
				lines := min(max(strings.Count(ed.Buffer.String(), "\n")+1, 2), 10)
				ui.Column(c.Key("val-j")).FillWidth().Height(float32(lines)*17+10).Clip().Radius(radiusBtn).
					Border(1, p.border).Background(p.surfaceSecondary).Children(func() {
					editor.View(c.Key("val-je"), ed).Grow(1).FillWidth()
				})
				if cur := ed.Buffer.String(); cur != buf {
					commit(cur)
				}
			case strings.Contains(text, "\n") || len([]rune(text)) > 60 || strings.Contains(buf, "\n"):
				in := ui.TextArea(c.Key("val-a"), &buf).Lines(2, 8).Placeholder(ph).FillWidth().Font(mono).FontSize(font2XS).ReadOnly(ro)
				if in.Changed() {
					commit(buf)
				}
			default:
				in := ui.TextInput(c.Key("val-i"), &buf).Placeholder(ph).FillWidth().Height(sizeBtnSm).Font(mono).FontSize(font2XS).ReadOnly(ro)
				if in.Changed() {
					commit(buf)
				}
			}
			tipBtn(c.Key("menu"), icChevronD, locales.ConnMore()).Size(20, 20).Absolute().Top(2).Right(4).PopoverMenuButton(func(m *ui.Menu) {
				t.fieldMenu(c, m, rows, col, text, ro)
			}, ui.PopoverMenuButtonOptions{Open: menuOpen})
		})
	})
}

// fieldMenu 是字段输入框右端下拉箭头里的菜单：快捷值、JSON 美化 / 压缩、面板级选项、复制。
func (t *tableTab) fieldMenu(c *ui.Context, m *ui.Menu, rows []int, col database.ColumnMeta, text string, ro bool) {
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
	if m.Item(locales.PreviewAutoJSON()).Checked(!t.w.sh.prefs.RawJSON).Chosen() {
		defer func() {
			t.w.sh.prefs.RawJSON = !t.w.sh.prefs.RawJSON
			t.w.sh.savePrefs()
			t.pv.sig = map[string]string{}
		}()
	}
	if m.Item(locales.TableQuickLook()).Shortcut(ui.Cmd, ui.KeyEnter).Chosen() {
		t.openQuickLook(rows[0], col.Name)
	}
	if m.Item(locales.PreviewCopy()).Chosen() {
		c.Services().WriteClipboard(text)
		t.w.sh.toastOK(locales.StatusCopied())
	}
}
