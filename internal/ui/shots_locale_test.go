package ui

import (
	"os"
	"testing"

	"github.com/egoist/mygo/i18n"
)

// @author ixxxxoooo
//
// TestShotsEnglish 用英文渲染几个文字最密的界面（欢迎页、新建连接、设置外观、命令面板、确认框），
// 用来检查英文比中文长时有没有被截断。设置 MINIDB_SHOTS 后输出到 <目录>/en-<场景>.png。
func TestShotsEnglish(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	i18n.SetLocaleFunc(func() string { return "en" }) // 文案函数读这里；回到 nil 就是源语言（中文）
	defer i18n.SetLocaleFunc(nil)
	e := welcomeEnv(t)
	e.tt.Frame()
	e.shot("en-1-welcome")

	e.sh.newConnection()
	e.tt.Frame()
	e.shot("en-2-conn-form")
	e.sh.dlg.open = false
	e.tt.Frame()

	e.sh.openSettings(setAppearance)
	e.tt.Frame()
	e.shot("en-3-settings-look")
	e.sh.set.open = false
	e.tt.Frame()

	e = newEnv(t) // 打开连接后的界面换一个干净的环境
	openWorkspace(e)
	e.tt.Frame()
	e.shot("en-4-workspace")
	e.sh.openPalette()
	e.tt.Frame()
	e.shot("en-5-palette")
	e.sh.pal.open = false
	e.tt.Frame()

	e.sh.askDanger("Delete table orders?", "This permanently deletes the table and its rows.", "Delete table", func() {})
	e.tt.Frame()
	e.shot("en-6-confirm")
}
