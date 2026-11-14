// @author ixxxxoooo

package database

import (
	"testing"

	"minidb/internal/model"
)

// TestOptionalMetadataInterfaces 校验可选接口与 Caps 声明一致：声明了能力就必须实现对应接口，
// 没声明的不应该被调用方当成有（调用方先看 Caps 再断言接口）。
func TestOptionalMetadataInterfaces(t *testing.T) {
	for _, typ := range AllDBTypes() {
		d, err := DialectOf(typ)
		if err != nil {
			t.Fatal(err)
		}
		c := d.Caps()
		if _, ok := d.(ForeignKeyReader); c.ForeignKeys && !ok {
			t.Errorf("%s 声明了 ForeignKeys 却没有实现 ForeignKeyReader", typ)
		}
		if _, ok := d.(TriggerReader); c.Triggers && !ok {
			t.Errorf("%s 声明了 Triggers 却没有实现 TriggerReader", typ)
		}
		if _, ok := d.(RoutineReader); c.Routines && !ok {
			t.Errorf("%s 声明了 Routines 却没有实现 RoutineReader", typ)
		}
	}
}

// TestUnsupportedMetadataReturnsEmpty 没有这些能力的方言，读取函数返回空而不是报错。
func TestUnsupportedMetadataReturnsEmpty(t *testing.T) {
	for _, typ := range []DBType{StarRocks, ClickHouse, SQLServer} {
		if typ != SQLServer { // SQL Server 有外键，会真的去查库
			if fks, err := GetForeignKeys(nil, nil, typ, "d", "t"); fks != nil || err != nil {
				t.Errorf("%s GetForeignKeys = %v, %v", typ, fks, err)
			}
		}
		if tr, err := GetTriggers(nil, nil, typ, "d", "t"); tr != nil || err != nil {
			t.Errorf("%s GetTriggers = %v, %v", typ, tr, err)
		}
		if rs, err := GetRoutines(nil, nil, typ, "d"); rs != nil || err != nil {
			t.Errorf("%s GetRoutines = %v, %v", typ, rs, err)
		}
		if _, err := GetRoutineDDL(nil, nil, typ, "d", "function", "f"); err == nil {
			t.Errorf("%s GetRoutineDDL 应当报不支持", typ)
		}
	}
}

// TestCapsReplaceTypeChecks 校验取代了类型判断的几项能力。
func TestCapsReplaceTypeChecks(t *testing.T) {
	cases := []struct {
		typ       DBType
		localFile bool
		backslash bool
		queryOnly bool
		dump      model.DumpTool
	}{
		{MySQL, false, true, false, DumpMySQL},
		{MariaDB, false, true, false, DumpMySQL},
		{TiDB, false, true, false, DumpMySQL},
		{StarRocks, false, true, false, DumpNone},
		{Postgres, false, false, false, DumpPostgres},
		{SQLServer, false, false, false, DumpNone},
		{ClickHouse, false, false, false, DumpNone},
		{SQLite, true, false, true, DumpNone},
	}
	for _, c := range cases {
		caps := CapsOf(c.typ)
		if caps.LocalFile() != c.localFile || caps.BackslashEscape != c.backslash || caps.QueryOnlyPragma != c.queryOnly || caps.Dump != c.dump {
			t.Errorf("%s: LocalFile=%v BackslashEscape=%v QueryOnlyPragma=%v Dump=%q", c.typ, caps.LocalFile(), caps.BackslashEscape, caps.QueryOnlyPragma, caps.Dump)
		}
	}
}

func TestQuoteTableQualified(t *testing.T) {
	if got := QuoteTable(MySQL, "db", "t"); got != "`db`.`t`" {
		t.Errorf("MySQL 带库名 = %s", got)
	}
	if got := QuoteTable(MySQL, "", "t"); got != "`t`" {
		t.Errorf("MySQL 无库名 = %s", got)
	}
	if got := QuoteTable(Postgres, "db", "t"); got != `"t"` {
		t.Errorf("Postgres 不带库名前缀 = %s", got)
	}
}

func TestCanReplaceInPlace(t *testing.T) {
	cases := []struct {
		typ       DBType
		kind, ddl string
		want      bool
	}{
		{Postgres, "function", "CREATE FUNCTION f()", true},
		{Postgres, "view", "CREATE VIEW v AS SELECT 1", false},
		{Postgres, "view", "create or replace view v as select 1", true},
		{MySQL, "function", "CREATE OR REPLACE FUNCTION f()", false},
		{SQLite, "view", "CREATE VIEW v AS SELECT 1", false},
	}
	for _, c := range cases {
		if got := CanReplaceInPlace(c.typ, c.kind, c.ddl); got != c.want {
			t.Errorf("CanReplaceInPlace(%s,%s,%q) = %v", c.typ, c.kind, c.ddl, got)
		}
	}
}

func TestPromptHintOf(t *testing.T) {
	if PromptHintOf(MySQL) != "" || PromptHintOf("oracle") != "" {
		t.Error("MySQL 与未知类型不应有提示")
	}
	for _, typ := range []DBType{MariaDB, TiDB, StarRocks, Postgres, SQLServer, ClickHouse, SQLite} {
		if PromptHintOf(typ) == "" {
			t.Errorf("%s 缺少 AI 语法提示", typ)
		}
	}
}
