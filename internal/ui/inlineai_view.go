package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// view 画编辑器上方的输入栏和结果预览；没打开时什么都不画。
func (a *inlineAI) view(c *ui.Context) {
	if !a.open {
		return
	}
	p := a.q.w.sh.tokens(c)
	c = c.Key(fmt.Sprintf("iai%d", a.q.n))
	bar := ui.Column(c).FillWidth().Shrink(0).Padding(8, 10).Gap(8).Background(p.surfaceSecondary).Children(func() {
		a.inputRow(c, p)
		if a.err != "" {
			ui.Row(c).Gap(6).AlignItems(ui.Start).FillWidth().Children(func() {
				ui.Icon(c, kindIcon(kindError)).FontSize(13).TextColor(kindColor(p, kindError)).Margin(1, 0, 0, 0)
				ui.Text(c, a.err).FontSize(font2XS).TextColor(p.fg).Selectable().Grow(1).MinWidth(0)
			})
		}
		if a.busy || a.sql != "" {
			a.resultCard(c, p)
		} else if a.err == "" {
			a.quickRow(c, p)
		}
	})
	// Esc 关闭：焦点在栏里的任何地方都算（输入框不把 Esc 交给 OnTextKey，因为全局的 Esc 快捷键先认领了它）
	if bar.Shortcut(0, ui.KeyEscape) {
		a.close()
	}
	hairline(c, p.border)
}

// inputRow 是输入框一行：图标、输入、生成 / 停止、关闭。
func (a *inlineAI) inputRow(c *ui.Context, p tokens) {
	placeholder := locales.InlineAiPlaceholder()
	if a.q.ed.SelectedText() != "" {
		placeholder = locales.InlineAiPlaceholderEdit()
	}
	ui.Row(c).Gap(8).AlignItems(ui.Start).FillWidth().Padding(4, 4, 4, 10).Radius(12).Border(1, p.border).Background(p.surface).Children(func() {
		ui.Icon(c, icSparkle).FontSize(14).TextColor(p.accent).Shrink(0).Margin(6, 0, 0, 0)
		in := ui.TextArea(c.Key("in"), &a.input).Lines(1, 4).Placeholder(placeholder).Grow(1).MinWidth(0).
			Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false).OnTextKey(a.inputKey)
		if a.focus {
			a.focus = false
			in.Focus()
		}
		if a.busy {
			b := tipOn(c, ui.Button(c.Key("stop"), "").Label(locales.InlineAiStop()), locales.InlineAiStop()).Size(26, 26).Padding(0).Radius(13).BorderWidth(0).Background(p.fg)
			if b.Children(func() { ui.Icon(c, icStop).FontSize(11).TextColor(p.surface) }).Clicked() {
				a.cancel()
			}
		} else {
			can := strings.TrimSpace(a.input) != ""
			bg := p.accent
			if !can {
				bg = p.accent.Alpha(0.35)
			}
			b := tipOn(c, ui.Button(c.Key("go"), "").Label(locales.InlineAiGenerate()), locales.InlineAiGenerate()).Size(26, 26).Padding(0).Radius(13).BorderWidth(0).Background(bg).Disabled(!can)
			if b.Children(func() { ui.Icon(c, icArrowUp).FontSize(14).TextColor(ui.Hex("#ffffff")) }).Clicked() {
				a.generate(opFree)
			}
		}
		if tipBtn(c.Key("x"), icX, locales.InlineAiClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			a.close()
		}
	})
}

// inputKey 是输入框的按键：↵ 生成、⇧↵ 换行（Esc 见 view）。
func (a *inlineAI) inputKey(k ui.TextKey) bool {
	switch {
	case k.Key == ui.KeyEnter && (k.Mods == 0 || k.Mods == ui.Cmd):
		if !a.busy {
			a.input = k.Text()
			a.generate(opFree)
		}
		return true
	}
	return false
}

// quickRow 是还没生成时的快捷动作和说明。
func (a *inlineAI) quickRow(c *ui.Context, p tokens) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Padding(0, 4).Children(func() {
		ui.Text(c, a.scopeLabel()).FontSize(font2XS).TextColor(p.fgMuted).SingleLine().Shrink(0)
		vline(c, p.border, 10)
		if actBtn(c.Key("q-opt"), icBulb, locales.InlineAiQuickOptimize()).Clicked() {
			a.generate(opOptimize)
		}
		if actBtn(c.Key("q-fix"), icShieldAlert, locales.InlineAiQuickFix()).Disabled(a.lastError() == "").Clicked() {
			a.generate(opFix)
		}
		if actBtn(c.Key("q-exp"), icInfo, locales.InlineAiQuickExplain()).Clicked() {
			a.explain()
		}
		ui.Spacer(c)
		ui.Text(c, locales.InlineAiHint()).FontSize(10).TextColor(p.fgMuted).SingleLine()
	})
}

// resultCard 是生成结果的预览：流式出现的 SQL，下面是应用 / 重新生成等操作。
func (a *inlineAI) resultCard(c *ui.Context, p tokens) {
	ui.Column(c.Key("res")).FillWidth().Radius(10).Border(1, p.border).Background(p.surface).Clip().Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Padding(6, 10, 0, 10).Children(func() {
			ui.Text(c, locales.InlineAiResult()).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary)
			ui.Text(c, a.scopeLabel()).FontSize(font2XS).TextColor(p.fgMuted).SingleLine().Grow(1).TruncateMiddle()
			if a.busy {
				ui.Spinner(c)
			}
		})
		scrollbars(c, ui.Scroll(c.Key("res-sql")).FillWidth().MaxHeight(220)).Children(func() {
			body := a.sql
			if body == "" {
				ui.Text(c, locales.InlineAiGenerating()).FontSize(font2XS).TextColor(p.fgMuted).Padding(8, 10)
				return
			}
			ui.RichText(c, sqlSpans(p, body, p.fg)...).Font(mono).FontSize(font2XS).Selectable().Padding(8, 10).FillWidth()
		})
		if a.busy {
			return
		}
		hairline(c, p.borderSubtle)
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Padding(6, 8).Children(func() {
			label := locales.InlineAiInsert()
			if a.orig != "" {
				label = locales.InlineAiReplace()
			}
			if actPrimary(c.Key("apply"), icCheck, label).Clicked() {
				a.apply()
			}
			if actBtn(c.Key("regen"), icRefresh, locales.InlineAiRegen()).Clicked() {
				a.regenerate()
			}
			if actBtn(c.Key("copy"), icCopy, locales.InlineAiCopy()).Clicked() {
				a.q.w.sh.copyText(a.sql)
			}
			if actBtn(c.Key("chat"), icBot, locales.InlineAiContinue()).Clicked() {
				a.continueInChat()
			}
			ui.Spacer(c)
			if actBtn(c.Key("drop"), icX, locales.InlineAiDiscard()).Clicked() {
				a.raw, a.sql, a.err = "", "", ""
			}
		})
	})
}
