package agent

import "strings"

// PartType 一条助手回复里的片段类型。
type PartType string

const (
	PartThinking PartType = "thinking" // 推理摘要
	PartText     PartType = "text"     // 模型文本（工具之间的说明，或最后的回答）
	PartTool     PartType = "tool"     // 一次工具调用及其结果
)

// Part 助手回复的一个片段。片段按真实发生的顺序排列，与对话界面的渲染顺序一致。
type Part struct {
	Type PartType  `json:"type"`
	Text string    `json:"text,omitempty"`
	Tool *ToolPart `json:"tool,omitempty"`
}

// ToolPart 一次工具调用：参数在调用时就有，结果到达前处于"运行中"。
type ToolPart struct {
	CallID string           `json:"callId"`
	Name   string           `json:"name"`
	Args   map[string]any   `json:"args,omitempty"`
	Result *ToolResultEvent `json:"result,omitempty"`
}

// Running 工具已发起、结果还没回来。
func (p *ToolPart) Running() bool { return p.Result == nil }

// Failed 工具已返回且失败。
func (p *ToolPart) Failed() bool { return p.Result != nil && !p.Result.OK }

// Transcript 把一次 run 的事件流折叠成有序片段与终态。
// 服务端用它持久化会话，界面用它渲染对话，两边共用同一份折叠逻辑，所以恢复的历史会话与实时看到的一模一样。
//
// @author ixxxxoooo
type Transcript struct {
	Parts       []Part       `json:"parts"`
	Done        bool         `json:"done,omitempty"`
	Error       string       `json:"error,omitempty"`
	ErrorCode   string       `json:"errorCode,omitempty"`
	Stopped     bool         `json:"stopped,omitempty"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
	Usage       Usage        `json:"usage"`
	Rounds      int          `json:"rounds,omitempty"`
}

// Apply 把一个事件折叠进 Transcript。
func (t *Transcript) Apply(ev Event) {
	switch ev.Type {
	case EventStepThinking:
		if delta, _ := ev.Payload.(string); delta != "" {
			t.appendText(PartThinking, delta)
		}
	case EventAnswerDelta:
		if delta, _ := ev.Payload.(string); delta != "" {
			t.appendText(PartText, delta)
		}
	case EventToolRequested:
		if call, ok := ev.Payload.(ToolCall); ok {
			t.Parts = append(t.Parts, Part{Type: PartTool, Tool: &ToolPart{CallID: call.CallID, Name: call.ToolName, Args: call.Arguments}})
		}
	case EventToolResult:
		if res, ok := ev.Payload.(ToolResultEvent); ok {
			t.setResult(res)
		}
	case EventRunDone:
		if d, ok := ev.Payload.(RunDone); ok {
			t.finish(d)
		}
	case EventRunError:
		if e, ok := ev.Payload.(RunError); ok {
			t.fail(e)
		}
	}
}

// appendText 往末尾同类型片段追加文字；类型变了就开一个新片段。
func (t *Transcript) appendText(typ PartType, delta string) {
	if n := len(t.Parts); n > 0 && t.Parts[n-1].Type == typ {
		t.Parts[n-1].Text += delta
		return
	}
	t.Parts = append(t.Parts, Part{Type: typ, Text: delta})
}

// setResult 把结果挂到对应调用上；找不到调用（事件乱序或丢失）时补一个，结果不丢。
func (t *Transcript) setResult(res ToolResultEvent) {
	for i := len(t.Parts) - 1; i >= 0; i-- {
		if p := t.Parts[i].Tool; p != nil && p.CallID == res.CallID {
			p.Result = &res
			return
		}
	}
	t.Parts = append(t.Parts, Part{Type: PartTool, Tool: &ToolPart{CallID: res.CallID, Name: res.ToolName, Result: &res}})
}

// finish 收尾：最终回答以 RunDone.Content 为准（已去掉建议块），替换流式累积的结尾文字。
func (t *Transcript) finish(d RunDone) {
	t.Done, t.Suggestions, t.Usage, t.Rounds = true, d.Suggestions, d.Usage, d.Rounds
	switch n := len(t.Parts); {
	case d.Content == "":
		if n > 0 && t.Parts[n-1].Type == PartText {
			t.Parts[n-1].Text, _ = SplitSuggestions(t.Parts[n-1].Text)
		}
	case n > 0 && t.Parts[n-1].Type == PartText:
		t.Parts[n-1].Text = d.Content
	default:
		t.Parts = append(t.Parts, Part{Type: PartText, Text: d.Content})
	}
}

// fail 记录失败；用户主动停止不算错误，只标记 Stopped。
func (t *Transcript) fail(e RunError) {
	t.Done = true
	if e.Code == ErrCodeCancelled {
		t.Stopped = true
		return
	}
	t.Error, t.ErrorCode = e.Message, e.Code
}

// Answer 返回最终回答：最后一个片段是文本时它就是回答；
// 工具调用之后还没有文字（或根本没有文字）则为空。
func (t *Transcript) Answer() string {
	if n := len(t.Parts); n > 0 && t.Parts[n-1].Type == PartText {
		return strings.TrimSpace(t.Parts[n-1].Text)
	}
	return ""
}

// Compact 返回适合持久化的副本：工具结果的行数、列数、单元格长度都有上限，
// 一次返回几千行的查询不会把会话存储撑大。
func (t *Transcript) Compact(maxRows, maxCell int) *Transcript {
	out := *t
	out.Parts = make([]Part, len(t.Parts))
	for i, p := range t.Parts {
		out.Parts[i] = p
		if p.Tool == nil || p.Tool.Result == nil {
			continue
		}
		tool := *p.Tool
		res := *p.Tool.Result
		if len(res.Rows) > maxRows {
			res.Rows, res.Truncated = res.Rows[:maxRows], true
		}
		rows := make([]map[string]any, len(res.Rows))
		for r, row := range res.Rows {
			rows[r] = make(map[string]any, len(row))
			for k, v := range row {
				rows[r][k] = clipValue(v, maxCell)
			}
		}
		res.Rows = rows
		res.Text = clipRunes(res.Text, 4000)
		tool.Result = &res
		out.Parts[i].Tool = &tool
	}
	return &out
}

// clipValue 把过长的字符串单元格截短，其余值原样保留。
func clipValue(v any, maxCell int) any {
	if s, ok := v.(string); ok {
		return clipRunes(s, maxCell)
	}
	return v
}

// clipRunes 按字符数截断，超出时以省略号收尾。
func clipRunes(s string, max int) string {
	if r := []rune(s); max > 0 && len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}
