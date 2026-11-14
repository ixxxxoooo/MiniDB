package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// dbSwitcher is the database picker the toolbar's database icon opens (⌘K).
type dbSwitcher struct {
	open  bool
	query string
	sel   int
	w     *Workspace
}

func (s *Shell) openSwitcher() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	s.sw = dbSwitcher{open: true, w: w}
	var dbs []model.DatabaseInfo
	s.up.bg(func() { dbs, _ = s.app.Databases.GetAllDatabases(w.cfg.ID) }, func() {
		if len(dbs) > 0 {
			w.dbs = dbs
		}
	})
}

func (s *Shell) switcherView(c *ui.Context) {
	d := &s.sw
	if !d.open || d.w == nil {
		return
	}
	p := s.tokens(c)
	q := strings.ToLower(strings.TrimSpace(d.query))
	var shown []model.DatabaseInfo
	for _, db := range d.w.dbs {
		if q == "" || strings.Contains(strings.ToLower(db.Name), q) {
			shown = append(shown, db)
		}
	}
	if d.sel >= len(shown) {
		d.sel = max(0, len(shown)-1)
	}
	choose := func(i int) {
		if i >= 0 && i < len(shown) {
			d.open = false
			d.w.switchDatabase(shown[i].Name)
		}
	}
	s.popup(c, &d.open, 460, &d.query, locales.DbSwitchSearch(), func() { d.sel = 0 }, func() { choose(d.sel) }, func(n int) { d.sel = min(max(d.sel+n, 0), max(len(shown)-1, 0)) }, func() {
		if len(shown) == 0 {
			ui.Text(c, locales.PaletteNone()).TextColor(p.fgMuted).Padding(14).TextAlign(ui.Center).FillWidth()
		}
		scrollbars(c, ui.Scroll(c.Key("dbs")).MaxHeight(320).FillWidth()).Children(func() {
			ui.Column(c).Gap(1).FillWidth().Children(func() {
				for i, db := range shown {
					i, db := i, db
					row := ui.Row(c.Key(db.Name)).Gap(10).Padding(7, 10).Radius(radiusBtn + 1).AlignItems(ui.Center).FillWidth()
					switch {
					case i == d.sel:
						row.Background(p.accent.Alpha(0.12))
					case row.Hovered():
						row.Background(p.hover)
					}
					row.Children(func() {
						ui.Icon(c, icDatabase).FontSize(14).TextColor(p.fgSecondary)
						ui.Text(c, db.Name).SingleLine().Grow(1)
						if db.Name == d.w.db {
							ui.Icon(c, icCheck).FontSize(14).TextColor(p.accent)
						}
						if db.TableCount > 0 {
							ui.Text(c, locales.DbSwitchTables(db.TableCount)).FontSize(font2XS - 1).TextColor(p.fgMuted)
						}
					})
					if row.Clicked() {
						choose(i)
					}
					if app.CapsOf(d.w.cfg.Type).CreateDatabase && !d.w.cfg.ReadOnly {
						row.ContextMenu(func(m *ui.Menu) {
							if m.Item(locales.DbRename()).Chosen() {
								defer func() { d.open = false; s.renameDatabase(d.w, db.Name) }()
							}
							if m.Item(locales.DbDrop()).Chosen() {
								defer func() { d.open = false; s.dropDatabase(d.w, db.Name) }()
							}
						})
					}
				}
			})
		})
	})
	switch {
	case c.Shortcut(0, ui.KeyDown):
		d.sel = min(d.sel+1, len(shown)-1)
	case c.Shortcut(0, ui.KeyUp):
		d.sel = max(d.sel-1, 0)
	}
}
