package ui

import (
	"regexp"
	"strings"
)

// @author ixxxxoooo
//
// 悬浮提示里的快捷键键帽（设计稿「悬浮提示」）：提示文字末尾的 ⌘T、(⇧⌘E)、Space 这类写法拆出来，画成半透明小键帽。

// tipKeysRe 匹配文字末尾的快捷键：可带括号，由修饰键 / 特殊键符号加一个字符组成，或者是 Space、Esc。
var tipKeysRe = regexp.MustCompile(`\s*[(（]?((?:[⌘⌥⌃⇧⏎↵⌫⇥⎋]+[A-Za-z0-9,.=\-\\/]?)|Space|Esc)[)）]?$`)

// splitTipKeys 把提示文字拆成「正文」和「快捷键各键」。没有快捷键时 keys 为空。
// 参数：text 是提示文字。返回值：正文、按键列表（每个符号一个，⇧⌘E 拆成 ⇧ ⌘ E）。
func splitTipKeys(text string) (label string, keys []string) {
	m := tipKeysRe.FindStringSubmatchIndex(text)
	if m == nil || m[0] == 0 {
		return text, nil
	}
	label = strings.TrimSpace(text[:m[0]])
	for _, r := range text[m[2]:m[3]] {
		keys = append(keys, string(r))
	}
	if strings.EqualFold(text[m[2]:m[3]], "space") {
		keys = []string{"Space"}
	} else if strings.EqualFold(text[m[2]:m[3]], "esc") {
		keys = []string{"esc"}
	}
	return label, keys
}
