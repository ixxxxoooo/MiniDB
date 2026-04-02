package database

import (
	"encoding/json"
	"regexp"
	"strings"
)

var reMarkdownSQLFence = regexp.MustCompile("(?i)```sql\\s*\\n([\\s\\S]*?)```")
var reAutoExecuteIntent = regexp.MustCompile(`(?i)(直接返回|返回|给我|帮我).*(结果|数据)|执行(这条|该)?sql|直接查(一下|出)|帮我查(一下|出来)?|查看(一下)?(结果|数据)`)

// AutoExecuteIntentType 描述当前用户意图分类，供是否自动执行决策使用。
type AutoExecuteIntentType string

const (
	AutoExecuteIntentUnknown      AutoExecuteIntentType = "unknown"
	AutoExecuteIntentNeedResults  AutoExecuteIntentType = "need_results"
	AutoExecuteIntentGenerateOnly AutoExecuteIntentType = "generate_sql_only"
	AutoExecuteIntentExplainOnly  AutoExecuteIntentType = "explain_only"
	AutoExecuteIntentNoExecute    AutoExecuteIntentType = "no_execute"
)

// AutoExecuteIntentMetaBlock AI 可选输出的结构化元数据块。仅服务端消费，不展示给用户。
type AutoExecuteIntentMetaBlock struct {
	AutoExecute struct {
		Enabled bool   `json:"enabled"`
		Mode    string `json:"mode,omitempty"`
		Reason  string `json:"reason,omitempty"`
	} `json:"autoExecute"`
}

// ExtractFirstSQLFenceFromMarkdown 从 Markdown 中提取首个 ```sql 代码块内容
func ExtractFirstSQLFenceFromMarkdown(md string) (sql string, ok bool) {
	m := reMarkdownSQLFence.FindStringSubmatch(md)
	if len(m) < 2 {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

// ExtractAutoExecuteMetaBlock 从回复中提取 AI 结构化元数据块，并返回剥离后的正文。
// 支持格式：```tableplus-ai-meta {"autoExecute":{...}} ```
func ExtractAutoExecuteMetaBlock(text string) (meta AutoExecuteIntentMetaBlock, cleaned string, ok bool) {
	re := regexp.MustCompile("(?is)```tableplus-ai-meta\\s*(\\{[\\s\\S]*?\\})\\s*```")
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return meta, strings.TrimSpace(text), false
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &meta); err != nil {
		return AutoExecuteIntentMetaBlock{}, strings.TrimSpace(text), false
	}
	cleaned = re.ReplaceAllString(text, "")
	cleaned = regexp.MustCompile(`\n{3,}`).ReplaceAllString(cleaned, "\n\n")
	return meta, strings.TrimSpace(cleaned), true
}

// StripAutoExecuteTag 移除 [AUTO_EXECUTE] 标记并整理多余空行
func StripAutoExecuteTag(text string) string {
	s := regexp.MustCompile(`(?i)\[AUTO_EXECUTE\]`).ReplaceAllString(text, "")
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// DetectAutoExecuteIntent 识别用户本轮意图：要结果 / 仅生成 SQL / 仅解释 / 明确不要执行 / 未知。
func DetectAutoExecuteIntent(lastUserMessage string) AutoExecuteIntentType {
	userText := strings.TrimSpace(lastUserMessage)
	if userText == "" {
		return AutoExecuteIntentUnknown
	}
	lower := strings.ToLower(userText)
	if strings.Contains(lower, "不要执行") || strings.Contains(lower, "别执行") || strings.Contains(lower, "仅生成") || strings.Contains(lower, "只生成") || strings.Contains(lower, "不要跑") {
		return AutoExecuteIntentNoExecute
	}
	if (strings.Contains(lower, "解释") || strings.Contains(lower, "说明")) && strings.Contains(lower, "sql") {
		return AutoExecuteIntentExplainOnly
	}
	if strings.Contains(lower, "生成") && strings.Contains(lower, "sql") {
		return AutoExecuteIntentGenerateOnly
	}
	if reAutoExecuteIntent.MatchString(userText) {
		return AutoExecuteIntentNeedResults
	}
	return AutoExecuteIntentUnknown
}

// WantsAutoExecuteFromConversation 根据最近一轮用户问题与助手回复，判断是否应该触发自动执行。
// 仅当：用户明确要结果/要直接执行 + 助手回复存在 SQL 代码块，且不是仅要求“生成 SQL”，才返回 true。
func WantsAutoExecuteFromConversation(lastUserMessage, assistantMarkdown string) bool {
	if _, ok := ExtractFirstSQLFenceFromMarkdown(assistantMarkdown); !ok {
		return false
	}
	return DetectAutoExecuteIntent(lastUserMessage) == AutoExecuteIntentNeedResults
}

// AutoExecutableCheckResult 自动执行 SQL 的安全校验结果（reasonCode 供前端 i18n）
type AutoExecutableCheckResult struct {
	Allowed    bool   `json:"allowed"`
	ReasonCode string `json:"reasonCode,omitempty"` // empty_sql, risky_sql, unknown_sql
	Verb       string `json:"verb,omitempty"`
}

// CheckAutoExecutableSelectSQL 判断是否允许 AI 自动执行（仅允许只读类语句）
func CheckAutoExecutableSelectSQL(sql string) AutoExecutableCheckResult {
	verb := SQLLeadingVerb(sql)
	if verb == "" {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "empty_sql"}
	}
	allow := map[string]struct{}{
		"select": {}, "show": {}, "desc": {}, "describe": {}, "explain": {}, "with": {},
	}
	if _, ok := allow[verb]; ok {
		return AutoExecutableCheckResult{Allowed: true}
	}
	risky := map[string]struct{}{
		"insert": {}, "update": {}, "delete": {}, "replace": {}, "create": {}, "alter": {}, "drop": {}, "truncate": {}, "rename": {},
		"grant": {}, "revoke": {}, "call": {}, "set": {}, "use": {}, "begin": {}, "start": {}, "commit": {}, "rollback": {}, "lock": {}, "unlock": {},
	}
	if _, ok := risky[verb]; ok {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "risky_sql", Verb: strings.ToUpper(verb)}
	}
	return AutoExecutableCheckResult{Allowed: false, ReasonCode: "unknown_sql", Verb: strings.ToUpper(verb)}
}
