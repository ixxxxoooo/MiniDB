package agent

import (
	"fmt"
	"strings"
)

// Validate 对工具参数做轻量 JSON Schema 校验（必填 + 类型粗校验）。
// 产物为结构化错误，不向模型暴露细节的“重试一次”机会。
func (r *ToolRegistry) Validate(t *Tool, input ToolInput) error {
	schema, ok := t.Parameters["properties"].(map[string]any)
	if !ok {
		schema = map[string]any{}
	}
	// required 数组
	var required []string
	if req, ok := t.Parameters["required"].([]string); ok {
		required = req
	} else if reqAny, ok := t.Parameters["required"].([]any); ok {
		for _, v := range reqAny {
			if s, ok := v.(string); ok {
				required = append(required, s)
			}
		}
	}
	for _, name := range required {
		if _, ok := input[name]; !ok {
			return fmt.Errorf("缺少必填参数: %s", name)
		}
	}
	for name, raw := range input {
		propSchema, ok := schema[name].(map[string]any)
		if !ok {
			continue
		}
		typ, _ := propSchema["type"].(string)
		if typ == "" {
			continue
		}
		if err := checkType(name, typ, raw); err != nil {
			return err
		}
	}
	return nil
}

func checkType(name, typ string, raw any) error {
	switch typ {
	case "string":
		if _, ok := raw.(string); !ok {
			return fmt.Errorf("参数 %s 必须是字符串", name)
		}
	case "integer":
		switch raw.(type) {
		case float64, int, int64:
		default:
			return fmt.Errorf("参数 %s 必须是整数", name)
		}
	case "boolean":
		if _, ok := raw.(bool); !ok {
			return fmt.Errorf("参数 %s 必须是布尔值", name)
		}
	case "array":
		if _, ok := raw.([]any); !ok {
			return fmt.Errorf("参数 %s 必须是数组", name)
		}
	}
	return nil
}

// Normalize 归一化已知别名（兼容旧工具参数命名），避免模型传错 key 时静默失败。
func NormalizeArguments(raw map[string]any) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(raw))
	alias := map[string]string{
		"tableName":    "table_name",
		"table_name":   "table_name",
		"tableNames":   "table_names",
		"table_names":  "table_names",
		"tables":       "table_names",
		"columns":      "columns",
		"columnNames":  "columns",
		"column_names": "columns",
		"top_k":        "limit",
		"topK":         "limit",
		"limit":        "limit",
	}
	for k, v := range raw {
		key := strings.TrimSpace(k)
		canon, hasAlias := alias[key]
		if hasAlias {
			out[canon] = v
		} else {
			out[key] = v
		}
	}
	return out
}
