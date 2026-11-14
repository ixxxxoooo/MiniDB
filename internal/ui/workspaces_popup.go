package ui

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"minidb/locales"
	"strings"
)

type wsPopup struct {
	open  bool
	query string
	sel   int
}

func (s *Shell) workspacesPopup(c *ui.Context) {
	d := &s.wsPop
	if !d.open {
		return
	}
	p := s.tokens(c)
	q := strings.ToLower(strings.TrimSpace(d.query))
	var idx []int
	for i, w := range s.spaces {
		if q == "" || strings.Contains(strings.ToLower(w.cfg.Name+" "+w.db), q) {
			idx = append(idx, i)
		}
	}
	d.sel = min(max(d.sel, 0), max(len(idx)-1, 0))
	choose := func() {
		if d.sel < len(idx) {
			s.active = idx[d.sel]
			d.open = false
		}
	}
	s.popup(c, &d.open, 460, &d.query, locales.WsAllTitle(), func() { d.sel = 0 }, choose, func(n int) { d.sel = min(max(d.sel+n, 0), max(len(idx)-1, 0)) }, func() {
		for k, i := range idx {
			w := s.spaces[i]
			row := ui.Row(c.Key(w.id())).Gap(10).Padding(7, 10).Radius(radiusBtn + 1).AlignItems(ui.Center).FillWidth()
			switch {
			case k == d.sel:
				row.Background(p.accent.Alpha(0.12))
			case row.Hovered():
				row.Background(p.hover)
			}
			row.Children(func() {
				dbTile(c, w.cfg.Type, 20, connColor(c, w.cfg.Color))
				ui.Text(c, w.cfg.Name).SingleLine().Grow(1)
				dbName := w.db
				if n := w.ordinal(); n > 1 {
					dbName = fmt.Sprintf("%s #%d", dbName, n)
				}
				ui.Text(c, dbName).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine()
				if i == s.active {
					ui.Icon(c, icCheck).FontSize(14).TextColor(p.accent)
				}
			})
			if row.Clicked() {
				d.sel = k
				choose()
			}
		}
	})
}

// closeOtherSpaces closes every workspace but w, asking first about changes.
func (s *Shell) closeOtherSpaces(keep *Workspace) {
	n := 0
	for _, w := range s.spaces {
		if w != keep {
			n += w.pendingTotal()
		}
	}
	s.discardGuard(n, func() {
		for i := len(s.spaces) - 1; i >= 0; i-- {
			if s.spaces[i] != keep {
				s.closeSpace(i)
			}
		}
		for i, w := range s.spaces {
			if w == keep {
				s.active = i
			}
		}
	})
}
