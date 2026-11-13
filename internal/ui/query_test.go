package ui

import (
	"testing"

	mui "github.com/egoist/mygo/ui"
)

func openWorkspace(e *env) *Workspace {
	e.sh.openConnection(e.sh.conns[0])
	return e.sh.spaces[0]
}

func TestQueryRunCurrentAndAll(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT name FROM users WHERE id = 1;\n\nSELECT COUNT(*) AS n FROM users")
	e.tt.Frame()

	// caret on the first statement: run current
	q.ed.Select(5, 5)
	q.run(false)
	e.has("用户001")
	if len(q.results) != 1 {
		t.Fatalf("results=%d", len(q.results))
	}
	e.shot("03-query")

	// run all: two results
	q.run(true)
	if len(q.results) != 2 {
		t.Fatalf("results=%d", len(q.results))
	}
	e.has("450")
	e.has("结果 1")

	// selection wins over the caret statement
	q.ed.Select(0, len("SELECT name FROM users WHERE id = 1"))
	q.run(false)
	if len(q.results) != 1 || len(q.results[0].res.Rows) != 1 {
		t.Fatalf("selection run: %d results", len(q.results))
	}
}

func TestQueryErrorsAndWrites(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELEC nope")
	q.run(false)
	if len(q.results) != 1 || q.results[0].res.Error == "" {
		t.Fatalf("a syntax error must come back as a result error: %+v", q.results)
	}
	e.shot("04-query-error")

	q.ed.Buffer.Set("INSERT INTO orders(user_id,total) VALUES (1, 9.5), (2, 3)")
	q.run(true)
	e.has("影响 2 行")

	q.ed.Buffer.Set("SELECT COUNT(*) AS n FROM orders")
	q.run(true)
	e.has("2")
}

func TestQueryReadOnlyConnection(t *testing.T) {
	e := newEnv(t)
	cfg := e.cfg
	cfg.ReadOnly = true
	if err := e.sh.app.Connections.SaveConnection(cfg); err != nil {
		t.Fatal(err)
	}
	e.sh.loadConnections()
	w := openWorkspace(e)
	q := w.newQuery("DELETE FROM users")
	q.run(true)
	e.has("只读")
	res, _ := e.sh.app.Queries.ExecuteSQL(e.cfg.ID, "", "SELECT COUNT(*) AS n FROM users")
	if res == nil || len(res.Rows) != 1 || res.Rows[0]["n"] == nil {
		t.Fatalf("users must be intact: %+v", res)
	}
	if v, _ := cellText(res.Rows[0]["n"]); v != "450" {
		t.Fatalf("rows left = %s", v)
	}
}

func TestQueryShortcutsAndHistory(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT 1 AS one")
	e.tt.Frame()
	e.tt.ClickAt(600, 250) // focus the editor
	e.tt.Key(mui.Cmd, mui.KeyEnter)
	e.tt.Frame()
	if len(q.results) != 1 {
		t.Fatalf("Cmd+Enter did not run: results=%d", len(q.results))
	}
	hist, _ := e.sh.app.History.List(e.cfg.ID, "", 0)
	if len(hist) != 1 || hist[0].SQL != "SELECT 1 AS one" {
		t.Fatalf("history = %+v", hist)
	}
	e.tt.Key(mui.Cmd, mui.KeyY)
	e.has("历史")
	e.has("SELECT 1 AS one")
	e.shot("05-history")
}

func TestQueryFormat(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("select id,name from users where id=1")
	q.format()
	if got := q.ed.Buffer.String(); got != "SELECT id,\n  name\nFROM users\nWHERE id = 1" {
		t.Fatalf("formatted = %q", got)
	}
}
