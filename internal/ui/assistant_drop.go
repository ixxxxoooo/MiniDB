package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 把表拖进 AI 助手的输入框（设计稿「AI 助手 · 拖拽引用」）：
// 侧栏的表 / 视图行、顶部的表 / 视图标签都能拖；拖进输入框范围时它变成虚线强调框并盖一层「松手引用 xxx」，
// 松手后在文字末尾补一个 @表名，输入框上方就多一枚表芯片——和敲 @ 引用走的是同一条路（引用就是文字里的 @名字）。

// tableDrag 是被拖的一张表或视图。
type tableDrag struct{ name string }

// dragTable 让一个元素（侧栏的表行）成为表的拖拽源。参数：el 是行元素，name 是表名。
func dragTable(el ui.Element, name string) { el.Drag(tableDrag{name}) }

// dragTab 让表 / 视图标签成为表的拖拽源；查询、概览等没有对应表的标签不能拖。参数：el 是标签元素，tb 是标签。
func dragTab(el ui.Element, tb tab) {
	if t, ok := tb.(*tableTab); ok && !t.isNew && t.table != "" {
		dragTable(el, t.table)
	}
}

// dropStyle 在输入框容器 card 上处理拖入：被拖的表在范围内时画成虚线强调框，松手时记下表名等 dropApply 落地。
// 返回正拖在上面的表名（没有拖入时为空），overlay 用它。
func (a *assistant) dropStyle(p tokens, card ui.Element) string {
	if d, ok := ui.Drop[tableDrag](card); ok {
		a.mention.dropped = d.name
	}
	d, over := ui.DragOver[tableDrag](card)
	if !over {
		return ""
	}
	card.BorderStyle(ui.BorderDashed).BorderColor(p.accent).Background(p.accentSoft)
	return d.name
}

// dropOverlay 在输入框上盖一层「松手引用 xxx」；name 为空（没有拖入）时什么都不画。
func (a *assistant) dropOverlay(c *ui.Context, p tokens, name string) {
	if name == "" {
		return
	}
	ui.Row(c.Key("ai-drop")).Absolute().Left(0).Top(0).Right(0).Bottom(0).Center().Gap(spaceSM).Radius(radiusDialog).Background(p.accentSoft).Transition(popTransition).Children(func() {
		ui.Icon(c, icTable).FontSize(btnFieldIcon).TextColor(p.accentText)
		ui.Text(c, locales.AiDropHint(name)).FontSize(fontBody).FontWeight(600).TextColor(p.accentText).SingleLine()
	})
}

// dropApply 把上一帧松手的表补成文字末尾的 @表名；已经引用过的不重复加。返回是否改了文字。参数：in 是输入框。
func (a *assistant) dropApply(in ui.Element) bool {
	name := a.mention.dropped
	if name == "" {
		return false
	}
	a.mention.dropped = ""
	for _, n := range app.MentionNames(a.input) {
		if strings.EqualFold(n, name) {
			in.Focus()
			return false
		}
	}
	text := []rune(a.input)
	add := "@" + name + " "
	if len(text) > 0 && text[len(text)-1] != ' ' && text[len(text)-1] != '\n' {
		add = " " + add
	}
	in.ReplaceText(len(text), len(text), add)
	in.Focus()
	return true
}
