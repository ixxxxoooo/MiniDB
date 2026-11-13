package ui

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/agent"
	"minidb/locales"
)

// stepHead 是一步的标题行：图标块、名称、说明、右侧状态，最右是展开箭头（悬停或展开时才显出来）。
// 点击切换展开。返回是否被点击。
func stepHead(c *ui.Context, p tokens, ic *ui.SVG, tint ui.Color, label, detail string, mono_ bool, running, open bool, status func()) bool {
	head := ui.Row(c.Key("head")).Gap(7).AlignItems(ui.Center).Padding(3, 4).FillWidth().Radius(6)
	hover := head.Hovered()
	if hover {
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
		breathe(ui.Text(c, label).FontSize(font2XS).FontWeight(500).Shrink(0), "lbl", running)
		tx := ui.Text(c, detail).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().Grow(1).TruncateMiddle()
		if mono_ {
			tx.Font(mono)
		}
		status()
		// 展开箭头：平时隐着，悬停或展开时淡入；展开时转 90°
		chev := ui.Icon(c.Key("chev"), icChevronR).FontSize(10).TextColor(p.fgMuted).Shrink(0)
		chev.Opacity(chev.Animate("o", map[bool]float32{true: 1, false: 0}[hover || open], motionToggle)).
			Rotate(chev.Animate("r", map[bool]float32{true: 90, false: 0}[open], motionToggle))
	})
	return head.Clicked()
}

// thinkSeconds 把推理耗时写成整秒，不足一秒算 1。
func thinkSeconds(ms int64) int { return int(max((ms+500)/1000, 1)) }

// thinkingRow 是一段推理：正在想时展开，标题是「思考中 · N 秒」并实时流出文字；
// 想完自动收成一行「思考了 N 秒」，可以点开回看。正文在一条左边线后面，颜色淡一档，和回答区分开。
func (a *assistant) thinkingRow(c *ui.Context, p tokens, v *partView, part agent.Part, live bool) {
	open := v.open == 1 || (v.open == 0 && live)
	if live && v.since.IsZero() {
		v.since = c.Now()
	}
	label := locales.AiThoughtBrief()
	switch {
	case live:
		label = locales.AiThinkingFor(thinkSeconds(c.Now().Sub(v.since).Milliseconds()))
		c.After(time.Second) // 每秒走一下字
	case part.DurationMs > 0:
		label = locales.AiThoughtFor(thinkSeconds(part.DurationMs))
	}
	ui.Column(c).Gap(3).FillWidth().Children(func() {
		detail := firstLines(part.Text, 1)
		if open {
			detail = "" // 展开时正文就在下面，标题行不再重复
		}
		if stepHead(c, p, icBrain, p.iconQuery, label, detail, false, live, open, func() {}) {
			v.open = map[bool]int{true: 2, false: 1}[open]
		}
		if open {
			ui.Row(c.Key("body")).FillWidth().AlignItems(ui.Stretch).Padding(0, 0, 4, 12).Transition(toggleTransition).Children(func() {
				ui.Box(c).Width(2).Radius(1).Shrink(0).Background(p.border)
				ui.Text(c, strings.TrimSpace(part.Text)).FontSize(font2XS-1).TextColor(p.fgSecondary).Selectable().Grow(1).MinWidth(0).Padding(1, 0, 1, 10)
			})
		}
	})
}

// toolRow 是一次工具调用：标题行显示名称、对象与状态；展开后依次是参数、SQL、结果。
// 失败的调用自动展开，错误原因和建议不用点开就能看到；最近一次返回了数据表的数据类工具也默认展开，
// 结果就在这一步后面，更早的数据结果收成一行，点开就是。
func (a *assistant) toolRow(c *ui.Context, p tokens, m *chatMsg, i int) {
	tool, v := m.tr.Parts[i].Tool, m.part(i)
	res := tool.Result
	sql, _ := tool.Args["sql"].(string)
	if res != nil && res.SQL != "" {
		sql = res.SQL
	}
	open := v.open == 1 || (v.open == 0 && (tool.Failed() || m.resultPart() == i))
	target := toolTarget(tool.Args)
	if target == "" && sql != "" {
		target = oneLine(sql)
	}
	tint := p.accent
	if tool.Failed() {
		tint = p.danger
	}
	ui.Column(c).Gap(3).FillWidth().Children(func() {
		clicked := stepHead(c, p, toolIcon(tool.Name), tint, toolLabel(tool.Name), target, true, tool.Running(), open, func() {
			// 右侧状态：运行中转圈；失败是红色的叉；成功是摘要、耗时和一个淡淡的对勾
			switch {
			case tool.Running():
				ui.Spinner(c).Size(10, 10)
			case tool.Failed():
				ui.Text(c, locales.AiToolFailed()).FontSize(10).TextColor(p.danger).Shrink(0)
				ui.Icon(c.Key("st"), icXCircle).FontSize(11).TextColor(p.danger).Shrink(0).Transition(popTransition)
			default:
				ui.Text(c, toolStatus(res)).FontSize(10).TextColor(p.fgMuted).SingleLine().Shrink(0)
				ui.Icon(c.Key("st"), icCheckC).FontSize(11).TextColor(p.success).Shrink(0).Transition(popTransition)
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
			if sql != "" {
				ui.Text(c, sql).FontSize(font2XS-1).Font(mono).Selectable().Padding(6, 8).Radius(6).Background(p.surfaceSecondary).FillWidth()
				ui.Row(c).Gap(4).Children(func() { a.sqlActions(c, "st", sql, false) })
			}
			a.toolResultBody(c, p, tool, v)
		})
	})
}

// toolResultBody 画工具的结果：失败是原因加建议；成功是表格、补充说明或文本。
func (a *assistant) toolResultBody(c *ui.Context, p tokens, tool *agent.ToolPart, v *partView) {
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
	case v.grid != nil:
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
