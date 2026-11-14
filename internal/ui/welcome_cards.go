package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 欢迎页的主体：按文件夹分组的连接卡片（每行 3 张）或列表行。

// welcomeCols 是卡片模式每行放几张。
const welcomeCols = 3

// welcomeBody 画可滚动的分组区；没有连接时是空状态。
func (s *Shell) welcomeBody(c *ui.Context, p tokens) {
	groups := s.welcomeGroups()
	if len(groups) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Gap(spaceSM).Children(func() {
			switch {
			case len(s.conns) == 0:
				ui.Icon(c, icDatabase).FontSize(30).TextColor(p.fgMuted)
				ui.Text(c, locales.ConnEmpty()).FontSize(fontBody).FontWeight(600)
				ui.Text(c, locales.ConnEmptyHint()).FontSize(fontDense).TextColor(p.fgMuted)
			case s.layout.connFolder != "":
				ui.Text(c, locales.WelcomeFolderEmpty()).FontSize(fontDense).TextColor(p.fgMuted)
			default:
				ui.Text(c, locales.ConnNoMatch()).FontSize(fontDense).TextColor(p.fgMuted)
			}
		})
		return
	}
	cur := s.listCursor(s.welcomeConns())
	idx := 0
	hasFolders := len(s.folders) > 0
	scrollbars(c, ui.Scroll(c.Key("wel-scroll")).FillWidth().Grow(1).MinHeight(0)).Children(func() {
		ui.Column(c).FillWidth().Padding(0, spaceLG+spaceXS, spaceMD, spaceLG+spaceXS).Gap(spaceSM + 2).Children(func() {
			for _, g := range groups {
				if hasFolders {
					s.welcomeGroupHead(c, p, g)
				}
				if s.layout.ConnList {
					for _, cfg := range g.conns {
						s.welcomeRow(c, p, cfg, idx == cur)
						idx++
					}
					continue
				}
				for i := 0; i < len(g.conns); i += welcomeCols {
					ui.Row(c.Key(itoa(idx))).FillWidth().Gap(spaceSM + 2).Children(func() {
						for j := 0; j < welcomeCols; j++ {
							if i+j >= len(g.conns) {
								ui.Box(c).Grow(1).Basis(0).MinWidth(0)
								continue
							}
							s.welcomeCard(c, p, g.conns[i+j], idx == cur)
							idx++
						}
					})
				}
			}
		})
	})
}

// welcomeGroupHead 是一组的标题：名字加连接数；「未分组」的标题也是连接的落点（拖进来就移出文件夹）。
func (s *Shell) welcomeGroupHead(c *ui.Context, p tokens, g connGroup) {
	id, name := "", locales.ConnUngrouped()
	if g.folder != nil {
		id, name = g.folder.ID, g.folder.Name
	}
	row := ui.Row(c.Key("wel-gh-"+id)).FillWidth().Height(30).Gap(6).AlignItems(ui.Center).Shrink(0).Margin(spaceXS, 0, 0, 0)
	if _, over := ui.DragOver[connDrag](row); over {
		row.Background(p.accent.Alpha(0.16)).Radius(radiusBtn)
	}
	if d, ok := ui.Drop[connDrag](row); ok {
		defer s.placeConn(d.id, id, "")
	}
	row.Children(func() {
		ui.Text(c, name).FontSize(fontDense).FontWeight(600)
		ui.Text(c, itoa(len(g.conns))).FontSize(fontCaption).TextColor(p.fgMuted)
	})
}

// welcomeStatusLine 是卡片 / 行上的状态：已打开的绿点，其余显示只读或安全级别。
func (s *Shell) welcomeStatusLine(c *ui.Context, p tokens, cfg model.ConnectionConfig) {
	dot, text := p.fgMuted, safeModeLabel(cfg.EffectiveSafeMode())
	switch {
	case s.connecting[cfg.ID]:
		dot, text = p.warning, locales.WelcomeConnecting()
	case s.isOpen(cfg.ID):
		dot, text = p.success, locales.ConnOpened()
	case cfg.ReadOnly:
		text = locales.ConnReadOnlyShort()
	}
	ui.Box(c).Size(sizeDot, sizeDot).Radius(sizeDot / 2).Background(dot).Shrink(0)
	ui.Text(c, text).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
}

// welcomeShell 给卡片 / 行套上共同的交互：拖动、接住拖来的连接、单击选中、双击连接、右键菜单。
func (s *Shell) welcomeShell(el ui.Element, c *ui.Context, p tokens, cfg model.ConnectionConfig, cursor bool) ui.Element {
	el.Drag(connDrag{cfg.ID})
	if d, ok := ui.Drop[connDrag](el); ok {
		defer s.placeConn(d.id, cfg.FolderID, cfg.ID)
	}
	if _, over := ui.DragOver[connDrag](el); over {
		el.Border(1, p.accent)
	}
	if el.DoubleClicked() {
		defer s.openConnection(cfg)
	} else if el.Clicked() {
		s.selected = cfg.ID
	}
	el.ContextMenu(func(m *ui.Menu) { s.connMenu(c, m, cfg) })
	return el
}

// welcomeCard 画一张连接卡：头像、名称、类型、环境标记、地址、状态。
func (s *Shell) welcomeCard(c *ui.Context, p tokens, cfg model.ConnectionConfig, cursor bool) {
	card := ui.Column(c.Key(cfg.ID)).Grow(1).Basis(0).MinWidth(0).Gap(spaceSM-1).Padding(spaceSM+2, spaceMD).Radius(radiusPanel).Background(p.surface).Border(1, p.border)
	switch {
	case cursor:
		card.Border(1, p.accent).Background(p.accentSoft2.Alpha(0.5))
	case card.Hovered():
		card.Background(p.hover)
	}
	s.welcomeShell(card, c, p, cfg, cursor).Children(func() {
		ui.Row(c).FillWidth().Gap(spaceSM).AlignItems(ui.Center).Children(func() {
			dbTile(c, cfg.Type, 32, connColor(c, cfgColor(cfg)))
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, cfg.Name).FontSize(fontBody).FontWeight(600).SingleLine()
				ui.Text(c, typeLabel(cfg.Type)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
			})
			sqChip(c, tagChipKind(cfg.Tag), tagLabel(cfg.Tag))
		})
		ui.Text(c, connAddr(cfg)).Font(mono).FontSize(fontCaption + 0.5).TextColor(p.fgSecondary).SingleLine().TruncateMiddle()
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() { s.welcomeStatusLine(c, p, cfg) })
	})
}

// welcomeRow 画列表模式的一行：头像、名称、类型、地址、环境标记、状态。
func (s *Shell) welcomeRow(c *ui.Context, p tokens, cfg model.ConnectionConfig, cursor bool) {
	row := ui.Row(c.Key(cfg.ID)).FillWidth().Height(sizeRow+10).Gap(spaceMD).Padding(0, spaceMD).Radius(radiusBtn).AlignItems(ui.Center).Shrink(0)
	switch {
	case cursor:
		row.Background(p.accentSoft2)
	case row.Hovered():
		row.Background(p.hover)
	}
	s.welcomeShell(row, c, p, cfg, cursor).Children(func() {
		dbTile(c, cfg.Type, 24, connColor(c, cfgColor(cfg)))
		ui.Text(c, cfg.Name).FontSize(fontCtl).FontWeight(600).SingleLine().Width(180).Shrink(0)
		ui.Text(c, typeLabel(cfg.Type)).FontSize(fontDense).TextColor(p.fgMuted).SingleLine().Width(90).Shrink(0)
		ui.Text(c, connAddr(cfg)).Font(mono).FontSize(fontDense).TextColor(p.fgSecondary).SingleLine().TruncateMiddle().Grow(1).MinWidth(0)
		sqChip(c, tagChipKind(cfg.Tag), tagLabel(cfg.Tag))
		ui.Row(c).Gap(6).AlignItems(ui.Center).Width(90).Shrink(0).Children(func() { s.welcomeStatusLine(c, p, cfg) })
	})
}
