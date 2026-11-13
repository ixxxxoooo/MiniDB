package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/agent"
	"minidb/internal/app"
)

func TestAssistantAnswersWithToolAndTable(t *testing.T) {
	e := newEnv(t)
	sql := "SELECT COUNT(*) AS n FROM users"
	srv, calls := fakeLLM(t, sql)
	if err := e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	e.tt.Frame()
	if !w.ai.ready {
		t.Fatal("assistant should see the saved configuration")
	}

	w.ai.send("有多少用户？")
	if w.ai.running {
		t.Fatal("run should have finished")
	}
	if len(w.ai.msgs) != 2 {
		t.Fatalf("msgs=%d", len(w.ai.msgs))
	}
	reply := w.ai.msgs[1]
	if !reply.tr.Done || reply.tr.Error != "" {
		t.Fatalf("done=%v err=%q", reply.tr.Done, reply.tr.Error)
	}
	if !strings.Contains(reply.answer(), "450") {
		t.Fatalf("answer = %q", reply.answer())
	}
	thinking, tool := -1, -1
	for i, part := range reply.tr.Parts {
		switch {
		case part.Type == agent.PartThinking:
			thinking = i
		case part.Tool != nil:
			tool = i
		}
	}
	if thinking < 0 || !strings.Contains(reply.tr.Parts[thinking].Text, "用户表") {
		t.Fatalf("thinking part missing: %+v", reply.tr.Parts)
	}
	if tool < 0 || reply.tr.Parts[tool].Tool.Name != "sql_readonly_execute" || reply.tr.Parts[tool].Tool.Result == nil || !reply.tr.Parts[tool].Tool.Result.OK {
		t.Fatalf("tool part: %+v", reply.tr.Parts)
	}
	grid := reply.part(tool).grid
	if grid == nil || len(grid.rows) != 1 || plainText(grid.rows[0]["n"]) != "450" {
		t.Fatalf("structured rows missing: %+v", grid)
	}
	if got := reply.tr.Parts[tool].Tool.Result.SQL; got != sql {
		t.Fatalf("sql = %q", got)
	}
	if reply.tr.Rounds != 2 {
		t.Fatalf("rounds = %d, want 2 (tool, then answer)", reply.tr.Rounds)
	}
	if calls.Load() != 2 {
		t.Fatalf("chat calls = %d, want 2 (tool, then answer)", calls.Load())
	}
	for i := range reply.tr.Parts {
		reply.part(i).open = 1
	}
	e.has("运行只读 SQL")
	e.has("450")
	e.shot("11-assistant")

	// the SQL block of the answer can be sent to a query tab and run
	w.ai.insertSQL(sql, true)
	q, ok := w.tabs[w.active].(*queryTab)
	if !ok || len(q.results) != 1 {
		t.Fatalf("insert+run: tab=%T", w.tabs[w.active])
	}

	// the conversation is stored and reopens exactly as it was: steps, result table and answer
	sessions, _ := e.sh.app.AI.ListAISessions(w.cfg.ID, 10)
	if len(sessions) != 1 {
		t.Fatalf("sessions = %d", len(sessions))
	}
	w.ai.reset()
	w.ai.loadSession(sessions[0].ID)
	if len(w.ai.msgs) != 2 || !w.ai.msgs[0].user || !strings.Contains(w.ai.msgs[1].answer(), "450") {
		t.Fatalf("restored: %+v", w.ai.msgs)
	}
	restored := w.ai.msgs[1]
	if restored.resultPart() < 0 || !restored.tr.Done {
		t.Fatalf("a restored answer keeps its steps and result table: result=%d", restored.resultPart())
	}
	e.shot("11b-assistant-restored")
}

func TestAssistantReportsErrors(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	defer srv.Close()
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.ai.send("hi")
	reply := w.ai.msgs[1]
	if !reply.tr.Done || reply.tr.Error == "" {
		t.Fatalf("an upstream failure must show as an error: done=%v err=%q", reply.tr.Done, reply.tr.Error)
	}
	if w.ai.running {
		t.Fatal("must not stay running")
	}
}

func TestAssistantNotConfigured(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.panel = 2
	e.has("还没有配置 AI")
	e.shot("12-ai-unconfigured")
	if err := e.tt.Click("配置 AI"); err != nil {
		t.Fatal(err)
	}
	e.has("服务地址")
}

// TestAssistantEmptyStateSuggestionsAndRegenerate 空对话显示提示卡，点一张就发出去；
// 每一步都按顺序排在回复里（没有折叠的活动日志）；"重新生成"用同一个问题再问一次。
func TestAssistantEmptyStateSuggestionsAndRegenerate(t *testing.T) {
	e := newEnv(t)
	srv, calls := fakeLLM(t, "SELECT COUNT(*) AS n FROM users")
	if err := e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	e.tt.Frame()
	e.tt.Frame()
	e.has("问问你的数据库")
	e.has("了解这个库")
	e.has("看看 orders") // 有表时多一张针对第一张表的卡片
	e.shot("93-ai-empty")
	if err := e.tt.Click("最大的表"); err != nil {
		t.Fatal(err)
	}
	e.tt.Frame()
	if len(w.ai.msgs) != 2 || w.ai.msgs[0].text != "统计行数最多的 5 张表" {
		t.Fatalf("点提示卡应发出对应问题: %+v", w.ai.msgs)
	}
	reply := w.ai.msgs[1]
	e.hasNot("已完成") // 不再有折叠的「已完成 N 个步骤」：每一步都排在回复里
	if reply.resultPart() < 0 {
		t.Fatal("最近一次数据结果应默认展开在它那一步后面")
	}
	e.shot("94-ai-collapsed")
	before := calls.Load()
	w.ai.regenerate()
	e.tt.Frame()
	if len(w.ai.msgs) != 2 || w.ai.msgs[0].text != "统计行数最多的 5 张表" || calls.Load() <= before {
		t.Fatalf("重新生成应再问一次且不堆叠: msgs=%d calls=%d→%d", len(w.ai.msgs), before, calls.Load())
	}
	if got := toolTarget(map[string]any{"table_name": "orders", "column": "status"}); got != "orders.status" {
		t.Fatalf("toolTarget=%q", got)
	}
}

func TestAssistantRecoversFromEmptyAnswerAfterTools(t *testing.T) {
	e := newEnv(t)
	srv := emptyAfterToolLLM(t, "SELECT COUNT(*) AS n FROM users", false)
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.ai.send("有多少用户？")
	reply := w.ai.msgs[1]
	if reply.tr.Error != "" || !strings.Contains(reply.answer(), "450") {
		t.Fatalf("空回答应触发总结请求并拿到答案: err=%q text=%q", reply.tr.Error, reply.answer())
	}
}

func TestAssistantShowsErrorWhenModelNeverAnswers(t *testing.T) {
	e := newEnv(t)
	srv := emptyAfterToolLLM(t, "SELECT COUNT(*) AS n FROM users", true)
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	w.ai.send("有多少用户？")
	reply := w.ai.msgs[1]
	if reply.tr.Error == "" {
		t.Fatalf("始终没有回答时必须报错而不是留空: %+v", reply)
	}
	e.tt.Frame()
	e.tt.Frame()
	e.has("450") // 查询结果的表格直接摆在回复里，不用点开日志
	e.shot("95-ai-empty-answer")
}

func TestAssistantInputShortcuts(t *testing.T) {
	e := newEnv(t)
	srv := emptyAfterToolLLM(t, "SELECT COUNT(*) AS n FROM users", false)
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	e.tt.Frame()
	e.tt.Frame()
	// 输入框自动聚焦；⇧↵ 换行而不发送
	e.tt.Type("第一行")
	e.tt.Key(mui.Shift, mui.KeyEnter)
	e.tt.Type("第二行")
	e.tt.Frame()
	if len(w.ai.msgs) != 0 || w.ai.input != "第一行\n第二行" {
		t.Fatalf("⇧↵ 应换行: msgs=%d input=%q", len(w.ai.msgs), w.ai.input)
	}
	// ↵ 发送并清空
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if len(w.ai.msgs) != 2 || w.ai.msgs[0].text != "第一行\n第二行" || w.ai.input != "" {
		t.Fatalf("↵ 应发送: msgs=%d input=%q", len(w.ai.msgs), w.ai.input)
	}
	// ↑ 在空输入时找回上一条提问；⌘↵ 同样发送
	e.tt.Key(0, mui.KeyUp)
	e.tt.Frame()
	if w.ai.input != "第一行\n第二行" {
		t.Fatalf("↑ 应找回上一条提问: %q", w.ai.input)
	}
	e.tt.Key(mui.Cmd, mui.KeyEnter)
	e.tt.Frame()
	if len(w.ai.msgs) != 4 {
		t.Fatalf("⌘↵ 应发送: msgs=%d", len(w.ai.msgs))
	}
}

func TestAssistantSessionsPopover(t *testing.T) {
	e := newEnv(t)
	srv, _ := fakeLLM(t, "SELECT COUNT(*) AS n FROM users")
	e.sh.app.Settings.SaveAIConfig(app.AIConfig{BaseURL: srv.URL + "/v1", APIKey: "k", Model: "m"})
	e.sh.app.AI.ReloadConfig()
	w := openWorkspace(e)
	w.panel = 2
	w.ai.send("有多少用户？")
	w.ai.openSessions()
	e.tt.Frame()
	e.tt.Frame()
	e.tt.Frame()
	e.has("历史对话")
	e.has("有多少用户？")
	e.shot("96-ai-sessions-popover")
}

// 点「重新生成」会在列表正在画的那一帧删掉旧回答：列表按旧行数取行也不能越界崩溃。
func TestAssistantListSurvivesShrinkMidFrame(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.panel = 2
	for i := 0; i < 6; i++ {
		w.ai.msgs = append(w.ai.msgs, &chatMsg{id: i, user: i%2 == 0, text: "问", tr: agent.Transcript{Done: true, Parts: []agent.Part{{Type: agent.PartText, Text: "答"}}}})
	}
	e.tt.Frame()
	key := w.ai.list.Key // 上一帧留下的取键函数，按 6 行建的
	w.ai.msgs = w.ai.msgs[:2]
	for i := 0; i < 6; i++ {
		_ = key(i) // 以前这里会 index out of range
	}
	e.tt.Frame()
	st := &structState{}
	if safeKey(3, 0, func() any { return st.cols[3].UID }) != 3 {
		t.Fatal("越界的行号应退回行号本身")
	}
}

// 回答里的 SQL 代码块点「执行」：块里有几条语句就执行几条，而不是整块当一条发出去。
func TestAssistantRunsEveryStatementOfABlock(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	w.ai.insertSQL("-- 用户数\nSELECT COUNT(*) AS a FROM users;\n\n-- 订单数\nSELECT COUNT(*) AS b FROM orders;", true)
	e.tt.Frame()
	q, ok := w.tabs[w.active].(*queryTab)
	if !ok || len(q.results) != 2 {
		t.Fatalf("两条语句应各有一个结果: tab=%T results=%d", w.tabs[w.active], len(q.results))
	}
	for i, r := range q.results {
		if r.res == nil || r.res.Error != "" {
			t.Fatalf("第 %d 条执行失败: %+v", i+1, r.res)
		}
	}
}
