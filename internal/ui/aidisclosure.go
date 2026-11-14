package ui

import "minidb/locales"

// aiDisclosed 在用户第一次使用 AI 之前，说明"库结构和查询到的数据会发送到所配置的外部服务"。
// 已经确认过就返回 true，调用方照常继续；否则弹出确认框并返回 false，
// 用户确认后记住选择，再调用 proceed 重新走一遍原来的操作。
func (s *Shell) aiDisclosed(proceed func()) bool {
	if s.prefs.AIDisclosed {
		return true
	}
	s.askWarn(locales.AiDisclosureTitle(), locales.AiDisclosureMsg(), locales.AiDisclosureOk(), func() {
		s.prefs.AIDisclosed = true
		s.savePrefs()
		proceed()
	})
	return false
}
