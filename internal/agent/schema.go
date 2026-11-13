package agent

import "sort"

// Prop 是工具参数 schema 里的一个属性。
type Prop map[string]any

// String 字符串参数。
func String(desc string) Prop { return Prop{"type": "string", "description": desc} }

// Integer 整数参数，min/max 写进 schema 给模型看；越界值由处理函数夹到范围内，不会报错。
func Integer(desc string, min, max int) Prop {
	return Prop{"type": "integer", "description": desc, "minimum": min, "maximum": max}
}

// StringList 字符串数组参数。
func StringList(desc string, maxItems int) Prop {
	p := Prop{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
	if maxItems > 0 {
		p["maxItems"] = maxItems
	}
	return p
}

// Enum 枚举字符串参数。
func Enum(desc string, values ...string) Prop {
	return Prop{"type": "string", "description": desc, "enum": values}
}

// Optional 把参数声明为可选：类型加上 null。strict 模式要求每个属性都出现在 required 里，
// 可选参数的正确写法就是"必填但允许 null"，模型传 null（或干脆省略）都表示没给。
func Optional(p Prop) Prop {
	out := Prop{}
	for k, v := range p {
		out[k] = v
	}
	out["type"] = []string{p["type"].(string), "null"}
	return out
}

// Object 构造工具的参数 schema：禁止多余字段，所有属性都进 required（可选参数用 Optional 声明）。
func Object(props map[string]Prop) map[string]any {
	properties := make(map[string]any, len(props))
	required := make([]string, 0, len(props))
	for name, p := range props {
		properties[name] = map[string]any(p)
		required = append(required, name)
	}
	sort.Strings(required)
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}
