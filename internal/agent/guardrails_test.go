package agent

import (
	"strings"
	"testing"
)

func TestGuardReadOnlySQL(t *testing.T) {
	allowed := []string{
		"SELECT * FROM users",
		"select 1;",
		"WITH a AS (SELECT 1) SELECT * FROM a",
		"SHOW TABLES",
		"EXPLAIN SELECT * FROM users",
		"SELECT 'drop table x; delete from y' AS s",
	}
	for _, sql := range allowed {
		if res := GuardReadOnlySQL(sql); res != nil {
			t.Errorf("%q should pass, got %+v", sql, res)
		}
	}
	denied := map[string]string{
		"":                                       "为空",
		"DELETE FROM users":                      "只读权限",
		"SELECT 1; DROP TABLE users":             "一条",
		"SELECT SLEEP(10)":                       "SLEEP",
		"SELECT * FROM t INTO OUTFILE '/tmp/x'":  "OUTFILE",
		"WITH x AS (SELECT 1) DELETE FROM users": "只读权限",
		"SELECT * FROM users FOR UPDATE":         "只读权限",
		"PRAGMA writable_schema=1":               "只支持",
	}
	for sql, want := range denied {
		res := GuardReadOnlySQL(sql)
		if res == nil || res.OK || res.ErrorCode != "guarded" || !strings.Contains(res.Text, want) || res.Hint == "" {
			t.Errorf("%q: want denial mentioning %q with a hint, got %+v", sql, want, res)
		}
	}
}
