package database

import "testing"

func TestSafeModeRequirement(t *testing.T) {
	sel := []string{"SELECT 1"}
	upd := []string{"SELECT 1", "UPDATE t SET a=1"}
	cases := []struct {
		mode SafeMode
		pw   bool
		sql  []string
		want SafeAction
	}{
		{SafeModeSilent, true, upd, SafeNone},
		{SafeModeAlert1, true, sel, SafeConfirm},
		{SafeModeAlert2, true, sel, SafeNone},
		{SafeModeAlert2, true, []string{"  explain select 1", "show tables"}, SafeNone},
		{SafeModeAlert2, true, upd, SafeConfirm},
		{SafeModeAlert2, true, []string{"-- c\nDELETE FROM t"}, SafeConfirm},
		{SafeModeSafe1, true, sel, SafePassword},
		{SafeModeSafe2, true, sel, SafeNone},
		{SafeModeSafe2, true, upd, SafePassword},
		{SafeModeSafe2, false, upd, SafeConfirm}, // 没有密码：退化为确认
		{SafeModeSafe1, true, []string{"", "  "}, SafeNone},
		{SafeMode("bogus"), true, upd, SafeNone},
	}
	for i, c := range cases {
		if got := SafeModeRequirement(c.mode, c.pw, c.sql); got != c.want {
			t.Errorf("case %d (%s %v): got %d want %d", i, c.mode, c.sql, got, c.want)
		}
	}
}

// 只看首个动词会把这些"看起来只读"的语句放过，导致 Alert2/Safe2 下无确认就写入。
func TestSafeModeRequiresConfirmForDisguisedWrites(t *testing.T) {
	disguised := []string{
		"EXPLAIN ANALYZE DELETE FROM t",
		"WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d",
		"SELECT * FROM t INTO OUTFILE '/tmp/x'",
		"SELECT * FROM t FOR UPDATE",
		"SELECT SLEEP(5)",
	}
	for _, q := range disguised {
		if got := SafeModeRequirement(SafeModeAlert2, true, []string{q}); got != SafeConfirm {
			t.Errorf("Alert2 应要求确认: %q got %d", q, got)
		}
		if got := SafeModeRequirement(SafeModeSafe2, true, []string{q}); got != SafePassword {
			t.Errorf("Safe2 应要求密码: %q got %d", q, got)
		}
	}
}

func TestEffectiveSafeModeKeepsProductionConfirm(t *testing.T) {
	if got := (ConnectionConfig{Tag: "production"}).EffectiveSafeMode(); got != SafeModeAlert2 {
		t.Fatalf("production without a setting is alert2, got %s", got)
	}
	if got := (ConnectionConfig{Tag: "production", SafeMode: "silent"}).EffectiveSafeMode(); got != SafeModeSilent {
		t.Fatalf("an explicit silent wins, got %s", got)
	}
	if got := (ConnectionConfig{}).EffectiveSafeMode(); got != SafeModeSilent {
		t.Fatalf("default is silent, got %s", got)
	}
}
