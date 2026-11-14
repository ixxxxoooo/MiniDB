package ui

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestShotsThemes 在浅色和深色两种主题下渲染同一组界面，用来对照检查线条（边框、分割线、表格线）。
// 设置 MINIDB_SHOTS 后输出到 <目录>/<场景>-light.png 和 -dark.png。
func TestShotsThemes(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	for _, dark := range []bool{false, true} {
		suffix := map[bool]string{false: "light", true: "dark"}[dark]
		e := newEnv(t)
		e.tt.SetDark(dark)
		tab := openUsers(e)
		w := e.sh.spaces[0]
		tab.g.chosen.Add(1)
		e.shot("t1-data-" + suffix)

		tab.mode = 1
		tab.modeChanged()
		e.shot("t2-structure-" + suffix)
		tab.mode = 0
		tab.modeChanged()

		q := w.newQuery("SELECT * FROM users LIMIT 20;")
		e.tt.Frame()
		q.run(false)
		e.shot("t3-query-" + suffix)

		w.panel = 2
		e.sh.app.Settings.SaveAIConfig(appAIConfig())
		e.shot("t4-ai-" + suffix)
		w.panel = 0

		for _, i := range []int{0, 1} {
			e.sh.openSettings(i)
			e.shot(fmt.Sprintf("t5-settings%d-%s", i, suffix))
		}
		e.sh.set.open = false

		e.sh.newConnectionDialog()
		e.shot("t6-connlist-" + suffix)
	}
}

// TestShotsThemes2 渲染更多的次级界面（执行计划、图表、ER 图、日志、各种对话框）的双主题截图。
func TestShotsThemes2(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	for _, dark := range []bool{false, true} {
		suffix := map[bool]string{false: "light", true: "dark"}[dark]
		e := newEnv(t)
		e.tt.SetDark(dark)
		tab := openUsers(e)
		w := e.sh.spaces[0]

		tab.showFilter = true
		e.shot("u1-filter-" + suffix)
		tab.showFilter = false

		tab.mode = 2
		tab.modeChanged()
		e.shot("u2-ddl-" + suffix)
		tab.mode = 3
		tab.modeChanged()
		e.shot("u3-doc-" + suffix)
		tab.mode = 0
		tab.modeChanged()

		q := w.newQuery("SELECT * FROM users WHERE id = 1")
		e.tt.Frame()
		q.explain()
		e.tt.Frame()
		e.tt.Frame()
		e.shot("u4-explain-" + suffix)

		q2 := w.newQuery("SELECT age, COUNT(*) AS n FROM users GROUP BY age")
		e.tt.Frame()
		q2.run(false)
		q2.page = pageChart
		e.shot("u5-chart-" + suffix)

		execSQL(t, e, w, `CREATE TABLE items (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id))`)
		w.loadTables()
		w.openER("")
		e.tt.Frame()
		e.shot("u6-er-" + suffix)

		e.sh.logSQL("main", "SELECT id, name FROM users WHERE age >= 30", 12*time.Millisecond, 20, "")
		e.sh.logSQL("main", "DROP TABLE nope", 3*time.Millisecond, -1, "no such table: nope")
		e.sh.consoleOpen = true
		for range 3 {
			e.tt.Frame()
		}
		e.shot("u7-console-" + suffix)
		e.sh.consoleOpen = false

		e.sh.exportTable(w, "users")
		e.shot("u8-export-" + suffix)
		e.sh.exp.open = false
		e.sh.importInto(w, "users")
		e.shot("u9-import-" + suffix)
		e.sh.imp.open = false

		e.sh.newConnectionDialog()
		e.sh.beginForm("postgres")
		e.shot("v1-connform-" + suffix)
		e.sh.dlg.sshOpen = true
		e.sh.dlg.cfg.SSH.Enabled = true
		e.shot("v2-connform-ssh-" + suffix)
		e.sh.dlg.open = false

		e.sh.openSwitcher()
		e.shot("v3-switcher-" + suffix)
		e.sh.sw.open = false
		for _, i := range []int{2, 5, 8, 9, 10} {
			e.sh.openSettings(i)
			e.shot(fmt.Sprintf("v4-settings%d-%s", i, suffix))
		}
	}
}
