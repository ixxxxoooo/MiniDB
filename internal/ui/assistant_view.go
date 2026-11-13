package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// view 画整个面板：顶栏、对话列表（或空状态）、输入框。
func (a *assistant) view(c *ui.Context, p tokens) {
	th := c.Theme()
	if !a.checked {
		a.checkConfigured()
	}
	// The original's panel is dense: 12px text, small buttons.
	ct := *th
	ct.Spacing, ct.FontSize = 2.5, font2XS
	ui.Row(c).Fill().Children(func() {
		ui.Themed(c, &ct).Grow(1).FillHeight().Background(p.surface).Children(func() {
			a.header(c, p)
			hairline(c, p.borderSubtle)
			switch {
			case len(a.msgs) == 0:
				a.emptyState(c, p)
			default:
				a.list.Key = func(i int) any { return a.msgs[i].id }
				a.list.FollowEnd = true
				listBars(c, ui.List(c.Key("chat"), &a.list, len(a.msgs), func(i int) { a.message(c, p, a.msgs[i], i == len(a.msgs)-1) }).Grow(1).Padding(12, 12).Gap(16), &a.list)
			}
			a.inputView(c, p)
		})
	})
}

// header 是面板顶部：标题、当前库（带只读标记）、新对话/历史/关闭。
func (a *assistant) header(c *ui.Context, p tokens) {
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(6).Padding(0, 8, 0, 12).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 6).Shrink(0).Children(func() {
			ui.Icon(c, icSparkle).FontSize(13).TextColor(p.accent)
			ui.Text(c, locales.AiTitle()).FontSize(font2XS + 1).FontWeight(600)
			tipOn(c, ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(0, 7).Height(18).Radius(9).Background(p.surfaceSecondary), locales.AiReadonlyTip()).Children(func() {
				ui.Icon(c, icLock).FontSize(9).TextColor(p.fgMuted)
				ui.Text(c, a.w.dbLabel()).FontSize(font2XS - 1).TextColor(p.fgSecondary).SingleLine().MaxWidth(120)
			})
			ui.Spacer(c)
			if tipBtn(c, icPlus, locales.AiNewChat()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				a.reset()
			}
			hist := tipBtn(c, icHistory, locales.AiSessions()).Size(sizeBtnSm, sizeBtnSm)
			if hist.Clicked() {
				if a.sessions.open {
					a.sessions.open = false
				} else {
					a.openSessions()
				}
			}
			a.sessionsView(c, p, hist)
			if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				a.w.panel = 0
			}
		})
	}).Padding(0)
}

// emptyState 是还没有对话时的样子：一句话说明能做什么，加几张可以直接点的提示卡。
func (a *assistant) emptyState(c *ui.Context, p tokens) {
	scrollbars(c, ui.Scroll(c.Key("ai-empty")).Grow(1).FillWidth()).Children(func() {
		ui.Column(c).FillWidth().Gap(14).Padding(22, 16, 16, 16).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Size(44, 44).Center().Radius(14).Background(p.accent.Alpha(0.12)).Children(func() {
				ui.Icon(c, icSparkle).FontSize(22).TextColor(p.accent)
			})
			ui.Column(c).Gap(4).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, locales.AiEmptyTitle()).FontSize(15).FontWeight(600)
				ui.Text(c, locales.AiEmptyHint()).TextColor(p.fgMuted).FontSize(font2XS).TextAlign(ui.Center).FillWidth()
			})
			if a.checked && !a.ready {
				ui.Column(c).Gap(8).Padding(12).FillWidth().Radius(radiusPanel).Background(p.warning.Alpha(0.10)).Border(1, p.warning.Alpha(0.35)).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icAlertTri).FontSize(13).TextColor(p.warning)
						ui.Text(c, locales.AiNotConfigured()).TextColor(p.fgSecondary).FontSize(font2XS)
					})
					if actPrimary(c, icSettings, locales.AiConfigure()).Clicked() {
						a.w.sh.openSettings(setAI)
					}
				})
			}
			ui.Column(c).Gap(6).FillWidth().Children(func() {
				for i, sg := range a.suggestions() {
					sg := sg
					card := ui.Column(c.Key(fmt.Sprint("sug", i))).Gap(2).Padding(8, 11).FillWidth().Radius(radiusPanel).Border(1, p.borderSubtle)
					if card.Hovered() {
						card.Background(p.hover).BorderColor(p.accent.Alpha(0.5))
					}
					card.Children(func() {
						ui.Text(c, sg[0]).FontSize(font2XS).FontWeight(600)
						ui.Text(c, sg[1]).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
					})
					if card.Clicked() {
						if a.ready {
							a.send(sg[1])
						} else {
							a.input = sg[1]
						}
					}
				}
			})
		})
	})
}

// inputView 是底部的输入框：一个圆角容器，文本在上，下面一行放引用表、提示和发送/停止。
func (a *assistant) inputView(c *ui.Context, p tokens) {
	ui.Column(c).Padding(4, 10, 10, 10).Gap(6).FillWidth().Shrink(0).Children(func() {
		a.mentionMenuView(c, p)
		box := ui.Column(c).Gap(2).FillWidth().Radius(12).Border(1, p.border).Background(p.surface).Padding(6, 8, 6, 8).Children(func() {
			a.mentionChips(c, p)
			in := ui.TextArea(c, &a.input).Lines(1, 6).Placeholder(locales.AiPlaceholder()).FillWidth().Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).
				AutoFocus().OnTextKey(a.inputKey)
			// 引用在文字里着色，和上面的 chip 同色；列表与 chip 都按这一帧的文字和光标刷新
			changed := a.mentionApply(in)
			in.TextRanges(mentionRanges(a.input, p.accent)...)
			s0, s1 := in.TextSelection()
			if a.mention.update(a.w.tables, a.input, s0, s1) || changed {
				a.w.sh.redraw()
			}
			ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
				tipBtn(c, icAt, locales.AiMention()).Size(sizeBtnSm, sizeBtnSm).Menu(func(m *ui.Menu) {
					n := 0
					for _, t := range a.w.tables {
						if n++; n > 40 {
							break
						}
						if m.Item(t.Name).Chosen() {
							a.input = strings.TrimRight(a.input, " ") + " @" + t.Name + " "
						}
					}
				})
				ui.Text(c, locales.AiHintSend()).FontSize(10).TextColor(p.fgMuted).SingleLine().Grow(1).TruncateMiddle()
				a.sendButton(c, p)
			})
		})
		// Esc：先关 @ 列表，没有列表时停止正在生成的回答
		// （输入框的 OnTextKey 收不到 Esc，全局的 Esc 快捷键先认领了它，所以挂在输入框的容器上）
		if box.Shortcut(0, ui.KeyEscape) {
			switch {
			case a.mention.open:
				a.mention.dismiss()
			case a.running:
				a.stop()
			}
		}
	})
}

// inputKey 是输入框的快捷键（TextArea 会先吃掉按键，所以必须在这里接）：
// ↵ 与 ⌘↵ 发送、⇧↵ 换行；运行中 Esc 停止；输入为空时 ↑ 找回上一条提问。
func (a *assistant) inputKey(k ui.TextKey) bool {
	if a.mention.key(k) {
		return true
	}
	switch {
	case k.Key == ui.KeyEnter && (k.Mods == 0 || k.Mods == ui.Cmd):
		if a.running {
			return true // 回答还在生成：不吞掉输入，也不重复发
		}
		text := k.Text()
		if strings.TrimSpace(text) == "" {
			return true
		}
		if !a.ready && a.checked {
			a.w.sh.toastErr(locales.AiNotConfigured())
			return true
		}
		k.SetText("")
		a.send(text)
		return true
	case k.Key == ui.KeyUp && k.Mods == 0 && k.Text() == "":
		for i := len(a.msgs) - 1; i >= 0; i-- {
			if a.msgs[i].user {
				k.SetText(a.msgs[i].text)
				return true
			}
		}
	}
	return false
}

// sendButton 是输入框右下角的圆形按钮：空闲时是向上箭头（发送），运行中是方块（停止）。
func (a *assistant) sendButton(c *ui.Context, p tokens) {
	white := ui.Hex("#ffffff")
	if a.running {
		b := tipOn(c, ui.Button(c.Key("ai-stop"), "").Label(locales.AiStop()), locales.AiStop()).Size(26, 26).Padding(0).Radius(13).BorderWidth(0).Background(p.fg)
		if b.Children(func() { ui.Icon(c, icStop).FontSize(11).TextColor(p.surface) }).Clicked() {
			a.stop()
		}
		return
	}
	can := strings.TrimSpace(a.input) != "" && (a.ready || !a.checked)
	bg := p.accent
	if !can {
		bg = p.accent.Alpha(0.35)
	}
	b := tipOn(c, ui.Button(c.Key("ai-send"), "").Label(locales.AiSend()), locales.AiSend()).Size(26, 26).Padding(0).Radius(13).BorderWidth(0).Background(bg).Disabled(!can)
	if b.Children(func() { ui.Icon(c, icArrowUp).FontSize(14).TextColor(white) }).Clicked() {
		a.send(a.input)
	}
}

// suggestionCards 空对话里的提示卡（标签，发出的问题）；有表时多一张针对第一张表的。
func suggestionCards(tables []database.TableInfo) [][2]string {
	out := [][2]string{
		{locales.AiSugLabelTables(), locales.AiSugTables()},
		{locales.AiSugLabelBiggest(), locales.AiSugBiggest()},
		{locales.AiSugLabelReview(), locales.AiSugReview()},
	}
	if len(tables) > 0 {
		out = append(out, [2]string{locales.AiSugLabelTable(tables[0].Name), locales.AiSugTable(tables[0].Name)})
	}
	return out
}
