package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ToolResultKind 结构化工具结果的类型。
type ToolResultKind string

const (
	ResultKindRows ToolResultKind = "rows" // 表格数据：Columns + Rows
	ResultKindText ToolResultKind = "text" // 文本数据：Text
	ResultKindErr  ToolResultKind = "error"
)

// ToolResult 工具执行的结构化结果（前端可原生渲染；模型视图由 FormatForModel 派生）。
type ToolResult struct {
	OK         bool              `json:"ok"`
	Kind       ToolResultKind    `json:"kind"`
	Columns    []string          `json:"columns,omitempty"`
	Rows       []map[string]any  `json:"rows,omitempty"`
	Text       string            `json:"text,omitempty"`
	Truncated  bool              `json:"truncated,omitempty"` // 行数/字节被截断
	DurationMs int64             `json:"durationMs,omitempty"`
	ErrorCode  string            `json:"errorCode,omitempty"`
	SQL        string            `json:"sql,omitempty"`     // 触发本次结果的 SQL（可选）
	ToolName   string            `json:"toolName,omitempty"`
	Data       map[string]any    `json:"data,omitempty"`    // 额外元数据
}

// Error 构造一个失败结果。
func (r ToolResult) Error() *ToolResult {
	r.OK = false
	if r.Kind == "" {
		r.Kind = ResultKindErr
	}
	return &r
}

// RowsResult 构造一个表格结果。
func RowsResult(columns []string, rows []map[string]any, truncated bool) *ToolResult {
	if columns == nil {
		columns = []string{}
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	return &ToolResult{
		OK:        true,
		Kind:      ResultKindRows,
		Columns:   columns,
		Rows:      rows,
		Truncated: truncated,
	}
}

// TextResult 构造一个文本结果。
func TextResult(text string) *ToolResult {
	return &ToolResult{OK: true, Kind: ResultKindText, Text: text}
}

// ToolInput 工具参数（已通过 JSON Schema 校验并归一化）。
type ToolInput map[string]any

// ToolHandler 工具实现。
type ToolHandler func(ctx context.Context, input ToolInput) *ToolResult

// Tool 注册表中的一个工具（单一事实来源）。
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema
	Handler     ToolHandler    `json:"-"`
	ReadOnly    bool           `json:"readOnly"`
	Timeout     time.Duration  `json:"-"`
	ResultKind  string         `json:"resultKind,omitempty"` // 前端提示：rows/text
	Hidden      bool           `json:"hidden,omitempty"`     // 注册但不暴露给 LLM
}

// Picker 从参数中安全提取常用字段。
func (in ToolInput) String(keys ...string) string {
	for _, k := range keys {
		if v, ok := in[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func (in ToolInput) Strings(keys ...string) []string {
	for _, k := range keys {
		if v, ok := in[k]; ok {
			switch val := v.(type) {
			case string:
				if strings.TrimSpace(val) != "" {
					return []string{strings.TrimSpace(val)}
				}
			case []any:
				out := make([]string, 0, len(val))
				for _, item := range val {
					if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
						out = append(out, strings.TrimSpace(s))
					}
				}
				return out
			}
		}
	}
	return nil
}

func (in ToolInput) Int(keys ...string) int {
	for _, k := range keys {
		if v, ok := in[k]; ok {
			switch val := v.(type) {
			case float64:
				return int(val)
			case int:
				return val
			case int64:
				return int(val)
			}
		}
	}
	return 0
}

// ToolRegistry 保存全部工具；是 LLM schemas、前端列表、prompt 的唯一来源。
type ToolRegistry struct {
	mu     sync.RWMutex
	tools  map[string]*Tool
	order  []string
	byName map[string]*Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:  make(map[string]*Tool),
		byName: make(map[string]*Tool),
	}
}

// Register 注册一个工具；同名工具被覆盖。
func (r *ToolRegistry) Register(t Tool) error {
	name := strings.TrimSpace(strings.ToLower(t.Name))
	if name == "" {
		return fmt.Errorf("tool: 名称不能为空")
	}
	if t.Handler == nil {
		return fmt.Errorf("tool %s: Handler 不能为空", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	existing := r.tools[name]
	r.tools[name] = &t
	r.byName[name] = &t
	if existing == nil {
		r.order = append(r.order, name)
	}
	return nil
}

// MustRegister 注册并 panic（用于包级初始化清单）。
func (r *ToolRegistry) MustRegister(t Tool) {
	if err := r.Register(t); err != nil {
		panic(err)
	}
}

// GetByName 按名称取工具（不区分大小写）。
func (r *ToolRegistry) GetByName(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[strings.ToLower(strings.TrimSpace(name))]
	return t, ok
}

// List 按注册顺序返回全部工具。
func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		if t := r.tools[name]; t != nil {
			out = append(out, *t)
		}
	}
	return out
}

// Visible 返回对 LLM 可见的工具（排除 Hidden）。
func (r *ToolRegistry) Visible() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Tool
	for _, name := range r.order {
		if t := r.tools[name]; t != nil && !t.Hidden {
			out = append(out, *t)
		}
	}
	return out
}

// Invoke 校验参数并执行工具；带超时（Tool.Timeout）与结果规范化。
// 校验失败返回结构错误（不给模型“再猜一次”的机会）。
func (r *ToolRegistry) Invoke(ctx context.Context, name string, input ToolInput) *ToolResult {
	t, ok := r.GetByName(name)
	if !ok {
		return (&ToolResult{ToolName: name}).Error()
	}
	if input == nil {
		input = ToolInput{}
	}
	if t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Timeout)
		defer cancel()
	}

	start := time.Now()
	// 参数校验：必填项 + 类型粗校验（JSON Schema 的轻量实现见 validate.go）
	verr := r.Validate(t, input)
	res := &ToolResult{ToolName: t.Name, DurationMs: 0}
	if verr != nil {
		res.OK = false
		res.Kind = ResultKindErr
		res.ErrorCode = "invalid_arguments"
		res.Text = "工具参数无效: " + verr.Error()
		return res
	}

	out := t.Handler(ctx, input)
	if out == nil {
		out = &ToolResult{}
	}
	if out.ToolName == "" {
		out.ToolName = t.Name
	}
	out.OK = out.OK && ctx.Err() == nil
	if !out.OK && out.Kind == "" {
		out.Kind = ResultKindErr
	}
	out.DurationMs = time.Since(start).Milliseconds()
	return out
}

// SuggestionsFrom 从模型正文中提取结构化“下一步建议”按钮（兼容旧 minidb-next-steps 块）。
// Phase3 迁移到结构化输出后将移除正文解析，本函数保持幂等：无块时返回 nil。
func SuggestionsFrom(content string) []Suggestion {
	if content == "" {
		return nil
	}
	re := regexp.MustCompile("(?is)```minidb-next-steps\\s*(\\{[\\s\\S]*?\\})\\s*```")
	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	// 取最后一个块，避免历史片段干扰
	raw := strings.TrimSpace(matches[len(matches)-1][1])
	var payload struct {
		Choices []struct {
			Label  string `json:"label"`
			Prompt string `json:"prompt"`
		} `json:"choices"`
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil
	}
	var out []Suggestion
	for _, c := range payload.Choices {
		label := strings.TrimSpace(c.Label)
		prompt := strings.TrimSpace(c.Prompt)
		if label == "" || prompt == "" || len(label) > 28 || len(prompt) > 120 {
			continue
		}
		out = append(out, Suggestion{Label: label, Prompt: prompt})
		if len(out) >= 4 {
			break
		}
	}
	return out
}

// FormatForModel 把结构化结果派生为模型消费的紧凑文本。
func (r *ToolResult) FormatForModel(maxRows, maxCell int) string {
	if r == nil {
		return "- 空结果"
	}
	switch r.Kind {
	case ResultKindRows:
		if len(r.Rows) == 0 {
			return "- 查询成功，0 行"
		}
		columns := r.Columns
		if len(columns) == 0 && len(r.Rows) > 0 {
			for k := range r.Rows[0] {
				columns = append(columns, k)
			}
			sort.Strings(columns)
		}
		limit := maxRows
		if limit <= 0 || len(r.Rows) < limit {
			limit = len(r.Rows)
		}
		var sb strings.Builder
		trunc := r.Truncated
		sb.WriteString(fmt.Sprintf("- 查询成功，返回 %d 行（展示前 %d 行%s）\n\n", len(r.Rows), limit, truncNote(trunc)))
		sb.WriteString("| " + strings.Join(columns, " | ") + " |\n")
		seps := make([]string, len(columns))
		for i := range seps {
			seps[i] = "---"
		}
		sb.WriteString("| " + strings.Join(seps, " | ") + " |\n")
		for i := 0; i < limit; i++ {
			row := r.Rows[i]
			cells := make([]string, 0, len(columns))
			for _, col := range columns {
				val := row[col]
				cell := "NULL"
				if val != nil {
					cell = strings.TrimSpace(fmt.Sprint(val))
					if cell == "" {
						cell = "''"
					}
				}
				if maxCell > 0 && len(cell) > maxCell {
					cell = cell[:maxCell] + "..."
				}
				cell = strings.ReplaceAll(cell, "\n", " ")
				cell = strings.ReplaceAll(cell, "|", "\\|")
				cells = append(cells, cell)
			}
			sb.WriteString("| " + strings.Join(cells, " | ") + " |\n")
		}
		return strings.TrimSpace(sb.String())
	case ResultKindText:
		return r.Text
	default:
		if r.Text != "" {
			return r.Text
		}
		return "- 工具执行失败"
	}
}

func truncNote(truncated bool) string {
	if truncated {
		return "，结果已被截断"
	}
	return ""
}