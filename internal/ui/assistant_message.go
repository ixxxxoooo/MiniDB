package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"minidb/internal/app"
	"minidb/locales"
)

// message 画一轮对话：用户是右侧的蓝色气泡；助手是通栏的——
// 推理、说明文字、工具调用（结果跟在各自那一步后面）按发生的顺序排下来，
// 最后一段文字就是回答，再往下是操作、建议。
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
		a.timeline(c, p, m)
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

// timeline 把一轮回答里的片段按发生的顺序排下来，就是 ReAct 的过程：
// 先想（推理）→ 说明要做什么（文字）→ 调工具 → 结果就跟在这一步后面 → 再想 → 再调下一个……最后一段文字是回答。
//   - 推理：正在想时展开，想完收起成一行，可以点开回看；
//   - 文字：模型在工具之间说的话和最终回答，同样的排版，正在流出来的那段末尾有个呼吸的小点；
//   - 工具：一行标题（名称、对象、状态，运行中有转圈），点开是参数、SQL 和结果；失败的、
//     以及最近一次返回了数据表的数据类工具默认展开，更早的收成一行。
//
// 模型在等待下一步（还没有动静，或刚拿到一个工具结果还在想）时，末尾有一行「思考中…」。
func (a *assistant) timeline(c *ui.Context, p tokens, m *chatMsg) {
	tr := &m.tr
	live := !tr.Done
	n := len(tr.Parts)
	fresh := !m.born.IsZero() // 这次运行里产生的回答：新的一步淡入；从历史恢复的不放动画
	for i, part := range tr.Parts {
		last := i == n-1
		if part.Type == app.AgentPartText && part.Text == "" {
			continue
		}
		row := ui.Column(c.Key(fmt.Sprint("p", i))).FillWidth()
		if fresh {
			row.Transition(stepTransition)
		}
		row.Children(func() {
			switch {
			case part.Type == app.AgentPartText:
				if live && last {
					markdown.Stream(c, part.Text, &a.md)
					streamCursor(c, p) // 还在流出来：末尾一个呼吸的小点
				} else {
					markdown.View(c, part.Text, &a.md)
				}
			case part.Type == app.AgentPartThinking:
				a.thinkingRow(c, p, m.part(i), part, live && last)
			case part.Tool != nil:
				a.toolRow(c, p, m, i)
			}
		})
	}
	if live && (n == 0 || (tr.Parts[n-1].Tool != nil && !tr.Parts[n-1].Tool.Running())) {
		ui.Row(c.Key("waiting")).Gap(6).AlignItems(ui.Center).Padding(3, 4).Children(func() {
			ui.Spinner(c).Size(11, 11)
			breathe(ui.Text(c, locales.AiWorking()).FontSize(font2XS).TextColor(p.fgSecondary), "work", true)
			if n == 0 {
				typingDots(c, p) // 还没有任何动静：在等模型的第一个字
			}
		})
	}
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
func (a *assistant) gridBox(c *ui.Context, p tokens, tool *app.AgentToolPart, view *partView) {
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
