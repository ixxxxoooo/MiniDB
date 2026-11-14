package app

import (
	"testing"

	"minidb/internal/model"
)

func TestSortedSQL(t *testing.T) {
	cases := []struct {
		db   model.DBType
		stmt string
		col  string
		desc bool
		want string
	}{
		{model.MySQL, "SELECT * FROM t;", "name", false, "SELECT * FROM (SELECT * FROM t) AS _minidb_sorted ORDER BY `name`"},
		{model.Postgres, " SELECT a FROM t ", "my col", true, `SELECT * FROM (SELECT a FROM t) AS _minidb_sorted ORDER BY "my col" DESC`},
		{model.SQLite, "SELECT 1", "", false, "SELECT 1"},
		// 列名里的引号必须被转义，不能逃出标识符
		{model.SQLite, "SELECT 1 AS a", `a" --`, false, `SELECT * FROM (SELECT 1 AS a) AS _minidb_sorted ORDER BY "a"" --"`},
	}
	for _, c := range cases {
		if got := SortedSQL(c.db, c.stmt, c.col, c.desc); got != c.want {
			t.Errorf("SortedSQL(%q,%q,%v)\n got %s\nwant %s", c.stmt, c.col, c.desc, got, c.want)
		}
	}
}
