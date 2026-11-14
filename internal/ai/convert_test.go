package ai

import (
	"reflect"
	"testing"
)

func TestStrictSchemaFillsDefaults(t *testing.T) {
	got := strictSchema(nil)
	if got["type"] != "object" || got["additionalProperties"] != false {
		t.Fatalf("nil 应得到空对象 schema: %v", got)
	}
	in := map[string]any{"properties": map[string]any{"b": 1, "a": 2}}
	out := strictSchema(in)
	if !reflect.DeepEqual(out["required"], []string{"a", "b"}) || out["type"] != "object" || out["additionalProperties"] != false {
		t.Fatalf("缺 required 时应补成排好序的全部属性: %v", out)
	}
	if _, touched := in["additionalProperties"]; touched {
		t.Fatal("不应改动传入的 map")
	}
}

func TestStrictSchemaKeepsExplicitRequired(t *testing.T) {
	out := strictSchema(map[string]any{"type": "object", "properties": map[string]any{"a": 1}, "required": []string{}})
	if !reflect.DeepEqual(out["required"], []string{}) {
		t.Fatalf("已声明的 required 不应被覆盖: %v", out["required"])
	}
}

func TestBuildChatMessagesSkipsEmptyAndMapsRoles(t *testing.T) {
	msgs := buildChatMessages("sys", []ChatMessage{
		{Role: "user", Content: "  hi "},
		{Role: "assistant", Content: ""},
		{Role: " Assistant ", Content: "ok"},
		{Role: "weird", Content: "x"},
	})
	// 系统提示 + hi + ok + x（空消息被跳过，未知角色按用户处理）
	if len(msgs) != 4 {
		t.Fatalf("消息数 = %d", len(msgs))
	}
	if msgs[0].OfSystem == nil || msgs[1].OfUser == nil || msgs[2].OfAssistant == nil || msgs[3].OfUser == nil {
		t.Fatalf("角色映射不对: %+v", msgs)
	}
}

func TestBuildResponseInputMessagesSkipsEmpty(t *testing.T) {
	in := buildResponseInputMessages([]ChatMessage{{Role: "user", Content: "a"}, {Role: "user", Content: "   "}})
	if len(in) != 1 {
		t.Fatalf("空消息应被跳过: %d", len(in))
	}
}

func TestSortedToolCallsOrdersAndFillsIDs(t *testing.T) {
	calls := sortedToolCalls(map[int64]*FunctionToolCall{
		2: {Name: "b", Arguments: "{}"},
		0: {Name: "a", ID: "id0"},
		1: {Name: "  "}, // 没有名字：丢弃
		3: nil,
	})
	if len(calls) != 2 || calls[0].Name != "a" || calls[1].Name != "b" {
		t.Fatalf("顺序或过滤不对: %+v", calls)
	}
	if calls[0].CallID != "id0" || calls[1].ID == "" || calls[1].CallID != calls[1].ID {
		t.Fatalf("缺失的 ID 应补齐: %+v", calls)
	}
}

func TestChatReasoningDelta(t *testing.T) {
	cases := map[string]string{
		``:                          "",
		`{"reasoning_content":"甲"}`: "甲",
		`{"reasoning":"乙"}`:         "乙",
		`{"thinking":"丙"}`:          "丙",
		`{"reasoning_details":[{"text":"a"},{"summary":"b"}]}`: "ab",
		`{"content":"不是推理"}`:                                   "",
	}
	for raw, want := range cases {
		if got := chatReasoningDelta(raw); got != want {
			t.Errorf("chatReasoningDelta(%s) = %q, want %q", raw, got, want)
		}
	}
}
