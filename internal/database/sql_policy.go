package database

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ExtractAutoExecuteMetaBlock 从回复中提取 AI 旧版结构化元数据块（minidb-meta），并返回剥离后的正文。
// 兼容旧链路残留；新 Agent 协议不再要求模型输出该块。
func ExtractAutoExecuteMetaBlock(text string) (meta struct {
	AutoExecute struct {
		Enabled bool   `json:"enabled"`
		Mode    string `json:"mode,omitempty"`
		Reason  string `json:"reason,omitempty"`
	} `json:"autoExecute"`
}, cleaned string, ok bool) {
	re := regexp.MustCompile("(?is)```minidb-meta\\s*(\\{[\\s\\S]*?\\})\\s*```")
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return meta, strings.TrimSpace(text), false
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &meta); err != nil {
		return struct {
			AutoExecute struct {
				Enabled bool   `json:"enabled"`
				Mode    string `json:"mode,omitempty"`
				Reason  string `json:"reason,omitempty"`
			} `json:"autoExecute"`
		}{}, strings.TrimSpace(text), false
	}
	cleaned = re.ReplaceAllString(text, "")
	cleaned = regexp.MustCompile(`\n{3,}`).ReplaceAllString(cleaned, "\n\n")
	return meta, strings.TrimSpace(cleaned), true
}

// AutoExecutableCheckResult 自动执行 SQL 的安全校验结果（reasonCode 供前端 i18n）
type AutoExecutableCheckResult struct {
	Allowed    bool   `json:"allowed"`
	ReasonCode string `json:"reasonCode,omitempty"` // empty_sql, multi_sql, risky_sql, unknown_sql
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

// CheckAutoExecutableReadOnlySingleSQL 判断是否允许自动执行单条只读 SQL。
// 允许一个结尾分号，但拒绝用分号串联的多语句。
func CheckAutoExecutableReadOnlySingleSQL(sql string) AutoExecutableCheckResult {
	if !isSingleSQLStatement(sql) {
		return AutoExecutableCheckResult{Allowed: false, ReasonCode: "multi_sql", Verb: strings.ToUpper(SQLLeadingVerb(sql))}
	}
	return CheckAutoExecutableSelectSQL(sql)
}