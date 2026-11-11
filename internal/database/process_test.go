package database

import (
	"context"
	"testing"
)

func TestParseProcessIDRejectsInjection(t *testing.T) {
	for _, bad := range []string{"", "abc", "1; DROP TABLE t", "-1", "1 OR 1=1", "0x10", "1.5"} {
		if _, err := ParseProcessID(bad); err == nil {
			t.Errorf("%q 应被拒绝", bad)
		}
	}
	for good, want := range map[string]uint64{"1": 1, " 42 ": 42, "18446744073709551615": 18446744073709551615} {
		if got, err := ParseProcessID(good); err != nil || got != want {
			t.Errorf("%q => %d, %v", good, got, err)
		}
	}
}

func TestKillProcessRejectsBadIDBeforeExecuting(t *testing.T) {
	db := openTestSQLite(t)
	if err := KillProcess(context.Background(), db, "mysql", "1; DROP TABLE x", false); err == nil {
		t.Fatal("非法 ID 应在执行前被拒绝")
	}
	if err := KillProcess(context.Background(), db, "sqlite", "1", false); err == nil {
		t.Fatal("SQLite 不支持终止进程")
	}
	if SupportsProcessList("sqlite") || !SupportsProcessList("postgres") || !SupportsProcessList("tidb") {
		t.Fatal("SupportsProcessList 判断错误")
	}
}
