package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

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
