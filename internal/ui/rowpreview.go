package ui

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
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
func (t *tableTab) chosenRows() []int { return t.g.chosenRows() }

// fieldValue is a field's text across rows: its value when they agree, else mixed.
func (t *tableTab) fieldValue(rows []int, col string) (text string, null, mixed bool) {
	return t.g.fieldValue(rows, col)
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
// 画法和查询结果页共用（rowpane.go），这里只给出能改值和下拉菜单。
func (t *tableTab) rowPreview(c *ui.Context, p tokens) {
	ro := t.w.cfg.ReadOnly || !t.w.rowEditable()
	t.pv.view(c, p, paneSource{
		g: t.g, ro: ro, rawJSON: t.w.sh.prefs.RawJSON,
		commit: func(rows []int, col, v string) bool {
			if !t.writable(rows[0]) {
				return false
			}
			for _, r := range rows {
				t.setCell(r, col, v)
			}
			return true
		},
		menu: func(m *ui.Menu, rows []int, col model.ColumnMeta, text string) {
			t.fieldMenu(c, m, rows, col, text, ro)
		},
		onClose: func() { t.showPreview = false },
		copy:    t.w.sh.copyText,
	})
}

// isJSONText 报告一段文字是不是 JSON 对象或数组（决定字段用带高亮的编辑器）。
func isJSONText(text string) bool {
	s := strings.TrimSpace(text)
	return len(s) >= 2 && (s[0] == '{' || s[0] == '[') && json.Valid([]byte(s))
}

// fieldMenu 是字段输入框右端下拉箭头里的菜单：快捷值、JSON 美化 / 压缩、面板级选项、复制。
func (t *tableTab) fieldMenu(c *ui.Context, m *ui.Menu, rows []int, col model.ColumnMeta, text string, ro bool) {
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
