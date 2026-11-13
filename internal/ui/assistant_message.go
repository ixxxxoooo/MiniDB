package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"minidb/internal/agent"
	"minidb/internal/app"
	"minidb/locales"
)

// message 画一轮对话：用户是右侧的蓝色气泡；助手是通栏的——
// 活动日志（推理、工具调用）、数据结果、回答、操作、建议依次排下来。
func (a *assistant) message(c *ui.Context, p tokens, m *chatMsg, last bool) {
	th := c.Theme()
	if m.user {
		a.userMessage(c, p, m, th.Accent)
		return
	}
	tr := &m.tr
	col := ui.Column(c).Gap(8).FillWidth()
	if m.fresh() {
		col.Transition(enterTransition) // 新回答从下方淡入
	}
	col.Children(func() {
		if n, _ := m.steps(); n > 0 || len(m.work()) > 0 || !tr.Done {
			a.workLog(c, p, m)
		}
		if i := m.resultPart(); i >= 0 {
			a.resultCard(c, p, m, i)
		}
		if t := m.tail(); t >= 0 && tr.Parts[t].Text != "" {
			if tr.Done {
				markdown.View(c, tr.Parts[t].Text, &a.md)
			} else {
				markdown.Stream(c, tr.Parts[t].Text, &a.md)
				streamCursor(c, p) // 还在流出来：末尾一个呼吸的小点
			}
		}
		if tr.Error != "" {
			a.errorBar(c, p, tr.Error, last)
		}
		if (tr.Stopped || m.stopping) && tr.Done {
			ui.Text(c, locales.AiStopped()).TextColor(p.fgMuted).FontSize(font2XS)
		}
		if tr.Done && tr.Error == "" && m.answer() != "" {
			a.actionRow(c, p, m, last)
		}
		if tr.Done && last && !a.running && len(tr.Suggestions) > 0 {
			ui.Row(c).Gap(6).FillWidth().Wrap().Children(func() {
				for i, sg := range tr.Suggestions {
					if ui.Button(c.Key(fmt.Sprint("sg", i)), sg.Label).Height(sizeBtnSm).FontSize(font2XS).Radius(999).Transition(staggerTransition(i)).Clicked() {
						a.send(sg.Prompt)
					}
				}
			})
		}
	})
}

// errorBar 是回答失败时的提示条，最后一条回答上带"重试"。
func (a *assistant) errorBar(c *ui.Context, p tokens, msg string, last bool) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 10).FillWidth().Radius(radiusPanel).Background(p.danger.Alpha(0.08)).Border(1, p.danger.Alpha(0.3)).Children(func() {
		ui.Icon(c, icAlertTri).FontSize(13).TextColor(p.danger)
		ui.Text(c, locales.AiError(msg)).TextColor(p.danger).Selectable().Grow(1).FontSize(font2XS)
		if last && !a.running && actBtn(c, icRefresh, locales.AiRetry()).Clicked() {
			a.regenerate()
		}
	})
}

// actionRow 是回答下面的一排小按钮：复制、重新生成，右侧是耗时与 token 用量。
func (a *assistant) actionRow(c *ui.Context, p tokens, m *chatMsg, last bool) {
	ui.Row(c).Gap(2).AlignItems(ui.Center).FillWidth().Children(func() {
		if tipBtn(c, icCopy, locales.AiCopy()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			a.w.sh.copyText(m.answer())
		}
		if last && !a.running && tipBtn(c, icRefresh, locales.AiRegenerate()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			a.regenerate()
		}
		ui.Spacer(c)
		u := m.tr.Usage
		switch {
		case u.TotalTokens > 0 && u.DurationMs > 0:
			ui.Text(c, locales.AiUsageTokens(int(u.TotalTokens), int(u.DurationMs))).FontSize(10).TextColor(p.fgMuted)
		case u.DurationMs > 0:
			ui.Text(c, locales.AiUsage(int(u.DurationMs))).FontSize(10).TextColor(p.fgMuted)
		}
	})
}

// workLog 是助手回答前做的事：一行摘要，点开是时间线。
// 运行中自动展开，实时看到它在做什么；完成后折叠成"已完成 N 个步骤"，需要核对时再点开。
func (a *assistant) workLog(c *ui.Context, p tokens, m *chatMsg) {
	live := !m.tr.Done
	open := m.logMode == 1 || (m.logMode == 0 && live)
	n, failed := m.steps()
	work := m.work()
	label := locales.AiWorking()
	switch {
	case live:
		if k := len(work) - 1; k >= 0 && work[k].Tool != nil && work[k].Tool.Running() {
			label = locales.AiWorkRunning(toolLabel(work[k].Tool.Name))
		}
	case m.tr.Usage.DurationMs > 0:
		label = locales.AiWorkDoneTime(n, int(m.tr.Usage.DurationMs))
	default:
		label = locales.AiWorkDone(n)
	}
	ui.Column(c).Gap(2).FillWidth().Children(func() {
		head := ui.Row(c.Key("log-head")).Gap(6).AlignItems(ui.Center).Padding(5, 8).FillWidth().Radius(8).Background(p.surfaceSecondary)
		if head.Hovered() {
			head.Background(p.hover)
		}
		head.Children(func() {
			switch {
			case live:
				ui.Spinner(c).Size(11, 11)
			case failed > 0:
				ui.Icon(c, icAlertTri).FontSize(12).TextColor(p.warning)
			default:
				ui.Icon(c, icCheck).FontSize(12).TextColor(p.success)
			}
			breathe(ui.Text(c, label).FontSize(font2XS).TextColor(p.fgSecondary).SingleLine().Grow(1).TruncateMiddle(), "work", live)
			if live && len(work) == 0 {
				typingDots(c, p) // 还没有任何动静：在等模型的第一个字
			}
			if len(work) > 0 {
				chev := icChevronR
				if open {
					chev = icChevronD
				}
				ui.Icon(c, chev).FontSize(11).TextColor(p.fgMuted)
			}
		})
		if head.Clicked() && len(work) > 0 {
			m.logMode = map[bool]int{true: 2, false: 1}[open]
		}
		if !open || len(work) == 0 {
			return
		}
		// 时间线：左侧一条细线把各片段串起来
		ui.Row(c.Key("timeline")).FillWidth().AlignItems(ui.Stretch).Transition(toggleTransition).Children(func() {
			ui.Box(c).Width(1).Margin(2, 0, 2, 11).Shrink(0).Background(p.border)
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Padding(2, 0, 2, 8).Children(func() {
				inlined := m.resultPart()
				for i, part := range work {
					a.stepRow(c.Key(fmt.Sprint("s", i)), p, m, i, part, live && i == len(work)-1 && m.tail() < 0, i == inlined)
				}
			})
		})
	})
}

// resultCard 把数据类工具的结果直接摆在回复里：标题（工具、行数）、SQL 和操作、结果表格。
func (a *assistant) resultCard(c *ui.Context, p tokens, m *chatMsg, i int) {
	tool, view := m.tr.Parts[i].Tool, m.part(i)
	sql := tool.Result.SQL
	ui.Column(c.Key("result-card")).Gap(6).FillWidth().Padding(8, 10).Radius(radiusPanel).Border(1, p.borderSubtle).Transition(enterTransition).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Icon(c, toolIcon(tool.Name)).FontSize(12).TextColor(p.accent)
			ui.Text(c, toolLabel(tool.Name)).FontSize(font2XS).FontWeight(600)
			ui.Text(c, locales.AiResultRows(len(view.grid.rows))).FontSize(font2XS - 1).TextColor(p.fgMuted)
			ui.Spacer(c)
			if sql != "" {
				a.sqlActions(c, "rc", sql, true)
			}
		})
		if sql != "" {
			tipOn(c, ui.Text(c, oneLine(sql)).FontSize(font2XS-1).Font(mono).TextColor(p.fgMuted).SingleLine().TruncateMiddle().FillWidth(), sql)
		}
		a.gridBox(c, p, tool, view)
	})
}

// sqlActions 是 SQL 旁的三个操作：复制、插入编辑器、执行。icons 为 true 时画成图标按钮（卡片标题里空间紧）。
func (a *assistant) sqlActions(c *ui.Context, key, sql string, icons bool) {
	btn := func(k string, ic *ui.SVG, label string) bool {
		if icons {
			return tipBtn(c.Key(key+k), ic, label).Size(sizeBtnSm, sizeBtnSm).Clicked()
		}
		return actBtn(c.Key(key+k), ic, label).Clicked()
	}
	if btn("-cp", icCopy, locales.AiCopySQL()) {
		a.w.sh.copyText(sql)
	}
	if btn("-in", icCode, locales.AiInsertSQL()) {
		a.insertSQL(sql, false)
	}
	if btn("-run", icPlay, locales.AiRunSQL()) {
		a.insertSQL(sql, true)
	}
}

// gridBox 画一步的结果表格；行不多时按行数定高，多了就封顶 260。
func (a *assistant) gridBox(c *ui.Context, p tokens, tool *agent.ToolPart, view *partView) {
	ui.Box(c).Height(min(260, 44+float32(len(view.grid.rows))*32)).FillWidth().Children(func() {
		view.grid.view(c, toolLabel(tool.Name), a.w.sh.toast)
	})
	if tool.Result != nil && tool.Result.Truncated {
		ui.Text(c, locales.AiRowsTruncated(len(view.grid.rows))).FontSize(10).TextColor(p.fgMuted)
	}
}

// headLines 取文本的前 n 行（保留换行），超出的用省略号收尾。
func headLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = append(lines[:n], "…")
	}
	return strings.Join(lines, "\n")
}

// userMessage 是用户的提问：右侧的蓝色气泡；引用了表时，气泡上方有同样的 chip，文字里的 @表名 加粗。
// 引用就写在文字里，所以从保存的会话恢复出来的消息和刚发出去的长得一样。
func (a *assistant) userMessage(c *ui.Context, p tokens, m *chatMsg, accent ui.Color) {
	col := ui.Column(c).Gap(4).FillWidth()
	if m.fresh() {
		col.Transition(enterTransition)
	}
	col.Children(func() {
		if names := app.MentionNames(m.text); len(names) > 0 {
			ui.Row(c).Gap(4).FillWidth().Children(func() {
				ui.Spacer(c)
				ui.Row(c.Key("msg-chips")).Gap(4).Wrap().MaxWidth(300).Children(func() {
					for _, name := range names {
						mentionChip(c, p, name, a.knownTable(name), nil)
					}
				})
			})
		}
		ui.Row(c).FillWidth().Children(func() {
			ui.Spacer(c)
			white := ui.Hex("#ffffff")
			ui.RichText(c, mentionSpans(m.text, white)...).Selectable().FontSize(font2XS+1).Padding(7, 12).Radius(12).Background(accent).MaxWidth(300)
		})
	})
}

// mentionSpans 把文字按 @引用 切成片段：普通文字用 base 色，引用加粗。
func mentionSpans(text string, base ui.Color) []ui.Span {
	rs := []rune(text)
	var spans []ui.Span
	at := 0
	for _, mt := range app.FindMentions(text) {
		if mt.Start > at {
			spans = append(spans, ui.Span{Text: string(rs[at:mt.Start]), Color: base})
		}
		spans = append(spans, ui.Span{Text: string(rs[mt.Start:mt.End]), Color: base, Weight: 700})
		at = mt.End
	}
	if at < len(rs) {
		spans = append(spans, ui.Span{Text: string(rs[at:]), Color: base})
	}
	return spans
}
