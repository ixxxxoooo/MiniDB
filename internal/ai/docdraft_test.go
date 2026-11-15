package ai

import "testing"

// 模型输出常带围栏和多余文字；只保留被要求的列，空说明丢掉。
func TestParseDocDraft(t *testing.T) {
	raw := "好的：\n```json\n{\"description\":\" 用户表 \",\"columns\":{\"age\":\"年龄\",\"extra\":\"没要\",\"name\":\"  \"}}\n```"
	d, err := parseDocDraft(raw, []string{"age", "name"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Description != "用户表" || len(d.Columns) != 1 || d.Columns["age"] != "年龄" {
		t.Fatalf("解析结果不对: %+v", d)
	}
	if _, err := parseDocDraft("没有 json", nil); err == nil {
		t.Fatal("没有 JSON 应返回错误")
	}
	if _, err := parseDocDraft("{坏的}", nil); err == nil {
		t.Fatal("坏 JSON 应返回错误")
	}
}
