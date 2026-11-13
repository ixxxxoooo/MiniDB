package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"

	"minidb/internal/database"
)

// safeEnv 打开工作区，并把连接设为指定的安全模式（保存了数据库密码 secret）。
func safeEnv(t *testing.T, mode database.SafeMode, tag string) (*env, *Workspace) {
	t.Helper()
	e := newEnv(t)
	cfg := e.cfg
	cfg.Password, cfg.SafeMode, cfg.Tag = "secret", string(mode), tag
	if err := e.sh.app.Connections.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	return e, openWorkspace(e)
}

func ranOK(q *queryTab) bool { return len(q.results) > 0 && q.failed == "" }

// 静默模式下语句直接执行，没有任何提示。
func TestSafeModeSilentRunsAtOnce(t *testing.T) {
	e, w := safeEnv(t, database.SafeModeSilent, "local")
	q := w.newQuery("UPDATE users SET age = 1 WHERE id = 1")
	e.tt.Frame()
	q.run(true)
	if e.sh.confirm.open || e.sh.pwp.open {
		t.Fatal("静默模式不应弹出任何确认")
	}
	if !ranOK(q) {
		t.Fatalf("应已执行: results=%d failed=%q", len(q.results), q.failed)
	}
}

// 提醒 2：只读语句直接执行，写入语句先确认；取消则不执行。
func TestSafeModeAlert2ConfirmsWritesOnly(t *testing.T) {
	e, w := safeEnv(t, database.SafeModeAlert2, "local")
	q := w.newQuery("SELECT 1")
	e.tt.Frame()
	q.run(true)
	if e.sh.confirm.open || !ranOK(q) {
		t.Fatal("SELECT 在提醒 2 下应直接执行")
	}

	q2 := w.newQuery("DELETE FROM orders")
	e.tt.Frame()
	q2.run(true)
	if !e.sh.confirm.open {
		t.Fatal("写入语句应先确认")
	}
	e.has("DELETE FROM orders")
	e.sh.confirm = confirmDialog{} // 取消
	if len(q2.results) != 0 || q2.running {
		t.Fatal("取消后不应执行")
	}

	q2.run(true)
	e.sh.confirm.action() // 同意
	if !ranOK(q2) {
		t.Fatalf("同意后应执行: %q", q2.failed)
	}
}

// 提醒 1：连只读语句也要确认。
func TestSafeModeAlert1ConfirmsEverything(t *testing.T) {
	e, w := safeEnv(t, database.SafeModeAlert1, "local")
	q := w.newQuery("SELECT 1")
	e.tt.Frame()
	q.run(true)
	if !e.sh.confirm.open {
		t.Fatal("提醒 1 下 SELECT 也应确认")
	}
}

// 安全 1：要输入数据库密码；错误密码被拒绝，正确密码才执行。
func TestSafeModeSafe1AsksForThePassword(t *testing.T) {
	e, w := safeEnv(t, database.SafeModeSafe1, "local")
	q := w.newQuery("SELECT 1")
	e.tt.Frame()
	q.run(true)
	if !e.sh.pwp.open {
		t.Fatal("安全 1 应要求输入密码")
	}
	e.tt.Frame()
	e.tt.Type("wrong")
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if !e.sh.pwp.open || e.sh.pwp.err == "" {
		t.Fatalf("错误密码应被拒绝并提示: open=%v err=%q", e.sh.pwp.open, e.sh.pwp.err)
	}
	if len(q.results) != 0 {
		t.Fatal("错误密码不应执行")
	}
	e.tt.Type("secret")
	e.tt.Key(0, mui.KeyEnter)
	e.tt.Frame()
	if e.sh.pwp.open {
		t.Fatal("正确密码后应关闭提示")
	}
	if !ranOK(q) {
		t.Fatalf("正确密码后应执行: %q", q.failed)
	}
}

// 没有显式设置安全模式的生产连接，沿用旧行为：写入前确认（提醒 2）。
func TestSafeModeProductionDefaultsToAlert2(t *testing.T) {
	e, w := safeEnv(t, "", "production")
	if got := w.cfg.EffectiveSafeMode(); got != database.SafeModeAlert2 {
		t.Fatalf("EffectiveSafeMode = %s", got)
	}
	w.openTable("users")
	e.tt.Frame()
	q := w.newQuery("INSERT INTO orders(user_id,total) VALUES(1,2)")
	e.tt.Frame()
	q.run(true)
	if !e.sh.confirm.open {
		t.Fatal("生产连接写入应先确认")
	}
}

// 工具栏切换级别会保存到连接；离开密码级别要先输入密码。
func TestSetSafeModePersistsAndGuardsLeavingSafe(t *testing.T) {
	e, w := safeEnv(t, database.SafeModeSilent, "local")
	e.sh.setSafeMode(w, database.SafeModeSafe2)
	if w.cfg.EffectiveSafeMode() != database.SafeModeSafe2 {
		t.Fatal("应切到安全 2")
	}
	conns, _ := e.sh.app.Connections.GetConnections()
	if conns[0].SafeMode != string(database.SafeModeSafe2) {
		t.Fatalf("应保存到连接: %q", conns[0].SafeMode)
	}
	if !conns[0].HasPassword {
		t.Fatal("保存级别不应丢掉已存的密码")
	}

	e.sh.loadConnections()
	w.cfg.HasPassword = true
	e.sh.setSafeMode(w, database.SafeModeSilent)
	if !e.sh.pwp.open || w.cfg.EffectiveSafeMode() != database.SafeModeSafe2 {
		t.Fatal("离开安全级别应先要密码")
	}
	e.sh.pwp.pw = "secret"
	e.sh.submitPassword()
	if w.cfg.EffectiveSafeMode() != database.SafeModeSilent {
		t.Fatal("密码正确后应切换")
	}
}

// 密码来源为每次询问的连接，打开前先弹出密码提示；取消则不连接。
func TestAskPasswordConnectionPrompts(t *testing.T) {
	e := newEnv(t)
	cfg := database.ConnectionConfig{ID: "m1", Name: "ask-mysql", Type: "mysql", Host: "127.0.0.1", Port: 1, User: "root", PasswordMode: "ask"}
	e.sh.openConnection(cfg)
	if !e.sh.pwp.open {
		t.Fatal("应先询问密码")
	}
	if len(e.sh.spaces) != 0 || e.sh.connecting["m1"] {
		t.Fatal("输入密码前不应连接")
	}
	e.tt.Frame()
	e.has(cfg.Name)
	e.sh.pwp = pwPrompt{}
}

func TestConnURLNeverHasPassword(t *testing.T) {
	got := connURL(database.ConnectionConfig{Type: "postgres", Host: "db.local", Port: 5432, User: "me", Password: "pw", Database: "app"})
	if got != "postgresql://me@db.local:5432/app" {
		t.Fatalf("connURL = %q", got)
	}
	if got := connURL(database.ConnectionConfig{Type: "sqlite", Database: "/tmp/a.db"}); got != "sqlite:///tmp/a.db" {
		t.Fatalf("sqlite connURL = %q", got)
	}
}
