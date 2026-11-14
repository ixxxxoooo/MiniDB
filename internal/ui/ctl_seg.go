package ui

import (
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 分段开关：应用里所有「几个互斥视图 / 选项」的切换都用它，外观同 pillTabs（设计稿 .seg）。

// segmented 画一个按内容宽度的分段开关：sel 是当前段的下标，labels 是各段文字；
// 点中别的段时改写 *sel 并返回 true。同一个父元素里有多个时，各自的第一段文字要不同（用它区分控件）。
func segmented(c *ui.Context, sel *int, labels ...string) bool {
	if len(labels) == 0 {
		return false
	}
	*sel = max(0, min(*sel, len(labels)-1))
	i := pillTabs(c, baseTokens(c), "seg-"+labels[0], labels, *sel, false)
	if i < 0 || i == *sel {
		return false
	}
	*sel = i
	return true
}
