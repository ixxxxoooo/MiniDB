package ui

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// 连接管理器是一个左右分栏的对话框：左边是可搜索的连接列表（顶部是"新建连接"），
// 右边是所选连接的表单；数据库类型是表单顶部的一排卡片，类型与表单合在一起，没有分步。

// tagColors are the status colours of the environment tags.
var tagColors = map[string]string{"local": "#34c759", "development": "#0a84ff", "test": "#ff9f0a", "staging": "#af52de", "production": "#ff453a"}

var tagOrder = []string{"local", "development", "test", "staging", "production"}

// connectionDialog 是连接管理器：左边是已保存的连接（最上面是"新建连接"），
// 右边是所选连接的表单。数据库类型就在表单顶部一排卡片里选，不再有"先选类型再进入"的两步流程。
func (s *Shell) connectionDialog(c *ui.Context) {
	d := &s.dlg
	if !d.open {
		return
	}
	p := s.tokens(c)
	s.connSync()
	ct := *c.Theme()
	ct.Spacing, ct.FontSize = 2.75, fontBase
	submit := false
	ui.Modal(c, &d.open, func() {
		ui.Themed(c, &ct).Background(ui.Transparent).Width(900).Height(600).Children(func() {
			ui.Row(c).Fill().Children(func() {
				s.connSidebar(c, p)
				ui.Box(c).Width(1).FillHeight().Background(p.borderSubtle)
				ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() { s.connDetail(c, p, &submit) })
			})
		})
	}).Padding(0).Clip()
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		s.saveConnection(false)
	}
	if submit {
		s.saveConnection(true)
	}
}

// connSync 在每帧开头理顺管理器的选择：刚打开时选中一条已有连接（没有就进入"新建"），
// 所选连接被删掉后也回到合理的位置。
func (s *Shell) connSync() {
	d := &s.dlg
	if !s.connsReady {
		return
	}
	if !d.ready {
		d.ready = true
		if cur := s.selectedConnection(); cur != nil && !d.draft {
			s.pickConnection(*cur)
		} else if len(s.conns) > 0 && !d.draft {
			s.pickConnection(s.conns[0])
		} else {
			s.startDraft("mysql")
		}
		return
	}
	if d.editing && !d.draft {
		found := false
		for _, cfg := range s.conns {
			if cfg.ID == d.cfg.ID {
				found = true
			}
		}
		if !found {
			if len(s.conns) > 0 {
				s.pickConnection(s.conns[0])
			} else {
				s.startDraft("mysql")
			}
		}
	}
}

// pickConnection 把一条已保存的连接装进右边的表单。
func (s *Shell) pickConnection(cfg database.ConnectionConfig) {
	open := s.dlg.open
	s.fillForm(cfg)
	s.dlg.open, s.dlg.ready = open, true
	s.selected = cfg.ID
}

// startDraft 开始填写一条新连接，类型先按 driver。
func (s *Shell) startDraft(driver string) {
	open, search := s.dlg.open, s.dlg.search
	s.beginForm(driver)
	s.dlg.open, s.dlg.search = open, search
}

// connSidebar 是左边的连接列表。
func (s *Shell) connSidebar(c *ui.Context, p tokens) {
	d := &s.dlg
	ui.Column(c).Width(290).FillHeight().Background(p.surfaceSecondary).Shrink(0).Padding(14, 10, 8, 10).Gap(8).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Padding(0, 4).FillWidth().Children(func() {
			ui.Text(c, locales.WelcomeTitle()).FontWeight(700).FontSize(15)
			ui.Spacer(c)
			ui.MenuButton(c, locales.ConnMore(), func(m *ui.Menu) {
				if m.Item(locales.ConnRestore()).Chosen() {
					s.importConnections("")
				}
				if m.Item(locales.ConnBackup()).Chosen() {
					s.exportConnections("")
				}
			}).Height(sizeBtnSm).FontSize(font2XS)
		})
		q := strings.ToLower(strings.TrimSpace(d.search))
		var shown []database.ConnectionConfig
		for _, cfg := range s.conns {
			if q == "" || strings.Contains(strings.ToLower(cfg.Name+" "+cfg.Host+" "+cfg.Database+" "+cfg.Tag), q) {
				shown = append(shown, cfg)
			}
		}
		// 上下键在列表里移动选择（搜索框有焦点时也一样），选中即装进表单
		move := func(delta int) {
			if len(shown) == 0 {
				return
			}
			sel := -1
			for i, cfg := range shown {
				if cfg.ID == s.selected && !d.draft {
					sel = i
				}
			}
			s.pickConnection(shown[min(max(sel+delta, 0), len(shown)-1)])
		}
		var search ui.Element
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 8).Height(sizeBtn).Radius(radiusBtn).Background(p.surface).Border(1, p.borderSubtle).FillWidth().Children(func() {
			ui.Icon(c, icSearch).FontSize(13).TextColor(p.fgMuted)
			search = ui.TextInput(c.Key("conn-search"), &d.search).Placeholder(locales.ConnSearchList()).FontSize(font2XS+1).Grow(1).Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
				OnTextKey(func(k ui.TextKey) bool {
					switch {
					case k.Mods != 0:
						return false
					case k.Key == ui.KeyDown:
						move(1)
					case k.Key == ui.KeyUp:
						move(-1)
					default:
						return false
					}
					s.redraw()
					return true
				})
		})
		if !d.draft && !d.focusSet { // 打开管理器时让搜索框拿到键盘：↑↓ 选择，↵ 连接
			d.focusSet = true
			search.Focus()
		}
		if search.Submitted() && !d.draft {
			if cur := s.selectedConnection(); cur != nil {
				d.open = false
				s.openConnection(*cur)
			}
		}
		// 最上面一行："新建连接"
		neu := ui.Row(c.Key("conn-new")).Gap(10).Padding(6, 10).Radius(radiusPanel).AlignItems(ui.Center).FillWidth()
		switch {
		case d.draft:
			neu.Background(p.accent.Alpha(0.12))
		case neu.Hovered():
			neu.Background(p.hover)
		}
		neu.Children(func() {
			ui.Row(c).Size(34, 34).Shrink(0).Center().Radius(34/4).Border(1.5, p.accent.Alpha(0.6)).Children(func() {
				ui.Icon(c, icPlus).FontSize(16).TextColor(p.accent)
			})
			ui.Text(c, locales.ConnNew()).FontSize(font2XS + 1).FontWeight(600).TextColor(p.accent)
		})
		if neu.Clicked() && !d.draft {
			s.startDraft("mysql")
		}
		if len(shown) == 0 && q != "" {
			ui.Text(c, locales.ConnNoMatch()).FontSize(font2XS).TextColor(p.fgMuted).Padding(12).TextAlign(ui.Center).FillWidth()
		}
		scrollbars(c, ui.Scroll(c.Key("conns")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Gap(1).FillWidth().Children(func() {
				for i := range shown {
					s.connRow(c, p, shown[i], !d.draft && s.selected == shown[i].ID)
				}
			})
		})
		ui.Text(c, locales.ConnListHint()).FontSize(font2XS-1).TextColor(p.fgMuted).FillWidth().SingleLine().Padding(0, 4).Shrink(0)
	})
}

// connRow is one connection of the list.
func (s *Shell) connRow(c *ui.Context, p tokens, cfg database.ConnectionConfig, selected bool) {
	d := &s.dlg
	row := ui.Row(c.Key(cfg.ID)).Gap(10).Padding(6, 10).Radius(radiusPanel).AlignItems(ui.Center).FillWidth()
	hovered := row.Hovered()
	switch {
	case selected:
		row.Background(p.accent.Alpha(0.12))
	case hovered:
		row.Background(p.hover)
	}
	var del bool
	row.Children(func() {
		dbTile(c, cfg.Type, 34, connColor(c, cfg.Color))
		ui.Column(c).Grow(1).Gap(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, cfg.Name).FontSize(font2XS + 1).FontWeight(600).SingleLine()
				if cfg.Tag != "" && cfg.Tag != "local" {
					ui.Text(c, tagLabel(cfg.Tag)).FontSize(10).FontWeight(500).TextColor(tagColor(c, cfg.Tag)).Padding(0, 5).Radius(4).Background(tagColor(c, cfg.Tag).Alpha(0.14))
				}
				if cfg.ReadOnly {
					ui.Icon(c, icLock).FontSize(11).TextColor(p.fgMuted)
				}
			})
			ui.Text(c, connSubtitle(cfg)).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
		})
		switch {
		case s.connecting[cfg.ID]:
			ui.Spinner(c)
		case s.isOpen(cfg.ID) && !hovered && !selected:
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(p.success)
				ui.Text(c, locales.ConnOpened()).FontSize(font2XS - 1).TextColor(p.fgSecondary)
			})
		}
		if hovered {
			del = tipBtn(c.Key("d"), icTrash, locales.CommonDelete()).Size(sizeBtnSm, sizeBtnSm).Clicked()
		}
	})
	switch {
	case del:
		defer s.deleteConnection(cfg)
	case row.DoubleClicked():
		d.open = false
		s.openConnection(cfg)
	case row.Clicked() && !selected:
		s.pickConnection(cfg)
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.ConnOpen()).Chosen() {
			defer func() { d.open = false; s.openConnection(cfg) }()
		}
		if m.Item(locales.ConnDuplicate()).Chosen() {
			defer s.duplicateConnection(cfg)
		}
		if m.Item(locales.ConnCopyURL()).Chosen() {
			c.Services().WriteClipboard(connURL(cfg))
			defer s.toastOK(locales.ConnCopiedURL())
		}
		m.Separator()
		if m.Item(locales.CommonDelete()).Chosen() {
			defer s.deleteConnection(cfg)
		}
	})
}

// connURL is a connection's address, as mysql://user@host:3306/db; the
// password is never part of it.
func connURL(cfg database.ConnectionConfig) string {
	scheme := cfg.Type
	if scheme == "postgres" {
		scheme = "postgresql"
	}
	if cfg.Type == "sqlite" {
		return "sqlite://" + cfg.Database
	}
	u := url.URL{Scheme: scheme, Host: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), Path: "/" + cfg.Database}
	if cfg.User != "" {
		u.User = url.User(cfg.User)
	}
	return u.String()
}

// ---- the form ----

// connField is a row of the form: its label at the left, right-aligned.
func connField(c *ui.Context, p tokens, label string, fn func()) {
	ui.Row(c).Gap(12).AlignItems(ui.Center).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(font2XS).TextColor(p.fgSecondary).Width(88).Shrink(0).TextAlign(ui.End)
		fn()
	})
}

// selectTag sets the environment and, unless the user chose a colour of their
// own, the status colour that goes with it.
func (d *connDialog) selectTag(tag string) {
	old := d.cfg.Tag
	d.cfg.Tag, d.tagSel = tag, tagLabel(tag)
	if !d.safeSet { // a production connection starts out guarded
		d.safeSel = safeModeLabel((database.ConnectionConfig{Tag: tag}).EffectiveSafeMode())
	}
	if d.cfg.Color == "" || d.cfg.Color == tagColors[old] {
		d.cfg.Color = tagColors[tag]
	}
}

// connDetail 是右边：头部（图标、名称、导入 URL、关闭）、表单、底部的测试/保存/连接。
func (s *Shell) connDetail(c *ui.Context, p tokens, submit *bool) {
	d := &s.dlg
	t := c.Theme()
	title, sub := locales.ConnNewTitle(typeLabel(d.cfg.Type)), locales.ConnDraftHint()
	if !d.draft {
		title, sub = d.cfg.Name, connSubtitle(d.cfg)
		if strings.TrimSpace(title) == "" {
			title = locales.ConnEditTitle()
		}
	}
	field := func(key string, v *string) ui.Element {
		in := ui.TextInput(c.Key(key), v).Grow(1).Height(sizeBtn)
		if in.Submitted() {
			*submit = true
		}
		return in
	}
	ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(14, 12, 12, 20).FillWidth().Shrink(0).Children(func() {
		dbTile(c, d.cfg.Type, 36, connColor(c, d.cfg.Color))
		ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontWeight(600).FontSize(15).SingleLine().TruncateMiddle()
			ui.Text(c, sub).FontSize(font2XS).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
		})
		if d.draft && d.cfg.Type != "sqlite" {
			if ui.Button(c, locales.ConnImportURL()).Height(sizeBtnSm).FontSize(font2XS).Clicked() {
				d.urlOpen = !d.urlOpen
			}
		}
		if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			d.open = false
		}
	})
	hairline(c, p.borderSubtle)
	if d.urlOpen {
		ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Padding(10, 20, 0, 20).Shrink(0).Children(func() {
			in := ui.TextInput(c.Key("url"), &d.urlText).Placeholder(locales.ConnURLHint()).Font(mono).FontSize(font2XS).Grow(1).Height(sizeBtn).AutoFocus()
			if in.Submitted() || ui.Button(c, locales.CommonApply()).Height(sizeBtn).Clicked() {
				if !d.applyURL(d.urlText) {
					d.msg, d.msgOK = locales.ConnURLBad(), false
				} else {
					d.urlOpen, d.urlText, d.msg = false, "", ""
				}
			}
		})
	}
	scrollbars(c, ui.Scroll(c.Key("connform")).FillWidth().Grow(1)).Children(func() {
		ui.Column(c).Gap(10).FillWidth().Padding(14, 26, 18, 20).Children(func() {
			connSection(c, p, locales.ConnPickType())
			s.typeCards(c, p)
			connField(c, p, locales.ConnName(), func() {
				if d.draft { // 新建时名称框直接拿到键盘
					field("name-new", &d.cfg.Name).AutoFocus()
				} else {
					field("name", &d.cfg.Name)
				}
			})
			connField(c, p, locales.ConnStatus(), func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					for _, tag := range tagOrder {
						tag := tag
						on := d.cfg.Tag == tag || (d.cfg.Tag == "" && tag == "local")
						col := ui.Hex(tagColors[tag])
						chip := ui.Row(c.Key("tag-"+tag)).Gap(6).AlignItems(ui.Center).Padding(0, 10).Height(sizeBtn-4).Radius(14).Border(1, p.borderSubtle)
						switch {
						case on:
							chip.Background(col.Alpha(0.14)).BorderColor(col)
						case chip.Hovered():
							chip.Background(p.hover)
						}
						chip.Children(func() {
							ui.Box(c).Size(8, 8).Radius(4).Background(col)
							ui.Text(c, tagLabel(tag)).FontSize(font2XS)
						})
						if chip.Clicked() {
							d.selectTag(tag)
						}
					}
				})
			})
			connField(c, p, locales.ConnColor(), func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					for _, col := range colorChoices {
						on := d.cfg.Color == col
						sw := ui.Button(c.Key(col), "").Label(col).Size(18, 18).Padding(0).Radius(9).Background(ui.Hex(col)).BorderWidth(0)
						if on {
							sw.BorderWidth(2).BorderColor(t.Surface)
							sw.Shadow(0, 0, 0, 2, ui.Hex(col))
						}
						if sw.Clicked() {
							d.cfg.Color = col
						}
					}
				})
			})
			connSection(c, p, locales.ConnSectionServer())
			if d.cfg.Type == "sqlite" {
				connField(c, p, locales.ConnFile(), func() {
					field("file", &d.cfg.Database).Placeholder("/path/to/file.db")
					if ui.Button(c, locales.ConnSSHChoose()).Height(sizeBtn).Clicked() {
						s.pickSQLiteFile()
					}
				})
			} else {
				connField(c, p, locales.ConnHost(), func() {
					field("host", &d.cfg.Host)
					ui.Text(c, locales.ConnPort()).FontSize(font2XS).TextColor(p.fgSecondary)
					field("port", &d.port).Grow(0).Width(84)
				})
				connField(c, p, locales.ConnUser(), func() { field("user", &d.cfg.User) })
				connField(c, p, locales.ConnPasswordMode(), func() {
					ui.Segmented(c, &d.pwMode, locales.ConnPasswordKeychain(), locales.ConnPasswordAsk(), locales.ConnPasswordNone())
				})
				if d.pwMode != 2 {
					connField(c, p, locales.ConnPassword(), func() {
						in := ui.TextInput(c.Key("pass"), &d.password).Password().Grow(1).Height(sizeBtn)
						if in.Submitted() {
							*submit = true
						}
						switch {
						case d.pwMode == 1:
							in.Placeholder(locales.ConnPasswordTestOnly())
						case d.editing && d.cfg.HasPassword:
							in.Placeholder(locales.ConnPasswordSaved())
						}
					})
				}
				if d.editing && d.cfg.HasPassword && d.pwMode == 0 {
					connField(c, p, "", func() { ui.Checkbox(c, &d.cfg.ClearPassword, locales.ConnClearPassword()) })
				}
				connField(c, p, locales.ConnDatabase(), func() { field("db", &d.cfg.Database) })
				if d.cfg.Type == "postgres" {
					connField(c, p, locales.ConnSSLMode(), func() {
						ui.Select(c, &d.cfg.SSLMode, []string{"disable", "require", "verify-ca", "verify-full"}).MinWidth(0).Width(160).Height(sizeBtn)
					})
				}
			}
			connField(c, p, locales.ConnSafeMode(), func() {
				if ui.Select(c, &d.safeSel, safeModeLabels()).MinWidth(0).Width(160).Height(sizeBtn).Changed() {
					d.safeSet = true
				}
				ui.Text(c, safeModeDesc(safeModeFromLabel(d.safeSel))).FontSize(font2XS).TextColor(p.fgMuted).SingleLine()
			})
			connField(c, p, "", func() { ui.Checkbox(c, &d.cfg.ReadOnly, locales.ConnReadOnly()) })
			if d.cfg.Type != "sqlite" {
				connField(c, p, "", func() { ui.Checkbox(c, &d.cfg.SSH.Enabled, locales.ConnSSH()) })
				if d.cfg.SSH.Enabled {
					ui.Column(c).Gap(10).Padding(12).FillWidth().Radius(radiusPanel).Background(p.surfaceSecondary).Border(1, p.borderSubtle).Children(func() {
						connField(c, p, locales.ConnSSHHost(), func() {
							field("sshhost", &d.cfg.SSH.Host)
							ui.Text(c, locales.ConnSSHPort()).FontSize(font2XS).TextColor(p.fgSecondary)
							field("sshport", &d.sshPort).Grow(0).Width(84)
						})
						connField(c, p, locales.ConnSSHUser(), func() { field("sshuser", &d.cfg.SSH.User) })
						connField(c, p, locales.ConnSSHAuth(), func() {
							auth := 0
							if d.sshAuth == "key" {
								auth = 1
							}
							if ui.Segmented(c, &auth, locales.ConnSSHPassword(), locales.ConnSSHKey()).Changed() {
								d.sshAuth = []string{"password", "key"}[auth]
							}
						})
						if d.sshAuth == "key" {
							connField(c, p, locales.ConnSSHKeyFile(), func() {
								field("sshkey", &d.cfg.SSH.KeyPath)
								if ui.Button(c, locales.ConnSSHChoose()).Height(sizeBtn).Clicked() {
									s.pickSSHKey()
								}
							})
						}
						connField(c, p, locales.ConnSSHSecret(), func() {
							in := ui.TextInput(c.Key("sshsecret"), &d.secret).Password().Grow(1).Height(sizeBtn)
							if in.Submitted() {
								*submit = true
							}
							if d.editing && d.cfg.SSH.HasSecret {
								in.Placeholder(locales.ConnPasswordSaved())
							}
						})
					})
				}
			}
		})
	})
	hairline(c, p.borderSubtle)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(10, 14).FillWidth().Shrink(0).Background(p.surfaceSecondary).Children(func() {
		if ui.Button(c, locales.ConnTest()).Height(sizeBtn).Disabled(d.testing).Clicked() {
			s.testConnection()
		}
		switch {
		case d.testing:
			ui.Spinner(c)
			ui.Spacer(c)
		case d.msg != "":
			col := p.danger
			if d.msgOK {
				col = p.success
			}
			ui.Text(c, d.msg).FontSize(font2XS).TextColor(col).SingleLine().Grow(1).TruncateMiddle().Tooltip(d.msg).Padding(0, 6)
		default:
			ui.Spacer(c)
		}
		if !d.draft {
			del := ui.Button(c, locales.CommonDelete()).Height(sizeBtn).TextColor(p.danger)
			if del.Clicked() {
				s.deleteConnection(d.cfg)
			}
		}
		if ui.Button(c, locales.CommonSave()).Height(sizeBtn).Clicked() {
			s.saveConnection(false)
		}
		if ui.PrimaryButton(c, locales.ConnConnect()).Height(sizeBtn).Padding(0, 16).Clicked() {
			*submit = true
		}
	})
	_ = t
}

// typeCards 是表单顶部的数据库类型选择：一排带彩色图标的卡片，点一下就切换，
// 默认端口、默认用户跟着变，下面的表单立刻随之调整。
func (s *Shell) typeCards(c *ui.Context, p tokens) {
	d := &s.dlg
	ui.Row(c).Gap(8).FillWidth().Children(func() {
		for _, ty := range dbTypes {
			ty := ty
			on := d.cfg.Type == ty.id
			card := ui.Column(c.Key("type-"+ty.id)).Gap(6).Padding(10, 4).Grow(1).Basis(0).MinWidth(0).AlignItems(ui.Center).Radius(radiusPanel).Border(1, p.borderSubtle)
			switch {
			case on:
				card.Background(p.accent.Alpha(0.10)).BorderColor(p.accent)
			case card.Hovered():
				card.Background(p.hover)
			}
			card.Children(func() {
				dbTile(c, ty.id, 30, ui.Color{})
				ui.Text(c, ty.label).FontSize(font2XS).FontWeight(500).SingleLine()
			})
			if card.Clicked() && !on {
				d.setType(ty.id)
			}
		}
	})
}

// setType 切换数据库类型：端口、用户名仍是上一种类型的默认值时一起换成新类型的默认值。
func (d *connDialog) setType(id string) {
	old := d.cfg.Type
	d.cfg.Type, d.typeSel = id, typeLabel(id)
	switch {
	case id == "sqlite":
		d.port = ""
	case d.port == "" || d.port == "0" || d.port == fmt.Sprint(defaultPort(old)):
		d.port = fmt.Sprint(defaultPort(id))
	}
	if d.cfg.User == defaultUser(old) {
		d.cfg.User = defaultUser(id)
	}
}

func defaultUser(typ string) string {
	switch typ {
	case "postgres":
		return "postgres"
	case "sqlite":
		return ""
	}
	return "root"
}

// connSection heads a group of fields of the connection form.
func connSection(c *ui.Context, p tokens, title string) {
	ui.Row(c.Key("sec-"+title)).Gap(8).AlignItems(ui.Center).FillWidth().Padding(4, 0, 0, 0).Children(func() {
		ui.Text(c, strings.ToUpper(title)).FontSize(10).FontWeight(600).TextColor(p.fgMuted).Shrink(0)
		ui.Box(c).Height(1).Grow(1).Background(p.borderSubtle)
	})
}

// applyURL fills the form from a connection URL such as
// mysql://user:pass@host:3306/db; it reports whether the URL was understood.
func (d *connDialog) applyURL(raw string) bool {
	typ, host, port, user, pass, db, ok := parseConnURL(raw)
	if !ok {
		return false
	}
	d.cfg.Type, d.typeSel = typ, typeLabel(typ)
	if typ == "sqlite" {
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

func parseConnURL(raw string) (typ, host string, port int, user, pass, db string, ok bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		return
	}
	switch strings.ToLower(u.Scheme) {
	case "mysql", "mariadb":
		typ = "mysql"
	case "postgres", "postgresql", "pg":
		typ = "postgres"
	case "tidb":
		typ = "tidb"
	case "starrocks":
		typ = "starrocks"
	case "sqlite", "sqlite3", "file":
		typ, db, ok = "sqlite", u.Host+u.Path, true
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
