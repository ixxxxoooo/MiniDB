package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/internal/sqltext"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 编辑器里的内联 AI（⌘J）：编辑器上方展开一条输入栏，用中文描述要什么，
// SQL 流式出现在输入栏下面的预览里，满意再"插入"或"替换"进编辑器，不满意就重新生成或放弃。
// 不会改动编辑器里的任何内容，直到用户点了插入 / 替换；也不会去打断侧边的对话。
//
// 作用范围（栏上会写明）：
//   - 选中了一段 SQL：说明怎么改，生成结果替换选中的内容；
//   - 没有选中：生成一条新语句，插入到光标处；
//   - 快捷动作"优化 / 修复报错"：针对选中的内容，没有选中就针对光标所在的语句，生成结果替换它；
//   - "解释"的产出是文字而不是 SQL，所以直接交给侧边对话。

// inlineOp 是一次内联生成的种类。
type inlineOp int

const (
	opFree     inlineOp = iota // 用户在输入框里写的要求
	opOptimize                 // 优化目标语句
	opFix                      // 修复目标语句的报错
)

// inlineAI 是一个查询标签里内联 AI 栏的状态。
type inlineAI struct {
	q *queryTab

	open  bool
	focus bool // 刚打开：这一帧把焦点交给输入框
	input string
	busy  bool
	reqID string
	gen   int // 每次生成加一，晚到的旧结果据此丢弃
	op    inlineOp
	scope string // 这次生成会怎样作用于编辑器，生成开始时定下

	raw string // 模型原始输出（流式增长）
	sql string // 从 raw 里取出的 SQL，预览与应用的都是它
	err string
	ask string // 产生 sql 的那条要求，"在对话中继续"时要带上

	// 应用结果时替换的区间：orig 为空表示没有目标，插入到光标处。
	// 区间是生成开始时记的；应用时编辑器可能已被改动，所以还要靠 orig 重新定位。
	orig       string
	start, end int
	started    time.Time
}

// scopeLabel 是栏上对作用范围的说明：已有结果时是那次生成定下的，否则按现在有没有选中来说。
func (a *inlineAI) scopeLabel() string {
	switch {
	case a.busy || a.sql != "":
		return a.scope
	case a.q.ed.SelectedText() != "":
		return locales.InlineAiScopeSelection()
	default:
		return locales.InlineAiScopeNew()
	}
}

// toggle 打开或收起输入栏。
func (a *inlineAI) toggle() {
	if a.open {
		a.close()
		return
	}
	a.open, a.focus = true, true
}

// close 收起输入栏，正在生成的也一并停掉，焦点还给编辑器。
func (a *inlineAI) close() {
	a.cancel()
	a.open = false
	a.q.root().want = a.q
}

// cancel 停止正在进行的生成。
func (a *inlineAI) cancel() {
	if a.busy {
		a.q.w.sh.app.AI.CancelChatStream(a.reqID)
		a.gen++ // 已发出的请求回来时不再改界面
		a.busy = false
	}
}

// target 是这次要处理的语句：选中的内容，没有选中则（allowCaret 时）取光标所在的语句。
func (a *inlineAI) target(allowCaret bool) (text string, start, end int, ok bool) {
	ed := a.q.ed
	if sel := ed.SelectedText(); strings.TrimSpace(sel) != "" {
		s := ed.Selection()
		return sel, s.Start(), s.End(), true
	}
	if !allowCaret {
		return "", 0, 0, false
	}
	if st, found := sqltext.At(ed.Buffer.String(), ed.Cursor()); found {
		return st.Text, st.Start, st.End, true
	}
	return "", 0, 0, false
}

// lastError 是最近一次运行里的报错：整批失败的，或者结果里带错误的那一条。
func (a *inlineAI) lastError() string {
	q := a.q
	if q.failed != "" {
		return q.failed
	}
	for i := len(q.results) - 1; i >= 0; i-- {
		if msg := q.results[i].res.Error; msg != "" {
			return msg
		}
	}
	return ""
}

// generate 发起一次生成。op 是种类；free 的要求取自输入框。
func (a *inlineAI) generate(op inlineOp) {
	if a.busy {
		return
	}
	sh := a.q.w.sh
	if !a.q.w.ai.ready && a.q.w.ai.checked {
		sh.toastErr(locales.AiNotConfigured())
		return
	}
	req := app.InlineSQLRequest{Instruction: strings.TrimSpace(a.input)}
	var orig string
	var start, end int
	switch op {
	case opFree:
		if req.Instruction == "" {
			return
		}
		if t, s, e, ok := a.target(false); ok {
			orig, start, end = t, s, e
		}
	case opOptimize, opFix:
		t, s, e, ok := a.target(true)
		if !ok {
			sh.toastWarn(locales.InlineAiNoTarget())
			return
		}
		orig, start, end = t, s, e
		if op == opOptimize {
			req.Instruction = locales.InlineAiOptimizeAsk()
		} else {
			if req.Error = a.lastError(); req.Error == "" {
				sh.toastWarn(locales.InlineAiNoError())
				return
			}
			req.Instruction = locales.InlineAiFixAsk()
		}
	}
	req.SQL = orig
	a.op = op
	switch {
	case orig == "":
		a.scope = locales.InlineAiScopeNew()
	case op != opFree && a.q.ed.SelectedText() == "":
		a.scope = locales.InlineAiScopeStatement()
	default:
		a.scope = locales.InlineAiScopeSelection()
	}
	req.Tables = mentionedTables(req.Instruction, a.q.w.tables)

	a.gen++
	gen := a.gen
	a.reqID = fmt.Sprintf("inl%d", requestSeq.Add(1))
	a.busy, a.raw, a.sql, a.err, a.ask = true, "", "", "", req.Instruction
	a.orig, a.start, a.end, a.started = orig, start, end, time.Now()
	reqID, w := a.reqID, a.q.w
	db := w.db

	var out string
	var err error
	sh.up.bg(func() {
		out, err = sh.app.AI.GenerateSQL(w.cfg.ID, db, req, reqID, func(delta string) {
			sh.up.post(func() {
				if a.gen != gen {
					return
				}
				a.raw += delta
				a.sql = app.ExtractSQL(a.raw)
				if sh.win != nil {
					sh.win.Invalidate()
				}
			})
		})
	}, func() {
		if a.gen != gen {
			return
		}
		a.busy = false
		if err != nil {
			a.err = errMessage(err)
			return
		}
		a.sql = out
	})
}

// explain 把目标语句交给侧边对话去解释：产出是文字，不适合放在编辑器里。
func (a *inlineAI) explain() {
	t, _, _, ok := a.target(true)
	if !ok {
		a.q.w.sh.toastWarn(locales.InlineAiNoTarget())
		return
	}
	a.q.askAI(locales.AiPromptExplain(t))
}

// apply 把生成的 SQL 写进编辑器：有目标就替换它，没有就插入到光标处。成功后收起输入栏。
func (a *inlineAI) apply() {
	if a.sql == "" || a.busy {
		return
	}
	ed := a.q.ed
	src := ed.Buffer.String()
	text := a.sql
	if a.orig != "" {
		// 编辑器在生成期间可能被改过：区间对不上就按原文重新找，找不到就当作插入
		start, end := a.start, a.end
		if end > len(src) || src[start:end] != a.orig {
			if i := strings.Index(src, a.orig); i >= 0 {
				start, end = i, i+len(a.orig)
			} else {
				a.orig = ""
			}
		}
		if a.orig != "" {
			// 区间后面已经跟着分号时，生成结果里的分号去掉，免得出现两个
			if end < len(src) && src[end] == ';' {
				text = strings.TrimRight(text, "; \n")
			}
			ed.Buffer.Edit(editor.Edit{Start: start, End: end, Text: text})
			a.finish(locales.InlineAiReplaced())
			return
		}
	}
	at := min(ed.Cursor(), len(src))
	if !strings.HasSuffix(strings.TrimSpace(text), ";") {
		text += ";"
	}
	switch {
	case strings.TrimSpace(src) == "":
		at = 0
		ed.Buffer.Edit(editor.Edit{Start: 0, End: len(src), Text: text})
	default:
		// 另起一段：和前后的内容之间各隔一个空行
		prefix, suffix := "", ""
		if before := src[:at]; strings.TrimSpace(before) != "" {
			prefix = "\n\n"
			if strings.HasSuffix(before, "\n\n") {
				prefix = ""
			} else if strings.HasSuffix(before, "\n") {
				prefix = "\n"
			}
		}
		if after := src[at:]; strings.TrimSpace(after) != "" {
			suffix = "\n\n"
		}
		ed.Buffer.Edit(editor.Edit{Start: at, End: at, Text: prefix + text + suffix})
	}
	a.finish(locales.InlineAiInserted())
}

// finish 应用完成：提示、清空这次的结果、收起输入栏。
func (a *inlineAI) finish(msg string) {
	a.q.w.sh.toastOK(msg)
	a.raw, a.sql, a.err, a.input = "", "", "", ""
	a.close()
}

// continueInChat 带着这次的要求和生成的 SQL 去侧边对话里接着聊。
func (a *inlineAI) continueInChat() {
	if a.sql == "" {
		return
	}
	prompt := locales.InlineAiChatPrompt(a.ask, a.sql)
	a.close()
	a.q.askAI(prompt)
}

// mentionedTables 找出要求里点名的表：写成 @表名 的，或者直接写了表名的（整词、不分大小写）。
func mentionedTables(text string, tables []model.TableInfo) []string {
	var out []string
	lower := strings.ToLower(text)
	for _, t := range tables {
		name := strings.ToLower(t.Name)
		if name != "" && (strings.Contains(lower, "@"+name) || hasWord(lower, name)) {
			out = append(out, t.Name)
		}
	}
	return out
}

// hasWord 报告 text 里是否有 word 这个整词（前后不是字母、数字、下划线）。
func hasWord(text, word string) bool {
	isWord := func(r byte) bool {
		return r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
	}
	for from := 0; ; {
		i := strings.Index(text[from:], word)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(word)
		if (i == 0 || !isWord(text[i-1])) && (end == len(text) || !isWord(text[end])) {
			return true
		}
		from = i + 1
	}
}

// ---- 界面 ----

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

// regenerate 用同样的方式再生成一次：自由输入的沿用输入框里的文字，优化 / 修复沿用对应的动作。
func (a *inlineAI) regenerate() {
	a.generate(a.op)
}
