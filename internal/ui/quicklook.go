package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/locales"
)

// quickLook is the state of the Quick Look dialog: one cell's value, read
// comfortably (long text, JSON as highlighted text or a tree) and edited if allowed.
type quickLook struct {
	row      int
	col      string
	ed       *editor.State // 文本编辑器；JSON 带语法高亮
	orig     string // 单元格里原来的文本
	shown    string // 打开时编辑器里的文本（JSON 已缩进），用来判断有没有改过
	null     bool
	isJSON   bool
	tree     bool
	info     valueInfo // 对原值的识别：种类和可读写法
	expanded map[string]bool
	collapse map[string]bool
}

// text is what the editor holds now.
func (q *quickLook) text() string { return q.ed.Buffer.String() }

// setText replaces the text, e.g. with the pretty-printed JSON.
func (q *quickLook) setText(s string) { q.ed.Buffer.Set(s) }

// openQuickLook opens Quick Look on a cell (⌘↵, a middle click, or the menu).
func (t *tableTab) openQuickLook(row int, col string) {
	if row < 0 || row >= len(t.g.rows) {
		return
	}
	text, null := cellText(t.g.rows[row][col])
	if null {
		text = ""
	}
	info := detectValue(text, time.Now(), time.Local)
	isJSON := info.kind == kindJSON
	shown := text
	if isJSON { // 缩进后读起来舒服；没改动就不算改过
		if pretty, ok := prettyValue(text); ok {
			shown = pretty
		}
	}
	buf := editor.NewBuffer(shown)
	if isJSON {
		buf.SetLanguage(editor.LanguageNamed("JSON"))
	}
	ed := &editor.State{Buffer: buf, SoftWrap: true, LineNumbers: editor.LineNumbersOff, NoFolding: !isJSON, ReadOnly: t.w.cfg.ReadOnly}
	t.ql = quickLook{row: row, col: col, ed: ed, orig: text, shown: shown, null: null, isJSON: isJSON, info: info, collapse: map[string]bool{}}
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
	dirty := q.text() != q.shown
	sh.dialog(c, &t.qlOpen, dlgOpts{title: locales.TableQuickLook(), sub: t.table + " · " + q.col, width: 700, height: 560, extra: func() {
		if q.isJSON {
			mode := 0
			if q.tree {
				mode = 1
			}
			if ui.Segmented(c, &mode, locales.QlRaw(), locales.QlTree()).Changed() {
				q.tree = mode == 1
			}
			if ui.Button(c, "PRETTY").Height(sizeBtnSm).FontSize(font2XS).Disabled(ro).Clicked() {
				if pretty, ok := prettyValue(q.text()); ok {
					q.setText(pretty)
					q.tree = false
				}
			}
			if ui.Button(c, "MINIFY").Height(sizeBtnSm).FontSize(font2XS).Disabled(ro).Clicked() {
				if mini, ok := minifyJSON(q.text()); ok {
					q.setText(mini)
					q.tree = false
				}
			}
		}
	}}, func() {
		t.qlKindRow(c, p, q)
		t.qlBody(c, p, q)
		t.qlDerived(c, p, q)
	}, func() {
		ui.Text(c, locales.QlChars(len([]rune(q.text())))).FontSize(font2XS - 1).TextColor(p.fgMuted).Grow(1)
		if ui.Button(c, locales.CommonClose()).Height(sizeBtn).Clicked() {
			t.qlOpen = false
		}
		if !ro {
			if ui.PrimaryButton(c, locales.CommonApply()).Height(sizeBtn).Disabled(!dirty).Clicked() {
				if t.writable(q.row) {
					t.setCell(q.row, q.col, q.text())
				}
				t.qlOpen = false
			}
		}
	})
}

// qlKindRow 在正文上方标出识别到的种类。
func (t *tableTab) qlKindRow(c *ui.Context, p tokens, q *quickLook) {
	if q.info.badge == "" && !q.null {
		return
	}
	ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Shrink(0).Children(func() {
		badge, col := q.info.badge, p.accent
		if q.null {
			badge, col = locales.TableNull(), p.fgMuted
		}
		ui.Text(c, badge).FontSize(font2XS - 1).FontWeight(500).TextColor(col).Padding(1, 7).Radius(8).Background(col.Alpha(0.12))
	})
}

// qlBody 是值本身：JSON 的树，或带高亮的文本。
func (t *tableTab) qlBody(c *ui.Context, p tokens, q *quickLook) {
	if q.isJSON && q.tree {
		var v any
		dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(q.text())))
		dec.UseNumber()
		if dec.Decode(&v) == nil {
			dlgBox(c, p).Grow(1).FillWidth().MinHeight(0).Children(func() {
				ui.Scroll(c.Key("qltree")).Grow(1).FillWidth().Children(func() {
					ui.Column(c).Padding(8).FillWidth().Children(func() { jsonNode(c, p, q, "$", "", v, 0) })
				})
			})
			return
		}
		q.tree = false
	}
	if sz := t.w.sh.prefs.EditorFont; sz > 0 {
		q.ed.Font.Size = sz
	}
	dlgBox(c, p).Grow(1).FillWidth().MinHeight(0).Clip().Children(func() {
		editor.View(c.Key("qled"), q.ed).Grow(1).FillWidth()
	})
}

// qlDerived 列出这个值的可读写法，每行可复制。
func (t *tableTab) qlDerived(c *ui.Context, p tokens, q *quickLook) {
	if len(q.info.derived) == 0 {
		return
	}
	ui.Text(c, locales.QlReadable()).FontSize(font2XS).FontWeight(500).TextColor(p.fgSecondary).Shrink(0)
	dlgBox(c, p).FillWidth().Shrink(0).MaxHeight(150).Clip().Children(func() {
		ui.Scroll(c.Key("qlderived")).FillWidth().Children(func() {
			for i, d := range q.info.derived {
				d := d
				ui.Row(c.Key(fmt.Sprint("qld", i))).Gap(10).AlignItems(ui.Center).Padding(4, 8, 4, 12).FillWidth().Children(func() {
					ui.Text(c, d.label).FontSize(font2XS).TextColor(p.fgMuted).Width(96).Shrink(0).SingleLine()
					ui.Text(c, d.value).Font(mono).FontSize(font2XS).Grow(1).MinWidth(0).Selectable()
					if tipBtn(c, icCopy, locales.PreviewCopy()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
						c.Services().WriteClipboard(d.value)
						t.w.sh.toast(locales.StatusCopied())
					}
				})
				if i < len(q.info.derived)-1 {
					hairline(c, p.borderSubtle)
				}
			}
		})
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
