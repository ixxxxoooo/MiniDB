package ui

import "testing"

func TestBrowseTable(t *testing.T) {
	e := newEnv(t)
	e.sh.newConnectionDialog()
	e.has("local-sqlite") // the dialog lists saved connections
	e.shot("01-connections")
	e.sh.dlg.open = false

	e.sh.openConnection(e.sh.conns[0])
	e.has("users")
	e.has("orders")
	if len(e.sh.spaces) != 1 || e.sh.active != 0 {
		t.Fatalf("spaces=%d active=%d", len(e.sh.spaces), e.sh.active)
	}
	e.sh.spaces[0].openTable("users")
	e.has("用户001")
	e.has("user1@example.com")
	e.has("NULL") // user10 has no email
	e.shot("02-table")

	tab := e.sh.spaces[0].tabs[0].(*tableTab)
	if tab.total != 450 || len(tab.g.rows) != 200 {
		t.Fatalf("total=%d rows=%d", tab.total, len(tab.g.rows))
	}
	// next page
	tab.page = 2
	tab.load()
	e.has("用户201")
	e.hasNot("用户001")

	// filter
	tab.filters[0] = filterRow{on: true, col: "name", op: "=", val: "用户007"}
	tab.applyFilter()
	e.has("用户007")
	if tab.total != 1 {
		t.Fatalf("filtered total=%d", tab.total)
	}
	tab.clearFilter()
	if tab.total != 450 {
		t.Fatalf("cleared total=%d", tab.total)
	}

	// sort via the table header
	tab.sortOrder.Column, tab.sortOrder.Descending, tab.sorted = "id", true, true
	tab.page = 1
	tab.load()
	e.has("用户450")
}
