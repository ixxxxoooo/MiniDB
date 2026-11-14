package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 欢迎页（设计稿 07）：没有任何打开的连接时占满内容区。
// 左边是 220px 的分类栏（全部连接 + 文件夹），右边是标题行（搜索、卡片 / 列表、导入、新建）、
// 按文件夹分组的连接卡片（或列表行）和一条 26px 的提示状态栏。
// 单击选中，双击或 ⏎ 连接，右键更多操作，拖到文件夹上移入。
// 「最近使用」「已固定」要有连接的使用记录才做得出来，目前没有这份数据，所以不画。

// welcomePage 画整页。
func (s *Shell) welcomePage(c *ui.Context, p tokens) {
	perfBuild("ui.build.welcome", func() {
		ui.Row(c.Key("welcome")).Fill().MinWidth(0).Children(func() {
			s.welcomeSide(c, p)
			ui.Box(c).Width(1).FillHeight().Background(p.border)
			ui.Column(c.Key("wel-main")).Grow(1).FillHeight().MinWidth(0).Background(p.surface).Children(func() {
				shown := s.welcomeConns()
				s.welcomeHead(c, p, shown)
				ui.Column(c.Key("wel-body")).Grow(1).FillWidth().MinHeight(0).Children(func() { s.welcomeBody(c, p) })
				s.welcomeStatus(c, p)
			})
		})
	})
}

// welcomeGroups 是页面上要画的分组：文件夹筛选生效时只留那一个文件夹，空组不画。
func (s *Shell) welcomeGroups() []connGroup {
	var out []connGroup
	for _, g := range s.connGroups() {
		if len(g.conns) == 0 {
			continue
		}
		if f := s.layout.connFolder; f != "" && (g.folder == nil || g.folder.ID != f) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// welcomeConns 是页面上看得到的连接，按显示顺序；光标和回车都在它上面走。
func (s *Shell) welcomeConns() []model.ConnectionConfig {
	var out []model.ConnectionConfig
	for _, g := range s.welcomeGroups() {
		out = append(out, g.conns...)
	}
	return out
}

// welcomeSide 画左边的分类栏：全部连接、文件夹列表和底部的提示卡。
func (s *Shell) welcomeSide(c *ui.Context, p tokens) {
	ui.Column(c.Key("wel-side")).Width(220).FillHeight().Shrink(0).Background(p.surfaceSecondary).Children(func() {
		ui.Column(c).FillWidth().Padding(spaceMD, spaceSM+2, spaceXS, spaceSM+2).Gap(2).Shrink(0).Children(func() {
			s.welcomeEntry(c, p, "", locales.WelcomeAll(), len(s.conns), nil)
		})
		ui.Row(c).FillWidth().Height(sizeRow).Padding(0, spaceSM+2, 0, spaceMD).Margin(spaceSM, 0, 0, 0).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Text(c, locales.WelcomeFolders()).FontSize(fontCaption).FontWeight(600).TextColor(p.fgMuted).Grow(1)
			if tipBtn(c.Key("wel-newfolder"), icPlus, locales.ConnFolderNew()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				s.newFolder()
			}
		})
		scrollbars(c, ui.Scroll(c.Key("wel-folders")).FillWidth().Grow(1).MinHeight(0)).Children(func() {
			ui.Column(c).FillWidth().Padding(0, spaceSM+2).Gap(2).Children(func() {
				for i := range s.folders {
					f := s.folders[i]
					n := 0
					for _, cfg := range s.conns {
						if cfg.FolderID == f.ID {
							n++
						}
					}
					s.welcomeEntry(c, p, f.ID, f.Name, n, &f)
				}
			})
		})
		ui.Column(c).FillWidth().Padding(spaceSM + 2).Shrink(0).Children(func() {
			ui.Column(c).FillWidth().Gap(spaceXS).Padding(spaceSM+2, spaceMD).Radius(radiusPanel).Background(p.surfaceTertiary).Children(func() {
				ui.Text(c, locales.WelcomeTipTitle()).FontSize(fontDense).FontWeight(600)
				ui.Text(c, locales.WelcomeTipBody()).FontSize(fontCaption).TextColor(p.fgSecondary).LineHeight(1.5)
			})
		})
	})
}

// welcomeEntry 是分类栏里的一项：点它切换筛选；文件夹项还能接住拖来的连接，右键重命名 / 删除。
func (s *Shell) welcomeEntry(c *ui.Context, p tokens, id, name string, n int, f *model.ConnectionFolder) {
	on := s.layout.connFolder == id
	row := ui.Row(c.Key("wel-ent-"+id)).FillWidth().Height(sizeRow).Padding(0, spaceSM).Gap(spaceSM - 2).Radius(radiusBtn).AlignItems(ui.Center).Shrink(0)
	_, over := ui.DragOver[connDrag](row)
	switch {
	case over:
		row.Background(p.accent.Alpha(0.16))
	case on:
		row.Background(p.accentSoft2)
	case row.Hovered():
		row.Background(p.hover)
	}
	if d, ok := ui.Drop[connDrag](row); ok && f != nil {
		defer s.placeConn(d.id, f.ID, "")
	}
	row.Children(func() {
		fg, w := p.fgSecondary, 500
		if on {
			fg, w = p.accentText, 600
		}
		if f != nil {
			ui.Icon(c, icFolder).FontSize(btnActIcon).TextColor(fg).Shrink(0)
		}
		ui.Text(c, name).FontSize(fontCtl).FontWeight(w).TextColor(fg).SingleLine().Grow(1).MinWidth(0)
		ui.Text(c, itoa(n)).FontSize(fontCaption).TextColor(p.fgMuted)
	})
	if row.Clicked() {
		s.layout.connFolder = id
	}
	if f != nil {
		fd := *f
		row.ContextMenu(func(m *ui.Menu) {
			if m.Item(locales.ConnFolderRename()).Chosen() {
				defer s.renameFolder(fd)
			}
			m.Separator()
			if m.Item(locales.ConnFolderDelete()).Chosen() {
				defer s.deleteFolder(fd)
			}
		})
	}
}

// welcomeHead 画标题行：「连接」大标题、搜索框、卡片 / 列表切换、导入、新建。
func (s *Shell) welcomeHead(c *ui.Context, p tokens, shown []model.ConnectionConfig) {
	ui.Row(c.Key("wel-head")).FillWidth().Gap(spaceSM).Padding(14, spaceLG+spaceXS, spaceSM+2, spaceLG+spaceXS).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, locales.WelcomeTitle()).FontSize(fontHeading).FontWeight(700).Grow(1)
		s.welcomeSearch(c, p, shown)
		sel := 0
		if s.layout.ConnList {
			sel = 1
		}
		if segmented(c, &sel, locales.WelcomeCards(), locales.WelcomeList()) {
			s.layout.ConnList = sel == 1
		}
		if fieldBtn(c, icUpload, locales.WelcomeImport()).Clicked() {
			s.importConnections("")
		}
		if fieldPrimary(c, icPlus, locales.ConnNew()).Clicked() {
			s.newConnection()
		}
	})
}

// welcomeSearch 是标题行里的搜索框：↑↓←→ 移动光标（输入框有焦点时也一样），⏎ 连接光标所在的那条，⌘F 聚焦。
func (s *Shell) welcomeSearch(c *ui.Context, p tokens, shown []model.ConnectionConfig) {
	cur := s.listCursor(shown)
	move := func(delta int) {
		if len(shown) > 0 {
			s.selected = shown[min(max(cur+delta, 0), len(shown)-1)].ID
		}
	}
	step := 3
	if s.layout.ConnList {
		step = 1
	}
	var in ui.Element
	ui.Row(c.Key("wel-search")).Width(240).Height(sizeInput).Padding(0, spaceSM).Gap(spaceSM-2).AlignItems(ui.Center).Radius(radiusBtn).Background(p.surface).Border(1, p.border).Shrink(0).Children(func() {
		ui.Icon(c, icSearch).FontSize(btnActIcon).TextColor(p.fgMuted)
		in = ui.TextInput(c.Key("wel-search-in"), &s.dlg.search).Placeholder(locales.ConnSearchList()).FontSize(fontCtl).Grow(1).MinWidth(0).Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
			OnTextKey(func(k ui.TextKey) bool {
				if k.Mods != 0 {
					return false
				}
				switch k.Key {
				case ui.KeyDown:
					move(step)
				case ui.KeyUp:
					move(-step)
				case ui.KeyRight:
					move(1)
				case ui.KeyLeft:
					move(-1)
				default:
					return false
				}
				s.redraw()
				return true
			})
		kbd(c, "⌘F")
	})
	if c.Shortcut(ui.Cmd, ui.KeyF) {
		in.Focus()
	} else if !s.dlg.open {
		in.AutoFocus()
	}
	if in.Submitted() && cur < len(shown) {
		s.openConnection(shown[cur])
	}
}

// welcomeStatus 是底部 26px 的提示条：左边是操作提示，右边是连接数和密码存放位置。
func (s *Shell) welcomeStatus(c *ui.Context, p tokens) {
	hairline(c, p.border)
	ui.Row(c.Key("wel-status")).FillWidth().Height(sizeStatus).Padding(0, spaceLG+spaceXS).AlignItems(ui.Center).Shrink(0).Background(p.surfaceSecondary).Children(func() {
		ui.Text(c, locales.WelcomeHint()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		ui.Spacer(c)
		ui.Text(c, locales.WelcomeCount(len(s.conns))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
	})
}
