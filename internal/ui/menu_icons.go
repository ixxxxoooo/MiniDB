package ui

import (
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 菜单项图标（设计稿 A 的右键菜单：左边是 14px 线条图标，右边是键帽）。
// 菜单画在窗口里（theme.go 里的 PopoverMenus），图标取菜单文字的颜色，危险项用红色。

var (
	menuIconMu    sync.Mutex
	menuIconCache = map[*ui.SVG]*mygo.MenuIcon{}
)

// menuIconOf 把项目里的线条图标转成菜单图标；每个图标只转一次。转换失败（不会发生）时返回 nil，菜单项就不带图标。
func menuIconOf(ic *ui.SVG) *mygo.MenuIcon {
	menuIconMu.Lock()
	defer menuIconMu.Unlock()
	if mi, ok := menuIconCache[ic]; ok {
		return mi
	}
	mi, err := mygo.NewMenuIconSVG(ic, true)
	if err != nil {
		mi = nil
	}
	menuIconCache[ic] = mi
	return mi
}

// menuItem 在菜单 m 里加一项：label 是文字，ic 是图标（nil 不带图标）。
// 返回菜单项，可继续 .Shortcut(...)、.Danger()、.Chosen()。
func menuItem(m *ui.Menu, label string, ic *ui.SVG) *ui.MenuItem {
	it := m.Item(label)
	if ic != nil {
		if mi := menuIconOf(ic); mi != nil {
			it.Icon(mi)
		}
	}
	return it
}

// dangerItem 同 menuItem，但按危险操作画：文字和图标都是红色。
func dangerItem(m *ui.Menu, label string, ic *ui.SVG) *ui.MenuItem {
	return menuItem(m, label, ic).Danger()
}

// menuItem2 同 menuItem，但是子菜单：label 是文字，ic 是图标，build 建子菜单的内容。
func menuItem2(m *ui.Menu, label string, ic *ui.SVG, build func(m *ui.Menu)) *ui.MenuItem {
	it := m.Submenu(label, build)
	if mi := menuIconOf(ic); mi != nil {
		it.Icon(mi)
	}
	return it
}
