package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/agent"
	"minidb/internal/app"
	"minidb/locales"
)

func (a *assistant) openSessions() {
	a.sessions.open = true
	a.refreshSessions()
}

// refreshSessions 重新读取历史对话列表（面板开着时删除一条后用）。
func (a *assistant) refreshSessions() {
	var items []app.AISessionView
	a.w.sh.up.bg(func() { items, _ = a.w.sh.app.AI.ListAISessions(a.w.cfg.ID, 50) }, func() { a.sessions.items = items })
}

// loadSession 打开一段历史对话：用户的话原样还原，助手的回答按保存的过程（推理、工具调用、结果、文字）还原，
// 没有保存过程的旧会话退化成一段纯文字。
func (a *assistant) loadSession(id string) {
	var msgs []app.AISessionMessageView
	var err error
	a.w.sh.up.bg(func() { msgs, err = a.w.sh.app.AI.GetAISessionMessages(id, 0) }, func() {
		if err != nil {
			a.w.sh.toastErr(errMessage(err))
			return
		}
		a.reset()
		a.session = id
		for _, m := range msgs {
			a.nextID++
			switch {
			case m.Role == "user":
				a.msgs = append(a.msgs, &chatMsg{id: a.nextID, user: true, text: m.Content, tr: agent.Transcript{Done: true}})
			case m.Role == "assistant" && m.Transcript != nil:
				m.Transcript.Done = true
				a.msgs = append(a.msgs, &chatMsg{id: a.nextID, tr: *m.Transcript})
			case m.Role == "assistant":
				a.msgs = append(a.msgs, &chatMsg{id: a.nextID, tr: agent.Transcript{Done: true, Parts: []agent.Part{{Type: agent.PartText, Text: m.Content}}}})
			}
		}
		a.sessions.open = false
	})
}

// sessionsView 是历史对话：在历史按钮下面弹出的一张小面板（不是居中的对话框），
// 当前对话有强调条，悬停出现删除；点一行就切到那段对话。
func (a *assistant) sessionsView(c *ui.Context, p tokens, anchor ui.Element) {
	if !a.sessions.open {
		return
	}
	ui.PopoverBase(c, anchor, &a.sessions.open, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6, 0, 0, 0).
			Width(330).MaxHeight(420).Background(p.surfaceElev).Border(1, p.border).Radius(10).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18)).Clip()
		ui.Column(c).FillWidth().Children(func() {
			ui.Row(c).Gap(6).Padding(9, 12, 7, 12).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, locales.AiSessions()).FontSize(font2XS).FontWeight(600)
				if n := len(a.sessions.items); n > 0 {
					ui.Text(c, fmt.Sprint(n)).FontSize(font2XS - 1).TextColor(p.fgMuted)
				}
				ui.Spacer(c)
			})
			ui.Box(c).Height(1).FillWidth().Background(p.borderSubtle)
			if len(a.sessions.items) == 0 {
				ui.Column(c).FillWidth().Padding(26, 12).Center().Gap(8).Children(func() {
					ui.Icon(c, icHistory).FontSize(22).TextColor(p.fgMuted)
					ui.Text(c, locales.AiSessionsEmpty()).TextColor(p.fgMuted).FontSize(font2XS)
				})
				return
			}
			now := time.Now()
			scrollbars(c, ui.Scroll(c.Key("sessions")).FillWidth().MaxHeight(370)).Children(func() {
				ui.Column(c).Gap(1).FillWidth().Padding(4).Children(func() {
					for _, it := range a.sessions.items {
						it := it
						cur := it.ID == a.session
						row := ui.Row(c.Key(it.ID)).Gap(8).Padding(6, 8, 6, 10).Radius(radiusBtn + 1).AlignItems(ui.Center).FillWidth()
						hovered := row.Hovered()
						switch {
						case hovered:
							row.Background(p.hover)
						case cur:
							row.Background(p.accent.Alpha(0.08))
						}
						row.Children(func() {
							ui.Box(c).Width(2).Height(26).Radius(1).Shrink(0).Background(map[bool]ui.Color{true: p.accent, false: {}}[cur])
							ui.Column(c).Grow(1).Gap(1).MinWidth(0).Children(func() {
								ui.Text(c, it.Title).FontSize(font2XS).SingleLine().TruncateMiddle().FillWidth()
								meta := relativeTime(it.UpdatedAt, now)
								if it.MessageCount > 0 {
									meta += " · " + locales.AiSessionMessages(it.MessageCount)
								}
								ui.Text(c, meta).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine()
							})
							if hovered {
								if tipBtn(c.Key("d"), icTrash, locales.AiDeleteChat()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
									a.w.sh.up.bg(func() { _ = a.w.sh.app.AI.DeleteAISession(it.ID) }, func() { a.refreshSessions() })
								}
							}
						})
						if row.Clicked() {
							a.loadSession(it.ID)
						}
					}
				})
			})
		})
	})
}
