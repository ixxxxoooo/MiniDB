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

// typeGrid 画数据库类型网格（设计稿 08）：每格是彩色图标加名字，一行 8 个，当前类型描强调色边、铺淡底。
// 点一格切换类型，默认端口、默认用户跟着变。参数 c 为界面上下文，p 为主题色板。
func (s *Shell) typeGrid(c *ui.Context, p tokens) {
	d := &s.dlg
	const cols = 8
	ui.Column(c.Key("type-grid")).FillWidth().Gap(spaceSM-2).Padding(spaceXS, spaceLG, spaceMD, spaceLG).Shrink(0).Children(func() {
		for i := 0; i < len(dbTypes); i += cols {
			ui.Row(c.Key(itoa(i))).FillWidth().Gap(spaceSM - 2).Children(func() {
				for j := 0; j < cols; j++ {
					if i+j >= len(dbTypes) {
						ui.Box(c).Grow(1).Basis(0).MinWidth(0)
						continue
					}
					ty := dbTypes[i+j]
					cell := ui.Column(c.Key("ty-"+string(ty.id))).Grow(1).Basis(0).MinWidth(0).Gap(spaceXS).Padding(spaceSM, 0, spaceSM-2, 0).AlignItems(ui.Center).Radius(radiusPanel-2).Border(1, p.border)
					switch {
					case ty.id == d.cfg.Type:
						cell.Border(1, p.accent).Background(p.accentSoft2)
					case cell.Hovered():
						cell.Background(p.hover)
					}
					cell.Children(func() {
						dbTile(c, ty.id, 26, ui.Color{})
						fg, w := p.fgSecondary, 500
						if ty.id == d.cfg.Type {
							fg, w = p.accentText, 600
						}
						ui.Text(c, ty.label).FontSize(fontCaption).FontWeight(w).TextColor(fg).SingleLine()
					})
					if cell.Clicked() {
						d.setType(ty.id)
					}
				}
			})
		}
	})
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
func defaultUser(typ model.DBType) string { return app.DefaultUser(typ) }

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
	typ, found := app.TypeFromURLScheme(u.Scheme)
	if !found {
		return
	}
	if !app.CapsOf(typ).Network {
		db, ok = u.Host+u.Path, true
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
	if db == "" {
		db = u.Query().Get("database") // sqlserver://…?database=x
	}
	return typ, host, port, user, pass, db, true
}
