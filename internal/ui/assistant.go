package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"minidb/internal/agent"
	"minidb/internal/ai"
	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/locales"
)

// step is one thing the agent did while answering: thought, or called a tool.
type step struct {
	target   string // 工具的作用对象：表名、关键词、列名……显示在步骤标题后面
	thinking bool
	text     string // thinking text
	callID   string
	tool     string
	sql      string
	running  bool
	result   *agent.ToolResultEvent
	grid     *resultGrid
	open     bool
}

// chatMsg is one turn of the conversation.
type chatMsg struct {
	id          int
	user        bool
	text        string
	steps       []*step
	done        bool
	err         string
	stopped     bool
	suggestions []agent.Suggestion
	ms          int64
	logMode     int // 活动日志：0 自动（运行中展开、完成后折叠），1 手动展开，2 手动折叠
}

// dataTools 是产出"数据"的工具：它们的结果表格直接摆在回复里，不必点开活动日志才看得到。
var dataTools = map[string]bool{"sql_readonly_execute": true, "table_sample": true, "column_values": true}

// resultStep 是回复里要直接展示的那份结果：最后一个成功返回了表格的数据类工具。
func (m *chatMsg) resultStep() *step {
	for i := len(m.steps) - 1; i >= 0; i-- {
		if st := m.steps[i]; !st.thinking && dataTools[st.tool] && st.grid != nil && st.result != nil && st.result.OK {
			return st
		}
	}
	return nil
}

// assistant is the AI panel's state, one per workspace.
type assistant struct {
	w *Workspace

	input    string
	msgs     []*chatMsg
	running  bool
	reqID    string
	session  string
	list     ui.ListState
	md       markdown.Options
	nextID   int
	ready    bool // configured: checked once
	checked  bool
	sessions struct {
		open  bool
		items []app.AISessionView
	}
	mention bool
}

func newAssistant(w *Workspace) *assistant {
	a := &assistant{w: w}
	a.md.CodeBlock = a.codeBlock
	a.md.Code = map[string]func(*ui.Context, *markdown.CodeBlockView){
		"minidb-next-steps": func(*ui.Context, *markdown.CodeBlockView) {}, // shown as buttons instead
		"minidb-meta":       func(*ui.Context, *markdown.CodeBlockView) {},
	}
	a.md.WrapCode = true
	return a
}

func (a *assistant) checkConfigured() {
	a.checked = true
	var cfg *app.AIConfig
	a.w.sh.up.bg(func() { cfg, _ = a.w.sh.app.Settings.GetAIConfig() }, func() {
		a.ready = cfg != nil && strings.TrimSpace(cfg.APIKey) != ""
	})
}

func (a *assistant) reset() {
	a.stop()
	a.msgs, a.session, a.running = nil, "", false
}

// ---- running ----

func (a *assistant) send(text string) {
	text = strings.TrimSpace(text)
	if text == "" || a.running {
		return
	}
	w := a.w
	a.nextID++
	a.msgs = append(a.msgs, &chatMsg{id: a.nextID, user: true, text: text, done: true})
	a.nextID++
	reply := &chatMsg{id: a.nextID}
	a.msgs = append(a.msgs, reply)
	a.input = ""
	a.running = true
	a.reqID = fmt.Sprintf("ai%d", requestSeq.Add(1))
	if a.session == "" {
		a.session = fmt.Sprintf("s%d", requestSeq.Add(1))
	}
	reqID, session, db := a.reqID, a.session, w.db
	w.sh.runs[reqID] = reply

	var history []ai.ChatMessage
	for _, m := range a.msgs[:len(a.msgs)-1] {
		role := "assistant"
		if m.user {
			role = "user"
		}
		if m.text != "" {
			history = append(history, ai.ChatMessage{Role: role, Content: m.text})
		}
	}
	var err error
	w.sh.up.bg(func() {
		_, err = w.sh.app.AI.ChatAIStream(w.cfg.ID, db, history, reqID, session)
	}, func() {
		delete(w.sh.runs, reqID)
		if a.reqID == reqID {
			a.running = false
		}
		if err != nil && !reply.done {
			reply.done = true
			if reply.stopped {
				return
			}
			reply.err = err.Error()
		}
		reply.done = true
	})
}

func (a *assistant) stop() {
	if !a.running {
		return
	}
	for _, m := range a.msgs {
		if !m.done && !m.user {
			m.stopped = true
		}
	}
	a.w.sh.app.AI.CancelChatStream(a.reqID)
}

// apply folds one agent event into the reply it belongs to.
func (m *chatMsg) apply(ev agent.Event) {
	switch ev.Type {
	case agent.EventStepThinking:
		delta, _ := ev.Payload.(string)
		if n := len(m.steps); n > 0 && m.steps[n-1].thinking {
			m.steps[n-1].text += delta
		} else {
			m.steps = append(m.steps, &step{thinking: true, text: delta})
		}
	case agent.EventToolRequested:
		call, ok := ev.Payload.(agent.ToolCall)
		if !ok {
			return
		}
		st := &step{callID: call.CallID, tool: call.ToolName, running: true}
		if sql, ok := call.Arguments["sql"].(string); ok {
			st.sql = sql
		}
		st.target = toolTarget(call.Arguments)
		m.steps = append(m.steps, st)
	case agent.EventToolResult:
		res, ok := ev.Payload.(agent.ToolResultEvent)
		if !ok {
			return
		}
		for _, st := range m.steps {
			if st.callID == res.CallID && !st.thinking {
				st.running, st.result = false, &res
				if sql, ok := res.Data["sql"].(string); ok && sql != "" {
					st.sql = sql
				}
				if res.Kind == string(agent.ResultKindRows) && len(res.Columns) > 0 {
					g := newResultGrid(false)
					cols := make([]database.ColumnMeta, len(res.Columns))
					for i, c := range res.Columns {
						cols[i] = database.ColumnMeta{Name: c}
					}
					g.set(cols, res.Rows)
					st.grid = g
				}
			}
		}
	case agent.EventAnswerDelta:
		delta, _ := ev.Payload.(string)
		m.text += delta
	case agent.EventRunDone:
		if d, ok := ev.Payload.(agent.RunDone); ok {
			if d.Content != "" {
				m.text = d.Content
			}
			m.suggestions, m.ms = d.Suggestions, d.Usage.DurationMs
		}
		m.done = true
	case agent.EventRunError:
		if e, ok := ev.Payload.(agent.RunError); ok {
			m.err = e.Message
		}
		m.done = true
	}
}

func (s *Shell) subscribeAgent() {
	s.runs = map[string]*chatMsg{}
	s.app.Bus.On(app.TopicAgent, func(p any) {
		ev, ok := p.(agent.Event)
		if !ok {
			return
		}
		s.up.post(func() {
			if m := s.runs[ev.RunID]; m != nil {
				m.apply(ev)
				if s.win != nil {
					s.win.Invalidate()
				}
			}
		})
	})
}

// ---- view ----

// toolLabel 是工具在界面上的名字。
func toolLabel(name string) string {
	switch name {
	case "table_list":
		return locales.AiToolTableList()
	case "table_fuzzy_match":
		return locales.AiToolTableFuzzyMatch()
	case "column_fuzzy_match":
		return locales.AiToolColumnFuzzyMatch()
	case "table_describe":
		return locales.AiToolTableDescribe()
	case "table_relationships":
		return locales.AiToolTableRelationships()
	case "sql_readonly_execute":
		return locales.AiToolSqlReadonlyExecute()
	case "table_ddl":
		return locales.AiToolTableDdl()
	case "table_stats":
		return locales.AiToolTableStats()
	case "table_sample":
		return locales.AiToolTableSample()
	case "column_values":
		return locales.AiToolColumnValues()
	case "table_profile":
		return locales.AiToolTableProfile()
	case "sql_explain_plan":
		return locales.AiToolSqlExplainPlan()
	}
	return name
}

// toolIcon 给每类工具一个一眼能认的图标：查找、结构、数据、执行。
func toolIcon(name string) *ui.SVG {
	switch name {
	case "table_fuzzy_match", "table_list":
		return icSearch
	case "column_fuzzy_match":
		return icColumns
	case "table_describe", "table_sample":
		return icTable
	case "table_ddl":
		return icCode
	case "table_relationships":
		return icShare
	case "table_stats", "table_profile":
		return icBars
	case "column_values":
		return icFilter
	case "sql_explain_plan":
		return icActivity
	case "sql_readonly_execute":
		return icPlay
	}
	return icBot
}

// toolTarget 把工具参数里最能说明"对什么做"的部分挑出来：表名、关键词、列名。
func toolTarget(args map[string]any) string {
	var parts []string
	add := func(v any) {
		switch x := v.(type) {
		case string:
			if x != "" {
				parts = append(parts, x)
			}
		case []any:
			for _, e := range x {
				if s, ok := e.(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
		}
	}
	if t, ok := args["table_name"]; ok {
		add(t)
	}
	add(args["table_names"])
	if col, ok := args["column"].(string); ok && col != "" && len(parts) > 0 {
		parts[0] += "." + col
	}
	add(args["keywords"])
	return strings.Join(parts, ", ")
}

// suggestions 是空对话里的几张提示卡：点一下就发出去。有表时再多一张针对第一张表的。
func (a *assistant) suggestions() [][2]string {
	out := [][2]string{
		{locales.AiSugLabelTables(), locales.AiSugTables()},
		{locales.AiSugLabelBiggest(), locales.AiSugBiggest()},
		{locales.AiSugLabelReview(), locales.AiSugReview()},
	}
	if len(a.w.tables) > 0 {
		name := a.w.tables[0].Name
		out = append(out, [2]string{locales.AiSugLabelTable(name), locales.AiSugTable(name)})
	}
	return out
}

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
	a.sessionsView(c)
}

// header 是面板顶部：标题、当前库（带只读标记）、新对话/历史/关闭。
func (a *assistant) header(c *ui.Context, p tokens) {
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(6).Padding(0, 8, 0, 12).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 6).Shrink(0).Children(func() {
			ui.Icon(c, icSparkle).FontSize(13).TextColor(p.accent)
			ui.Text(c, locales.AiTitle()).FontSize(font2XS + 1).FontWeight(600)
			ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(0, 7).Height(18).Radius(9).Background(p.surfaceSecondary).Tooltip(locales.AiReadonlyTip()).Children(func() {
				ui.Icon(c, icLock).FontSize(9).TextColor(p.fgMuted)
				ui.Text(c, a.w.dbLabel()).FontSize(font2XS - 1).TextColor(p.fgSecondary).SingleLine().MaxWidth(120)
			})
			ui.Spacer(c)
			if tipBtn(c, icPlus, locales.AiNewChat()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				a.reset()
			}
			if tipBtn(c, icHistory, locales.AiSessions()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				a.openSessions()
			}
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
					if ui.PrimaryButton(c, locales.AiConfigure()).Clicked() {
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
	ui.Column(c).Padding(4, 10, 10, 10).FillWidth().Shrink(0).Children(func() {
		ui.Column(c).Gap(2).FillWidth().Radius(12).Border(1, p.border).Background(p.surface).Padding(6, 8, 6, 8).Children(func() {
			ui.TextArea(c, &a.input).Lines(1, 6).Placeholder(locales.AiPlaceholder()).FillWidth().Border(0, ui.Color{}).Background(ui.Transparent).FocusRing(false)
			ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.MenuButton(c, locales.AiMention(), func(m *ui.Menu) {
					n := 0
					for _, t := range a.w.tables {
						if n++; n > 40 {
							break
						}
						if m.Item(t.Name).Chosen() {
							a.input = strings.TrimRight(a.input, " ") + " @" + t.Name + " "
						}
					}
				}).Height(sizeBtnSm).FontSize(font2XS - 1)
				ui.Text(c, locales.AiHintSend()).FontSize(10).TextColor(p.fgMuted).SingleLine().Grow(1).TruncateMiddle()
				a.sendButton(c, p)
			})
		})
	})
	if c.Shortcut(ui.Cmd, ui.KeyEnter) && a.w.panel == 2 && a.ready {
		a.send(a.input)
	}
}

// sendButton 是输入框右下角的圆形按钮：空闲时是向上箭头（发送），运行中是方块（停止）。
func (a *assistant) sendButton(c *ui.Context, p tokens) {
	white := ui.Hex("#ffffff")
	if a.running {
		b := ui.Button(c.Key("ai-stop"), "").Label(locales.AiStop()).Tooltip(locales.AiStop()).Size(26, 26).Padding(0).Radius(13).BorderWidth(0).Background(p.fg)
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
	b := ui.Button(c.Key("ai-send"), "").Label(locales.AiSend()).Tooltip(locales.AiSend()).Size(26, 26).Padding(0).Radius(13).BorderWidth(0).Background(bg).Disabled(!can)
	if b.Children(func() { ui.Icon(c, icArrowUp).FontSize(14).TextColor(white) }).Clicked() {
		a.send(a.input)
	}
}

// message 画一轮对话：用户是右侧的蓝色气泡；助手是通栏的——活动日志、回答、操作、建议依次排下来。
func (a *assistant) message(c *ui.Context, p tokens, m *chatMsg, last bool) {
	th := c.Theme()
	if m.user {
		ui.Row(c).FillWidth().Children(func() {
			ui.Spacer(c)
			ui.Text(c, m.text).Selectable().FontSize(font2XS+1).TextColor(ui.Hex("#ffffff")).Padding(7, 12).Radius(12).Background(th.Accent).MaxWidth(300)
		})
		return
	}
	ui.Column(c).Gap(8).FillWidth().Children(func() {
		if len(m.steps) > 0 || !m.done {
			a.workLog(c, p, m)
		}
		if st := m.resultStep(); st != nil {
			a.resultCard(c, p, st)
		}
		if m.text != "" {
			if m.done {
				markdown.View(c, m.text, &a.md)
			} else {
				markdown.Stream(c, m.text, &a.md)
			}
		}
		if m.err != "" {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 10).FillWidth().Radius(radiusPanel).Background(p.danger.Alpha(0.08)).Border(1, p.danger.Alpha(0.3)).Children(func() {
				ui.Icon(c, icAlertTri).FontSize(13).TextColor(p.danger)
				ui.Text(c, locales.AiError(m.err)).TextColor(p.danger).Selectable().Grow(1).FontSize(font2XS)
				if last && !a.running && ui.Button(c, locales.AiRetry()).Height(sizeBtnSm).FontSize(font2XS).Clicked() {
					a.regenerate()
				}
			})
		}
		if m.stopped && m.done {
			ui.Text(c, locales.AiStopped()).TextColor(p.fgMuted).FontSize(font2XS)
		}
		if m.done && m.text != "" && m.err == "" {
			ui.Row(c).Gap(2).AlignItems(ui.Center).FillWidth().Children(func() {
				if tipBtn(c, icCopy, locales.AiCopy()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					a.w.sh.copyText(visibleAnswer(m.text))
				}
				if last && !a.running && tipBtn(c, icRefresh, locales.AiRegenerate()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					a.regenerate()
				}
				ui.Spacer(c)
				if m.ms > 0 {
					ui.Text(c, locales.AiUsage(int(m.ms))).FontSize(10).TextColor(p.fgMuted)
				}
			})
		}
		if m.done && last && !a.running && len(m.suggestions) > 0 {
			ui.Row(c).Gap(6).FillWidth().Wrap().Children(func() {
				for i, sg := range m.suggestions {
					sg := sg
					chip := ui.Button(c.Key(fmt.Sprint("sg", i)), sg.Label).Height(sizeBtnSm).FontSize(font2XS).Radius(999)
					if chip.Clicked() {
						a.send(sg.Prompt)
					}
				}
			})
		}
	})
}

// visibleAnswer 去掉回答末尾给界面用的结构化块（下一步建议、元数据），复制出去的是干净正文。
func visibleAnswer(text string) string {
	for _, tag := range []string{"```minidb-next-steps", "```minidb-meta"} {
		if i := strings.Index(text, tag); i >= 0 {
			text = text[:i]
		}
	}
	return strings.TrimSpace(text)
}

// workLog 是助手回答前做的事：一行摘要，点开是时间线。
// 运行中自动展开，实时看到它在查什么；完成后折叠成"已完成 N 个步骤"，需要核对时再点开。
func (a *assistant) workLog(c *ui.Context, p tokens, m *chatMsg) {
	live := !m.done
	open := m.logMode == 1 || (m.logMode == 0 && live)
	failed := 0
	for _, st := range m.steps {
		if !st.thinking && st.result != nil && !st.result.OK {
			failed++
		}
	}
	label := locales.AiWorking()
	switch {
	case live && len(m.steps) > 0:
		if last := m.steps[len(m.steps)-1]; !last.thinking && last.running {
			label = locales.AiWorkRunning(toolLabel(last.tool))
		}
	case !live && m.ms > 0:
		label = locales.AiWorkDoneTime(len(m.steps), int(m.ms))
	case !live:
		label = locales.AiWorkDone(len(m.steps))
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
			ui.Text(c, label).FontSize(font2XS).TextColor(p.fgSecondary).SingleLine().Grow(1).TruncateMiddle()
			if len(m.steps) > 0 {
				chev := icChevronR
				if open {
					chev = icChevronD
				}
				ui.Icon(c, chev).FontSize(11).TextColor(p.fgMuted)
			}
		})
		if head.Clicked() && len(m.steps) > 0 {
			if open {
				m.logMode = 2
			} else {
				m.logMode = 1
			}
		}
		if !open {
			return
		}
		// 时间线：左侧一条细线把各步骤串起来
		ui.Row(c).FillWidth().AlignItems(ui.Stretch).Children(func() {
			ui.Box(c).Width(1).Margin(2, 0, 2, 11).Shrink(0).Background(p.border)
			ui.Column(c).Gap(1).Grow(1).MinWidth(0).Padding(2, 0, 2, 8).Children(func() {
				for i, st := range m.steps {
					a.stepView(c.Key(fmt.Sprint("s", i)), p, st, live && i == len(m.steps)-1 && m.text == "", st == m.resultStep())
				}
			})
		})
	})
}

// stepView 是时间线上的一步：思考，或一次工具调用。点标题展开参数、SQL 和结果。
func (a *assistant) stepView(c *ui.Context, p tokens, st *step, live, inlined bool) {
	label, ic := locales.AiThought(), icBulb
	if !st.thinking {
		label, ic = toolLabel(st.tool), toolIcon(st.tool)
	}
	open := st.open || (st.thinking && live) // 正在想的那一步实时展开
	ui.Column(c).Gap(3).FillWidth().Children(func() {
		head := ui.Row(c.Key("head")).Gap(7).AlignItems(ui.Center).Padding(3, 4).FillWidth().Radius(6)
		if head.Hovered() {
			head.Background(p.hover)
		}
		head.Children(func() {
			tint := p.accent
			if st.thinking {
				tint = p.warning
			}
			if st.result != nil && !st.result.OK {
				tint = p.danger
			}
			ui.Row(c).Size(18, 18).Shrink(0).Center().Radius(5).Background(tint.Alpha(0.13)).Children(func() {
				ui.Icon(c, ic).FontSize(11).TextColor(tint)
			})
			ui.Text(c, label).FontSize(font2XS).FontWeight(500).Shrink(0)
			target := st.target
			switch {
			case st.thinking:
				target = firstLines(st.text, 1)
			case target == "" && st.sql != "":
				target = oneLine(st.sql)
			}
			tx := ui.Text(c, target).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine().Grow(1).TruncateMiddle()
			if !st.thinking {
				tx.Font(mono)
			}
			switch {
			case st.running:
				ui.Spinner(c).Size(10, 10)
			case !st.thinking && st.result != nil && !st.result.OK:
				ui.Text(c, locales.AiToolFailed()).FontSize(10).TextColor(p.danger)
			case !st.thinking && st.result != nil && st.result.DurationMs > 0:
				ui.Text(c, locales.AiUsage(int(st.result.DurationMs))).FontSize(10).TextColor(p.fgMuted)
			}
		})
		if head.Clicked() {
			st.open = !open
		}
		if !open {
			return
		}
		ui.Column(c).Gap(6).FillWidth().Padding(0, 0, 4, 25).Children(func() {
			if st.thinking {
				ui.Text(c, st.text).FontSize(font2XS - 1).TextColor(p.fgSecondary).Selectable().FillWidth()
				return
			}
			if st.sql != "" && !inlined { // 结果卡里已有 SQL 与按钮
				ui.Text(c, st.sql).FontSize(font2XS-1).Font(mono).Selectable().Padding(6, 8).Radius(6).Background(p.surfaceSecondary).FillWidth()
				ui.Row(c).Gap(4).Children(func() {
					if ui.Button(c.Key("cp"), locales.AiCopySQL()).Height(sizeBtnSm).FontSize(font2XS - 1).Clicked() {
						a.w.sh.copyText(st.sql)
					}
					if ui.Button(c.Key("in"), locales.AiInsertSQL()).Height(sizeBtnSm).FontSize(font2XS - 1).Clicked() {
						a.insertSQL(st.sql, false)
					}
					if ui.Button(c.Key("run"), locales.AiRunSQL()).Height(sizeBtnSm).FontSize(font2XS - 1).Clicked() {
						a.insertSQL(st.sql, true)
					}
				})
			}
			switch {
			case st.grid != nil && inlined: // 表格已经摆在回复里了，这里不重复画
			case st.grid != nil:
				a.gridBox(c, p, st)
			case st.result != nil && st.result.Text != "":
				// 保留原有换行，太长就截到 40 行
				ui.Text(c, headLines(st.result.Text, 40)).FontSize(font2XS-1).Font(mono).Selectable().Padding(6, 8).Radius(6).Background(p.surfaceSecondary).FillWidth()
			}
		})
	})
}

// gridBox 画一步的结果表格；行不多时按行数定高，多了就封顶 260。
func (a *assistant) gridBox(c *ui.Context, p tokens, st *step) {
	ui.Box(c).Height(min(260, 44+float32(len(st.grid.rows))*32)).FillWidth().Children(func() {
		st.grid.view(c, toolLabel(st.tool), a.w.sh.toast)
	})
	if st.result != nil && st.result.Truncated {
		ui.Text(c, locales.AiRowsTruncated(len(st.grid.rows))).FontSize(10).TextColor(p.fgMuted)
	}
}

// resultCard 把查询结果直接摆在回复里：标题（工具、行数）、SQL 和操作、结果表格。
func (a *assistant) resultCard(c *ui.Context, p tokens, st *step) {
	ui.Column(c.Key("result-card")).Gap(6).FillWidth().Padding(8, 10).Radius(radiusPanel).Border(1, p.borderSubtle).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Children(func() {
			ui.Icon(c, toolIcon(st.tool)).FontSize(12).TextColor(p.accent)
			ui.Text(c, toolLabel(st.tool)).FontSize(font2XS).FontWeight(600)
			ui.Text(c, locales.AiResultRows(len(st.grid.rows))).FontSize(font2XS - 1).TextColor(p.fgMuted)
			ui.Spacer(c)
			if st.sql != "" {
				if tipBtn(c.Key("rc-cp"), icCopy, locales.AiCopySQL()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					a.w.sh.copyText(st.sql)
				}
				if tipBtn(c.Key("rc-in"), icCode, locales.AiInsertSQL()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					a.insertSQL(st.sql, false)
				}
				if tipBtn(c.Key("rc-run"), icPlay, locales.AiRunSQL()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
					a.insertSQL(st.sql, true)
				}
			}
		})
		if st.sql != "" {
			ui.Text(c, oneLine(st.sql)).FontSize(font2XS - 1).Font(mono).TextColor(p.fgMuted).SingleLine().TruncateMiddle().FillWidth().Tooltip(st.sql)
		}
		a.gridBox(c, p, st)
	})
}

// headLines 取文本的前 n 行（保留换行），超出的用省略号收尾。
func headLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = append(lines[:n], "…")
	}
	return strings.Join(lines, "\n")
}

// regenerate 丢掉最后一条回答，用同一个问题再问一次。
func (a *assistant) regenerate() {
	if a.running || len(a.msgs) < 2 {
		return
	}
	last := a.msgs[len(a.msgs)-1]
	asked := a.msgs[len(a.msgs)-2]
	if last.user || !asked.user {
		return
	}
	a.msgs = a.msgs[:len(a.msgs)-2]
	a.send(asked.text)
}

// insertSQL puts sql into the active query tab (a new one when the active tab
// is not a query), and runs it when asked.
func (a *assistant) insertSQL(sql string, run bool) {
	w := a.w
	var q *queryTab
	if w.active < len(w.tabs) {
		q, _ = w.tabs[w.active].(*queryTab)
	}
	if q == nil {
		q = w.newQuery("")
	}
	q.insertSQL(sql)
	if run {
		q.runGated([]string{strings.TrimSpace(sql)})
	}
}

// codeBlock draws a code block, with insert and run buttons under SQL.
func (a *assistant) codeBlock(c *ui.Context, b *markdown.CodeBlockView) {
	markdown.DefaultCodeBlock(c, b)
	lang := strings.ToLower(b.Language)
	if b.Open || (lang != "sql" && lang != "mysql" && lang != "postgresql" && lang != "sqlite") {
		return
	}
	code := strings.TrimSpace(b.Code())
	ui.Row(c.Key(fmt.Sprint("cb", b.Index))).Gap(6).Margin(4, 0, 0, 0).Children(func() {
		if ui.Button(c, locales.AiInsertSQL()).Clicked() {
			a.insertSQL(code, false)
		}
		if ui.Button(c, locales.AiRunSQL()).Clicked() {
			a.insertSQL(code, true)
		}
	})
}

// ---- sessions ----

func (a *assistant) openSessions() {
	a.sessions.open = true
	var items []app.AISessionView
	a.w.sh.up.bg(func() { items, _ = a.w.sh.app.AI.ListAISessions(50) }, func() { a.sessions.items = items })
}

func (a *assistant) loadSession(id string) {
	var msgs []app.AISessionMessageView
	var err error
	a.w.sh.up.bg(func() { msgs, err = a.w.sh.app.AI.GetAISessionMessages(id, 0) }, func() {
		if err != nil {
			a.w.sh.toastErr(err.Error())
			return
		}
		a.reset()
		a.session = id
		for _, m := range msgs {
			if m.Role != "user" && m.Role != "assistant" {
				continue
			}
			a.nextID++
			a.msgs = append(a.msgs, &chatMsg{id: a.nextID, user: m.Role == "user", text: m.Content, done: true})
		}
		a.sessions.open = false
	})
}

func (a *assistant) sessionsView(c *ui.Context) {
	if !a.sessions.open {
		return
	}
	sh := a.w.sh
	p := sh.tokens(c)
	sh.dialog(c, &a.sessions.open, dlgOpts{title: locales.AiSessions(), width: 520, height: 440}, func() {
		if len(a.sessions.items) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
				ui.Icon(c, icHistory).FontSize(26).TextColor(p.fgMuted)
				ui.Text(c, locales.AiSessionsEmpty()).TextColor(p.fgMuted)
			})
		}
		scrollbars(c, ui.Scroll(c.Key("sessions")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Gap(2).FillWidth().Children(func() {
				for _, it := range a.sessions.items {
					it := it
					row := ui.Row(c.Key(it.ID)).Gap(8).Padding(8, 10).Radius(radiusBtn + 1).AlignItems(ui.Center).FillWidth()
					hovered := row.Hovered()
					if hovered {
						row.Background(p.hover)
					}
					row.Children(func() {
						ui.Column(c).Grow(1).Gap(1).MinWidth(0).Children(func() {
							ui.Text(c, it.Title).SingleLine()
							ui.Text(c, it.UpdatedAt.Format("2006-01-02 15:04")).FontSize(font2XS - 1).TextColor(p.fgMuted)
						})
						if hovered {
							if tipBtn(c.Key("d"), icTrash, locales.AiDeleteChat()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
								a.w.sh.up.bg(func() { _ = a.w.sh.app.AI.DeleteAISession(it.ID) }, func() { a.openSessions() })
							}
						}
					})
					if row.Clicked() {
						a.loadSession(it.ID)
					}
				}
			})
		})
	}, nil)
}
