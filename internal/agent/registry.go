package agent

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"minidb/internal/logger"
)

// ToolRegistry 保存全部工具，按注册顺序排列。
//
// @author ixxxxoooo
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
	order []string
}

// NewToolRegistry 创建空注册表。
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: map[string]*Tool{}}
}

// Register 注册一个工具；同名覆盖。
func (r *ToolRegistry) Register(t Tool) error {
	name := strings.ToLower(strings.TrimSpace(t.Name))
	if name == "" {
		return fmt.Errorf("tool: 名称不能为空")
	}
	if t.Handler == nil {
		return fmt.Errorf("tool %s: Handler 不能为空", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = &t
	return nil
}

// MustRegister 注册失败时 panic（工具清单是写死的，出错是编程错误）。
func (r *ToolRegistry) MustRegister(t Tool) {
	if err := r.Register(t); err != nil {
		panic(err)
	}
}

// Get 按名称取工具（不区分大小写）。
func (r *ToolRegistry) Get(name string) (*Tool, bool) {
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
		out = append(out, *r.tools[name])
	}
	return out
}

// Invoke 校验参数并执行工具：带超时、panic 保护，结果统一补全耗时与工具名。
// 参数不合法、工具不存在、被取消都返回带错误码的失败结果，而不是 error。
func (r *ToolRegistry) Invoke(ctx context.Context, name string, input ToolInput) (res *ToolResult) {
	start := time.Now()
	defer func() {
		res.ToolName = strings.ToLower(strings.TrimSpace(name))
		res.DurationMs, res.DurationUs = time.Since(start).Milliseconds(), time.Since(start).Microseconds()
	}()

	t, ok := r.Get(name)
	if !ok {
		names := make([]string, 0, len(r.order))
		for _, tool := range r.List() {
			names = append(names, tool.Name)
		}
		return Fail("unknown_tool", fmt.Sprintf("没有名为 %q 的工具", name), "可用工具: "+strings.Join(names, ", "))
	}
	// 助手只被允许调用只读工具：新增工具忘了声明 ReadOnly 时，宁可调不通，也不能默默放行
	if !t.ReadOnly {
		return Fail("not_read_only", fmt.Sprintf("工具 %s 没有声明为只读，助手不能调用", t.Name), "")
	}
	if input == nil {
		input = ToolInput{}
	}
	if err := r.Validate(t, input); err != nil {
		return Fail("invalid_arguments", err.Error(), "按工具的参数说明修正后重新调用")
	}
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = DefaultToolTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	defer func() {
		if p := recover(); p != nil {
			logger.Error("[Agent] 工具 %s panic: %v\n%s", t.Name, p, debug.Stack())
			res = Fail("internal", fmt.Sprintf("工具 %s 内部出错", t.Name), "")
		}
	}()
	out := t.Handler(ctx, input)
	switch {
	case out == nil:
		return Fail("internal", "工具没有返回结果", "")
	case ctx.Err() == context.DeadlineExceeded:
		return Fail("timeout", fmt.Sprintf("工具执行超过 %s 被中止", timeout), "缩小范围（加 WHERE / LIMIT）后重试")
	case ctx.Err() != nil:
		return Fail("cancelled", "已取消", "")
	}
	return out
}
