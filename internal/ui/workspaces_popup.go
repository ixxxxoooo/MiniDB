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
		popupHeading(c, p, locales.WsOpened())
		for k, i := range idx {
			w := s.spaces[i]
			row := ui.Row(c.Key(w.id())).Gap(spaceSM+2).Padding(0, spaceSM+2).Height(34).Radius(radiusBtn).AlignItems(ui.Center).FillWidth()
			switch {
			case k == d.sel:
				row.Background(p.accentSoft2)
			case row.Hovered():
				row.Background(p.hover)
			}
			row.Children(func() {
				dbTile(c, w.cfg.Type, 22, connColor(c, cfgColor(w.cfg)))
				ui.Text(c, w.cfg.Name).FontSize(fontCtl).FontWeight(500).SingleLine().Grow(1)
				dbName := w.dbLabel()
				if n := w.ordinal(); n > 1 {
					dbName = fmt.Sprintf("%s #%d", dbName, n)
				}
				ui.Text(c, dbName).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().MaxWidth(140).TruncateMiddle()
				if w.cfg.Tag != "" && w.cfg.Tag != "local" {
					sqChip(c, tagChipKind(w.cfg.Tag), tagLabel(w.cfg.Tag))
				}
				if i == s.active {
					ui.Icon(c, icCheck).FontSize(14).TextColor(p.accent)
				}
			})
			if row.Clicked() {
				d.sel = k
				choose()
			}
		}
		s.wsPopupActions(c, p)
	})
}

// popupHeading 是弹层列表里的小标题（「已打开」「xx 的数据库」）。
func popupHeading(c *ui.Context, p tokens, text string) {
	ui.Text(c, text).FontSize(fontCaption).FontWeight(600).TextColor(p.fgMuted).Padding(spaceXS, spaceSM, spaceXS, spaceSM)
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

// wsPopupActions 是「所有连接」列表底部的两个动作：打开别的连接、断开当前连接。
// 标题栏瘦身以后，这两个入口放在侧栏连接卡点开的列表里。
func (s *Shell) wsPopupActions(c *ui.Context, p tokens) {
	hairline(c, p.borderSubtle)
	act := func(key string, ic *ui.SVG, label string) bool {
		row := ui.Row(c.Key(key)).Gap(10).Padding(7, 10).Radius(radiusBtn).AlignItems(ui.Center).FillWidth()
		if row.Hovered() {
			row.Background(p.hover)
		}
		row.Children(func() {
			ui.Icon(c, ic).FontSize(btnFieldIcon).TextColor(p.fgSecondary)
			ui.Text(c, label).FontSize(fontCtl).SingleLine()
		})
		return row.Clicked()
	}
	if act("ws-new", icPlus, locales.ToolbarNewConnection()+s.keyHint("new-conn")) {
		s.wsPop.open = false
		s.newConnectionDialog()
	}
	if w := s.activeWorkspace(); w != nil && act("ws-close", icUnplug, locales.ToolbarDisconnect()) {
		s.wsPop.open = false
		s.closeSpaceGuarded(w)
	}
}
