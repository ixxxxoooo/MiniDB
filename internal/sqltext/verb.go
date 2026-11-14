package sqltext

import (
	"regexp"
	"strings"
)

// 全仓库唯一的"SQL 首个动词"判定实现。安全模式、只读护栏、结构变更检测、
// 查询路由都从这里取动词和动词集合，不要再各自写一份。

var (
	reLeadingBlockComment = regexp.MustCompile(`(?s)^\s*/\*[\s\S]*?\*/\s*`)
	reLeadingLineComment  = regexp.MustCompile(`(?m)^\s*--[^\n]*(?:\n|\z)\s*`)
)

// StripLeadingComments 去除 SQL 前导的块注释与行注释，返回去掉首尾空白的剩余部分。
func StripLeadingComments(sql string) string {
	text := strings.TrimSpace(sql)
	for text != "" {
		prev := text
		text = reLeadingBlockComment.ReplaceAllString(text, "")
		text = reLeadingLineComment.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
		if text == prev {
			break
		}
	}
	return text
}

// LeadingVerb 返回去掉前导注释后的首个 SQL 关键字（小写）；没有则返回空串。
func LeadingVerb(sql string) string {
	cleaned := StripLeadingComments(sql)
	var b strings.Builder
	for _, r := range cleaned {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		default:
			return b.String()
		}
	}
	return b.String()
}

// ReadOnlyVerbs 是只读类语句的首个动词。
var ReadOnlyVerbs = map[string]struct{}{
	"select": {}, "show": {}, "desc": {}, "describe": {}, "explain": {}, "with": {},
}

// RiskyVerbs 是会写数据、改结构、改会话状态或占用服务器资源的首个动词。
var RiskyVerbs = map[string]struct{}{
	"insert": {}, "update": {}, "delete": {}, "replace": {}, "create": {}, "alter": {}, "drop": {}, "truncate": {}, "rename": {},
	"grant": {}, "revoke": {}, "call": {}, "set": {}, "use": {}, "begin": {}, "start": {}, "commit": {}, "rollback": {}, "lock": {}, "unlock": {},
	"merge": {}, "copy": {}, "do": {}, "load": {}, "analyze": {}, "optimize": {}, "vacuum": {}, "kill": {}, "handler": {},
}

// IsReadOnlyVerb 判断动词是否属于只读类。
func IsReadOnlyVerb(verb string) bool {
	_, ok := ReadOnlyVerbs[verb]
	return ok
}

// IsRiskyVerb 判断动词是否属于风险类。
func IsRiskyVerb(verb string) bool {
	_, ok := RiskyVerbs[verb]
	return ok
}

// IsSchemaChange 判断语句是不是结构变更（CREATE / ALTER / DROP / RENAME）。
func IsSchemaChange(sql string) bool {
	switch LeadingVerb(sql) {
	case "create", "alter", "drop", "rename":
		return true
	}
	return false
}
