package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
)

// @author ixxxxoooo
//
// 数据网格单元格里的值怎么画（设计稿 A 的 .grid td）：
//   - 普通文字：界面字体 12.5、主文字色；
//   - 数字、日期时间：等宽 11.5，数字用信息蓝、时间用语法字符串绿；
//   - 主键列：等宽 600 加粗、主文字色；
//   - 枚举（enum）：前面一个状态色小圆点；
//   - NULL：斜体、最淡的一档灰。

// valueKind 是单元格值的画法。
type valueKind int

const (
	valText valueKind = iota
	valNumber
	valTime
	valPK
	valEnum
)

// kindOf 判断某一列的值该怎么画；isPK 为真时一律按主键画。
func kindOf(col model.ColumnMeta, isPK bool) valueKind {
	t := strings.ToLower(col.Type)
	switch {
	case isPK:
		return valPK
	case strings.HasPrefix(t, "enum"):
		return valEnum
	case isNumericType(col.Type):
		return valNumber
	case strings.Contains(t, "date"), strings.Contains(t, "time"), strings.Contains(t, "year"):
		return valTime
	}
	return valText
}

// enumColor 是枚举值前面小圆点的颜色：成功绿、等待橙、进行蓝、失败红，认不出的用最淡的灰。
func enumColor(p tokens, v string) ui.Color {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "paid", "active", "success", "succeeded", "ok", "done", "completed", "enabled", "published", "delivered", "online":
		return p.success
	case "pending", "draft", "processing", "waiting", "review", "reviewing", "unpaid":
		return p.warning
	case "shipped", "sent", "open", "running", "in_progress", "shipping":
		return p.info
	case "refunded", "failed", "error", "cancelled", "canceled", "rejected", "banned", "disabled", "offline", "deleted":
		return p.danger
	}
	return p.fgFaint
}

// valueText 在单元格里画一个值（调用方已经套好了 cellBox）。null 时画 NULL。
func valueText(c *ui.Context, p tokens, kind valueKind, text string, null bool) {
	if null {
		t := ui.Text(c, text).FontSize(fontCtl).FixedLineHeight(14).TextColor(nullColor(p)).SingleLine()
		if nullStyled {
			t.Italic()
		}
		return
	}
	text = oneLine(text)
	switch kind {
	case valNumber:
		ui.Text(c, text).Font(mono).FontSize(fontCaption + 0.5).FixedLineHeight(14).TextColor(p.valNum).SingleLine()
	case valTime:
		ui.Text(c, text).Font(mono).FontSize(fontCaption + 0.5).FixedLineHeight(14).TextColor(p.valTime).SingleLine()
	case valPK:
		ui.Text(c, text).Font(mono).FontSize(fontCtl).FontWeight(600).FixedLineHeight(14).TextColor(p.fg).SingleLine()
	case valEnum:
		ui.Row(c).Gap(spaceSM - 3).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(sizeDot, sizeDot).Radius(sizeDot / 2).Background(enumColor(p, text)).Shrink(0)
			ui.Text(c, text).FontSize(fontCtl).FixedLineHeight(14).TextColor(p.fg).SingleLine()
		})
	default:
		ui.Text(c, text).FontSize(fontCtl).FixedLineHeight(14).TextColor(p.fg).SingleLine()
	}
}

// pkOf 报告某一列是不是主键（表数据页由 tableTab 提供，查询结果没有主键信息）。
func (g *resultGrid) pkOf(name string) bool { return g.pk != nil && g.pk(name) }

// inferNumeric 给没有类型信息的列（查询结果）按值猜：前 50 个非空值全是数字，就当数值列
// （右对齐、蓝色等宽），和设计稿里查询结果的样子一致。
func (g *resultGrid) inferNumeric(cols []model.ColumnMeta, rows []map[string]any) {
	g.numLike = map[string]bool{}
	for _, c := range cols {
		if c.Type != "" {
			continue
		}
		seen, ok := 0, true
		for _, r := range rows {
			if seen >= 50 || !ok {
				break
			}
			switch r[c.Name].(type) {
			case nil:
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
				seen++
			default:
				ok = false
			}
		}
		g.numLike[c.Name] = ok && seen > 0
	}
}

// numCol 报告某一列是不是数值列：类型是数值，或者没有类型但值全是数字。
func (g *resultGrid) numCol(col model.ColumnMeta) bool {
	return isNumericType(col.Type) || g.numLike[col.Name]
}

// kindOf 同包级的 kindOf，再加上按值猜出来的数值列。
func (g *resultGrid) kindOf(col model.ColumnMeta, isPK bool) valueKind {
	if k := kindOf(col, isPK); k != valText || !g.numLike[col.Name] {
		return k
	}
	return valNumber
}
