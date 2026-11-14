package ui

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 侧栏表行右端的附注：固定的小圆点和行数（设计稿里表名右边那列灰字：64、212、12.4k）。

// sideRowTail 画一行右端：固定项先画小圆点，再画行数。行数是库里的估算值（0 表示没有数据，不画）。
func sideRowTail(c *ui.Context, p tokens, r sideRow) {
	if r.section == "pinned" {
		ui.Text(c, "●").FontSize(font2XS - 3).TextColor(p.fgMuted)
	}
	if r.info.RowCount > 0 {
		ui.Text(c, shortCount(r.info.RowCount)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
	}
}

// shortCount 把行数缩成侧栏里放得下的写法：212、12.4k、1.2M。
func shortCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return itoa(int(n))
}
