package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// quickLook is the state of the Quick Look dialog: one cell's value, read
// comfortably (long text, JSON as raw text or a tree) and edited if allowed.
type quickLook struct {
	row      int
	col      string
	text     string
	orig     string
	null     bool
	isJSON   bool
	tree     bool
	expanded map[string]bool
	collapse map[string]bool
}

// openQuickLook opens Quick Look on a cell (⌘↵, a middle click, or the menu).
func (t *tableTab) openQuickLook(row int, col string) {
	if row < 0 || row >= len(t.g.rows) {
		return
	}
	text, null := cellText(t.g.rows[row][col])
	if null {
		text = ""
	}
	var probe any
	isJSON := false
	if tr := strings.TrimSpace(text); len(tr) > 1 && (tr[0] == '{' || tr[0] == '[') {
		isJSON = json.Unmarshal([]byte(tr), &probe) == nil
	}
	t.ql = quickLook{row: row, col: col, text: text, orig: text, null: null, isJSON: isJSON, tree: isJSON, collapse: map[string]bool{}}
	t.qlOpen = true
}

// quickLookDialog shows Quick Look.
func (t *tableTab) quickLookDialog(c *ui.Context) {
	if !t.qlOpen {
		return
	}
	q := &t.ql
	sh := t.w.sh
	p := sh.tokens(c)
	ro := t.w.cfg.ReadOnly
	dirty := q.text != q.orig
	sh.dialog(c, &t.qlOpen, dlgOpts{title: locales.TableQuickLook(), sub: t.table + " · " + q.col, width: 640, height: 480, extra: func() {
		if q.isJSON {
			mode := 0
			if q.tree {
				mode = 1
			}
			if ui.Segmented(c, &mode, "Raw", "Tree").Changed() {
				q.tree = mode == 1
			}
			if ui.Button(c, "PRETTY").Height(sizeBtnSm).FontSize(font2XS).Disabled(ro).Clicked() {
				if pretty, ok := prettyValue(q.text); ok {
					q.text, q.tree = pretty, false
				}
			}
			if ui.Button(c, "MINIFY").Height(sizeBtnSm).FontSize(font2XS).Disabled(ro).Clicked() {
				if mini, ok := minifyJSON(q.text); ok {
					q.text, q.tree = mini, false
				}
			}
		}
	}}, func() {
		if q.isJSON && q.tree {
			var v any
			dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(q.text)))
			dec.UseNumber()
			if dec.Decode(&v) == nil {
				dlgBox(c, p).Grow(1).FillWidth().Children(func() {
					ui.Scroll(c.Key("qltree")).Grow(1).FillWidth().Children(func() {
						ui.Column(c).Padding(8).FillWidth().Children(func() { jsonNode(c, p, q, "$", "", v, 0) })
					})
				})
				return
			}
			q.tree = false
		}
		ph := ""
		if q.null {
			ph = locales.TableNull()
		}
		ui.TextArea(c.Key("qltext"), &q.text).Lines(8, 40).Placeholder(ph).Grow(1).FillWidth().Font(mono).FontSize(font2XS + 1).ReadOnly(ro)
	}, func() {
		ui.Text(c, fmt.Sprintf("%d", len([]rune(q.text)))+" ch").FontSize(font2XS - 1).TextColor(p.fgMuted).Grow(1)
		if ui.Button(c, locales.CommonClose()).Height(sizeBtn).Clicked() {
			t.qlOpen = false
		}
		if !ro {
			if ui.PrimaryButton(c, locales.CommonApply()).Height(sizeBtn).Disabled(!dirty).Clicked() {
				if t.writable(q.row) {
					t.setCell(q.row, q.col, q.text)
				}
				t.qlOpen = false
			}
		}
	})
}

// jsonNode draws one value of a JSON tree; objects and arrays fold.
func jsonNode(c *ui.Context, p tokens, q *quickLook, path, key string, v any, depth int) {
	label := func() {
		if key != "" {
			ui.Text(c, key).Font(mono).FontSize(font2XS).TextColor(p.iconView).SingleLine()
			ui.Text(c, ":").Font(mono).FontSize(font2XS).TextColor(p.fgMuted)
		}
	}
	scalar := func(text string, col ui.Color) {
		ui.Row(c.Key(path)).Gap(4).AlignItems(ui.Center).Padding(1, 0, 1, float32(depth)*14+16).FillWidth().Children(func() {
			label()
			ui.Text(c, text).Font(mono).FontSize(font2XS).TextColor(col).Selectable()
		})
	}
	fold := func(open, close string, n int, children func()) {
		folded := q.collapse[path] || (depth >= 2 && !q.expanded[path])
		ic := icChevronD
		if folded {
			ic = icChevronR
		}
		row := ui.Row(c.Key(path)).Gap(4).AlignItems(ui.Center).Padding(1, 0, 1, float32(depth)*14).FillWidth()
		row.Children(func() {
			ui.Icon(c, ic).FontSize(11).TextColor(p.fgMuted).Width(12)
			label()
			ui.Text(c, open+" "+fmt.Sprint(n)+" "+close).Font(mono).FontSize(font2XS - 1).TextColor(p.fgMuted)
		})
		if row.Clicked() {
			if folded {
				q.collapse[path] = false
				if q.expanded == nil {
					q.expanded = map[string]bool{}
				}
				q.expanded[path] = true
			} else {
				q.collapse[path] = true
				delete(q.expanded, path)
			}
		}
		if !folded {
			children()
		}
	}
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fold("{", "keys }", len(keys), func() {
			for _, k := range keys {
				jsonNode(c, p, q, path+"."+k, k, x[k], depth+1)
			}
		})
	case []any:
		fold("[", "items ]", len(x), func() {
			for i, it := range x {
				jsonNode(c, p, q, fmt.Sprintf("%s[%d]", path, i), fmt.Sprint(i), it, depth+1)
			}
		})
	case string:
		scalar(`"`+x+`"`, p.fg)
	case json.Number:
		scalar(x.String(), p.valNum)
	case bool:
		scalar(fmt.Sprint(x), p.iconView)
	case nil:
		scalar("null", p.fgMuted)
	default:
		scalar(fmt.Sprint(x), p.fg)
	}
}

var _ = bytes.NewBuffer
