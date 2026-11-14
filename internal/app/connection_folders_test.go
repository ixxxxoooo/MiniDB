package app

import (
	"encoding/json"
	"testing"

	"minidb/internal/model"
)

// saveTestConn 保存一条最小的 SQLite 连接，返回它的 ID。
func saveTestConn(t *testing.T, s *ConnectionService, name, folder string) string {
	t.Helper()
	id := newConnectionID()
	cfg := model.ConnectionConfig{ID: id, Name: name, Type: model.SQLite, Database: ":memory:", FolderID: folder}
	if err := s.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	return id
}

func connNames(t *testing.T, s *ConnectionService, folder string) []string {
	t.Helper()
	all, err := s.GetConnections()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range all {
		if c.FolderID == folder {
			out = append(out, c.Name)
		}
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFolderCRUD(t *testing.T) {
	s := newTestConnectionService(t)
	if _, err := s.SaveFolder(model.ConnectionFolder{Name: "  "}); model.CodeOf(err) != model.CodeFolderNameEmpty {
		t.Fatalf("空名字应报错，实际 %v", err)
	}
	a, _ := s.SaveFolder(model.ConnectionFolder{Name: "生产"})
	b, _ := s.SaveFolder(model.ConnectionFolder{Name: "测试"})
	list, _ := s.ListFolders()
	if len(list) != 2 || list[0].ID != a || list[1].ID != b {
		t.Fatalf("新文件夹应按创建顺序排列: %+v", list)
	}
	if _, err := s.SaveFolder(model.ConnectionFolder{ID: a, Name: "线上"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFolderCollapsed(a, true); err != nil {
		t.Fatal(err)
	}
	// 重命名不能丢掉折叠状态和顺序
	if _, err := s.SaveFolder(model.ConnectionFolder{ID: a, Name: "线上2"}); err != nil {
		t.Fatal(err)
	}
	list, _ = s.ListFolders()
	if list[0].Name != "线上2" || !list[0].Collapsed || list[0].ID != a {
		t.Fatalf("重命名后状态不对: %+v", list[0])
	}
	if _, err := s.SaveFolder(model.ConnectionFolder{ID: "nope", Name: "x"}); model.CodeOf(err) != model.CodeFolderNotFound {
		t.Fatalf("不存在的文件夹应报错，实际 %v", err)
	}
}

func TestPlaceConnectionOrdersWithinFolder(t *testing.T) {
	s := newTestConnectionService(t)
	f, _ := s.SaveFolder(model.ConnectionFolder{Name: "F"})
	x := saveTestConn(t, s, "x", "")
	y := saveTestConn(t, s, "y", "")
	z := saveTestConn(t, s, "z", "")
	// 新连接排在末尾，而不是按名字
	if got := connNames(t, s, ""); !equalStrings(got, []string{"x", "y", "z"}) {
		t.Fatalf("初始顺序 = %v", got)
	}
	if err := s.PlaceConnection(z, "", x); err != nil { // z 插到 x 前
		t.Fatal(err)
	}
	if got := connNames(t, s, ""); !equalStrings(got, []string{"z", "x", "y"}) {
		t.Fatalf("重排后 = %v", got)
	}
	if err := s.PlaceConnection(y, f, ""); err != nil { // y 进文件夹末尾
		t.Fatal(err)
	}
	if got := connNames(t, s, f); !equalStrings(got, []string{"y"}) {
		t.Fatalf("文件夹里 = %v", got)
	}
	if got := connNames(t, s, ""); !equalStrings(got, []string{"z", "x"}) {
		t.Fatalf("未分组里 = %v", got)
	}
	if err := s.PlaceConnection(x, "不存在", ""); model.CodeOf(err) != model.CodeFolderNotFound {
		t.Fatalf("目标文件夹不存在应报错，实际 %v", err)
	}
	if err := s.PlaceConnection("nope", "", ""); model.CodeOf(err) != model.CodeConnNotFound {
		t.Fatalf("连接不存在应报错，实际 %v", err)
	}
}

func TestMovingKeepsSecretsEncrypted(t *testing.T) {
	s := newTestConnectionService(t)
	f, _ := s.SaveFolder(model.ConnectionFolder{Name: "F"})
	id := newConnectionID()
	cfg := model.ConnectionConfig{ID: id, Name: "pg", Type: model.Postgres, Host: "h", Port: 5432, User: "u", Password: "secret-pw"}
	if err := s.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	if err := s.PlaceConnection(id, f, ""); err != nil {
		t.Fatal(err)
	}
	all, err := s.loadConnectionsWithSecrets()
	if err != nil || len(all) != 1 {
		t.Fatalf("读取失败: %v %d", err, len(all))
	}
	if all[0].Password != "secret-pw" || all[0].FolderID != f {
		t.Fatalf("移动后密码或文件夹不对: %+v", all[0])
	}
}

func TestDeleteFolderUngroupsConnections(t *testing.T) {
	s := newTestConnectionService(t)
	f, _ := s.SaveFolder(model.ConnectionFolder{Name: "F"})
	saveTestConn(t, s, "a", f)
	if err := s.DeleteFolder(f); err != nil {
		t.Fatal(err)
	}
	if got := connNames(t, s, ""); !equalStrings(got, []string{"a"}) {
		t.Fatalf("删除文件夹后连接应回到未分组: %v", got)
	}
	if list, _ := s.ListFolders(); len(list) != 0 {
		t.Fatalf("文件夹没删掉: %+v", list)
	}
}

func TestImportFoldersAndV1Backup(t *testing.T) {
	s := newTestConnectionService(t)
	local, _ := s.SaveFolder(model.ConnectionFolder{Name: "本机"})
	known, err := s.importFolders([]model.ConnectionFolder{
		{ID: local, Name: "备份里的名字"}, // 同 ID：保留本机的
		{ID: "new1", Name: " 新文件夹 "},
		{ID: "", Name: "没有 ID"},
		{ID: "blank", Name: " "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !known[local] || !known["new1"] || known["blank"] || len(known) != 2 {
		t.Fatalf("导入后的文件夹集合不对: %v", known)
	}
	list, _ := s.ListFolders()
	if len(list) != 2 || list[0].Name != "本机" || list[1].Name != "新文件夹" {
		t.Fatalf("导入结果: %+v", list)
	}
	// 旧版（v1）备份没有 folders 字段，照常解析
	var b ConnectionsBackup
	if err := json.Unmarshal([]byte(`{"version":1,"connections":[{"id":"c1","name":"n"}]}`), &b); err != nil || len(b.Folders) != 0 || len(b.Connections) != 1 {
		t.Fatalf("v1 备份解析失败: %v %+v", err, b)
	}
}

func TestSaveFolderRejectsDuplicateName(t *testing.T) {
	s := newTestConnectionService(t)
	id, err := s.SaveFolder(model.ConnectionFolder{Name: "生产"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveFolder(model.ConnectionFolder{Name: " 生产 "}); err == nil {
		t.Fatal("同名文件夹应被拒绝")
	}
	// 改自己的名字（含只改大小写）不算重名
	if _, err := s.SaveFolder(model.ConnectionFolder{ID: id, Name: "生产"}); err != nil {
		t.Fatalf("保存自己不该报重名: %v", err)
	}
}
