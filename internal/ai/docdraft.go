package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// @author ixxxxoooo
//
// 表文档页的「AI 补全注释」：让模型给缺注释的列各写一句说明，必要时再给整张表写一句概述。
// 结果只是草稿，由界面带「AI」标记显示，不会写进数据库。

// DocDraft 是模型给出的草稿：Description 是表的一句话概述（没要求时为空），Columns 是 列名 → 一句说明。
type DocDraft struct {
	Description string            `json:"description"`
	Columns     map[string]string `json:"columns"`
}

const docDraftSystemPrompt = `你是数据库文档助手。根据表结构，为缺少注释的字段各写一句简短的中文说明（不超过 20 个字，不要重复字段名），必要时再为整张表写一句概述。
只输出一个 JSON 对象，不要任何其他文字：{"description": "表的一句话概述，没有被要求时留空", "columns": {"列名": "一句说明"}}`

// GenerateDocDraft 为 tableName 生成草稿。参数：need 是要补注释的列名；needDesc 为真时同时要表概述。
// 返回值：草稿（只含被要求的列）；表不存在或模型输出不是 JSON 时返回错误。
func (c *Client) GenerateDocDraft(ctx context.Context, schema *SchemaContext, tableName string, need []string, needDesc bool) (*DocDraft, error) {
	var ts *TableSchema
	for i := range schema.Tables {
		if schema.Tables[i].Name == tableName {
			ts = &schema.Tables[i]
			break
		}
	}
	if ts == nil {
		return nil, fmt.Errorf("表 %s 不存在", tableName)
	}
	ask := "需要补注释的字段：" + strings.Join(need, "、")
	if needDesc {
		ask += "\n同时需要表的一句话概述。"
	} else {
		ask += "\n不需要表概述，description 留空。"
	}
	msg := fmt.Sprintf("数据库类型: %s\n\n表结构:\n%s\n\n%s", schema.DatabaseType, BuildTablesDDL([]TableSchema{*ts}), ask)
	out, err := c.Chat(ctx, docDraftSystemPrompt, msg)
	if err != nil {
		return nil, err
	}
	return parseDocDraft(out, need)
}

// parseDocDraft 从模型输出里取出 JSON 对象（容忍前后的说明文字和 ``` 围栏），并只保留被要求的列。
// 参数：raw 是模型原始输出；need 是被要求的列名。返回值：草稿；找不到合法 JSON 时返回错误。
func parseDocDraft(raw string, need []string) (*DocDraft, error) {
	i, j := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if i < 0 || j < i {
		return nil, fmt.Errorf("AI 没有返回 JSON")
	}
	var d DocDraft
	if err := json.Unmarshal([]byte(raw[i:j+1]), &d); err != nil {
		return nil, fmt.Errorf("AI 返回的 JSON 无法解析: %w", err)
	}
	want := map[string]bool{}
	for _, n := range need {
		want[n] = true
	}
	cols := map[string]string{}
	for k, v := range d.Columns {
		if v = strings.TrimSpace(v); want[k] && v != "" {
			cols[k] = v
		}
	}
	return &DocDraft{Description: strings.TrimSpace(d.Description), Columns: cols}, nil
}
