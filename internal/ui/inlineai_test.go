package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// sqlLLM 只会回答一段 SQL（分两次流出）；bodies 记下每次请求体。
func sqlLLM(t *testing.T, answer string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/responses") {
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"message":"not found"}}`)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(delta map[string]any, finish any) {
			b, _ := json.Marshal(map[string]any{"id": "c", "object": "chat.completion.chunk", "model": "m",
				"choices": []map[string]any{{"index": 0, "delta": delta, "finish_reason": finish}}})
			fmt.Fprintf(w, "data: %s\n\n", b)
		}
		half := len(answer) / 2
		send(map[string]any{"role": "assistant", "content": answer[:half]}, nil)
		send(map[string]any{"content": answer[half:]}, nil)
		send(map[string]any{}, "stop")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), bodies...) }
}

const answerSQL = "```sql\nSELECT status, COUNT(*) FROM users GROUP BY status\n```"
const wantSQL = "SELECT status, COUNT(*) FROM users GROUP BY status"

// ⌘J 展开输入栏，用中文写要求、回车生成，结果先放在预览里，点"插入"才进编辑器。
func TestInlineAIKeyboardFlowInsertsAtTheCaret(t *testing.T) {
	e := newEnv(t)
	srv, bodies := sqlLLM(t, answerSQL)
	e.useLLM(srv)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1")
	q.ed.Select(len("SELECT 1"), len("SELECT 1"))
	e.tt.Frame()

	e.tt.Key(mui.Cmd, mui.KeyJ)
	e.tt.Frame()
	if !q.ia.open {
		t.Fatal("⌘J 应展开内联 AI 栏")
	}
	e.has("生成新语句，插入到光标处")
	e.shot("27-inline-ai")
	e.tt.Type("统计每个状态的用户数")
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()

	if q.ia.sql != wantSQL || q.ia.busy {
		t.Fatalf("应生成并停下: sql=%q busy=%v err=%q", q.ia.sql, q.ia.busy, q.ia.err)
	}
	if got := q.ed.Buffer.String(); got != "SELECT 1" {
		t.Fatalf("生成结果在点插入前不该动编辑器: %q", got)
	}
	e.has("AI 生成的 SQL")
	e.has("GROUP BY status")
	e.shot("28-inline-ai-result")
	if len(bodies()) != 1 || !strings.Contains(bodies()[0], "统计每个状态的用户数") || !strings.Contains(bodies()[0], "users: id INTEGER PK") {
		t.Fatalf("请求里应带着要求和相关表结构: %v", bodies())
	}

	q.ia.apply()
	if got := q.ed.Buffer.String(); got != "SELECT 1\n\n"+wantSQL+";" {
		t.Fatalf("应另起一段插入并补分号: %q", got)
	}
	if q.ia.open || q.ia.sql != "" || q.ia.input != "" {
		t.Fatalf("应用后应收起并清空: %+v", q.ia)
	}
}

// 选中了一段 SQL 再写要求，生成结果替换选中的内容；区间后面已经有分号时不再多一个。
func TestInlineAIReplacesTheSelection(t *testing.T) {
	e := newEnv(t)
	srv, bodies := sqlLLM(t, answerSQL)
	e.useLLM(srv)
	w := openWorkspace(e)
	q := w.newQuery("SELECT * FROM users;\nSELECT 2")
	q.ed.Select(0, len("SELECT * FROM users"))
	q.inline().toggle()
	q.ia.input = "按状态分组统计"
	q.ia.generate(opFree)
	if q.ia.orig != "SELECT * FROM users" || !strings.Contains(bodies()[0], "SELECT * FROM users") {
		t.Fatalf("选中的内容应作为要改写的语句: orig=%q", q.ia.orig)
	}
	q.ia.apply()
	if got := q.ed.Buffer.String(); got != wantSQL+";\nSELECT 2" {
		t.Fatalf("替换后只剩一个分号: %q", got)
	}
}

// 生成期间编辑器被改过，应用时按原文重新定位；原文没了就当作插入。
func TestInlineAIRelocatesTheTargetAfterEdits(t *testing.T) {
	e := newEnv(t)
	srv, _ := sqlLLM(t, answerSQL)
	e.useLLM(srv)
	w := openWorkspace(e)
	q := w.newQuery("SELECT * FROM users")
	q.ed.Select(0, len("SELECT * FROM users"))
	q.inline().toggle()
	q.ia.input = "分组"
	q.ia.generate(opFree)
	q.ed.Buffer.Edit(editorInsertAt(0, "-- 备注\n"))
	q.ia.apply()
	if got := q.ed.Buffer.String(); got != "-- 备注\n"+wantSQL {
		t.Fatalf("前面多了内容也应替换对地方: %q", got)
	}

	q2 := w.newQuery("SELECT * FROM users")
	q2.ed.Select(0, len("SELECT * FROM users"))
	q2.inline().toggle()
	q2.ia.input = "分组"
	q2.ia.generate(opFree)
	q2.ed.Buffer.Edit(editorReplaceAll(q2.ed.Buffer.String(), "SELECT 9"))
	q2.ia.apply()
	if got := q2.ed.Buffer.String(); !strings.Contains(got, wantSQL) || !strings.Contains(got, "SELECT 9") {
		t.Fatalf("原文被改没了就插入，不丢用户的内容: %q", got)
	}
}

// 优化针对光标所在的语句；修复需要最近有报错；解释交给侧边对话。
func TestInlineAIQuickActions(t *testing.T) {
	e := newEnv(t)
	srv, bodies := sqlLLM(t, answerSQL)
	e.useLLM(srv)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1;\n\nSELECT * FROM users")
	q.ed.Select(len("SELECT 1;\n\nSEL"), len("SELECT 1;\n\nSEL"))
	ia := q.inline()
	ia.toggle()

	ia.generate(opFix)
	if len(bodies()) != 0 || !strings.Contains(toastText(e), "没有报错") {
		t.Fatalf("没有报错时修复应提示而不是请求模型: %v", toastText(e))
	}
	ia.generate(opOptimize)
	if ia.orig != "SELECT * FROM users" || ia.scope != "针对光标所在的语句" {
		t.Fatalf("优化应针对光标所在的语句: %q / %q", ia.orig, ia.scope)
	}
	if !strings.Contains(bodies()[0], "优化这条语句") {
		t.Fatal("要求里应有优化的说明")
	}

	q.failed = "no such table: usres"
	ia.generate(opFix)
	if !strings.Contains(bodies()[1], "no such table: usres") {
		t.Fatalf("修复应把报错一并交给模型: %s", bodies()[1])
	}

	ia.explain()
	if w.panel != 2 || len(w.ai.msgs) == 0 || !strings.Contains(w.ai.msgs[0].text, "SELECT * FROM users") {
		t.Fatalf("解释应交给侧边对话: panel=%d", w.panel)
	}
}

// 空编辑器、没有目标时：优化给出提示；Esc 关闭栏并把焦点还给编辑器。
func TestInlineAIQuickActionNeedsATargetAndEscCloses(t *testing.T) {
	e := newEnv(t)
	srv, bodies := sqlLLM(t, answerSQL)
	e.useLLM(srv)
	w := openWorkspace(e)
	q := w.newQuery("")
	e.tt.Frame() // 编辑器先画出来，栏之后才打开（和真实使用一致）
	q.inline().toggle()
	q.ia.generate(opOptimize)
	if len(bodies()) != 0 || !strings.Contains(toastText(e), "先选中或写下") {
		t.Fatalf("没有语句时应提示: %v", toastText(e))
	}
	e.tt.Frame()
	e.tt.Frame() // 焦点在栏打开后的下一帧才落到输入框
	e.tt.Key(0, mui.KeyEscape)
	e.tt.Frame()
	if q.ia.open {
		t.Fatal("Esc 应关闭栏")
	}
}

// 模型出错时在栏里显示原因，不动编辑器，也不留下半截结果。
func TestInlineAIShowsErrors(t *testing.T) {
	e := newEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		io.WriteString(w, `{"error":{"message":"bad key"}}`)
	}))
	t.Cleanup(srv.Close)
	e.useLLM(srv)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1")
	q.inline().toggle()
	q.ia.input = "随便写点"
	q.ia.generate(opFree)
	if q.ia.err == "" || q.ia.sql != "" || q.ia.busy || q.ed.Buffer.String() != "SELECT 1" {
		t.Fatalf("%+v", q.ia)
	}
	e.tt.Frame()
	e.has("bad key")
}

func TestMentionedTables(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	if got := mentionedTables("看看 @users 和 orders_2024 的关系，不要 userstats", w.tables); len(got) != 1 || got[0] != "users" {
		t.Fatalf("只认 @表名或整词: %v", got)
	}
	if !hasWord("select * from users where", "users") || hasWord("superusers", "users") || hasWord("users_x", "users") {
		t.Fatal("整词判断有误")
	}
}

func editorInsertAt(at int, text string) editor.Edit {
	return editor.Edit{Start: at, End: at, Text: text}
}

func editorReplaceAll(old, text string) editor.Edit {
	return editor.Edit{Start: 0, End: len(old), Text: text}
}

// toastText 是目前所有提示拼起来的文字。
func toastText(e *env) string {
	var seen []string
	for _, m := range e.sh.toasts {
		seen = append(seen, m.msg)
	}
	return strings.Join(seen, " ")
}
