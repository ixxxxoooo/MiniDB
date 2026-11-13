package agent

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Validate 按工具的参数 schema 校验入参：必填、未知参数、类型、枚举。
// 错误信息写成"哪里错了 + 怎么改"，让模型下一次调用能改对，而不是盲猜。
func (r *ToolRegistry) Validate(t *Tool, input ToolInput) error {
	props, _ := t.Parameters["properties"].(map[string]any)
	for name := range input {
		if _, ok := props[name]; !ok {
			return fmt.Errorf("未知参数 %q；%s 只接受: %s", name, t.Name, strings.Join(sortedKeys(props), ", "))
		}
	}
	for _, name := range requiredNames(t.Parameters) {
		prop, _ := props[name].(map[string]any)
		if _, ok := input[name]; !ok && !allowsNull(prop) {
			return fmt.Errorf("缺少必填参数 %q（%s）", name, describe(prop))
		}
	}
	for name, v := range input {
		prop, _ := props[name].(map[string]any)
		if err := checkValue(name, prop, v); err != nil {
			return err
		}
	}
	return nil
}

// checkValue 校验单个参数值的类型与枚举。
func checkValue(name string, prop map[string]any, v any) error {
	switch typeOf(prop) {
	case "string":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("参数 %q 必须是字符串，收到 %T", name, v)
		}
		if enum, ok := prop["enum"].([]string); ok && !contains(enum, s) {
			return fmt.Errorf("参数 %q 只能是 %s 之一，收到 %q", name, strings.Join(enum, " / "), s)
		}
	case "integer":
		switch n := v.(type) {
		case int, int64:
		case float64:
			if n != float64(int64(n)) {
				return fmt.Errorf("参数 %q 必须是整数，收到 %v", name, n)
			}
		default:
			return fmt.Errorf("参数 %q 必须是整数，收到 %T", name, v)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("参数 %q 必须是布尔值", name)
		}
	case "array":
		items, ok := v.([]any)
		if !ok {
			return fmt.Errorf("参数 %q 必须是字符串数组，如 [\"a\",\"b\"]", name)
		}
		for _, it := range items {
			if _, ok := it.(string); !ok {
				return fmt.Errorf("参数 %q 的元素必须是字符串", name)
			}
		}
	}
	return nil
}

// NormalizeArguments 规整模型传来的参数：键名 camelCase 转 snake_case；值为 null 的参数视同没传。
func NormalizeArguments(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		if v == nil {
			continue
		}
		out[snakeCase(strings.TrimSpace(k))] = v
	}
	return out
}

// snakeCase 把 tableNames 转成 table_names。
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// typeOf 返回属性的主类型（忽略 null）。
func typeOf(prop map[string]any) string {
	switch t := prop["type"].(type) {
	case string:
		return t
	case []string:
		for _, x := range t {
			if x != "null" {
				return x
			}
		}
	case []any:
		for _, x := range t {
			if s, _ := x.(string); s != "null" {
				return s
			}
		}
	}
	return ""
}

// allowsNull 报告属性是否声明为可选（类型里带 null）。
func allowsNull(prop map[string]any) bool {
	switch t := prop["type"].(type) {
	case []string:
		return contains(t, "null")
	case []any:
		for _, x := range t {
			if s, _ := x.(string); s == "null" {
				return true
			}
		}
	}
	return false
}

// requiredNames 取 schema 的 required 列表。
func requiredNames(schema map[string]any) []string {
	switch req := schema["required"].(type) {
	case []string:
		return req
	case []any:
		out := make([]string, 0, len(req))
		for _, v := range req {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// describe 取属性的说明文字，用于缺参提示。
func describe(prop map[string]any) string {
	if d, _ := prop["description"].(string); d != "" {
		return d
	}
	return typeOf(prop)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
