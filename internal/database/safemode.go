package database

import (
	"strings"

	"minidb/internal/sqltext"
)

// readOnlyVerb 判断语句首个动词是否属于只读类（SELECT / EXPLAIN / SHOW / DESC）。
func readOnlyVerb(sql string) bool {
	verb := sqltext.LeadingVerb(sql)
	// WITH 后面可以跟 DELETE，不能当作只读
	return verb != "with" && sqltext.IsReadOnlyVerb(verb)
}

// SafeModeRequirement 判断在给定级别下发送这批语句前需要做什么。
// hasPassword 为 false（SQLite、无密码连接）时，Safe 级退化为确认。
func SafeModeRequirement(mode SafeMode, hasPassword bool, stmts []string) SafeAction {
	var live []string
	for _, s := range stmts {
		if strings.TrimSpace(s) != "" {
			live = append(live, s)
		}
	}
	if len(live) == 0 || mode == SafeModeSilent || !mode.Valid() {
		return SafeNone
	}
	// 级别 2 只管会写入的语句：全部是只读类就放行
	if mode == SafeModeAlert2 || mode == SafeModeSafe2 {
		allRead := true
		for _, s := range live {
			if !readOnlyVerb(s) {
				allRead = false
				break
			}
		}
		if allRead {
			return SafeNone
		}
	}
	if mode == SafeModeSafe1 || mode == SafeModeSafe2 {
		if hasPassword {
			return SafePassword
		}
		return SafeConfirm
	}
	return SafeConfirm
}
