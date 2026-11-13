package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// columnsDialog chooses which columns of the grid are shown (⌘⌥F), as
// TablePlus's "Show Selected Column" does.
func (t *tableTab) columnsDialog(c *ui.Context) {
	if !t.colsOpen {
		return
	}
	sh := t.w.sh
	p := sh.tokens(c)
	sh.dialog(c, &t.colsOpen, dlgOpts{title: locales.TableColumnsTitle(), sub: t.table, width: 380, extra: func() {
		if ui.Button(c, locales.TableColumnsAll()).Height(sizeBtnSm).FontSize(font2XS).Clicked() {
			t.g.hidden = nil
			t.g.rebuild()
			t.g.list.Columns = ui.TableLayout{}
		}
		if ui.Button(c, locales.TableColumnsNone()).Height(sizeBtnSm).FontSize(font2XS).Clicked() {
			for _, col := range t.g.cols[1:] { // at least the first column stays
				t.g.setHidden(col.Name, true)
			}
		}
	}}, func() {
		ui.SearchField(c.Key("colsq"), &t.colsQ).Placeholder(locales.TableColumnsSearch()).Height(sizeBtn).FillWidth()
		q := strings.ToLower(strings.TrimSpace(t.colsQ))
		dlgBox(c, p).FillWidth().Children(func() {
			ui.Scroll(c.Key("colslist")).MaxHeight(320).FillWidth().Children(func() {
				ui.Column(c).Padding(6).Gap(1).FillWidth().Children(func() {
					for _, col := range t.g.cols {
						col := col
						if q != "" && !strings.Contains(strings.ToLower(col.Name), q) {
							continue
						}
						shown := !t.g.hidden[col.Name]
						ui.Row(c.Key(col.Name)).Gap(8).AlignItems(ui.Center).Padding(3, 6).FillWidth().Children(func() {
							if ui.Checkbox(c.Key("cb"), &shown, "").Changed() {
								t.g.setHidden(col.Name, !shown)
							}
							ui.Text(c, col.Name).Grow(1).SingleLine().Font(mono).FontSize(font2XS)
							ui.Text(c, col.Type).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine()
						})
					}
				})
			})
		})
	}, nil)
}

// hiddenCount is the number of columns left out of the grid.
func (t *tableTab) hiddenCount() int { return len(t.g.hidden) }
