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

func toolLabel(name string) string {
	switch name {
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
	case "table_profile":
		return locales.AiToolTableProfile()
	case "sql_explain_plan":
		return locales.AiToolSqlExplainPlan()
	}
	return name
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
			ui.Toolbar(c, func() {
				ui.Row(c).Gap(2).Padding(0, 8, 0, 12).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 4).Shrink(0).Children(func() {
					ui.Icon(c, icSparkle).FontSize(13).TextColor(p.accent)
					ui.Text(c, locales.AiTitle()).FontSize(font2XS).FontWeight(500).Margin(0, 0, 0, 4)
					ui.Spacer(c)
					if tipBtn(c, icPlus, locales.AiNewChat()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
						a.reset()
					}
					if tipBtn(c, icHistory, locales.AiSessions()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
						a.openSessions()
					}
					if tipBtn(c, icTrash, locales.AiClear()).Size(sizeBtnSm, sizeBtnSm).Disabled(len(a.msgs) == 0).Clicked() {
						a.reset()
					}
					if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
						a.w.panel = 0
					}
				})
			}).Padding(0)
			ui.Text(c, locales.AiContext(a.w.dbLabel())).FontSize(11).TextColor(p.fgMuted).Padding(0, 12, 6, 12).SingleLine()
			hairline(c, p.borderSubtle)
			if a.checked && !a.ready {
				ui.Column(c).Gap(8).Padding(14).FillWidth().Children(func() {
					ui.Text(c, locales.AiNotConfigured()).TextColor(p.fgSecondary).FontSize(font2XS + 1)
					if ui.PrimaryButton(c, locales.AiConfigure()).Clicked() {
						a.w.sh.openSettings(setAI)
					}
				})
			}
			if len(a.msgs) == 0 {
				ui.Column(c).Grow(1).Center().Gap(6).Padding(20).Children(func() {
					ui.Icon(c, icSparkle).FontSize(28).TextColor(p.fgMuted)
					ui.Text(c, locales.AiEmptyTitle()).FontWeight(500)
					ui.Text(c, locales.AiEmptyHint()).TextColor(p.fgMuted).FontSize(font2XS).TextAlign(ui.Center)
				})
			} else {
				a.list.Key = func(i int) any { return a.msgs[i].id }
				a.list.FollowEnd = true
				listBars(c, ui.List(c.Key("chat"), &a.list, len(a.msgs), func(i int) { a.message(c, a.msgs[i]) }).Grow(1).Padding(10, 12).Gap(12), &a.list)
			}
			a.inputView(c)
		})
	})
	a.sessionsView(c)
}

func (a *assistant) inputView(c *ui.Context) {
	th := c.Theme()
	ui.Box(c).Height(1).Background(th.Border).FillWidth()
	ui.Column(c).Gap(6).Padding(8, 10).FillWidth().Children(func() {
		ui.TextArea(c, &a.input).Lines(1, 6).Placeholder(locales.AiPlaceholder()).FillWidth()
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
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
			})
			ui.Spacer(c)
			if a.running {
				if ui.Button(c, locales.AiStop()).Clicked() {
					a.stop()
				}
			} else if ui.PrimaryButton(c, locales.AiSend()).Disabled(strings.TrimSpace(a.input) == "").Clicked() {
				a.send(a.input)
			}
		})
	})
	if c.Shortcut(ui.Cmd, ui.KeyEnter) && a.w.panel == 2 {
		a.send(a.input)
	}
}

func (a *assistant) message(c *ui.Context, m *chatMsg) {
	th := c.Theme()
	p := baseTokens(c)
	if m.user {
		ui.Row(c).FillWidth().Children(func() {
			ui.Spacer(c)
			ui.Text(c, m.text).Selectable().FontSize(font2XS+1).TextColor(ui.Hex("#ffffff")).Padding(7, 12).Radius(8).Background(th.Accent).MaxWidth(320)
		})
		return
	}
	ui.Column(c).Gap(8).FillWidth().Padding(10, 12).Radius(radiusPanel).Border(1, p.borderSubtle).Children(func() {
		if len(m.steps) > 0 || !m.done {
			ui.Text(c, locales.AiPreparing()).FontSize(11).TextColor(p.fgMuted)
		}
		for i, st := range m.steps {
			a.stepView(c.Key(fmt.Sprint("s", i)), st)
		}
		if !m.done && m.text == "" && len(m.steps) == 0 {
			ui.Row(c).Gap(8).Children(func() {
				ui.Spinner(c)
				ui.Text(c, locales.AiWorking()).TextColor(th.TextMuted)
			})
		}
		if m.text != "" {
			if m.done {
				markdown.View(c, m.text, &a.md)
			} else {
				markdown.Stream(c, m.text, &a.md)
			}
		}
		if m.err != "" {
			ui.Text(c, locales.AiError(m.err)).TextColor(th.Danger).Selectable()
		}
		if m.stopped && m.done {
			ui.Text(c, locales.AiStopped()).TextColor(th.TextMuted).FontSize(12)
		}
		if m.done && len(m.suggestions) > 0 {
			ui.Row(c).Gap(6).FillWidth().Children(func() {
				for i, sg := range m.suggestions {
					if ui.Button(c.Key(fmt.Sprint("sg", i)), sg.Label).Clicked() {
						a.send(sg.Prompt)
					}
				}
			})
		}
		if m.done && m.ms > 0 && m.err == "" {
			ui.Text(c, locales.AiUsage(int(m.ms))).FontSize(11).TextColor(th.TextMuted)
		}
	})
}

func (a *assistant) stepView(c *ui.Context, st *step) {
	th := c.Theme()
	label := locales.AiThinking()
	if !st.thinking {
		label = toolLabel(st.tool)
	}
	pp := baseTokens(c)
	ui.Column(c).Gap(4).FillWidth().Padding(5, 8).Radius(8).Border(1, pp.borderSubtle).Children(func() {
		head := ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth()
		head.Children(func() {
			if st.open {
				ui.Icon(c, icChevronD).TextColor(th.TextMuted)
			} else {
				ui.Icon(c, icChevronR).TextColor(th.TextMuted)
			}
			ui.Text(c, label).FontSize(font2XS).FontWeight(500)
			if !st.thinking && st.sql != "" {
				ui.Text(c, oneLine(st.sql)).FontSize(font2XS - 1).Font(mono).TextColor(pp.fgMuted).SingleLine().Grow(1)
			}
			switch {
			case st.running:
				ui.Spinner(c)
			case !st.thinking && st.result != nil && !st.result.OK:
				ui.Text(c, locales.AiToolFailed()).FontSize(11).TextColor(th.Danger)
			case !st.thinking && st.result != nil:
				ui.Icon(c, icCheck).FontSize(12).TextColor(th.Success)
			}
			ui.Spacer(c)
			if st.result != nil && st.result.DurationMs > 0 {
				ui.Text(c, locales.AiUsage(int(st.result.DurationMs))).FontSize(11).TextColor(th.TextMuted)
			}
		})
		if head.Clicked() {
			st.open = !st.open
		}
		if !st.open {
			if st.thinking {
				ui.Text(c, firstLines(st.text, 1)).FontSize(12).TextColor(th.TextMuted).SingleLine()
			}
			return
		}
		if st.thinking {
			ui.Text(c, st.text).FontSize(12).TextColor(th.TextMuted).Selectable()
			return
		}
		if st.sql != "" {
			ui.Text(c, st.sql).FontSize(font2XS).Font(mono).Selectable().Padding(6, 8).Radius(6).Background(pp.surfaceSecondary)
			ui.Row(c).Gap(6).Children(func() {
				if ui.Button(c, locales.AiInsertSQL()).Clicked() {
					a.insertSQL(st.sql, false)
				}
				if ui.Button(c, locales.AiRunSQL()).Clicked() {
					a.insertSQL(st.sql, true)
				}
			})
		}
		if st.grid != nil {
			ui.Box(c).Height(min(220, 44+float32(len(st.grid.rows))*32)).FillWidth().Children(func() {
				st.grid.view(c, label, a.w.sh.toast)
			})
			if st.result.Truncated {
				ui.Text(c, locales.AiRowsTruncated(len(st.grid.rows))).FontSize(11).TextColor(th.TextMuted)
			}
		} else if st.result != nil && st.result.Text != "" {
			ui.Text(c, firstLines(st.result.Text, 12)).FontSize(12).Selectable().TextColor(th.TextMuted)
		}
	})
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
