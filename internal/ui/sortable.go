package ui

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// sortSpec 记录一张列表当前按哪一列排序；col 为空表示保持数据来源的原顺序。
type sortSpec struct {
	col  string
	desc bool
}

// click 处理一次点击表头：换到新列时先按 descFirst 的方向排（数字、耗时这类列先看最大的更有用），
// 再点同一列翻转方向，第三次点击回到原顺序。
func (s *sortSpec) click(col string, descFirst bool) {
	switch {
	case s.col != col:
		s.col, s.desc = col, descFirst
	case s.desc == descFirst:
		s.desc = !descFirst
	default:
		s.col, s.desc = "", false
	}
}

// sortHead 画一个可点击的表头单元，当前排序列后面带升降箭头。width 为 0 时占满剩余宽度。
// 被点中时返回 true，调用方负责 spec.click 之后重新排序。
func sortHead(c *ui.Context, p tokens, key, label string, width float32, spec *sortSpec) bool {
	on := spec.col == key
	head := ui.Row(c.Key("sort-" + key)).Gap(3).AlignItems(ui.Center).Height(26).Shrink(0)
	if width > 0 {
		head.Width(width)
	} else {
		head.Grow(1).MinWidth(0).Shrink(1)
	}
	col := p.fgSecondary
	if on {
		col = p.accent
	} else if head.Hovered() {
		col = p.fg
	}
	tip := label
	if on {
		tip += " · " + map[bool]string{false: locales.SortAsc(), true: locales.SortDesc()}[spec.desc]
	}
	head = tipOn(c, head, tip)
	head.Children(func() {
		ui.Text(c, label).FontSize(font2XS).FontWeight(600).TextColor(col).SingleLine().Shrink(1).MinWidth(0)
		if on {
			ic := icSortUp
			if spec.desc {
				ic = icSortDown
			}
			ui.Icon(c, ic).FontSize(10).TextColor(p.accent).Shrink(0)
		}
	})
	return head.Clicked()
}

// cmpNatural 比较两个字符串：都是整数时按数值，否则不区分大小写按字典序。
func cmpNatural(a, b string) int {
	x, errA := strconv.ParseInt(a, 10, 64)
	y, errB := strconv.ParseInt(b, 10, 64)
	if errA == nil && errB == nil {
		return cmp.Compare(x, y)
	}
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}
