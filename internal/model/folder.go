package model

// ConnectionFolder 是连接列表里的一个文件夹（只有一级，不嵌套）。
// 连接通过 ConnectionConfig.FolderID 指向它；FolderID 为空的连接属于「未分组」。
type ConnectionFolder struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Order     int    `json:"order"`               // 文件夹之间的顺序，小的在前
	Collapsed bool   `json:"collapsed,omitempty"` // 在列表里是否折叠
}
