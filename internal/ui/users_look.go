package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 「用户与权限」里账号的画法（设计稿 users 页）：圆形字母头像 + 名字 / @主机，超级用户带琥珀色芯片。

// userBadge 是账号的圆形字母头像，size 为直径。
func userBadge(c *ui.Context, p tokens, name string, size float32) {
	letter := "?"
	if r := []rune(strings.ToUpper(strings.TrimSpace(name))); len(r) > 0 {
		letter = string(r[0])
	}
	ui.Row(c).Size(size, size).Shrink(0).Center().Radius(size / 2).Background(p.accentSoft).Children(func() {
		ui.Text(c, letter).FontSize(fontCaption).FontWeight(600).TextColor(p.accent)
	})
}

// userLabel 在一行里画「头像 + 名字（粗）+ @主机（淡）」，超级用户后面跟一枚芯片；big 为真用大头像。
func userLabel(c *ui.Context, p tokens, u app.UserInfo, big bool) {
	size := float32(sizeAvatar)
	if big {
		size = sizeDlgTile
	}
	ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Grow(1).MinWidth(0).Children(func() {
		userBadge(c, p, u.Name, size)
		ui.Row(c).Gap(0).AlignItems(ui.Baseline).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, u.Name).FontSize(fontBody).FontWeight(600).SingleLine()
			if u.Host != "" {
				ui.Text(c, "@"+u.Host).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
			}
		})
		if u.Superuser {
			chip(c, chipWarn, locales.UsersSuperuser(), false)
		}
	})
}
