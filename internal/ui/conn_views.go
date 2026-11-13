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

// The connection dialogs follow TablePlus: a connection manager with a
// searchable list; "+" opens a picker of database types; the form that
// follows has the name, a status colour, the server, and an SSH section, with
// Test, Save and Connect at the foot.

// tagColors are the status colours of the environment tags.
var tagColors = map[string]string{"local": "#34c759", "development": "#0a84ff", "test": "#ff9f0a", "staging": "#af52de", "production": "#ff453a"}

var tagOrder = []string{"local", "development", "test", "staging", "production"}

func (s *Shell) connectionDialog(c *ui.Context) {
	d := &s.dlg
	if !d.open {
		return
	}
	p := s.tokens(c)
	switch d.view {
	case 0:
		s.connListView(c, p)
	case 1:
		s.driverPickerView(c, p)
	default:
		s.connFormView(c, p)
	}
}

// ---- the list of connections ----

// connListView lists saved connections: a click selects, a double click or
// Enter connects, the arrows move.
func (s *Shell) connListView(c *ui.Context, p tokens) {
	d := &s.dlg
	s.dialog(c, &d.open, dlgOpts{title: locales.WelcomeTitle(), width: 580, height: 480}, func() {
		q := strings.ToLower(strings.TrimSpace(d.search))
		var shown []database.ConnectionConfig
		for _, cfg := range s.conns {
			if q == "" || strings.Contains(strings.ToLower(cfg.Name+" "+cfg.Host+" "+cfg.Database+" "+cfg.Tag), q) {
				shown = append(shown, cfg)
			}
		}
		// the arrows move the selection even while the search field has the focus
		move := func(delta int) {
			if len(shown) == 0 {
				return
			}
			sel := -1
			for i, cfg := range shown {
				if cfg.ID == s.selected {
					sel = i
				}
			}
			s.selected = shown[min(max(sel+delta, 0), len(shown)-1)].ID
		}
		var search ui.Element
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 8).Height(sizeBtn).Radius(radiusBtn).Background(p.surfaceSecondary).FillWidth().Children(func() {
			ui.Icon(c, icSearch).FontSize(13).TextColor(p.fgMuted)
			search = ui.TextInput(c.Key("conn-search"), &d.search).Placeholder(locales.ConnSearchList()).FontSize(font2XS+1).Grow(1).AutoFocus().Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
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
		sel := -1
		for i, cfg := range shown {
			if cfg.ID == s.selected {
				sel = i
			}
		}
		if len(shown) > 0 {
			switch {
			case c.Shortcut(0, ui.KeyDown):
				move(1)
			case c.Shortcut(0, ui.KeyUp):
				move(-1)
			}
			if sel < 0 {
				s.selected = shown[0].ID
				sel = 0
			}
			if search.Submitted() {
				cur := s.selectedConnection()
				if cur != nil {
					d.open = false
					s.openConnection(*cur)
				}
			}
		}
		if len(shown) == 0 {
			msg := locales.WelcomeEmpty()
			if q != "" {
				msg = locales.ConnNoMatch()
			}
			ui.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
				ui.Icon(c, icDatabase).FontSize(32).TextColor(p.fgMuted)
				ui.Text(c, msg).TextColor(p.fgSecondary)
				if ui.PrimaryButton(c, locales.ConnCreateNew()).Height(sizeBtn).Clicked() {
					d.view = 1
				}
			})
		}
		ui.Scroll(c.Key("conns")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).Gap(1).FillWidth().Children(func() {
				for i := range shown {
					s.connRow(c, p, shown[i], s.selected == shown[i].ID)
				}
			})
		})
	}, func() {
		if tipBtn(c, icPlus, locales.ConnNew()+s.keyHint("new-conn")).Size(sizeBtn, sizeBtn).Clicked() {
			d.view = 1
		}
		ui.MenuButton(c, locales.ConnMore(), func(m *ui.Menu) {
			if m.Item(locales.ConnRestore()).Chosen() {
				s.importConnections("")
			}
			if m.Item(locales.ConnBackup()).Chosen() {
				s.exportConnections("")
			}
		}).Height(sizeBtn)
		ui.Text(c, locales.ConnListHint()).FontSize(font2XS-1).TextColor(p.fgMuted).Grow(1).SingleLine().Padding(0, 4)
		cur := s.selectedConnection()
		if ui.PrimaryButton(c, locales.ConnConnect()).Height(sizeBtn).Padding(0, 18).Disabled(cur == nil).Clicked() && cur != nil {
			d.open = false
			s.openConnection(*cur)
		}
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
	var edit, del bool
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
		if hovered || selected {
			edit = tipBtn(c.Key("e"), icSettings, locales.CommonEdit()).Size(sizeBtnSm, sizeBtnSm).Clicked()
			del = tipBtn(c.Key("d"), icTrash, locales.CommonDelete()).Size(sizeBtnSm, sizeBtnSm).Clicked()
		}
	})
	switch {
	case edit:
		defer s.editConnection(cfg)
	case del:
		defer s.deleteConnection(cfg)
	case row.DoubleClicked():
		d.open = false
		s.openConnection(cfg)
	case row.Clicked():
		s.selected = cfg.ID
	}
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.ConnOpen()).Chosen() {
			defer func() { d.open = false; s.openConnection(cfg) }()
		}
		if m.Item(locales.CommonEdit()).Chosen() {
			defer s.editConnection(cfg)
		}
		if m.Item(locales.ConnDuplicate()).Chosen() {
			defer s.duplicateConnection(cfg)
		}
		if m.Item(locales.ConnCopyURL()).Chosen() {
			c.Services().WriteClipboard(connURL(cfg))
			defer s.toast(locales.ConnCopiedURL())
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

// ---- the type picker ----

// driverPickerView is the first step of a new connection: a search and a grid
// of database types; Create (or a double click, or Enter) goes on to the form.
func (s *Shell) driverPickerView(c *ui.Context, p tokens) {
	d := &s.dlg
	q := strings.ToLower(strings.TrimSpace(d.pickQ))
	var shown []string
	for _, ty := range dbTypes {
		if q == "" || strings.Contains(strings.ToLower(ty.label), q) || strings.Contains(ty.id, q) {
			shown = append(shown, ty.id)
		}
	}
	idx := -1
	for i, id := range shown {
		if id == d.pickSel {
			idx = i
		}
	}
	if idx < 0 && len(shown) > 0 {
		idx = 0
		d.pickSel = shown[0]
	}
	const perRow = 4
	create := func() {
		if d.pickSel != "" {
			s.beginForm(d.pickSel)
		}
	}
	s.dialog(c, &d.open, dlgOpts{title: locales.ConnPickDriver(), width: 520}, func() {
		var in ui.Element
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 8).Height(sizeBtn).Radius(radiusBtn).Background(p.surfaceSecondary).FillWidth().Children(func() {
			ui.Icon(c, icSearch).FontSize(13).TextColor(p.fgMuted)
			in = ui.TextInput(c.Key("type-search"), &d.pickQ).Placeholder(locales.ConnSearchTypes()).FontSize(font2XS+1).Grow(1).AutoFocus().Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
				OnTextKey(func(k ui.TextKey) bool {
					step := 0
					switch {
					case k.Mods != 0:
						return false
					case k.Key == ui.KeyDown:
						step = perRow
					case k.Key == ui.KeyUp:
						step = -perRow
					case k.Key == ui.KeyRight && d.pickQ == "":
						step = 1
					case k.Key == ui.KeyLeft && d.pickQ == "":
						step = -1
					default:
						return false
					}
					for i, id := range shown {
						if id == d.pickSel {
							d.pickSel = shown[min(max(i+step, 0), len(shown)-1)]
							break
						}
					}
					s.redraw()
					return true
				})
		})
		if in.Submitted() {
			create()
		}
		if len(shown) > 0 {
			switch {
			case c.Shortcut(0, ui.KeyRight):
				idx = min(idx+1, len(shown)-1)
			case c.Shortcut(0, ui.KeyLeft):
				idx = max(idx-1, 0)
			case c.Shortcut(0, ui.KeyDown):
				idx = min(idx+perRow, len(shown)-1)
			case c.Shortcut(0, ui.KeyUp):
				idx = max(idx-perRow, 0)
			}
			d.pickSel = shown[idx]
		}
		if len(shown) == 0 {
			ui.Text(c, locales.ConnNoTypes()).TextColor(p.fgMuted).Padding(24).TextAlign(ui.Center).FillWidth()
		}
		for r := 0; r < len(shown); r += perRow {
			ui.Row(c.Key(fmt.Sprint("r", r))).Gap(10).FillWidth().Children(func() {
				for i := r; i < r+perRow; i++ {
					if i >= len(shown) {
						break
					}
					id := shown[i]
					on := id == d.pickSel
					card := ui.Column(c.Key(id)).Width(113.5).Shrink(0).Padding(14, 4, 12, 4).Radius(radiusPanel).Border(1, p.borderSubtle).AlignItems(ui.Center).Gap(8)
					switch {
					case on:
						card.Background(p.accent.Alpha(0.10)).BorderColor(p.accent)
					case card.Hovered():
						card.Background(p.hover)
					}
					card.Children(func() {
						dbTile(c, id, 44, ui.Color{})
						ui.Text(c, typeLabel(id)).FontSize(font2XS).FontWeight(600).SingleLine()
						sub := locales.ConnDefaultPort(fmt.Sprint(defaultPort(id)))
						if id == "sqlite" {
							sub = locales.ConnFile()
						}
						ui.Text(c, sub).FontSize(10).TextColor(p.fgMuted).SingleLine()
					})
					switch {
					case card.DoubleClicked():
						d.pickSel = id
						create()
					case card.Clicked():
						d.pickSel = id
					}
				}
			})
		}
	}, func() {
		if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
			d.view = 0
		}
		if ui.PrimaryButton(c, locales.ConnCreate()).Height(sizeBtn).Padding(0, 18).Disabled(d.pickSel == "").Clicked() {
			create()
		}
	})
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

func (s *Shell) connFormView(c *ui.Context, p tokens) {
	d := &s.dlg
	t := c.Theme()
	title := locales.ConnNewTitle(typeLabel(d.cfg.Type))
	sub := ""
	if d.editing {
		title, sub = locales.ConnEditTitle(), d.cfg.Name
	}
	submit := false
	field := func(key string, v *string) ui.Element {
		in := ui.TextInput(c.Key(key), v).Grow(1).Height(sizeBtn)
		if in.Submitted() {
			submit = true
		}
		return in
	}
	opts := dlgOpts{title: title, sub: sub, width: 560,
		lead: func() { dbTile(c, d.cfg.Type, 34, ui.Hex(dbColors[d.cfg.Type])) },
		extra: func() {
			if !d.editing && d.cfg.Type != "sqlite" {
				if ui.Button(c, locales.ConnImportURL()).Height(sizeBtnSm).FontSize(font2XS).Clicked() {
					d.urlOpen = !d.urlOpen
				}
			}
		},
	}
	s.dialog(c, &d.open, opts, func() {
		if d.urlOpen {
			ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Children(func() {
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
		ui.Scroll(c.Key("connform")).FillWidth().MaxHeight(430).Children(func() {
			ui.Column(c).Gap(10).FillWidth().Padding(0, 12, 0, 0).Children(func() {
				connField(c, p, locales.ConnName(), func() { field("name", &d.cfg.Name).AutoFocus() })
				if d.editing {
					connField(c, p, locales.ConnType(), func() {
						if ui.Select(c, &d.typeSel, typeLabels()).MinWidth(0).Width(160).Height(sizeBtn).Changed() {
							for _, ty := range dbTypes {
								if ty.label == d.typeSel {
									old := defaultPort(d.cfg.Type)
									d.cfg.Type = ty.id
									if d.port == fmt.Sprint(old) || d.port == "" || d.port == "0" {
										d.port = fmt.Sprint(defaultPort(ty.id))
									}
								}
							}
						}
					})
				}
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
								submit = true
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
									submit = true
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
	}, func() {
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
		if ui.Button(c, locales.CommonCancel()).Height(sizeBtn).Clicked() {
			if d.editing {
				d.open = false
			} else {
				d.view = 0
			}
		}
		if ui.Button(c, locales.CommonSave()).Height(sizeBtn).Clicked() {
			s.saveConnection(false)
		}
		if ui.PrimaryButton(c, locales.ConnConnect()).Height(sizeBtn).Padding(0, 16).Clicked() {
			submit = true
		}
	})
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		s.saveConnection(false)
	}
	if submit {
		s.saveConnection(true)
	}
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
