package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 生产环境的顶部色带：连接的环境标签是 production 时，窗口最上面压一条 3px 的红带，
// 不管在哪个标签、哪个页面都看得见，防止在线上库里误操作。连接颜色只用在头像和这条色带上。

// envBand 画窗口顶部的环境色带；当前连接不是生产环境时什么也不画。
func (s *Shell) envBand(c *ui.Context, p tokens) {
	w := s.activeWorkspace()
	if w == nil || w.cfg.Tag != "production" {
		return
	}
	ui.Box(c.Key("env-band")).Height(envBandH).FillWidth().Shrink(0).Background(p.danger)
}
