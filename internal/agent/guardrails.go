package agent

import (
	"context"
	"strings"

	"minidb/internal/database"
)

// GuardrailChain 是执行器上下文的护栏集合。
// 只读 SQL 的判定统一委托给 database.CheckReadOnlySQL，避免出现两套规则不一致的实现。
type GuardrailChain struct{}

// NewDefaultGuardrailChain 返回默认护栏（面向文本转 SQL Agent）。
func NewDefaultGuardrailChain() *GuardrailChain {
	return &GuardrailChain{}
}

// CheckReadOnlySQL 对一句完整的 SQL 做只读校验，返回拒绝原因（denied=false 表示放行）。
// reason: empty_sql / multi_sql / unknown_sql / non_readonly_sql / danger_function
func (g *GuardrailChain) CheckReadOnlySQL(sql string) (denied bool, reason, verb string) {
	check := database.CheckReadOnlySQL(sql)
	verb = strings.ToLower(check.Verb)
	if check.Allowed {
		return false, "", verb
	}
	reason = check.ReasonCode
	if reason == "risky_sql" || (reason == "unknown_sql" && verb != "") {
		reason = "non_readonly_sql"
	}
	return true, reason, verb
}

// BuildReadOnlyGuard 返回一个可注入 Executor 的护栏函数：拦截任何非只读 SQL 工具调用。
func (g *GuardrailChain) BuildReadOnlyGuard() GuardFn {
	return func(ctx context.Context, rc *RunContext, call StepCall) GuardResult {
		name := strings.ToLower(strings.TrimSpace(call.Name))
		if name == "sql_readonly_execute" || name == "sql_explain_plan" {
			sqlText := ""
			if v, ok := rc.ToolArgs["sql"]; ok {
				if str, ok := v.(string); ok {
					sqlText = str
				}
			}
			denied, reason, verb := g.CheckReadOnlySQL(sqlText)
			if denied {
				return GuardResult{
					Denied: true,
					Code:   "guarded_" + reason,
					Reason: "只读 SQL 校验失败: " + reason + " (verb=" + verb + ")",
				}
			}
		}
		return GuardResult{}
	}
}
