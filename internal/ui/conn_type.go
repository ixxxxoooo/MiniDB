package ui

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 配置页里的数据库类型：一个带彩色图标的下拉，不占一整排的位置；
// 以及与类型相关的默认值（端口、用户）和从 URL 回填表单。

// typeSelect 画数据库类型下拉：触发器里是当前类型的图标和名字，
// 弹出的列表里每个类型一行（图标 + 名字，当前的带勾）。切换后默认端口、默认用户跟着变。
// 参数 c 为界面上下文，p 为主题色板；无返回值。
func (s *Shell) typeSelect(c *ui.Context, p tokens) {
	d := &s.dlg
	cur := d.cfg.Type
	sel := ui.SelectBase(c.Key("dbtype"), &cur)
	sel.Trigger.MinWidth(170).Height(sizeBtn).Padding(0, 10, 0, 8).Gap(9).AlignItems(ui.Center).Radius(radiusBtn).Background(p.surface).Border(1, p.border).Children(func() {
		dbTile(c, cur, 20, ui.Color{})
		ui.Text(c, typeLabel(cur)).FontSize(font2XS + 1).Grow(1).SingleLine()
		ui.Icon(c, icChevronD).FontSize(12).TextColor(p.fgMuted)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Padding(4).Gap(1).Background(p.surfaceElev).Border(1, p.border).Radius(10).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18))
		for _, ty := range dbTypes {
			item := sel.Item(ty.id).Padding(6, 8).Gap(10).AlignItems(ui.Center).Radius(radiusBtn)
			switch {
			case item.Highlighted():
				item.Background(p.hover)
			case ty.id == cur:
				item.Background(p.accent.Alpha(0.08))
			}
			item.Children(func() {
				dbTile(c, ty.id, 22, ui.Color{})
				ui.Text(c, ty.label).FontSize(font2XS + 1).Grow(1).SingleLine()
				if ty.id == cur {
					ui.Icon(c, icCheck).FontSize(14).TextColor(p.accent)
				}
			})
		}
	})
	if sel.Trigger.Changed() {
		d.setType(cur)
	}
}

// setType 切换数据库类型：端口、用户名仍是上一种类型的默认值时一起换成新类型的默认值。
// 参数 id 为新类型的标识；无返回值。
func (d *connDialog) setType(id model.DBType) {
	old := d.cfg.Type
	d.cfg.Type, d.typeSel = id, typeLabel(id)
	switch {
	case !app.CapsOf(id).Network:
		d.port = ""
	case d.port == "" || d.port == "0" || d.port == fmt.Sprint(defaultPort(old)):
		d.port = fmt.Sprint(defaultPort(id))
	}
	if d.cfg.User == defaultUser(old) {
		d.cfg.User = defaultUser(id)
	}
}

// defaultUser 是各类型的默认用户名（sqlite 没有）。
func defaultUser(typ model.DBType) string {
	if d, err := app.DialectOf(typ); err == nil {
		return d.DefaultUser()
	}
	return ""
}

// applyURL fills the form from a connection URL such as
// mysql://user:pass@host:3306/db; it reports whether the URL was understood.
func (d *connDialog) applyURL(raw string) bool {
	typ, host, port, user, pass, db, ok := parseConnURL(raw)
	if !ok {
		return false
	}
	d.cfg.Type, d.typeSel = typ, typeLabel(typ)
	if !app.CapsOf(typ).Network {
		d.cfg.Database = db
		return true
	}
	d.cfg.Host = host
	if port == 0 {
		port = defaultPort(typ)
	}
	d.port = strconv.Itoa(port)
	d.cfg.User, d.password, d.cfg.Database = user, pass, db
	if d.cfg.Name == "" {
		d.cfg.Name = host
	}
	return true
}

// parseConnURL 把连接地址拆成各个字段；不认识的地址 ok 为 false。
func parseConnURL(raw string) (typ model.DBType, host string, port int, user, pass, db string, ok bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return
	}
	switch strings.ToLower(u.Scheme) {
	case "mysql", "mariadb":
		typ = model.MySQL
	case "postgres", "postgresql", "pg":
		typ = model.Postgres
	case "tidb":
		typ = model.TiDB
	case "starrocks":
		typ = model.StarRocks
	case "sqlite", "sqlite3", "file":
		typ, db, ok = model.SQLite, u.Host+u.Path, true
		return
	default:
		return
	}
	host = u.Hostname()
	if host == "" {
		return
	}
	if ps := u.Port(); ps != "" {
		port, _ = strconv.Atoi(ps)
	}
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	db = strings.TrimPrefix(u.Path, "/")
	return typ, host, port, user, pass, db, true
}
