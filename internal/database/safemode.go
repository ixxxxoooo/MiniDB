package database

import "strings"

// readOnlyStatement 判断一条语句是否确实只读。
// 与 AI 通道、只读连接共用 CheckReadOnlySQL：只看首个动词会放过
// EXPLAIN ANALYZE DELETE、SELECT ... INTO OUTFILE、WITH ... DELETE 等会写入的语句。
func readOnlyStatement(sql string) bool {
	return CheckReadOnlySQL(sql).Allowed
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
			if !readOnlyStatement(s) {
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
