package ui

import (
	"testing"

	"minidb/internal/model"
)

// @author ixxxxoooo

// TestConnGroupsAndCollapse 连接按文件夹分组：文件夹在前、未分组在后；
// 折叠的文件夹里的连接不进可见列表，搜索时一律展开且只留命中的组。
func TestConnGroupsAndCollapse(t *testing.T) {
	e := newEnv(t)
	s := e.sh
	s.conns = []model.ConnectionConfig{
		{ID: "a", Name: "alpha", FolderID: "f1"},
		{ID: "b", Name: "beta", FolderID: "f2"},
		{ID: "c", Name: "gamma"},
		{ID: "d", Name: "delta", FolderID: "gone"}, // 文件夹不存在：落到未分组
	}
	s.folders = []model.ConnectionFolder{{ID: "f1", Name: "生产"}, {ID: "f2", Name: "测试", Collapsed: true}}

	gs := s.connGroups()
	if len(gs) != 3 || gs[0].folder.ID != "f1" || gs[1].folder.ID != "f2" || gs[2].folder != nil {
		t.Fatalf("分组不对: %+v", gs)
	}
	if len(gs[2].conns) != 2 || gs[2].conns[0].ID != "c" || gs[2].conns[1].ID != "d" {
		t.Fatalf("未分组应含无文件夹与文件夹已失效的连接: %+v", gs[2].conns)
	}
	var ids []string
	for _, cfg := range s.filteredConns() {
		ids = append(ids, cfg.ID)
	}
	if len(ids) != 3 || ids[0] != "a" || ids[1] != "c" || ids[2] != "d" {
		t.Fatalf("折叠的文件夹里的连接不该可见: %v", ids)
	}

	s.dlg.search = "beta"
	gs = s.connGroups()
	if len(gs) != 1 || gs[0].folder.ID != "f2" || !s.groupOpen(gs[0]) {
		t.Fatalf("搜索应只留命中的组并展开: %+v", gs)
	}
	if got := s.filteredConns(); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("搜索结果不对: %+v", got)
	}
}

// TestConnGroupsWithoutFolders 没有文件夹时只有一个未分组，行为与改造前一致。
func TestConnGroupsWithoutFolders(t *testing.T) {
	e := newEnv(t)
	s := e.sh
	s.conns = []model.ConnectionConfig{{ID: "a", Name: "alpha"}}
	s.folders = nil
	gs := s.connGroups()
	if len(gs) != 1 || gs[0].folder != nil || len(gs[0].conns) != 1 {
		t.Fatalf("分组不对: %+v", gs)
	}
}

// TestConnListShowsFolderHeaders 列表页画出文件夹标题，折叠后组内连接消失。
func TestConnListShowsFolderHeaders(t *testing.T) {
	e := newEnv(t)
	s := e.sh
	s.conns = []model.ConnectionConfig{{ID: "a", Name: "alpha-conn", FolderID: "f1"}, {ID: "c", Name: "gamma-conn"}}
	s.folders = []model.ConnectionFolder{{ID: "f1", Name: "生产环境"}}
	s.newConnectionDialog()
	e.tt.Frame()
	e.tt.Frame()
	e.has("生产环境")
	e.has("未分组")
	e.has("alpha-conn")
	e.click("生产环境")
	e.hasNot("alpha-conn")
	e.has("gamma-conn")
}
