package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/locales"
)

var dbTypes = []struct{ id, label string }{
	{"mysql", "MySQL"}, {"postgres", "PostgreSQL"}, {"sqlite", "SQLite"}, {"tidb", "TiDB"}, {"starrocks", "StarRocks"},
}

func typeLabel(id string) string {
	for _, t := range dbTypes {
		if t.id == id {
			return t.label
		}
	}
	return id
}

func defaultPort(typ string) int {
	switch typ {
	case "postgres":
		return 5432
	case "tidb":
		return 4000
	case "starrocks":
		return 9030
	case "sqlite":
		return 0
	}
	return 3306
}

func connSubtitle(cfg database.ConnectionConfig) string {
	if cfg.Type == "sqlite" {
		return "SQLite · " + cfg.Database
	}
	sub := fmt.Sprintf("%s · %s@%s:%d", typeLabel(cfg.Type), cfg.User, cfg.Host, cfg.Port)
	if cfg.Database != "" {
		sub += "/" + cfg.Database
	}
	return sub
}

// duplicateConnection saves a copy of cfg (password included) under a new name.
func (s *Shell) duplicateConnection(cfg database.ConnectionConfig) {
	name := locales.ConnDuplicateName(cfg.Name)
	var err error
	var id string
	s.up.bg(func() { id, err = s.app.Connections.DuplicateConnection(cfg.ID, name) }, func() {
		if err != nil {
			s.toastErr(err.Error())
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

func (s *Shell) deleteConnection(cfg database.ConnectionConfig) {
	s.askDanger(locales.ConnDeleteTitle(), locales.ConnDeleteMsg(cfg.Name), locales.CommonDelete(), func() {
		var err error
		s.up.bg(func() { err = s.app.Connections.DeleteConnection(cfg.ID) }, func() {
			if err != nil {
				s.toastErr(err.Error())
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

func (s *Shell) exportConnections(passphrase string) {
	var path string
	var err error
	s.up.bg(func() { path, err = s.app.Connections.ExportConnections(passphrase) }, func() {
		switch {
		case err != nil:
			s.toastErr(err.Error())
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
			s.toastErr(err.Error())
			return
		}
		if res.imported+res.skipped > 0 {
			s.toastOK(locales.ConnImported(res.imported, res.skipped))
		}
		s.loadConnections()
	})
}

// ---- connection dialog ----

type connDialog struct {
	open     bool
	editing  bool
	cfg      database.ConnectionConfig
	typeSel  string
	tagSel   string
	port     string
	sshPort  string
	sshAuth  string
	password string
	secret   string
	sshOpen  bool
	testing  bool
	msg      string
	msgOK    bool
	keyBtn   bool
	form     bool   // 正在配置页（否则是列表页）
	base     string // 配置页载入时表单内容的指纹，用来判断有没有未保存的修改
	draft    bool   // 配置页里是一条尚未保存的新连接
	ready    bool   // 管理器已完成了初始页面的选择
	focusSet bool   // 搜索框已拿过一次键盘焦点
	search   string
	urlOpen  bool
	urlText  string
	safeSel  string // Safe Mode level, by its label
	safeSet  bool   // the user chose one, so a new environment no longer picks it
	pwMode   int    // 0 keychain, 1 ask every time, 2 none
}

var colorChoices = []string{"#34c759", "#0a84ff", "#af52de", "#ff9f0a", "#ff453a", "#8e8e93"}

// newConnectionDialog 打开连接管理器；先停在列表页还是直接进配置页，由 connSync 在连接列表就绪后决定。
func (s *Shell) newConnectionDialog() {
	s.dlg = connDialog{open: true}
}

// newConnection 打开管理器并直接进入"新建连接"表单。
func (s *Shell) newConnection() {
	s.dlg = connDialog{open: true, ready: true, form: true, draft: true, typeSel: "MySQL", tagSel: tagLabel("local"), sshAuth: "password", safeSel: safeModeLabel(s.prefs.defaultSafeMode()), safeSet: s.prefs.DefaultSafe != ""}
	s.dlg.cfg = database.ConnectionConfig{Type: "mysql", Host: "127.0.0.1", Port: 3306, User: "root", SSLMode: "disable", Tag: "local", Color: colorChoices[0]}
	s.dlg.cfg.SSH.Port = 22
	s.dlg.cfg.SSH.AuthType = "password"
	s.dlg.port, s.dlg.sshPort = "3306", "22"
	s.dlg.rebase()
}

// editConnection 打开管理器并进入这条连接的配置页。
func (s *Shell) editConnection(cfg database.ConnectionConfig) {
	s.fillForm(cfg)
	s.selected = cfg.ID
}

// fillForm 把一条已保存的连接装进对话框的表单（不会回填密码）。
func (s *Shell) fillForm(cfg database.ConnectionConfig) {
	d := connDialog{open: true, ready: true, form: true, editing: true, cfg: cfg, typeSel: typeLabel(cfg.Type), tagSel: tagLabel(cfg.Tag), safeSel: safeModeLabel(cfg.EffectiveSafeMode()), safeSet: true, search: s.dlg.search, focusSet: s.dlg.focusSet}
	switch cfg.PasswordMode {
	case app.PasswordAsk:
		d.pwMode = 1
	case app.PasswordNone:
		d.pwMode = 2
	}
	d.port = fmt.Sprint(cfg.Port)
	d.sshPort = fmt.Sprint(max(cfg.SSH.Port, 22))
	d.sshAuth = cfg.SSH.AuthType
	if d.sshAuth == "" {
		d.sshAuth = "password"
	}
	d.sshOpen = cfg.SSH.Enabled
	d.rebase()
	s.dlg = d
}

func typeLabels() []string {
	out := make([]string, len(dbTypes))
	for i, t := range dbTypes {
		out[i] = t.label
	}
	return out
}

func formRow(c *ui.Context, label string, fn func()) {
	ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, label).TextColor(c.Theme().TextMuted).Width(84).TextAlign(ui.End)
		fn()
	})
}

// build turns the dialog's fields into a config, or reports what is missing.
func (d *connDialog) build() (database.ConnectionConfig, string) {
	cfg := d.cfg
	for _, ty := range dbTypes {
		if ty.label == d.typeSel {
			cfg.Type = ty.id
		}
	}
	cfg.Tag = "local"
	for _, tag := range tagOrder {
		if tagLabel(tag) == d.tagSel {
			cfg.Tag = tag
		}
	}
	cfg.SafeMode = string(safeModeFromLabel(d.safeSel))
	cfg.PasswordMode = []string{"", app.PasswordAsk, app.PasswordNone}[d.pwMode]
	if cfg.PasswordMode != "" {
		cfg.ClearPassword = cfg.PasswordMode == app.PasswordNone
	}
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		return cfg, locales.ConnRequired(locales.ConnName())
	}
	if cfg.Type == "sqlite" {
		if strings.TrimSpace(cfg.Database) == "" {
			return cfg, locales.ConnRequired(locales.ConnFile())
		}
		cfg.Host, cfg.Port, cfg.User, cfg.SSH = "", 0, "", database.SSHConfig{}
		return cfg, ""
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return cfg, locales.ConnRequired(locales.ConnHost())
	}
	var port int
	fmt.Sscan(d.port, &port)
	if port <= 0 || port > 65535 {
		return cfg, locales.ConnRequired(locales.ConnPort())
	}
	cfg.Port = port
	cfg.Password = d.password
	if cfg.SSH.Enabled {
		var sp int
		fmt.Sscan(d.sshPort, &sp)
		if sp <= 0 {
			sp = 22
		}
		cfg.SSH.Port, cfg.SSH.AuthType, cfg.SSH.Secret = sp, d.sshAuth, d.secret
		if strings.TrimSpace(cfg.SSH.Host) == "" {
			return cfg, locales.ConnRequired(locales.ConnSSHHost())
		}
	} else {
		cfg.SSH = database.SSHConfig{}
	}
	return cfg, ""
}

func (s *Shell) testConnection() {
	d := &s.dlg
	cfg, problem := d.build()
	if problem != "" {
		d.msg, d.msgOK = problem, false
		return
	}
	d.testing, d.msg = true, ""
	var ok bool
	var msg string
	s.up.bg(func() { ok, msg = s.app.Connections.TestConnection(cfg) }, func() {
		d.testing, d.msgOK = false, ok
		if ok {
			d.msg = locales.ConnTestOk()
		} else {
			d.msg = locales.ConnTestFail(msg)
		}
	})
}

// saveConnection 保存表单。open 为真时保存后直接连接并关闭管理器；
// 否则回到列表页，光标落在刚保存的那一条上。
func (s *Shell) saveConnection(open bool) {
	d := &s.dlg
	cfg, problem := d.build()
	if problem != "" {
		d.msg, d.msgOK = problem, false
		return
	}
	var err error
	s.up.bg(func() { err = s.app.Connections.SaveConnection(cfg) }, func() {
		if err != nil {
			d.msg, d.msgOK = err.Error(), false
			return
		}
		s.loadConnections()
		if open {
			d.open = false
			s.openSavedByName(cfg)
			return
		}
		s.toastOK(locales.ConnSaved(cfg.Name))
		s.reselectSaved(cfg)
	})
}

// reselectSaved 在连接列表重新加载后，回到列表页并把光标放在刚保存的连接上。
func (s *Shell) reselectSaved(cfg database.ConnectionConfig) {
	var conns []database.ConnectionConfig
	s.up.bg(func() { conns, _ = s.app.Connections.GetConnections() }, func() {
		for _, c := range conns {
			if (cfg.ID != "" && c.ID == cfg.ID) || (cfg.ID == "" && c.Name == cfg.Name && c.Type == cfg.Type) {
				if s.dlg.open {
					s.showList()
					s.selected = c.ID
				}
				return
			}
		}
	})
}

// openSavedByName opens a just-saved connection once the list has reloaded.
func (s *Shell) openSavedByName(cfg database.ConnectionConfig) {
	var conns []database.ConnectionConfig
	s.up.bg(func() { conns, _ = s.app.Connections.GetConnections() }, func() {
		for _, c := range conns {
			if (cfg.ID != "" && c.ID == cfg.ID) || (cfg.ID == "" && c.Name == cfg.Name && c.Type == cfg.Type) {
				s.openConnection(c)
				return
			}
		}
	})
}

// dbColors are the tile colors of the database types.
var dbColors = map[string]string{"mysql": "#00758f", "postgres": "#336791", "sqlite": "#0f80cc", "tidb": "#e23d2f", "starrocks": "#6c4cff"}

// dbMark is the short mark on a database tile.
var dbMark = map[string]string{"mysql": "My", "postgres": "Pg", "sqlite": "Sl", "tidb": "Ti", "starrocks": "SR"}

// dbTile draws a database type as a rounded colored square with its mark.
func dbTile(c *ui.Context, kind string, size float32, col ui.Color) ui.Element {
	if col == (ui.Color{}) {
		col = ui.Hex(dbColors[kind])
	}
	t := ui.Column(c).Size(size, size).Shrink(0).Radius(size / 4).Background(col).Center()
	return t.Children(func() {
		ui.Text(c, dbMark[kind]).FontSize(size * 0.38).FontWeight(700).TextColor(ui.Hex("#ffffff"))
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

func (s *Shell) selectedConnection() *database.ConnectionConfig {
	for i := range s.conns {
		if s.conns[i].ID == s.selected {
			return &s.conns[i]
		}
	}
	return nil
}

// beginForm 开始一条新连接，类型先按 driver。
func (s *Shell) beginForm(driver string) {
	s.newConnection()
	s.dlg.setType(driver)
	s.dlg.rebase()
}

func (s *Shell) pickSQLiteFile() {
	var p string
	s.up.bg(func() {
		p, _ = s.host.OpenFile(app.OpenOptions{Title: locales.ConnFile(), Filters: []app.FileFilter{{Name: "SQLite", Exts: []string{"db", "sqlite", "sqlite3", "db3"}}, {Name: "*", Exts: []string{"*"}}}})
	}, func() {
		if p != "" {
			s.dlg.cfg.Database = p
		}
	})
}

func (s *Shell) pickSSHKey() {
	var p string
	var err error
	s.up.bg(func() { p, err = s.app.Connections.PickSSHKeyFile() }, func() {
		if err == nil && p != "" {
			s.dlg.cfg.SSH.KeyPath = p
		}
	})
}
