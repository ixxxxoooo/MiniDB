package ui

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// historyPanel lists a connection's SQL history in a dialog.
type historyPanel struct {
	open    bool
	items   []app.HistoryItem
	query   string
	favOnly int // 0 all, 1 favorites
}

func (h *historyPanel) toggle(q *queryTab) {
	h.open = !h.open
	if h.open {
		h.reload(q)
	}
}

func (h *historyPanel) reload(q *queryTab) {
	w := q.w
	var items []app.HistoryItem
	w.sh.up.bg(func() { items, _ = w.sh.app.History.List(w.cfg.ID, h.query, 300) }, func() { h.items = items })
}

func (h *historyPanel) view(c *ui.Context, q *queryTab) {
	if !h.open {
		return
	}
	p := q.w.sh.tokens(c)
	q.w.sh.dialog(c, &h.open, dlgOpts{title: locales.HistoryTitle(), width: 700, height: 500, extra: func() {
		if ui.Segmented(c, &h.favOnly, locales.HistoryTitle(), locales.HistoryFavorites()).Changed() {
			h.reload(q)
		}
	}}, func() {
		if ui.SearchField(c, &h.query).Placeholder(locales.HistorySearch()).Height(sizeBtn).Changed() {
			h.reload(q)
		}
		shown := h.items
		if h.favOnly == 1 {
			shown = nil
			for _, it := range h.items {
				if it.Favorite {
					shown = append(shown, it)
				}
			}
		}
		if len(shown) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
				ui.Icon(c, icHistory).FontSize(26).TextColor(p.fgMuted)
				ui.Text(c, locales.HistoryEmpty()).TextColor(p.fgMuted)
			})
		}
		ui.Scroll(c.Key("history")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).Gap(2).FillWidth().Children(func() {
				for _, it := range shown {
					it := it
					row := ui.Column(c.Key(it.ID)).Gap(3).Padding(8, 10).Radius(radiusBtn + 1).FillWidth()
					hovered := row.Hovered()
					if hovered {
						row.Background(p.hover)
					}
					row.Children(func() {
						ui.Text(c, firstLines(it.SQL, 3)).SingleLine().FontSize(font2XS + 1).Font(mono)
						ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
							ui.Text(c, it.ExecutedAt.Format("2006-01-02 15:04:05")+histDB(it.Database)).FontSize(font2XS - 1).TextColor(p.fgMuted)
							ui.Spacer(c)
							// the actions appear with the pointer; a star stays on favourites
							if it.Favorite && !hovered {
								ui.Text(c, "★").FontSize(font2XS).TextColor(p.warning)
							}
							if hovered {
								label := locales.HistoryFavorite()
								if it.Favorite {
									label = locales.HistoryUnfavorite()
								}
								if ui.Button(c, label).Height(22).FontSize(font2XS - 1).Clicked() {
									q.w.sh.up.bg(func() { _ = q.w.sh.app.History.ToggleFavorite(it.ID) }, func() { h.reload(q) })
								}
								if ui.Button(c, locales.HistoryInsert()).Height(22).FontSize(font2XS - 1).Clicked() {
									q.insertSQL(it.SQL)
									h.open = false
								}
							}
						})
					})
					if row.DoubleClicked() {
						q.insertSQL(it.SQL)
						h.open = false
					}
				}
			})
		})
	}, func() {
		if ui.Button(c, locales.HistoryClear()).Height(sizeBtn).Clicked() {
			q.w.sh.up.bg(func() { _ = q.w.sh.app.History.Clear(q.w.cfg.ID) }, func() { h.reload(q) })
		}
	})
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = append(lines[:n], "…")
	}
	return strings.Join(lines, " ⏎ ")
}

// insertSQL puts sql into the editor at the caret, on its own line.
func (q *queryTab) insertSQL(sql string) {
	src := q.ed.Buffer.String()
	text := sql
	if strings.TrimSpace(src) != "" {
		text = "\n" + sql
		q.ed.Select(len(src), len(src))
	}
	q.ed.Insert(text)
}

func histDB(db string) string {
	if db == "" {
		return ""
	}
	return " · " + filepath.Base(db)
}
