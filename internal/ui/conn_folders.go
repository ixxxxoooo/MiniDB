package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// 连接管理器列表页的文件夹：把连接按文件夹分组，文件夹可折叠、重命名、删除；
// 连接拖到文件夹标题上就移进去，拖到另一条连接上就插到它前面（同时进它所在的文件夹）。

// connDrag 是拖动中的一条连接。
type connDrag struct{ id string }

// connGroup 是列表里的一组：一个文件夹（folder 不为空），或没有进任何文件夹的「未分组」。
type connGroup struct {
	folder *model.ConnectionFolder
	conns  []model.ConnectionConfig
}

// loadConnections 重新读取连接和文件夹，读完后（只一次）恢复上次的会话。
func (s *Shell) loadConnections() {
	var conns []model.ConnectionConfig
	var folders []model.ConnectionFolder
	var err error
	s.up.bg(func() {
		conns, err = s.app.Connections.GetConnections()
		if err == nil {
			folders, err = s.app.Connections.ListFolders()
		}
	}, func() {
		s.connsReady = true
		if err != nil {
			s.connsErr = errMessage(err)
			return
		}
		s.connsErr = ""
		s.conns, s.folders = conns, folders
		s.restoreSession()
	})
}

// connMatches 判断连接是否命中搜索词（名称、主机、库名、环境）。
func connMatches(cfg model.ConnectionConfig, q string) bool {
	return q == "" || strings.Contains(strings.ToLower(cfg.Name+" "+cfg.Host+" "+cfg.Database+" "+cfg.Tag), q)
}

// connGroups 按文件夹分组，文件夹在前、「未分组」在最后；搜索时只留有命中连接的组。
// 没有任何文件夹时只有一个没有标题的「未分组」。
func (s *Shell) connGroups() []connGroup {
	q := strings.ToLower(strings.TrimSpace(s.dlg.search))
	known := map[string]int{}
	groups := make([]connGroup, 0, len(s.folders)+1)
	for i := range s.folders {
		known[s.folders[i].ID] = len(groups)
		groups = append(groups, connGroup{folder: &s.folders[i]})
	}
	rest := connGroup{}
	for _, cfg := range s.conns {
		if !connMatches(cfg, q) {
			continue
		}
		if gi, ok := known[cfg.FolderID]; ok && cfg.FolderID != "" {
			groups[gi].conns = append(groups[gi].conns, cfg)
		} else {
			rest.conns = append(rest.conns, cfg)
		}
	}
	out := groups[:0]
	for _, g := range groups {
		if q != "" && len(g.conns) == 0 {
			continue
		}
		out = append(out, g)
	}
	if len(rest.conns) > 0 || (len(s.folders) > 0 && q == "") {
		out = append(out, rest)
	}
	return out
}

// groupOpen 报告这一组的连接是不是展开着：搜索时一律展开，免得命中的连接藏在折叠的文件夹里。
func (s *Shell) groupOpen(g connGroup) bool {
	return g.folder == nil || !g.folder.Collapsed || strings.TrimSpace(s.dlg.search) != ""
}

// filteredConns 是列表里当前看得到的连接（按显示顺序，折叠的文件夹里的不算），光标和回车都在它上面走。
func (s *Shell) filteredConns() []model.ConnectionConfig {
	var shown []model.ConnectionConfig
	for _, g := range s.connGroups() {
		if s.groupOpen(g) {
			shown = append(shown, g.conns...)
		}
	}
	return shown
}

// connGroupsView 画全部分组：标题（有文件夹时）加上组里的连接行。cur 是光标在可见连接里的下标。
func (s *Shell) connGroupsView(c *ui.Context, p tokens, cur int) {
	idx := 0
	hasFolders := len(s.folders) > 0
	for _, g := range s.connGroups() {
		if hasFolders {
			s.folderHeader(c, p, g)
		}
		if !s.groupOpen(g) {
			continue
		}
		for _, cfg := range g.conns {
			s.connRow(c, p, cfg, idx == cur)
			idx++
		}
	}
}

// folderHeader 是一组的标题行：折叠箭头、名字、连接数；点一下折叠 / 展开，文件夹还有右键菜单。
// 把连接拖到标题上就移进这个文件夹（「未分组」的标题则是移出文件夹）。
func (s *Shell) folderHeader(c *ui.Context, p tokens, g connGroup) {
	id, name, collapsed := "", locales.ConnUngrouped(), false
	if g.folder != nil {
		id, name, collapsed = g.folder.ID, g.folder.Name, g.folder.Collapsed && strings.TrimSpace(s.dlg.search) == ""
	}
	row := ui.Row(c.Key("fh-"+id)).Gap(6).Padding(5, 6, 5, 6).Radius(radiusPanel).AlignItems(ui.Center).FillWidth()
	if _, over := ui.DragOver[connDrag](row); over {
		row.Background(p.accent.Alpha(0.16))
	} else if row.Hovered() {
		row.Background(p.hover)
	}
	if d, ok := ui.Drop[connDrag](row); ok {
		defer s.placeConn(d.id, id, "")
	}
	row.Children(func() {
		chev := icChevronD
		if collapsed {
			chev = icChevronR
		}
		ui.Icon(c, chev).FontSize(12).TextColor(p.fgMuted)
		if g.folder != nil {
			ui.Icon(c, icFolder).FontSize(13).TextColor(p.fgSecondary)
		}
		ui.Text(c, name).FontSize(font2XS + 1).FontWeight(600).TextColor(p.fgSecondary).SingleLine()
		ui.Text(c, itoa(len(g.conns))).FontSize(font2XS - 1).TextColor(p.fgMuted)
	})
	if row.Clicked() && g.folder != nil {
		defer s.toggleFolder(*g.folder)
	}
	if g.folder == nil {
		return
	}
	f := *g.folder
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.ConnFolderRename()).Chosen() {
			defer s.renameFolder(f)
		}
		m.Separator()
		if m.Item(locales.ConnFolderDelete()).Chosen() {
			defer s.deleteFolder(f)
		}
	})
}

// toggleFolder 折叠 / 展开文件夹并记住。
func (s *Shell) toggleFolder(f model.ConnectionFolder) {
	for i := range s.folders {
		if s.folders[i].ID == f.ID {
			s.folders[i].Collapsed = !f.Collapsed
		}
	}
	collapsed := !f.Collapsed
	var err error
	s.up.bg(func() { err = s.app.Connections.SetFolderCollapsed(f.ID, collapsed) }, func() {
		if err != nil {
			s.toastErr(errMessage(err))
		}
	})
}

// placeConn 把连接放进文件夹 folderID（空为未分组），插在 before 之前；完成后重新读取列表。
func (s *Shell) placeConn(id, folderID, before string) {
	if id == before {
		return
	}
	var err error
	s.up.bg(func() { err = s.app.Connections.PlaceConnection(id, folderID, before) }, func() {
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		s.loadConnections()
	})
}

// newFolder 问名字后新建文件夹。
func (s *Shell) newFolder() {
	s.prompt(promptDialog{
		title: locales.ConnFolderNew(), label: locales.ConnFolderName(), ok: locales.CommonSave(),
		action: func(name string, _ bool, _ string) { s.saveFolder(model.ConnectionFolder{Name: name}) },
	})
}

// renameFolder 改文件夹的名字。
func (s *Shell) renameFolder(f model.ConnectionFolder) {
	s.prompt(promptDialog{
		title: locales.ConnFolderRename(), label: locales.ConnFolderName(), value: f.Name, ok: locales.CommonSave(),
		action: func(name string, _ bool, _ string) { f.Name = name; s.saveFolder(f) },
	})
}

func (s *Shell) saveFolder(f model.ConnectionFolder) {
	var err error
	s.up.bg(func() { _, err = s.app.Connections.SaveFolder(f) }, func() {
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		s.loadConnections()
	})
}

// deleteFolder 确认后删除文件夹；里面的连接回到「未分组」。
func (s *Shell) deleteFolder(f model.ConnectionFolder) {
	s.askDanger(locales.ConnFolderDelete(), locales.ConnFolderDeleteMsg(f.Name), locales.CommonDelete(), func() {
		var err error
		s.up.bg(func() { err = s.app.Connections.DeleteFolder(f.ID) }, func() {
			if err != nil {
				s.toastErr(errMessage(err))
				return
			}
			s.loadConnections()
		})
	})
}

// moveToMenu 是连接右键菜单里的「移动到」：未分组和每个文件夹各一项，当前所在的置灰。
func (s *Shell) moveToMenu(m *ui.Menu, cfg model.ConnectionConfig) {
	if len(s.folders) == 0 {
		return
	}
	m.Submenu(locales.ConnMoveTo(), func(m *ui.Menu) {
		if m.Item(locales.ConnUngrouped()).Disabled(cfg.FolderID == "").Chosen() {
			defer s.placeConn(cfg.ID, "", "")
		}
		for _, f := range s.folders {
			if m.Item(f.Name).Disabled(cfg.FolderID == f.ID).Chosen() {
				defer s.placeConn(cfg.ID, f.ID, "")
			}
		}
	})
}

// folderChoices 是表单里「文件夹」下拉的选项：未分组在最前，后面是各文件夹的名字。
func (d *connDialog) folderChoices() []string {
	out := []string{locales.ConnUngrouped()}
	for _, f := range d.folders {
		out = append(out, f.Name)
	}
	return out
}

// folderLabel 是文件夹 ID 对应的下拉显示名；找不到按未分组。
func (d *connDialog) folderLabel(id string) string {
	for _, f := range d.folders {
		if f.ID == id {
			return f.Name
		}
	}
	return locales.ConnUngrouped()
}

// folderID 是下拉当前选中的文件夹 ID；选的是未分组（或名字对不上）为空。
func (d *connDialog) folderID() string {
	for _, f := range d.folders {
		if f.Name == d.folderSel {
			return f.ID
		}
	}
	return ""
}

// connFolderField 画表单里的「文件夹」下拉；还没有文件夹时不显示。
func (s *Shell) connFolderField(c *ui.Context, p tokens) {
	d := &s.dlg
	if len(d.folders) == 0 {
		return
	}
	connField(c, p, locales.ConnFolder(), func() {
		ui.Select(c.Key("conn-folder"), &d.folderSel, d.folderChoices()).Width(200).Height(sizeBtn)
	})
}
