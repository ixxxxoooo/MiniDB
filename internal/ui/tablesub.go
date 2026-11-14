package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"

	"minidb/locales"
)

// ddlState is the read-only CREATE statement of a table.
type ddlState struct {
	loaded  bool
	loading bool
	shownAt time.Time
	err     string
	ed      *editor.State
}

func (d *ddlState) load(t *tableTab) {
	d.loading = true
	w := t.w
	db := w.db
	var ddl string
	var err error
	w.sh.up.bg(func() { ddl, err = w.sh.app.Databases.GetDDL(w.cfg.ID, db, t.table) }, func() {
		d.loaded, d.loading = true, false
		if err != nil {
			d.err = errMessage(err)
			return
		}
		buf := editor.NewBuffer(ddl)
		buf.SetLanguage(editor.LanguageNamed("SQL"))
		d.ed = &editor.State{Buffer: buf, ReadOnly: true, SoftWrap: true}
	})
}

func (t *tableTab) ddlView(c *ui.Context) {
	th := c.Theme()
	d := &t.ddl
	if !d.loaded && !d.loading {
		d.load(t)
	}
	switch {
	case !d.loaded:
		quietLoading(c, baseTokens(c), &d.shownAt)
	case d.err != "":
		ui.Text(c, locales.TableLoadFail(d.err)).TextColor(th.Danger).Padding(16).Selectable()
	default:
		// 只留一个复制图标，浮在编辑器右上角，不再占一整行
		p := baseTokens(c)
		editorBars(c, d.ed)
		ui.Box(c).Grow(1).FillWidth().Children(func() {
			editor.View(c, d.ed).Fill()
			if tipBtn(c.Key("ddl-copy"), icCopy, locales.DdlCopy()).Size(sizeBtnSm+2, sizeBtnSm+2).Radius(radiusBtn).Absolute().Top(8).Right(16).Background(p.surface).Clicked() {
				c.Services().WriteClipboard(d.ed.Buffer.String())
				t.w.sh.toastOK(locales.StatusCopied())
			}
		})
	}
}

// docState is the table's Markdown documentation.
type docState struct {
	loaded     bool
	err        string
	ed         *editor.State
	saved      string
	generating bool
	md         markdown.Options
	split      float32
	loading    bool
	shownAt    time.Time
}

func (d *docState) load(t *tableTab) {
	d.loading = true
	w := t.w
	db := w.db
	var text string
	var err error
	w.sh.up.bg(func() { text, err = w.sh.app.Docs.GetTableDoc(w.cfg.ID, db, t.table) }, func() {
		d.loaded = true
		d.split = 50
		if err != nil {
			d.err = errMessage(err)
			return
		}
		buf := editor.NewBuffer(text)
		buf.SetLanguage(editor.LanguageNamed("Markdown"))
		d.ed = &editor.State{Buffer: buf, SoftWrap: true}
		d.saved = text
	})
}

func (d *docState) dirty() bool { return d.ed != nil && d.ed.Buffer.String() != d.saved }

func (t *tableTab) saveDoc() {
	d := &t.doc
	if d.ed == nil {
		return
	}
	w := t.w
	db, text := w.db, d.ed.Buffer.String()
	var err error
	w.sh.up.bg(func() { err = w.sh.app.Docs.SaveTableDoc(w.cfg.ID, db, t.table, text) }, func() {
		if err != nil {
			w.sh.toastErr(errMessage(err))
			return
		}
		d.saved = text
		w.sh.toastOK(locales.DocSaved())
	})
}

func (t *tableTab) generateDoc() {
	d := &t.doc
	if d.generating || d.ed == nil {
		return
	}
	d.generating = true
	w := t.w
	db := w.db
	var text string
	var err error
	w.sh.up.bg(func() { text, err = w.sh.app.AI.GenerateTableDoc(w.cfg.ID, db, t.table) }, func() {
		d.generating = false
		if err != nil {
			w.sh.toastErr(errMessage(err))
			return
		}
		d.ed.Buffer.Set(strings.TrimSpace(text))
	})
}

func (t *tableTab) docView(c *ui.Context) {
	th := c.Theme()
	d := &t.doc
	switch {
	case !d.loaded:
		quietLoading(c, baseTokens(c), &d.shownAt)
		return
	case d.err != "":
		ui.Text(c, locales.TableLoadFail(d.err)).TextColor(th.Danger).Padding(16).Selectable()
		return
	}
	ui.Row(c).Gap(8).Padding(6, 12).AlignItems(ui.Center).FillWidth().Children(func() {
		if actPrimary(c, icSave, locales.DocSave()).Disabled(!d.dirty()).Clicked() {
			t.saveDoc()
		}
		if actBtn(c, icSparkle, locales.DocGenerate()).Disabled(d.generating).Clicked() {
			t.generateDoc()
		}
		if d.generating {
			ui.Spinner(c)
			ui.Text(c, locales.DocGenerating()).TextColor(th.TextMuted).FontSize(12)
		}
		if d.dirty() {
			ui.Text(c, locales.DocUnsaved()).TextColor(th.Warning).FontSize(12)
		}
	})
	ui.SplitPercent(c, &d.split, func() {
		editorBars(c, d.ed)
		editor.View(c, d.ed).Grow(1).FillWidth()
	}, func() {
		scrollbars(c, ui.Scroll(c.Key("docprev")).Fill()).Children(func() {
			ui.Column(c).Padding(12, 16).FillWidth().Children(func() {
				markdown.View(c, d.ed.Buffer.String(), &d.md)
			})
		})
	}).Grow(1).FillWidth()
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		t.saveDoc()
	}
}
