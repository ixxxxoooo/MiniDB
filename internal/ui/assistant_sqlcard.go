package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 助手回答里的 SQL 代码块，按设计稿画成一张卡：
// 标题条左边是「SQL · 只读」，右边依次是 复制 / 放进编辑器（淡强调色）/ 运行（主按钮）；下面是带高亮的代码。

// isSQLFence 判断代码块围栏上写的语言是不是 SQL 一类。
func isSQLFence(lang string) bool {
	switch strings.ToLower(lang) {
	case "sql", "mysql", "postgresql", "sqlite":
		return true
	}
	return false
}

// sqlCard 画回答里的一个 SQL 代码块。参数：c 上下文；b 是 markdown 给的代码块。
func (a *assistant) sqlCard(c *ui.Context, b *markdown.CodeBlockView) {
	a.sqlBox(c, fmt.Sprint("sqlcard", b.Index), strings.TrimSpace(b.Code()), b.Open, false, func() { b.Body(c) })
}

// sqlBox 画一张 SQL 卡：标题条 + 带高亮的代码（body 负责画）。回答里的代码块和工具步骤展开后的 SQL 共用它。
// 参数：key 区分同屏的多张卡；code 是要复制 / 放进编辑器 / 运行的 SQL；streaming 为真（还在流式输出）时
// 按钮不画，等代码块收尾再出现，免得对半截 SQL 点了「运行」；compact 是窄的位置（缩进后的工具步骤），「复制」只留图标。
func (a *assistant) sqlBox(c *ui.Context, key, code string, streaming, compact bool, body func()) {
	p := a.w.sh.tokens(c)
	ui.Column(c.Key(key)).FillWidth().Radius(radiusPanel).Border(1, p.border).Background(p.surface).Clip().Children(func() {
		ui.Row(c.Key("sqlcard-head")).Gap(spaceXS).AlignItems(ui.Center).FillWidth().Height(sizeSideSub+2).Padding(0, spaceSM+2).Background(p.surfaceSecondary).Children(func() {
			ui.Text(c, locales.AiSQLReadonlyTitle()).FontSize(fontCaption).TextColor(p.fgSecondary).SingleLine().Shrink(0)
			ui.Spacer(c)
			if streaming {
				return
			}
			if compact {
				if tipBtn(c.Key("cp"), icCopy, locales.AiCopySQL()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					a.w.sh.copyText(code)
				}
			} else if ghostBtn(c.Key("cp"), icCopy, locales.AiCopyShort()).Clicked() {
				a.w.sh.copyText(code)
			}
			if softBtn(c.Key("in"), icCode, locales.AiInsertSQL()).Clicked() {
				a.insertSQL(code, false)
			}
			if actPrimary(c.Key("run"), icPlay, locales.AiRunSQL()).Clicked() {
				a.insertSQL(code, true)
			}
		})
		ui.Box(c.Key("sqlcard-line")).FillWidth().Height(1).Background(p.border)
		ui.Box(c.Key("sqlcard-body")).FillWidth().Padding(spaceSM, spaceSM+2).Children(body)
	})
}
