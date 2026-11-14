package ui

import (
	"fmt"
	"minidb/internal/model"
	"os"
	"testing"
)

// TestShotsDark renders a few screens in dark mode when MINIDB_SHOTS is set.
func TestShotsDark(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newEnv(t)
	e.tt.SetDark(true)
	tab := openUsers(e)
	tab.g.chosen.Add(1)
	tab.setCell(2, "name", "edited")
	tab.addRow()
	e.shot("30-dark-table")
	w := e.sh.spaces[0]
	w.panel = 2
	e.sh.app.Settings.SaveAIConfig(appAIConfig())
	e.shot("31-dark-ai")
}

// TestShotsDialogs renders the dialogs and sub-views when MINIDB_SHOTS is set.
func TestShotsDialogs(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newEnv(t)
	tab := openUsers(e)
	w := e.sh.spaces[0]
	tab.showFilter = true
	e.shot("40-filter")
	tab.showFilter = false
	tab.mode = 3
	tab.modeChanged()
	e.shot("41-doc")
	tab.mode = 0
	e.sh.openSwitcher()
	e.shot("42-switcher")
	e.sh.sw.open = false
	e.sh.openProcesses(w)
	e.shot("43-process")
	e.sh.proc.open = false
	e.sh.exportTable(w, "users")
	e.shot("44-export")
	e.sh.exp.open = false
	e.sh.importInto(w, "users")
	e.shot("45-import")
	e.sh.imp.open = false
	for i := 0; i <= setAbout; i++ {
		e.sh.openSettings(i)
		e.shot(fmt.Sprintf("46-settings%02d", i))
	}
	e.sh.set.open = false
	e.sh.newConnection()
	e.shot("47-conn-mysql")
	e.sh.dlg.sshOpen = true
	e.shot("48-conn-ssh")
}

// TestShotsRail renders the workspace rail with two connections.
func TestShotsRail(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newEnv(t)
	openUsers(e)
	cfg2 := e.cfg
	cfg2.ID, cfg2.Name, cfg2.Color, cfg2.Tag = "c2", "prod-sqlite", "#ff453a", "production"
	if err := e.sh.app.Connections.SaveConnection(cfg2); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	e.sh.openConnection(cfg2)
	e.tt.Frame()
	e.shot("49-rail")
}

// TestShotsConnections renders the connection dialogs.
func TestShotsConnections(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newEnv(t)
	for i, name := range []string{"prod-orders", "staging", "analytics"} {
		cfg := e.cfg
		cfg.ID, cfg.Name = "x"+string(rune('a'+i)), name
		cfg.Type, cfg.Host, cfg.Port, cfg.User, cfg.Database = []model.DBType{model.MySQL, model.Postgres, model.StarRocks}[i], "10.0.0."+string(rune('1'+i)), 3306+i, "root", "app"
		cfg.Tag, cfg.Color = []string{"production", "test", "local"}[i], []string{"#ff453a", "#ff9f0a", "#34c759"}[i]
		if err := e.sh.app.Connections.SaveConnection(cfg); err != nil {
			t.Fatal(err)
		}
	}
	e.sh.loadConnections()
	e.sh.newConnectionDialog()
	e.sh.selected = e.sh.conns[1].ID
	e.tt.Move(640, 300)
	e.shot("80-conn-list")
	e.sh.beginForm("postgres")
	e.shot("82-conn-form")
	e.sh.dlg.cfg.SSH.Enabled = true
	e.sh.dlg.urlOpen = true
	e.sh.dlg.msg, e.sh.dlg.msgOK = "连接成功", true
	e.shot("83-conn-form-ssh")
}

// TestShotsRowStates renders deleted, edited and new rows.
func TestShotsRowStates(t *testing.T) {
	if os.Getenv("MINIDB_SHOTS") == "" {
		t.Skip("set MINIDB_SHOTS to render screenshots")
	}
	e := newEnv(t)
	tab := openUsers(e)
	tab.g.chosen.Add(0)
	tab.g.chosen.Add(1)
	tab.deleteSelected()
	tab.setCell(3, "name", "edited")
	tab.addRow()
	tab.g.list.ScrollTo(0, 0)
	e.tt.Frame()
	e.shot("90-row-states")
}
