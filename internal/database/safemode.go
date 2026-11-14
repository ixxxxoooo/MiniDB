package database

import (
	"strings"

	"minidb/internal/sqltext"
)

// SafeMode 是连接的安全级别，对应 TablePlus 的 Safe Mode：
//
//	silent  不提示（默认）
//	alert1  发送任何语句前都要确认
//	alert2  除 SELECT / EXPLAIN / SHOW 外都要确认
//	safe1   发送任何语句前都要输入数据库密码
//	safe2   除 SELECT / EXPLAIN / SHOW 外都要输入数据库密码
type SafeMode string

const (
	SafeModeSilent SafeMode = "silent"
	SafeModeAlert1 SafeMode = "alert1"
	SafeModeAlert2 SafeMode = "alert2"
	SafeModeSafe1  SafeMode = "safe1"
	SafeModeSafe2  SafeMode = "safe2"
)

// SafeModes 按严格程度从低到高列出全部级别。
var SafeModes = []SafeMode{SafeModeSilent, SafeModeAlert1, SafeModeAlert2, SafeModeSafe1, SafeModeSafe2}

// SafeAction 是发送语句前需要用户完成的动作。
type SafeAction int

const (
	SafeNone     SafeAction = iota // 直接发送
	SafeConfirm                    // 弹出确认
	SafePassword                   // 输入数据库密码
)

// Valid 判断是不是已知级别。
func (m SafeMode) Valid() bool {
	for _, v := range SafeModes {
		if v == m {
			return true
		}
	}
	return false
}

// EffectiveSafeMode 返回连接实际生效的级别。
// 没有显式设置时：生产环境标签沿用旧版"提交前二次确认"的行为，按 alert2 处理；其余为 silent。
func (c ConnectionConfig) EffectiveSafeMode() SafeMode {
	if m := SafeMode(c.SafeMode); m.Valid() {
		return m
	}
	if c.Tag == "production" {
		return SafeModeAlert2
	}
	return SafeModeSilent
}

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
