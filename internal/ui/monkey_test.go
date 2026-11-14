package ui

import (
	"strings"
	"testing"

	mui "github.com/egoist/mygo/ui"
)

// clickEverything 把当前界面上每一个文字都点一遍（每点一次按 Esc 收拾弹窗），
// 任何一次点击引发的崩溃都会连同被点的文字一起报出来。用来找“不知道点了什么就崩了”的问题。
func clickEverything(t *testing.T, e *env, scene string) {
	t.Helper()
	e.tt.Frame()
	seen := map[string]bool{}
	for _, txt := range e.tt.Texts() {
		// 切换主题要走真实窗口的主线程，无窗口的测试里会一直等下去，跳过
		if txt == "" || seen[txt] || txt == "切换到深色" || txt == "切换到浅色" {
			continue
		}
		seen[txt] = true
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("[%s] 点击 %q 崩溃: %v", scene, txt, r)
				}
			}()
			_ = e.tt.Click(txt)
			e.tt.Frame()
			e.tt.Frame()
			if !strings.HasPrefix(scene, "过滤条") { // Esc 会把过滤条本身关掉，后面就点不到它了
				e.tt.Key(0, mui.KeyEscape)
			}
			e.tt.Frame()
		}()
	}
	t.Logf("[%s] 点了 %d 个不同的文字", scene, len(seen))
}

func TestMonkeyClicks(t *testing.T) {
	scenes := map[string]func(e *env){
		"数据":  func(e *env) { openUsers(e) },
		"结构":  func(e *env) { tab := openUsers(e); tab.mode = 1; tab.modeChanged() },
		"DDL": func(e *env) { tab := openUsers(e); tab.mode = 2; tab.modeChanged() },
		"文档":  func(e *env) { tab := openUsers(e); tab.mode = 3; tab.modeChanged() },
		"过滤条": func(e *env) {
			openUsers(e)
			e.tt.Key(mui.Cmd, mui.KeyF)
			e.tt.Frame()
		},
		"查询": func(e *env) {
			w := openWorkspace(e)
			q := w.newQuery("SELECT * FROM users LIMIT 5; SELECT * FROM orders")
			e.tt.Frame()
			q.run(true)
		},
		"过滤条-多行": func(e *env) {
			tab := openUsers(e)
			e.tt.Key(mui.Cmd, mui.KeyF)
			e.tt.Frame()
			tab.filters = []filterRow{newFilterRow(), newFilterRow(), newFilterRow()}
		},
		"AI":   func(e *env) { w := openWorkspace(e); w.panel = 2 },
		"连接列表": func(e *env) { e.sh.newConnectionDialog() },
		"ER":   func(e *env) { w := openWorkspace(e); w.openER("") },
		"概览":   func(e *env) { w := openWorkspace(e); w.openOverview() },
		"表-多标签": func(e *env) {
			w := openWorkspace(e)
			w.openTable("users")
			w.openTable("orders")
			w.newQuery("SELECT 1")
		},
		"设置":  func(e *env) { e.sh.openSettings(0) },
		"快捷键": func(e *env) { e.sh.openSettings(setShortcuts) },
		"监控":  func(e *env) { w := openWorkspace(e); e.sh.openMonitor(w, 0) },
	}
	for name, setup := range scenes {
		name, setup := name, setup
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			setup(e)
			for range 3 {
				e.tt.Frame()
			}
			clickEverything(t, e, name)
		})
	}
}
