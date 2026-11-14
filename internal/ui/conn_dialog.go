package ui

import (
	"strings"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)
import (
	"fmt"
)

type connDialog struct {
	open      bool
	editing   bool
	cfg       model.ConnectionConfig
	typeSel   string
	tagSel    string
	port      string
	sshPort   string
	sshAuth   string
	password  string
	secret    string
	sshOpen   bool
	testing   bool
	msg       string
	msgOK     bool
	form      bool   // 正在配置页（否则是列表页）
	base      string // 配置页载入时表单内容的指纹，用来判断有没有未保存的修改
	draft     bool   // 配置页里是一条尚未保存的新连接
	ready     bool   // 管理器已完成了初始页面的选择
	focusSet  bool   // 搜索框已拿过一次键盘焦点
	search    string
	urlOpen   bool
	urlText   string
	safeSel   string                   // Safe Mode level, by its label
	safeSet   bool                     // the user chose one, so a new environment no longer picks it
	pwMode    int                      // 0 keychain, 1 ask every time, 2 none
	folders   []model.ConnectionFolder // 打开表单时的文件夹列表（下拉用）
	folderSel string                   // 下拉当前选中的文件夹名
	fromPage  bool                     // 从欢迎页打开的：没有连接列表对话框可回，保存 / 返回后直接关掉
	tab       int                      // 配置页当前的标签（见 conn_form_tabs.go）
}

// newConnectionDialog 响应「打开连接列表」：已经有打开的连接时弹出连接管理器；
// 一个连接都没打开时，欢迎页本身就是连接列表，不再叠一层对话框，只让搜索框拿回键盘。
func (s *Shell) newConnectionDialog() {
	if len(s.spaces) == 0 && len(s.conns) > 0 {
		s.dlg = connDialog{search: s.dlg.search}
		return
	}
	s.openConnList()
}

// openConnList 打开连接管理器；先停在列表页还是直接进配置页，由 connSync 在连接列表就绪后决定。
func (s *Shell) openConnList() {
	s.dlg = connDialog{open: true}
}

// newConnection 打开管理器并直接进入"新建连接"表单。
func (s *Shell) newConnection() {
	fromPage := s.openedFromPage()
	s.dlg = connDialog{open: true, fromPage: fromPage, ready: true, form: true, draft: true, typeSel: typeLabel(app.DefaultDBType()), tagSel: tagLabel("local"), sshAuth: "password", safeSel: safeModeLabel(s.prefs.defaultSafeMode()), safeSet: s.prefs.DefaultSafe != "", folders: s.folders, folderSel: locales.ConnUngrouped()}
	s.dlg.cfg = model.ConnectionConfig{Type: app.DefaultDBType(), Host: "127.0.0.1", Port: defaultPort(app.DefaultDBType()), User: defaultUser(app.DefaultDBType()), SSLMode: "disable", Tag: "local"}
	s.dlg.cfg.SSH.Port = 22
	s.dlg.cfg.SSH.AuthType = "password"
	s.dlg.port, s.dlg.sshPort = fmt.Sprint(defaultPort(app.DefaultDBType())), "22"
	s.dlg.rebase()
}

// editConnection 打开管理器并进入这条连接的配置页。
func (s *Shell) editConnection(cfg model.ConnectionConfig) {
	fromPage := s.openedFromPage()
	s.fillForm(cfg)
	s.dlg.fromPage = fromPage
	s.selected = cfg.ID
}

// openedFromPage 报告此刻打开配置页是不是从欢迎页来的：没有打开的连接，也没有列表对话框开着。
func (s *Shell) openedFromPage() bool { return len(s.spaces) == 0 && !s.dlg.open }

// fillForm 把一条已保存的连接装进对话框的表单（不会回填密码）。
func (s *Shell) fillForm(cfg model.ConnectionConfig) {
	d := connDialog{open: true, ready: true, form: true, editing: true, cfg: cfg, typeSel: typeLabel(cfg.Type), tagSel: tagLabel(cfg.Tag), safeSel: safeModeLabel(cfg.EffectiveSafeMode()), safeSet: true, search: s.dlg.search, focusSet: s.dlg.focusSet, folders: s.folders}
	d.folderSel = d.folderLabel(cfg.FolderID)
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

// build turns the dialog's fields into a config, or reports what is missing.
func (d *connDialog) build() (model.ConnectionConfig, string) {
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
	cfg.FolderID = d.folderID()
	cfg.PasswordMode = []string{"", app.PasswordAsk, app.PasswordNone}[d.pwMode]
	if cfg.PasswordMode != "" {
		cfg.ClearPassword = cfg.PasswordMode == app.PasswordNone
	}
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		return cfg, locales.ConnRequired(locales.ConnName())
	}
	if !app.CapsOf(cfg.Type).Network {
		if strings.TrimSpace(cfg.Database) == "" {
			return cfg, locales.ConnRequired(locales.ConnFile())
		}
		cfg.Host, cfg.Port, cfg.User, cfg.SSH = "", 0, "", model.SSHConfig{}
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
		cfg.SSH = model.SSHConfig{}
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
	var err error
	s.up.bg(func() { err = s.app.Connections.TestConnection(cfg) }, func() {
		d.testing, d.msgOK = false, err == nil
		if err == nil {
			d.msg = locales.ConnTestOk()
		} else {
			d.msg = locales.ConnTestFail(errMessage(err))
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
			d.msg, d.msgOK = errMessage(err), false
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
func (s *Shell) reselectSaved(cfg model.ConnectionConfig) {
	var conns []model.ConnectionConfig
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
func (s *Shell) openSavedByName(cfg model.ConnectionConfig) {
	var conns []model.ConnectionConfig
	s.up.bg(func() { conns, _ = s.app.Connections.GetConnections() }, func() {
		for _, c := range conns {
			if (cfg.ID != "" && c.ID == cfg.ID) || (cfg.ID == "" && c.Name == cfg.Name && c.Type == cfg.Type) {
				s.openConnection(c)
				return
			}
		}
	})
}

// beginForm 开始一条新连接，类型先按 driver。
func (s *Shell) beginForm(driver model.DBType) {
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
