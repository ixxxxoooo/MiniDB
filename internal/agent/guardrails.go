package agent

import (
	"context"
	"regexp"
	"strings"
)

// GuardrailChain 是执行器上下文的护栏集合。
// 当前实现聚焦 Agent 最关键的护栏：只读 SQL 语句分类 + 危险调用拒绝。
type GuardrailChain struct {
	// DangerFunctions 命中即拒绝的数据库函数/关键字（按小写子串匹配，覆盖 MySQL/PG/SQLite）。
	// 例：SLEEP( / PG_SLEEP( / LOAD_FILE( / SHUTDOWN / BENCHMARK( / OUTFILE / DUMPFILE
	DangerFunctions []string

	// AllowVerbs 只读语句的允许动词（SQLLeadingVerb 结果）。
	AllowVerbs []string

	// StatementTimeout 每条语句的硬超时（由执行器透传给 DB 层）。
}

// NewDefaultGuardrailChain 返回默认护栏（面向文本转 SQL Agent）。
func NewDefaultGuardrailChain() *GuardrailChain {
	return &GuardrailChain{
		DangerFunctions: []string{
			"sleep(", "pg_sleep(", "benchmark(", "load_file(", "shutdown", "dumpfile", "outfile",
			"master_pos_wait(", "get_lock(", "release_lock(", "sys_exec", "xp_cmdshell",
		},
		AllowVerbs: []string{"select", "show", "desc", "describe", "explain", "with"},
	}
}

// CheckReadOnlySQL 对一句完整的 SQL 做多道检查，返回拒绝原因（空 = 放行）。
// 检查顺序：空→多语句→动词→危险函数。
func (g *GuardrailChain) CheckReadOnlySQL(sql string) (denied bool, reason, verb string) {
	s := strings.TrimSpace(sql)
	if s == "" {
		return true, "empty_sql", ""
	}
	// 多语句（分号后还有内容）
	if !isSingleStatement(s) {
		return true, "multi_sql", firstVerb(s)
	}
	verb = firstVerb(s)
	if verb == "" {
		return true, "unknown_sql", ""
	}
	allowed := false
	for _, v := range g.AllowVerbs {
		if v == verb {
			allowed = true
			break
		}
	}
	if !allowed {
		return true, "non_readonly_sql", verb
	}
	lower := strings.ToLower(s)
	for _, d := range g.DangerFunctions {
		if strings.Contains(lower, d) {
			return true, "danger_function", verb
		}
	}
	return false, "", verb
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

var reLeadingWord = regexp.MustCompile(`^[a-zA-Z]+`)

func firstVerb(sql string) string {
	s := strings.TrimSpace(sql)
	if s == "" {
		return ""
	}
	m := reLeadingWord.FindString(s)
	if m == "" {
		return ""
	}
	return strings.ToLower(m)
}

// isSingleStatement 判定单条语句：分号后不允许再有内容（引号内分号忽略）。
func isSingleStatement(sql string) bool {
	inSingle := false
	inDouble := false
	escaped := false
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return false
	}
	for i, r := range trimmed {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inSingle {
			escaped = true
			continue
		}
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ';':
			if !inSingle && !inDouble && strings.TrimSpace(trimmed[i+1:]) != "" {
				return false
			}
		}
	}
	return true
}
