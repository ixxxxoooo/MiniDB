package ui

import (
	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 切换数据库浮层（设计稿 dpop）：点侧栏顶部的库名，从它下面弹出 312px 的浮层：
// 顶部搜索框「切换数据库… ⌘K」，每个库一行（图标、库名、「大小 · N 张表」，当前库加粗打勾），
// 底部「新建数据库…」。键盘：↑↓ 移动高亮，回车切库，Esc 或点外面收起。
// 状态沿用 s.sw（⌘K 的切库对话框也是它）：侧栏收起、没有锚点时才画成居中的对话框。

// dbSub 是库那一行的第二行文字：「248 MB · 24 张表」；没有大小就只写表数，什么都没有返回空串。
func dbSub(db model.DatabaseInfo) string {
	switch {
	case db.TableCount <= 0:
		return ""
	case db.Size > 0:
		return locales.DbSwitchSub(humanBytes(db.Size), db.TableCount)
	}
	return locales.DbSwitchTables(db.TableCount)
}

// dbPopChrome 是库那一行里文字以外占的宽度（浮层与行的内边距、库图标、打勾及它们之间的间距），估算浮层宽度用。
const dbPopChrome = 96

// dbPopWidth 是浮层的宽度：随最长的库名 / 副标题变宽，在 [sizeDbPop, sizeDbPopMax] 之间；
// 按全部库算而不是按搜索过滤后的，输入搜索词时浮层宽度不会跟着跳。参数：dbs 是连接下的全部库。返回值：宽度（逻辑像素）。
func dbPopWidth(dbs []model.DatabaseInfo) float32 {
	need := float32(0)
	for _, db := range dbs {
		need = max(need, estimateTextWidth(db.Name, 7.2), estimateTextWidth(dbSub(db), 6.2))
	}
	return min(max(need+dbPopChrome, sizeDbPop), sizeDbPopMax)
}

// dbPopover 画浮层。必须在库名画出来的同一帧里、库名之后调用（锚点 s.sw.anchor）。
func (s *Shell) dbPopover(c *ui.Context) {
	d := &s.sw
	p := s.tokens(c)
	shown := d.shown()
	d.sel = min(max(d.sel, 0), max(len(shown)-1, 0))
	choose := func(i int) {
		if i >= 0 && i < len(shown) {
			d.open = false
			d.w.switchDatabase(shown[i].Name)
		}
	}
	move := func(n int) { d.sel = min(max(d.sel+n, 0), max(len(shown)-1, 0)); s.redraw() }
	anchor := c.Resolve(d.anchor)
	ui.PopoverBase(c, anchor, &d.open, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomLeft, ui.AnchorTopLeft).Margin(spaceXS, 0, 0, 0).Width(dbPopWidth(d.w.dbs)).MaxHeight(520).
			Background(p.surfaceElev).Border(1, p.border).Radius(radiusDialog).Shadow(0, shadowY, shadowBlur, 0, p.shadow).Clip()
		enterPose(c, panel, posePop, ui.Snappy).Transition(popExit)
		ui.Column(c).FillWidth().Padding(spaceSM - 2).Gap(1).Children(func() {
			s.dbPopSearch(c, p, d, move, func() { choose(d.sel) })
			if len(shown) == 0 {
				ui.Text(c, locales.PaletteNone()).TextColor(p.fgMuted).Padding(spaceLG).TextAlign(ui.Center).FillWidth()
			}
			scrollbars(c, ui.Scroll(c.Key("dbpop-list")).FillWidth().MaxHeight(sizeDbRow*8)).Children(func() {
				ui.Column(c).Gap(1).FillWidth().Children(func() {
					for i, db := range shown {
						i, db := i, db
						s.dbPopRow(c, p, db, i == d.sel, func() { d.sel = i; choose(i) })
					}
				})
			})
			if app.CapsOf(d.w.cfg.Type).CreateDatabase {
				hairline(c, p.borderSubtle).Margin(spaceXS, spaceSM)
				row := connPopRow(c, p, "dbpop-new", sizeRow+2, false)
				row.Children(func() {
					ui.Icon(c, icDatabasePlus).FontSize(btnFieldIcon).TextColor(p.fgSecondary)
					ui.Text(c, locales.DbNew()).FontSize(fontCtl).SingleLine().Grow(1)
				})
				if row.Clicked() && !d.w.cfg.ReadOnly {
					d.open = false
					s.newDatabase(d.w)
				}
			}
		})
	})
}

// dbPopSearch 是浮层顶部的搜索框；↑↓ 移动高亮，回车切库。
func (s *Shell) dbPopSearch(c *ui.Context, p tokens, d *dbSwitcher, move func(int), submit func()) {
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Padding(0, spaceSM+2).Height(sizeInput).FillWidth().Radius(radiusBtn).Border(1, p.border).Background(p.surface).Margin(2, 2, spaceSM-2, 2).Children(func() {
		ui.Icon(c, icSearch).FontSize(btnActIcon).TextColor(p.fgMuted)
		in := ui.TextInput(c.Key("dbpop-q"), &d.query).Placeholder(locales.DbSwitchSearch()).FontSize(fontCtl).Grow(1).AutoFocus().Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
			OnTextKey(func(k ui.TextKey) bool {
				if k.Mods != 0 {
					return false
				}
				switch k.Key {
				case ui.KeyDown:
					move(1)
				case ui.KeyUp:
					move(-1)
				default:
					return false
				}
				return true
			})
		if s.cmdKeys("switcher") != "" {
			kbd(c, s.cmdKeys("switcher"))
		}
		if in.Changed() {
			d.sel = 0
		}
		if in.Submitted() {
			submit()
		}
	})
}

// dbPopRow 画一个库：图标、库名 + 「大小 · N 张表」两行，当前库加粗、右端打勾；有建库能力的连接可右键改名、删除。
func (s *Shell) dbPopRow(c *ui.Context, p tokens, db model.DatabaseInfo, on bool, pick func()) {
	w := s.sw.w
	isCur := db.Name == w.db || !app.CapsOf(w.cfg.Type).MultiDatabase // 文件型库只有这一个库，就是当前库
	row := connPopRow(c, p, "dbp-"+db.Name, sizeDbRow, on)
	row.Children(func() {
		ui.Icon(c, icDatabase).FontSize(btnFieldIcon + 2).TextColor(p.fgSecondary).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			name := ui.Text(c, db.Name).FontSize(fontCtl).SingleLine()
			if isCur {
				name.FontWeight(600)
			}
			if sub := dbSub(db); sub != "" {
				ui.Text(c, sub).FontSize(fontCaption - 0.5).TextColor(p.fgMuted).SingleLine()
			}
		})
		if isCur {
			ui.Icon(c, icCheck).FontSize(btnFieldIcon).TextColor(p.accent).Shrink(0)
		}
	})
	if row.Clicked() {
		pick()
	}
	if app.CapsOf(w.cfg.Type).CreateDatabase && !w.cfg.ReadOnly {
		row.ContextMenu(func(m *ui.Menu) {
			if m.Item(locales.DbRename()).Chosen() {
				defer func() { s.sw.open = false; s.renameDatabase(w, db.Name) }()
			}
			if m.Item(locales.DbDrop()).Chosen() {
				defer func() { s.sw.open = false; s.dropDatabase(w, db.Name) }()
			}
		})
	}
}
