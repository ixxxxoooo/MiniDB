package ui

import (
	"strings"
	"time"

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
	// 侧栏顶部库名的锚点：库名画出来的那一帧记下，侧栏收起（没有锚点）时回退成居中弹窗
	anchor    ui.Handle
	anchored  bool
	lastShown time.Time // 浮层最近一次画出来的时间（toggleSwitcher 用它认出「刚被这次按下收起」）
}

// popReopenGap 是浮层被点外面收起之后，同一次点击不再把它重新打开的时间窗。
const popReopenGap = 500 * time.Millisecond

// toggleSwitcher 是点入口的行为：开着就收起，没开就打开。参数：outside 为真表示入口不是浮层的锚点
// （查询页工具栏的库按钮）——按下那一刻浮层已被「点外面」收起，随后的点击不能再把它打开，否则永远收不掉；
// 锚点（侧栏库名）按下时浮层不会被收起，点击时它还开着，直接收起即可。
func (s *Shell) toggleSwitcher(outside bool) {
	if s.sw.open {
		s.sw.open = false
		return
	}
	if outside && time.Since(s.sw.lastShown) < popReopenGap {
		return
	}
	s.openSwitcher()
}

func (s *Shell) openSwitcher() {
	w := s.activeWorkspace()
	if w == nil {
		return
	}
	// anchored 要原样带过来：库名按钮在同一帧里先记了锚点再触发打开，这里一清零，
	// 这一帧会误画成居中的对话框，下一帧才换成贴着库名的浮层，出场动画就和连接浮层不一样了。
	s.sw = dbSwitcher{open: true, w: w, anchor: s.sw.anchor, anchored: s.sw.anchored}
	s.refreshDBs(w)
}

// refreshDBs 在后台重新取 w 的库列表，取到了就替换 w.dbs（取不到则保留旧的）。
func (s *Shell) refreshDBs(w *Workspace) {
	var dbs []model.DatabaseInfo
	s.up.bg(func() { dbs, _ = s.app.Databases.GetAllDatabases(w.cfg.ID) }, func() {
		if len(dbs) > 0 {
			w.dbs = dbs
		}
	})
}

// shown 是按搜索词过滤后的库列表。
func (d *dbSwitcher) shown() []model.DatabaseInfo {
	q := strings.ToLower(strings.TrimSpace(d.query))
	var out []model.DatabaseInfo
	for _, db := range d.w.dbs {
		if q == "" || strings.Contains(strings.ToLower(db.Name), q) {
			out = append(out, db)
		}
	}
	return out
}

func (s *Shell) switcherView(c *ui.Context) {
	d := &s.sw
	if !d.open || d.w == nil {
		return
	}
	if d.anchored { // 侧栏上的库名在画面上：浮层贴着它弹出（设计稿 dpop）
		d.anchored = false
		s.dbPopover(c)
		return
	}
	p := s.tokens(c)
	shown := d.shown()
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
		popupHeading(c, p, locales.DbSwitchOf(d.w.cfg.Name))
		scrollbars(c, ui.Scroll(c.Key("dbs")).MaxHeight(320).FillWidth()).Children(func() {
			ui.Column(c).Gap(1).FillWidth().Children(func() {
				for i, db := range shown {
					i, db := i, db
					row := ui.Row(c.Key(db.Name)).Gap(spaceSM+2).Padding(0, spaceSM+2).Height(30).Radius(radiusBtn).AlignItems(ui.Center).FillWidth()
					switch {
					case i == d.sel:
						row.Background(p.accentSoft2)
					case row.Hovered():
						row.Background(p.hover)
					}
					row.Children(func() {
						ui.Icon(c, icDatabase).FontSize(14).TextColor(p.fgSecondary)
						name := ui.Text(c, db.Name).FontSize(fontCtl).SingleLine().Grow(1)
						if db.Name == d.w.db {
							name.FontWeight(600)
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
