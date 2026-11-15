package ui

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
)

// @author ixxxxoooo
//
// 侧栏顶部（设计稿 dbrow）：第一行是当前库（库名 + 上下箭头，点它弹出切库浮层）加「新建数据库」「更多」两个图标，
// 非默认安全模式或只读时下面多一行芯片；再下面是随时都在的筛选框。连接本身挪到了标题栏的连接胶囊（titlebar_conn.go）。
// 侧栏收起时它们也一起收起。

// sideHeader 画侧栏顶部的整块：当前库一行、安全模式芯片、筛选框。
func (w *Workspace) sideHeader(c *ui.Context, p tokens) {
	s := w.sh
	ui.Column(c.Key("side-head")).Padding(0, spaceSM+2, 0, spaceSM+2).FillWidth().Shrink(0).Children(func() {
		s.dbHeader(c, p, w)
	})
	w.sideFilter(c)
}

// cardSubtitle 是连接行的第二行：数据库类型（带版本）和地址；本地文件库显示文件名。
func cardSubtitle(w *Workspace) string {
	ver := typeLabel(w.cfg.Type)
	if w.version != "" {
		if strings.HasPrefix(strings.ToLower(w.version), strings.ToLower(ver)) {
			ver = w.version
		} else {
			ver += " " + w.version
		}
	}
	if !app.CapsOf(w.cfg.Type).Network {
		return ver + " · " + filepath.Base(w.cfg.Database)
	}
	return ver + " · " + w.cfg.Host + ":" + itoa(w.cfg.Port)
}

// tagChipKind 是环境标签对应的芯片语义：生产是失败色，测试和预发是警告色，其余是成功色。
func tagChipKind(tag string) chipKind {
	switch tag {
	case "production":
		return chipErr
	case "test", "staging":
		return chipWarn
	}
	return chipOK
}
