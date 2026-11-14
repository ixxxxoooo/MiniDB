package app

import (
	"fmt"
	"sort"
	"strings"

	"minidb/internal/model"
)

// 连接文件夹：只有一级。文件夹记录存在 connection_folders，连接靠 FolderID 指向文件夹，
// 文件夹和连接各自的 Order 决定列表里的先后。移动连接只改存储里的原始记录（密码、密钥保持加密原样，不解密）。

const foldersBucket = "connection_folders"

// sortFolders 给文件夹排序：序号小的在前，序号相同按名称（不分大小写）、ID。
func sortFolders(fs []model.ConnectionFolder) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		if x, y := strings.ToLower(a.Name), strings.ToLower(b.Name); x != y {
			return x < y
		}
		return a.ID < b.ID
	})
}

// SortConnections 按「序号（0 排最后）→ 名称 → ID」排序，列表展示和移动都用这个顺序。
func SortConnections(cs []model.ConnectionConfig) {
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if a.Order != b.Order {
			if a.Order == 0 || b.Order == 0 {
				return a.Order != 0
			}
			return a.Order < b.Order
		}
		if x, y := strings.ToLower(a.Name), strings.ToLower(b.Name); x != y {
			return x < y
		}
		return a.ID < b.ID
	})
}

// ListFolders 返回全部文件夹，按顺序排好。
func (s *ConnectionService) ListFolders() ([]model.ConnectionFolder, error) {
	items, err := s.store.List(foldersBucket, func() interface{} { return &model.ConnectionFolder{} })
	if err != nil {
		return nil, fmt.Errorf("获取文件夹列表失败: %w", err)
	}
	out := make([]model.ConnectionFolder, 0, len(items))
	for _, it := range items {
		out = append(out, *it.(*model.ConnectionFolder))
	}
	sortFolders(out)
	return out, nil
}

// SaveFolder 新建（ID 为空）或重命名文件夹，返回文件夹 ID。名字不能为空；新文件夹排在最后。
func (s *ConnectionService) SaveFolder(f model.ConnectionFolder) (string, error) {
	f.Name = strings.TrimSpace(f.Name)
	if f.Name == "" {
		return "", model.Errf(model.CodeFolderNameEmpty, "文件夹名称不能为空")
	}
	all, err := s.ListFolders()
	if err != nil {
		return "", err
	}
	for _, o := range all {
		if o.ID != f.ID && strings.EqualFold(o.Name, f.Name) {
			return "", model.Errf(model.CodeFolderNameDup, "已有同名文件夹: %s", f.Name)
		}
	}
	if f.ID == "" {
		f.ID = newConnectionID()
		f.Order = 1
		if n := len(all); n > 0 {
			f.Order = all[n-1].Order + 1
		}
	} else {
		var old model.ConnectionFolder
		if err := s.store.Get(foldersBucket, f.ID, &old); err != nil {
			return "", model.Errf(model.CodeFolderNotFound, "文件夹不存在: %s", f.ID)
		}
		f.Order, f.Collapsed = old.Order, old.Collapsed // 重命名不动顺序和折叠状态
	}
	if err := s.store.Put(foldersBucket, f.ID, f); err != nil {
		return "", fmt.Errorf("保存文件夹失败: %w", err)
	}
	return f.ID, nil
}

// SetFolderCollapsed 记住文件夹在列表里是否折叠。
func (s *ConnectionService) SetFolderCollapsed(id string, collapsed bool) error {
	var f model.ConnectionFolder
	if err := s.store.Get(foldersBucket, id, &f); err != nil {
		return model.Errf(model.CodeFolderNotFound, "文件夹不存在: %s", id)
	}
	if f.Collapsed == collapsed {
		return nil
	}
	f.Collapsed = collapsed
	return s.store.Put(foldersBucket, id, f)
}

// DeleteFolder 删除文件夹；里面的连接不删，回到「未分组」。
func (s *ConnectionService) DeleteFolder(id string) error {
	raws, err := s.rawConnections()
	if err != nil {
		return err
	}
	for _, c := range raws {
		if c.FolderID != id {
			continue
		}
		c.FolderID = ""
		if err := s.store.Put("connections", c.ID, c); err != nil {
			return fmt.Errorf("移出文件夹失败: %w", err)
		}
	}
	return s.store.Delete(foldersBucket, id)
}

// rawConnections 读出存储里的原始连接记录（密码和密钥仍是密文），只供改序号、改文件夹时回写。
func (s *ConnectionService) rawConnections() ([]model.ConnectionConfig, error) {
	items, err := s.store.List("connections", func() interface{} { return &model.ConnectionConfig{} })
	if err != nil {
		return nil, fmt.Errorf("获取连接列表失败: %w", err)
	}
	out := make([]model.ConnectionConfig, 0, len(items))
	for _, it := range items {
		out = append(out, *it.(*model.ConnectionConfig))
	}
	return out, nil
}

// PlaceConnection 把连接 connID 放进文件夹 folderID（空为未分组），插在 beforeID 这条连接之前；
// beforeID 为空或不在目标文件夹里就放到末尾。目标文件夹里的序号会整体重排成 1..n。
func (s *ConnectionService) PlaceConnection(connID, folderID, beforeID string) error {
	if folderID != "" {
		var f model.ConnectionFolder
		if err := s.store.Get(foldersBucket, folderID, &f); err != nil {
			return model.Errf(model.CodeFolderNotFound, "文件夹不存在: %s", folderID)
		}
	}
	raws, err := s.rawConnections()
	if err != nil {
		return err
	}
	var moving *model.ConnectionConfig
	for i := range raws {
		if raws[i].ID == connID {
			moving = &raws[i]
		}
	}
	if moving == nil {
		return model.Errf(model.CodeConnNotFound, "连接不存在: %s", connID)
	}
	var target []model.ConnectionConfig // 目标文件夹里（不含被移动的）按现有顺序
	for _, c := range raws {
		if c.FolderID == folderID && c.ID != connID {
			target = append(target, c)
		}
	}
	SortConnections(target)
	ordered := make([]model.ConnectionConfig, 0, len(target)+1)
	placed := false
	for _, c := range target {
		if c.ID == beforeID {
			ordered, placed = append(ordered, *moving), true
		}
		ordered = append(ordered, c)
	}
	if !placed {
		ordered = append(ordered, *moving)
	}
	for i, c := range ordered {
		c.FolderID, c.Order = folderID, i+1
		if err := s.store.Put("connections", c.ID, c); err != nil {
			return fmt.Errorf("调整连接顺序失败: %w", err)
		}
	}
	return nil
}

// nextConnectionOrder 是文件夹（空为未分组）里下一条新连接该用的序号：排在现有的最后。
func (s *ConnectionService) nextConnectionOrder(folderID string) int {
	raws, err := s.rawConnections()
	if err != nil {
		return 0
	}
	n := 0
	for _, c := range raws {
		if c.FolderID == folderID {
			n = max(n, c.Order)
		}
	}
	return n + 1
}
