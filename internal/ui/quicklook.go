package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/locales"
)

// quickLook is the state of the Quick Look dialog: one cell's value, read
// comfortably (long text, JSON as highlighted text or a tree) and edited if allowed.
type quickLook struct {
	row    int
	col    string
	ed     *editor.State // 文本编辑器；JSON 带语法高亮
	orig   string        // 单元格里原来的文本
	shown  string        // 打开时编辑器里的文本（JSON 已缩进），用来判断有没有改过
	null   bool
	isJSON bool
	tree   bool
	info   valueInfo // 对原值的识别：种类和可读写法
	jt     jsonTree  // JSON 树的折叠状态
	blob   blobState // 二进制值的状态，见 quicklook_blob.go
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
	ed := &editor.State{Buffer: buf, SoftWrap: true, LineNumbers: editor.LineNumbersOff, NoFolding: !isJSON, ReadOnly: t.w.cfg.ReadOnly || !t.w.rowEditable()}
	noBracketMatch(ed)
	t.ql = quickLook{row: row, col: col, ed: ed, orig: text, shown: shown, null: null, isJSON: isJSON, info: info}
	if b, ok := blobOf(t.g.colMeta(col), t.g.rows[row][col]); ok {
		t.ql.openBlob(b, t.w.cfg.ReadOnly || !t.w.rowEditable())
	}
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
	ro := t.w.cfg.ReadOnly || !t.w.rowEditable()
	dirty := q.text() != q.shown
	if q.blob.on {
		dirty = q.blobDirty()
	}
	sh.dialog(c, &t.qlOpen, dlgOpts{title: locales.TableQuickLook(), sub: t.table + " · " + q.col, width: 700, height: 560, extra: func() {
		if q.blob.on {
			t.qlBlobHeader(c, q, ro)
		}
		if q.isJSON {
			mode := 0
			if q.tree {
				mode = 1
			}
			if ui.Segmented(c, &mode, locales.QlRaw(), locales.QlTree()).Changed() {
				q.tree = mode == 1
			}
			if actBtn(c, icFormat, "PRETTY").Disabled(ro).Clicked() {
				if pretty, ok := prettyValue(q.text()); ok {
					q.setText(pretty)
					q.tree = false
				}
			}
			if actBtn(c, icUglify, "MINIFY").Disabled(ro).Clicked() {
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
		foot := locales.QlChars(len([]rune(q.text())))
		if q.blob.on {
			n := len(q.blob.orig)
			if b, ok := q.bytes(); ok {
				n = len(b)
			}
			foot = locales.QlByteCount(n)
		}
		ui.Text(c, foot).FontSize(font2XS - 1).TextColor(p.fgMuted).Grow(1)
		if footBtn(c, locales.CommonClose()).Clicked() {
			t.qlOpen = false
		}
		if !ro {
			if footPrimary(c, locales.CommonApply()).Disabled(!dirty).Clicked() {
				if q.blob.on {
					b, ok := q.bytes()
					if !ok {
						sh.toastWarn(locales.QlHexInvalid())
						return
					}
					if t.writable(q.row) {
						t.setCell(q.row, q.col, b) // []byte：写库时按二进制参数绑定
					}
					t.qlOpen = false
					return
				}
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
		ui.Text(c, badge).FontSize(font2XS-1).FontWeight(500).TextColor(col).Padding(1, 7).Radius(8).Background(col.Alpha(0.12))
	})
}

// qlBody 是值本身：JSON 的树，或带高亮的文本。
func (t *tableTab) qlBody(c *ui.Context, p tokens, q *quickLook) {
	if q.blob.on && (q.blob.dump || q.blob.huge) {
		dlgBox(c, p).Grow(1).FillWidth().MinHeight(0).Clip().Children(func() {
			scrollbars(c, ui.ScrollBoth(c.Key("qlhex")).Grow(1).FillWidth()).Children(func() {
				ui.Text(c, q.dumpText()).Font(mono).FontSize(font2XS).Selectable().Padding(8, 10)
			})
		})
		return
	}
	if q.isJSON && q.tree {
		if v, ok := parseJSONValue(q.text()); ok {
			dlgBox(c, p).Grow(1).FillWidth().MinHeight(0).Children(func() {
				scrollbars(c, ui.Scroll(c.Key("qltree")).Grow(1).FillWidth()).Children(func() {
					ui.Column(c).Padding(8).FillWidth().Children(func() { q.jt.node(c, p, t.w.sh.copyText, "$", "", v, 0) })
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
		editorBars(c, q.ed)
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
		scrollbars(c, ui.Scroll(c.Key("qlderived")).FillWidth()).Children(func() {
			for i, d := range q.info.derived {
				d := d
				ui.Row(c.Key(fmt.Sprint("qld", i))).Gap(10).AlignItems(ui.Center).Padding(4, 8, 4, 12).FillWidth().Children(func() {
					ui.Text(c, d.label).FontSize(font2XS).TextColor(p.fgMuted).Width(96).Shrink(0).SingleLine()
					ui.Text(c, d.value).Font(mono).FontSize(font2XS).Grow(1).MinWidth(0).Selectable()
					if tipBtn(c, icCopy, locales.PreviewCopy()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
						c.Services().WriteClipboard(d.value)
						t.w.sh.toastOK(locales.StatusCopied())
					}
				})
				if i < len(q.info.derived)-1 {
					hairline(c, p.borderSubtle)
				}
			}
		})
	})
}
