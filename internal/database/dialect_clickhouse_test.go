package database

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestClickHouseDialectBasics(t *testing.T) {
	d, err := DialectOf(ClickHouse)
	if err != nil {
		t.Fatal(err)
	}
	if d.Label() != "ClickHouse" || d.DefaultPort() != 9000 || d.DefaultUser() != "default" || d.Driver() != "clickhouse" {
		t.Fatalf("基本信息不对: %s %d %s %s", d.Label(), d.DefaultPort(), d.DefaultUser(), d.Driver())
	}
	c := d.Caps()
	if c.RowEdit || c.ForeignKeys || c.Triggers || c.Routines || c.ReadOnlyTx || c.RowsAffectedReliable || c.CreateTable || c.AlterTable || c.Explain {
		t.Errorf("ClickHouse 不该有这些能力: %+v", c)
	}
	if !c.PerDatabasePool || !c.Network || !c.ProcessList || !c.CreateDatabase || !c.MultiDatabase {
		t.Errorf("缺少应有的能力: %+v", c)
	}
	if got := d.QuoteIdent("a`b\\c"); got != "`a\\`b\\\\c`" {
		t.Errorf("QuoteIdent = %s", got)
	}
	if got := d.QuoteString(`it's \n`); got != `'it\'s \\n'` {
		t.Errorf("QuoteString = %s", got)
	}
	if d.QuoteIdent("") != "" {
		t.Error("空标识符应返回空串")
	}
}

// 只有 ClickHouse 的网格是只读的；其余类型都要声明可编辑，防止新增方言时漏写。
func TestRowEditCaps(t *testing.T) {
	for _, tp := range AllDBTypes() {
		want := tp != ClickHouse
		if got := CapsOf(tp).RowEdit; got != want {
			t.Errorf("%s: RowEdit = %v，应为 %v", tp, got, want)
		}
	}
}

func TestClickHouseDSN(t *testing.T) {
	d := clickhouseDialect{}
	dsn := d.DSN(&ConnectionConfig{Host: "h", Port: 9440, User: "u", Password: "p@:/", Database: "ev", SSLMode: SSLRequired})
	for _, want := range []string{"clickhouse://u:p%40%3A%2F@h:9440/ev?", "secure=true", "skip_verify=true", "dial_timeout="} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN %s 缺少 %s", dsn, want)
		}
	}
	dsn = d.DSN(&ConnectionConfig{Host: "h", Port: 9000, User: "u", SSLMode: SSLVerifyFull})
	if !strings.Contains(dsn, "/default?") || !strings.Contains(dsn, "secure=true") || strings.Contains(dsn, "skip_verify") {
		t.Errorf("verify-full 的 DSN 不对: %s", dsn)
	}
	if dsn = d.DSN(&ConnectionConfig{Host: "h", Port: 9000, User: "u"}); strings.Contains(dsn, "secure") {
		t.Errorf("不加密时不该有 secure: %s", dsn)
	}
}

func TestClickHouseSyntaxAndStatements(t *testing.T) {
	s := SyntaxOf(ClickHouse)
	if s.Placeholder(3) != "?" || s.CastText("`a`") != "toString(`a`)" {
		t.Errorf("语法不对: %s %s", s.Placeholder(3), s.CastText("`a`"))
	}
	if got := AppendPage(ClickHouse, "SELECT 1", 10, 20); got != "SELECT 1 LIMIT 10 OFFSET 20" {
		t.Errorf("分页 = %s", got)
	}
	if got, _ := BuildCreateDatabaseSQL(ClickHouse, "ev", ""); got != "CREATE DATABASE `ev`" {
		t.Errorf("建库 = %s", got)
	}
	if got, _ := BuildRenameDatabaseSQL(ClickHouse, "a", "b"); got != "RENAME DATABASE `a` TO `b`" {
		t.Errorf("改库名 = %s", got)
	}
	if got, _ := BuildRenameTableSQL(ClickHouse, "a", "b"); got != "RENAME TABLE `a` TO `b`" {
		t.Errorf("改表名 = %s", got)
	}
	clone, _ := BuildCloneTableSQL(ClickHouse, "a", "b", true)
	if !reflect.DeepEqual(clone, []string{"CREATE TABLE `b` AS `a`", "INSERT INTO `b` SELECT * FROM `a`"}) {
		t.Errorf("复制表 = %v", clone)
	}
	create, _ := BuildCreateImportTableSQL(ClickHouse, "ev", "t", []NewTableColumn{{Name: "a", Kind: "int"}, {Name: "b", Kind: "float"}, {Name: "c", Kind: "text"}})
	if !strings.Contains(create, "`a` Int64") || !strings.Contains(create, "`b` Float64") || !strings.Contains(create, "`c` String") || !strings.HasSuffix(create, "ENGINE = MergeTree ORDER BY tuple()") {
		t.Errorf("导入建表 = %s", create)
	}
}

func TestSplitKeyExpr(t *testing.T) {
	cases := map[string][]string{
		"":                                nil,
		"id":                              {"id"},
		"id, day":                         {"id", "day"},
		"toDate(ts), cityHash64(a, b), x": {"toDate(ts)", "cityHash64(a, b)", "x"},
	}
	for in, want := range cases {
		if got := splitKeyExpr(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitKeyExpr(%q) = %v，应为 %v", in, got, want)
		}
	}
}

type stringer string

func (s stringer) String() string { return "S:" + string(s) }

func TestNormalizeOtherValue(t *testing.T) {
	s := "x"
	var nilPtr *int32
	n := int32(7)
	cases := []struct {
		in   any
		want any
	}{
		{int8(-3), int64(-3)},
		{uint16(9), int64(9)},
		{float32(0.1), 0.1},
		{true, true},
		{stringer("a"), "S:a"},
		{net.ParseIP("10.0.0.1"), "10.0.0.1"},
		{[]string{"x", "y"}, `["x","y"]`},
		{map[string]string{"k": "v"}, `{"k":"v"}`},
		{&s, "x"},
		{&n, int64(7)},
		{nilPtr, nil},
	}
	for _, c := range cases {
		if got := normalizeScannedValue(c.in, "T"); !reflect.DeepEqual(got, c.want) {
			t.Errorf("normalize(%#v) = %#v，应为 %#v", c.in, got, c.want)
		}
	}
	if got := normalizeScannedValue(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), "Date32"); got != "2024-01-02" {
		t.Errorf("Date32 = %v", got)
	}
}
