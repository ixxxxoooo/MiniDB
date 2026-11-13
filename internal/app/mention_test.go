package app

import (
	"reflect"
	"strings"
	"testing"

	"minidb/internal/ai"
)

func TestFindMentions(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"看看 @users 和 @orders 的关系", []string{"users", "orders"}},
		{"@users", []string{"users"}},
		{"联系 a@b.com 或 @users。", []string{"users"}}, // 邮箱里的 @ 不算；句号不是名字的一部分
		{"@shop.orders 里有多少行", []string{"shop.orders"}},
		{"表@users", []string{"users"}},                // 中文紧挨着 @ 也算
		{"@ users 和 @", nil},                          // 单独的 @ 不是引用
		{"@@users @_t1 @t$x", []string{"_t1", "t$x"}}, // 连着的 @@ 不算；下划线、$ 是名字字符
		{"请 @用户表 看看", []string{"用户表"}},
	}
	for _, c := range cases {
		var got []string
		for _, m := range FindMentions(c.in) {
			got = append(got, m.Name)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("FindMentions(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFindMentionsGivesRuneRangesIncludingTheAt(t *testing.T) {
	text := "查 @users 吧"
	ms := FindMentions(text)
	if len(ms) != 1 || ms[0].Start != 2 || ms[0].End != 8 || string([]rune(text)[ms[0].Start:ms[0].End]) != "@users" {
		t.Fatalf("%+v", ms)
	}
}

func TestMentionNamesDedupesIgnoringCase(t *testing.T) {
	if got := MentionNames("@Users @orders @users"); !reflect.DeepEqual(got, []string{"Users", "orders"}) {
		t.Fatal(got)
	}
}

func TestResolveMentionsOrdersLatestFirstAndReportsMissing(t *testing.T) {
	schema := &ai.SchemaContext{Tables: []ai.TableSchema{{Name: "users"}, {Name: "orders"}, {Name: "items"}}}
	msgs := []ai.ChatMessage{
		{Role: "user", Content: "看 @users"},
		{Role: "assistant", Content: "好的 @items"}, // 助手说的不算
		{Role: "user", Content: "再看 @shop.ORDERS 和 @nope"},
	}
	tables, missing := resolveMentions(schema, msgs)
	var names []string
	for _, tb := range tables {
		names = append(names, tb.Name)
	}
	if !reflect.DeepEqual(names, []string{"orders", "users"}) || !reflect.DeepEqual(missing, []string{"nope"}) {
		t.Fatalf("tables=%v missing=%v", names, missing)
	}
	var many []ai.ChatMessage
	for i := 0; i < 10; i++ {
		schema.Tables = append(schema.Tables, ai.TableSchema{Name: "t" + string(rune('a'+i))})
		many = append(many, ai.ChatMessage{Role: "user", Content: "@t" + string(rune('a'+i))})
	}
	if got, _ := resolveMentions(schema, many); len(got) != mentionedTableLimit {
		t.Fatalf("最多 %d 张: %d", mentionedTableLimit, len(got))
	}
}

func TestSystemPromptPutsMentionedTablesLast(t *testing.T) {
	p := buildSystemPrompt(promptInfo{dbType: "sqlite", dbName: "d", overview: "- users: id", mentioned: "CREATE TABLE orders (...)", missing: []string{"nope"}})
	i, j := strings.Index(p, "库结构总览"), strings.Index(p, "用户用 @ 点名的表")
	if i < 0 || j < i || !strings.HasSuffix(strings.TrimSpace(p), "如实告诉用户。）") || !strings.Contains(p, "CREATE TABLE orders") || !strings.Contains(p, "@nope") {
		t.Fatalf("点名的表要放在总览之后、整段最末：\n%s", p)
	}
	if plain := buildSystemPrompt(promptInfo{dbType: "sqlite", overview: "x"}); strings.Contains(plain, "点名") {
		t.Fatal("没点名就不该出现这一段")
	}
}

// 对话里 @ 点名的表，完整结构会进到发给模型的系统提示词里。
func TestChatSendsMentionedTablesToTheModel(t *testing.T) {
	h := newAIHarness(t)
	srv, bodies := gateway(t, func(n int, body string, s *sse) { s.text("好的"); s.done("r") })
	h.useGateway(t, srv.URL)
	_, err := h.a.AI.ChatAIStream(h.connID, h.db, []ai.ChatMessage{{Role: "user", Content: "@order_items 里每个 sku 卖了多少？顺便看看 @ghost"}}, "m-1", "")
	if err != nil {
		t.Fatal(err)
	}
	body := (*bodies)[0]
	for _, want := range []string{"用户用 @ 点名的表", "CREATE TABLE order_items", "@ghost"} {
		if !strings.Contains(body, want) {
			t.Fatalf("请求里缺少 %q", want)
		}
	}
}
