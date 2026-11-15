package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// dbTypes 是连接表单里可选的数据库类型，顺序与名字来自 database 包的方言。
var dbTypes = func() []struct {
	id    model.DBType
	label string
} {
	var out []struct {
		id    model.DBType
		label string
	}
	for _, t := range model.AllDBTypes() {
		out = append(out, struct {
			id    model.DBType
			label string
		}{t, typeLabel(t)})
	}
	return out
}()

func typeLabel(id model.DBType) string { return app.DialectLabel(id) }

func defaultPort(typ model.DBType) int { return app.DefaultPort(typ) }

func connSubtitle(cfg model.ConnectionConfig) string {
	if !app.CapsOf(cfg.Type).Network {
		return "SQLite · " + cfg.Database
	}
	sub := fmt.Sprintf("%s · %s@%s:%d", typeLabel(cfg.Type), cfg.User, cfg.Host, cfg.Port)
	if cfg.Database != "" {
		sub += "/" + cfg.Database
	}
	return sub
}

// duplicateConnection saves a copy of cfg (password included) under a new name.
func (s *Shell) duplicateConnection(cfg model.ConnectionConfig) {
	name := locales.ConnDuplicateName(cfg.Name)
	var err error
	var id string
	s.up.bg(func() { id, err = s.app.Connections.DuplicateConnection(cfg.ID, name) }, func() {
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		s.selected = id // 光标落在新复制的那条上
		s.loadConnections()
	})
}

func tagLabel(tag string) string {
	switch tag {
	case "production":
		return locales.ConnTagProduction()
	case "test":
		return locales.ConnTagTest()
	case "development":
		return locales.ConnTagDevelopment()
	case "staging":
		return locales.ConnTagStaging()
	}
	return locales.ConnTagLocal()
}

func (s *Shell) deleteConnection(cfg model.ConnectionConfig) {
	s.askDanger(locales.ConnDeleteTitle(), locales.ConnDeleteMsg(cfg.Name), locales.CommonDelete(), func() {
		var err error
		s.up.bg(func() { err = s.app.Connections.DeleteConnection(cfg.ID) }, func() {
			if err != nil {
				s.toastErr(errMessage(err))
				return
			}
			for i := len(s.spaces) - 1; i >= 0; i-- {
				if s.spaces[i].cfg.ID == cfg.ID {
					s.closeSpace(i)
				}
			}
			s.loadConnections()
			if s.dlg.open && s.dlg.form { // 在配置页里删掉的，回到列表
				s.showList()
			}
		})
	})
}

// exportConnections 导出连接备份。口令为空时只能明文导出，先让用户明确确认。
func (s *Shell) exportConnections(passphrase string) {
	if passphrase == "" {
		s.askDanger(locales.ConnExportPlainTitle(), locales.ConnExportPlainMsg(), locales.ConnExportPlainOK(), func() {
			s.runConnectionsExport(s.app.Connections.ExportConnectionsPlaintext)
		})
		return
	}
	s.runConnectionsExport(func() (string, error) { return s.app.Connections.ExportConnections(passphrase) })
}

func (s *Shell) runConnectionsExport(export func() (string, error)) {
	var path string
	var err error
	s.up.bg(func() { path, err = export() }, func() {
		switch {
		case err != nil:
			s.toastErr(errMessage(err))
		case path != "":
			s.toastOK(locales.ConnExported(path))
		}
	})
}

func (s *Shell) importConnections(passphrase string) {
	var res struct{ imported, skipped int }
	var err error
	s.up.bg(func() {
		r, e := s.app.Connections.ImportConnections(passphrase)
		res.imported, res.skipped, err = r.Imported, r.Skipped, e
	}, func() {
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		if res.imported+res.skipped > 0 {
			s.toastOK(locales.ConnImported(res.imported, res.skipped))
		}
		s.loadConnections()
	})
}

// ---- connection dialog ----

// dbColors are the tile colors of the database types.
var dbColors = map[model.DBType]string{model.MySQL: "#00758f", model.MariaDB: "#003545", model.Postgres: "#336791", model.SQLServer: "#a91d22", model.ClickHouse: "#c89b00", model.SQLite: "#0f80cc", model.TiDB: "#e23d2f", model.StarRocks: "#6c4cff"}

// dbTile 画数据库头像（设计稿 .dbtile）：白底圆角块（深色主题是浅灰底，品牌色才看得清）里放该数据库的品牌图标，
// 连接色是块的细环；没给连接色（col 为零值）时用普通描边。没有品牌图标的类型（TiDB、StarRocks）用通用数据库字形加品牌色，
// 不认识的类型用中性灰，不能 panic（ui.Hex 遇到空串会 panic）。参数：size 是块的边长。
func dbTile(c *ui.Context, kind model.DBType, size float32, col ui.Color) ui.Element {
	p := baseTokens(c)
	bg, ring := p.surface, p.border // 浅色主题的 surface 是白；深色主题用前景色（近白）托底，品牌色才看得清
	if c.Theme().Dark {
		bg = p.fg.Alpha(0.92)
	}
	if col != (ui.Color{}) {
		ring = col.Alpha(0.55)
	}
	glyph, fg, multi := icDatabase, p.fgMuted, false
	if b, ok := dbBrand[kind]; ok {
		glyph, fg, multi = b.glyph, ui.Hex(b.color), b.multi
	} else if h, ok := dbColors[kind]; ok {
		fg = ui.Hex(h)
	}
	bw := float32(1.5)
	if size < 20 {
		bw = 1
	}
	t := ui.Column(c).Size(size, size).Shrink(0).Radius(size*6/22).Background(bg).Border(bw, ring).Center()
	return t.Children(func() {
		if multi { // 自带多种颜色的字形（StarRocks 官方 logo）：用 Image 按它自己的颜色画，不按品牌色着色
			ui.Image(c, glyph).Size(size*0.52, size*0.6)
			return
		}
		ui.Icon(c, glyph).FontSize(size * 0.62).TextColor(fg)
	})
}

// isOpen reports whether a connection has a workspace.
func (s *Shell) isOpen(id string) bool {
	for _, w := range s.spaces {
		if w.cfg.ID == id {
			return true
		}
	}
	return false
}

func (s *Shell) selectedConnection() *model.ConnectionConfig {
	for i := range s.conns {
		if s.conns[i].ID == s.selected {
			return &s.conns[i]
		}
	}
	return nil
}
