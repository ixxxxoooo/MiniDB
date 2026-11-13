package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"minidb/internal/agent"
	"minidb/internal/ai"
	"minidb/internal/app"
	"minidb/internal/database"
	"minidb/internal/sqltext"
	"minidb/locales"
)

// partView 是回答里一个片段的界面状态：展开/折叠，以及工具结果的表格。
type partView struct {
	open   int // 0 自动（失败的工具与正在进行的思考展开），1 手动展开，2 手动折叠
	grid   *resultGrid
	gridOf *agent.ToolResultEvent // grid 是按哪一份结果建的；结果换了就重建
	since  time.Time              // 推理片段：界面第一次看到它正在进行的时刻，用来实时显示「思考中 · N 秒」
}

// chatMsg 是对话里的一轮：用户的话，或助手的一次回答。
// 助手的回答由 agent.Transcript 描述——按发生顺序排列的推理、文字与工具调用片段，
// 与服务端保存的会话是同一份结构，所以恢复的历史与实时看到的完全一致。
type chatMsg struct {
	id       int
	user     bool
	text     string           // 用户消息的文字
	tr       agent.Transcript // 助手回答的过程
	views    []*partView      // 与 tr.Parts 一一对应的界面状态
	stopping bool             // 用户已点停止，等后端收尾
	born     time.Time        // 这一轮是在这次运行中产生的时刻；从保存的会话恢复的为零值，不放入场动画
}

// fresh 报告这一轮是不是刚产生的：只有刚产生的才放入场动画，恢复一个旧会话时不会一屏消息同时弹出来。
func (m *chatMsg) fresh() bool { return !m.born.IsZero() && time.Since(m.born) < 3*time.Second }

// part 返回第 i 个片段的界面状态，必要时（新片段、结果更新）补建表格。
func (m *chatMsg) part(i int) *partView {
	for len(m.views) < len(m.tr.Parts) {
		m.views = append(m.views, &partView{})
	}
	v := m.views[i]
	if tool := m.tr.Parts[i].Tool; tool != nil && tool.Result != nil && v.gridOf != tool.Result {
		v.gridOf, v.grid = tool.Result, nil
		if res := tool.Result; res.Kind == string(agent.ResultKindRows) && len(res.Columns) > 0 {
			cols := make([]database.ColumnMeta, len(res.Columns))
			for k, name := range res.Columns {
				cols[k] = database.ColumnMeta{Name: name}
			}
			v.grid = newResultGrid(false)
			v.grid.set(cols, res.Rows)
		}
	}
	return v
}

// tail 是最终回答所在的片段下标：最后一个片段是文字时它就是回答；否则（还在调用工具，或没有文字）为 -1。
func (m *chatMsg) tail() int {
	if n := len(m.tr.Parts); n > 0 && m.tr.Parts[n-1].Type == agent.PartText {
		return n - 1
	}
	return -1
}

// work 是回答之前做的事：推理、工具调用，以及工具之间的文字说明。
func (m *chatMsg) work() []agent.Part {
	if t := m.tail(); t >= 0 {
		return m.tr.Parts[:t]
	}
	return m.tr.Parts
}

// answer 是这一轮的文字：用户的话，或助手的最终回答。
func (m *chatMsg) answer() string {
	if m.user {
		return m.text
	}
	return m.tr.Answer()
}

// dataTools 是产出"数据"的工具：最近一次的结果表格默认展开。
var dataTools = map[string]bool{"sql_readonly_execute": true, "table_sample": true, "column_values": true}

// resultPart 是默认展开结果的那一步：最后一个成功返回了表格的数据类工具（更早的数据结果收成一行）。
func (m *chatMsg) resultPart() int {
	parts := m.work()
	for i := len(parts) - 1; i >= 0; i-- {
		if t := parts[i].Tool; t != nil && dataTools[t.Name] && t.Result != nil && t.Result.OK && m.part(i).grid != nil {
			return i
		}
	}
	return -1
}

// assistant 是 AI 面板的状态，每个工作区一个。
//
// @author ixxxxoooo
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
	mention  mentionMenu // 输入框里 @ 弹出的表名列表
	ready    bool        // 已配置 AI（只检查一次）
	checked  bool
	sessions struct {
		open  bool
		items []app.AISessionView
	}
}

func newAssistant(w *Workspace) *assistant {
	a := &assistant{w: w}
	a.md.CodeBlock = a.codeBlock
	a.md.Code = map[string]func(*ui.Context, *markdown.CodeBlockView){
		agent.SuggestionTag: func(*ui.Context, *markdown.CodeBlockView) {}, // 流式输出时先别把半截建议块画出来；完成后以按钮呈现
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

// ---- 运行 ----

// send 发出一个问题并开始接收回答。
func (a *assistant) send(text string) {
	text = strings.TrimSpace(text)
	if text == "" || a.running {
		return
	}
	w := a.w
	a.nextID++
	a.msgs = append(a.msgs, &chatMsg{id: a.nextID, user: true, text: text, tr: agent.Transcript{Done: true}, born: time.Now()})
	a.nextID++
	reply := &chatMsg{id: a.nextID, born: time.Now()}
	a.msgs = append(a.msgs, reply)
	a.input = ""
	a.running = true
	a.reqID = fmt.Sprintf("ai%d", requestSeq.Add(1))
	if a.session == "" {
		a.session = fmt.Sprintf("s%d", time.Now().UnixNano()) // 跨重启也不会和已保存的会话撞号
	}
	reqID, session, db := a.reqID, a.session, w.db
	w.sh.runs[reqID] = reply

	var history []ai.ChatMessage
	for _, m := range a.msgs[:len(a.msgs)-1] {
		role := "assistant"
		if m.user {
			role = "user"
		}
		if t := m.answer(); t != "" {
			history = append(history, ai.ChatMessage{Role: role, Content: t})
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
		if !reply.tr.Done {
			// 后端没来得及发出结束事件（如请求一开始就被拒绝）：在这里把这一轮收尾
			reply.tr.Done = true
			switch {
			case reply.stopping:
				reply.tr.Stopped = true
			case err != nil:
				reply.tr.Error = err.Error()
			}
		}
	})
}

// stop 停止正在生成的回答。
func (a *assistant) stop() {
	if !a.running {
		return
	}
	for _, m := range a.msgs {
		if !m.user && !m.tr.Done {
			m.stopping = true
		}
	}
	a.w.sh.app.AI.CancelChatStream(a.reqID)
}

// regenerate 丢掉最后一条回答，用同一个问题再问一次。
// 已保存的会话里那一轮问答也一并删掉，否则恢复历史时同一个问题会出现两次。
func (a *assistant) regenerate() {
	if a.running || len(a.msgs) < 2 {
		return
	}
	last, asked := a.msgs[len(a.msgs)-1], a.msgs[len(a.msgs)-2]
	if last.user || !asked.user {
		return
	}
	a.msgs = a.msgs[:len(a.msgs)-2]
	session, text := a.session, asked.text
	a.w.sh.up.bg(func() { _ = a.w.sh.app.AI.DropAISessionTail(session, 2) }, func() { a.send(text) })
}

// subscribeAgent 订阅助手事件，把每个事件折叠进它所属的那条回答。
func (s *Shell) subscribeAgent() {
	s.runs = map[string]*chatMsg{}
	s.app.Bus.On(app.TopicAgent, func(p any) {
		ev, ok := p.(agent.Event)
		if !ok {
			return
		}
		s.up.post(func() {
			if m := s.runs[ev.RunID]; m != nil {
				m.tr.Apply(ev)
				if s.win != nil {
					s.win.Invalidate()
				}
			}
		})
	})
}

// suggestions 是空对话里的几张提示卡：点一下就发出去。有表时再多一张针对第一张表的。
func (a *assistant) suggestions() [][2]string {
	return suggestionCards(a.w.tables)
}

// insertSQL 把 SQL 放进当前查询页（当前页不是查询时新开一个），要求时顺便执行。
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
		// 一个代码块里可能有好几条语句：拆开后全部执行，每条一个结果页（整块当成一条发给数据库会报错）
		if stmts := sqltext.Texts(sql); len(stmts) > 0 {
			q.runGated(stmts)
		}
	}
}

// codeBlock 画代码块；SQL 代码块下面带"插入编辑器"和"执行"按钮。
func (a *assistant) codeBlock(c *ui.Context, b *markdown.CodeBlockView) {
	markdown.DefaultCodeBlock(c, b)
	lang := strings.ToLower(b.Language)
	if b.Open || (lang != "sql" && lang != "mysql" && lang != "postgresql" && lang != "sqlite") {
		return
	}
	code := strings.TrimSpace(b.Code())
	ui.Row(c.Key(fmt.Sprint("cb", b.Index))).Gap(6).Margin(4, 0, 0, 0).Children(func() {
		if actBtn(c, icCode, locales.AiInsertSQL()).Clicked() {
			a.insertSQL(code, false)
		}
		if actBtn(c, icPlay, locales.AiRunSQL()).Clicked() {
			a.insertSQL(code, true)
		}
	})
}
