package agent

import (
	"context"
	"strings"
	"time"
)

// ToolResultKind 结构化工具结果的类型。
type ToolResultKind string

const (
	ResultKindRows ToolResultKind = "rows" // 表格：Columns + Rows
	ResultKindText ToolResultKind = "text" // 文本
	ResultKindErr  ToolResultKind = "error"
)

// ToolResult 工具执行的结构化结果：界面原生渲染它，模型看到的文本由 FormatForModel 派生。
type ToolResult struct {
	OK         bool             `json:"ok"`
	Kind       ToolResultKind   `json:"kind"`
	Summary    string           `json:"summary,omitempty"`
	Columns    []string         `json:"columns,omitempty"`
	Rows       []map[string]any `json:"rows,omitempty"`
	Text       string           `json:"text,omitempty"`
	Notes      string           `json:"notes,omitempty"` // 补充说明（如索引、关联），界面显示在表格下方
	Hint       string           `json:"hint,omitempty"`
	Truncated  bool             `json:"truncated,omitempty"`
	DurationMs int64            `json:"durationMs,omitempty"`
	ErrorCode  string           `json:"errorCode,omitempty"`
	SQL        string           `json:"sql,omitempty"`
	ToolName   string           `json:"toolName,omitempty"`
	// ModelText 非空时模型看到它而不是对 Rows 的表格渲染：
	// 元数据类工具（表结构、统计）界面要结构化的行，模型要更紧凑的文字。
	ModelText string `json:"-"`
	// TotalRows 完整结果的行数（Rows 被截断时大于 len(Rows)），用于告诉模型"还有多少没看到"。
	TotalRows int64 `json:"-"`
}

// Rows 构造表格结果：summary 是一句话摘要，total 是完整结果的行数（未知填 0）。
func Rows(columns []string, rows []map[string]any, total int64, summary string) *ToolResult {
	if columns == nil {
		columns = []string{}
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	if total < int64(len(rows)) {
		total = int64(len(rows))
	}
	return &ToolResult{OK: true, Kind: ResultKindRows, Columns: columns, Rows: rows, Summary: summary, TotalRows: total, Truncated: total > int64(len(rows))}
}

// Text 构造文本结果。
func Text(summary, text string) *ToolResult {
	return &ToolResult{OK: true, Kind: ResultKindText, Summary: summary, Text: text}
}

// Fail 构造失败结果：message 说明哪里错了，hint 告诉模型下一步怎么改（可为空）。
// 错误码是稳定的机器可读值，供界面与统计使用。
func Fail(code, message, hint string) *ToolResult {
	return &ToolResult{OK: false, Kind: ResultKindErr, ErrorCode: code, Text: message, Hint: hint}
}

// ToolInput 工具参数（已通过 schema 校验并归一化）。
type ToolInput map[string]any

// String 取字符串参数（去空白）；不存在时返回空串。
func (in ToolInput) String(key string) string {
	s, _ := in[key].(string)
	return strings.TrimSpace(s)
}

// Strings 取字符串数组参数（去空白、去空项）。
func (in ToolInput) Strings(key string) []string {
	items, _ := in[key].([]any)
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// Int 取整数参数；不存在时返回 0。
func (in ToolInput) Int(key string) int {
	switch n := in[key].(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

// ToolHandler 工具实现。
type ToolHandler func(ctx context.Context, input ToolInput) *ToolResult

// DefaultToolTimeout 工具未单独指定超时时的上限。
const DefaultToolTimeout = 60 * time.Second

// Tool 注册表中的一个工具，是 LLM 函数定义与界面工具列表的唯一来源。
//
// @author ixxxxoooo
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema（用 Object/String/... 构造）
	Handler     ToolHandler
	ReadOnly    bool
	Timeout     time.Duration
	ResultKind  string // 界面提示：rows | text
}
