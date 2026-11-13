package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/agent"
	"minidb/locales"
)

// stepRow 画时间线上的一行：推理、工具之间的说明文字，或一次工具调用。
// live 表示这是正在进行的最后一步；inlined 表示这一步的结果表格已经摆在回复里了。
func (a *assistant) stepRow(c *ui.Context, p tokens, m *chatMsg, i int, part agent.Part, live, inlined bool) {
	switch {
	case part.Type == agent.PartText:
		// 模型在调用工具前说的话：作为过程的一部分淡淡地留在时间线上，不和最终回答混在一起
		ui.Text(c, part.Text).FontSize(font2XS-1).TextColor(p.fgSecondary).Selectable().FillWidth().Padding(3, 4)
	case part.Type == agent.PartThinking:
		a.thinkingRow(c, p, m.part(i), part.Text, live)
	case part.Tool != nil:
		a.toolRow(c, p, m, i, inlined)
	}
}

// stepHead 是一步的标题行：图标块、名称、说明、右侧状态。点击切换展开。返回是否被点击。
func stepHead(c *ui.Context, p tokens, ic *ui.SVG, tint ui.Color, label, detail string, mono_ bool, running bool, status func()) bool {
	head := ui.Row(c.Key("head")).Gap(7).AlignItems(ui.Center).Padding(3, 4).FillWidth().Radius(6)
	if head.Hovered() {
		head.Background(p.hover)
	}
	head.Children(func() {
		// 图标块：运行中呼吸；颜色按状态过渡（比如工具失败时由强调色渐变成红色）
		tile := ui.Row(c.Key("tile")).Size(18, 18).Shrink(0).Center().Radius(5)
		to := ui.Animation{Duration: 250 * time.Millisecond}
		bg, fg := tile.AnimateColor("bg", tint.Alpha(0.13), to), tile.AnimateColor("fg", tint, to)
		breathe(tile.Background(bg), "run", running).Children(func() {
			ui.Icon(c, ic).FontSize(11).TextColor(fg)
		})
		ui.Text(c, label).FontSize(font2XS).FontWeight(500).Shrink(0)
		tx := ui.Text(c, detail).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().Grow(1).TruncateMiddle()
		if mono_ {
			tx.Font(mono)
		}
		status()
	})
	return head.Clicked()
}

// thinkingRow 是一段推理：正在想的那一步实时展开，想完自动收起，之后可以手动展开回看。
func (a *assistant) thinkingRow(c *ui.Context, p tokens, v *partView, text string, live bool) {
	open := v.open == 1 || (v.open == 0 && live)
	ui.Column(c).Gap(3).FillWidth().Children(func() {
		detail := firstLines(text, 1)
		if open {
			detail = "" // 展开时正文就在下面，标题行不再重复
		}
		if stepHead(c, p, icBulb, p.warning, locales.AiThought(), detail, false, live, func() {
			if live {
				ui.Spinner(c).Size(10, 10)
			}
		}) {
			v.open = map[bool]int{true: 2, false: 1}[open]
		}
		if open {
			ui.Column(c.Key("body")).FillWidth().Padding(0, 0, 4, 25).Transition(toggleTransition).Children(func() {
				ui.Text(c, text).FontSize(font2XS - 1).TextColor(p.fgSecondary).Selectable().FillWidth()
			})
		}
	})
}

// toolRow 是一次工具调用：标题行显示名称、对象与状态；展开后依次是参数、SQL、结果。
// 失败的调用自动展开，错误原因和建议不用点开就能看到。
func (a *assistant) toolRow(c *ui.Context, p tokens, m *chatMsg, i int, inlined bool) {
	tool, v := m.tr.Parts[i].Tool, m.part(i)
	res := tool.Result
	sql, _ := tool.Args["sql"].(string)
	if res != nil && res.SQL != "" {
		sql = res.SQL
	}
	open := v.open == 1 || (v.open == 0 && tool.Failed())
	target := toolTarget(tool.Args)
	if target == "" && sql != "" {
		target = oneLine(sql)
	}
	tint := p.accent
	if tool.Failed() {
		tint = p.danger
	}
	ui.Column(c).Gap(3).FillWidth().Children(func() {
		clicked := stepHead(c, p, toolIcon(tool.Name), tint, toolLabel(tool.Name), target, true, tool.Running(), func() {
			switch {
			case tool.Running():
				ui.Spinner(c).Size(10, 10)
			case tool.Failed():
				ui.Text(c, locales.AiToolFailed()).FontSize(10).TextColor(p.danger)
			default:
				ui.Text(c, toolStatus(res)).FontSize(10).TextColor(p.fgMuted)
			}
		})
		if clicked {
			v.open = map[bool]int{true: 2, false: 1}[open]
		}
		if !open {
			return
		}
		ui.Column(c.Key("detail")).Gap(6).FillWidth().Padding(0, 0, 4, 25).Transition(toggleTransition).Children(func() {
			if lines := argLines(tool.Args); len(lines) > 0 {
				ui.Text(c, joinLines(lines)).FontSize(font2XS - 1).Font(mono).TextColor(p.fgMuted).Selectable().FillWidth()
			}
			if sql != "" && !inlined { // 结果卡里已有 SQL 与按钮
				ui.Text(c, sql).FontSize(font2XS-1).Font(mono).Selectable().Padding(6, 8).Radius(6).Background(p.surfaceSecondary).FillWidth()
				ui.Row(c).Gap(4).Children(func() { a.sqlActions(c, "st", sql, false) })
			}
			a.toolResultBody(c, p, tool, v, inlined)
		})
	})
}

// toolResultBody 画工具的结果：失败是原因加建议；成功是表格、补充说明或文本。
func (a *assistant) toolResultBody(c *ui.Context, p tokens, tool *agent.ToolPart, v *partView, inlined bool) {
	res := tool.Result
	if res == nil {
		return
	}
	switch {
	case !res.OK:
		ui.Column(c).Gap(3).FillWidth().Padding(6, 8).Radius(6).Background(p.danger.Alpha(0.08)).Children(func() {
			ui.Text(c, headLines(res.Text, 12)).FontSize(font2XS - 1).TextColor(p.danger).Selectable().FillWidth()
			if res.Hint != "" {
				ui.Text(c, locales.AiToolHint(res.Hint)).FontSize(font2XS - 1).TextColor(p.fgSecondary).Selectable().FillWidth()
			}
		})
	case v.grid != nil && !inlined: // 内联时表格已经摆在回复里了，这里不重复画
		a.gridBox(c, p, tool, v)
	case v.grid == nil && res.Text != "":
		ui.Text(c, headLines(res.Text, 40)).FontSize(font2XS-1).Font(mono).Selectable().Padding(6, 8).Radius(6).Background(p.surfaceSecondary).FillWidth()
	}
	if res.OK && res.Notes != "" {
		ui.Text(c, headLines(res.Notes, 40)).FontSize(font2XS - 1).Font(mono).TextColor(p.fgSecondary).Selectable().FillWidth()
	}
}

// toolStatus 是成功的工具在标题行右侧的说明：摘要与耗时，如 "12 张表 · 4 ms"。
func toolStatus(res *agent.ToolResultEvent) string {
	switch {
	case res == nil:
		return ""
	case res.Summary != "" && res.DurationMs > 0:
		return res.Summary + " · " + locales.AiUsage(int(res.DurationMs))
	case res.Summary != "":
		return res.Summary
	case res.DurationMs > 0:
		return locales.AiUsage(int(res.DurationMs))
	}
	return ""
}

// joinLines 用换行把几行拼起来。
func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
