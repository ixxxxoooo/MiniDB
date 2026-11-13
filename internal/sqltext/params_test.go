package sqltext

import (
	"reflect"
	"testing"
)

func names(refs []ParamRef) (out []string) {
	for _, r := range refs {
		out = append(out, r.Name)
	}
	return
}

func TestFindParamsFormats(t *testing.T) {
	cases := []struct {
		f    ParamFormat
		sql  string
		want []string
	}{
		{ParamColon, "SELECT * FROM t WHERE a = :id AND b = :name", []string{"id", "name"}},
		{ParamColon, "SELECT a::int, b FROM t WHERE c = :x", []string{"x"}}, // PG 的 :: 不算
		{ParamColon, "SELECT '10:30', :t", []string{"t"}},                   // 字符串里的不算
		{ParamAt, "SELECT @@version, @v", []string{"v"}},
		{ParamDollar, "SELECT $1, $name", []string{"name"}}, // $1 是 PG 的位置参数
		{ParamBrace, "WHERE a = {id} AND b = {{no}}", []string{"id"}},
		{ParamDoubleBrace, "WHERE a = {{id}} AND b = {no}", []string{"id"}},
		{ParamDollarBrace, "WHERE a = ${id} AND b = {no}", []string{"id"}},
		{ParamColon, "SELECT 1 -- :ignored\n, :real /* :skip */", []string{"real"}},
		{ParamColon, "SELECT `a:b`, :c", []string{"c"}},
	}
	for _, c := range cases {
		if got := names(FindParams(c.sql, c.f, false)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s %q = %v, want %v", c.f, c.sql, got, c.want)
		}
	}
}

func TestFindParamsInQuotes(t *testing.T) {
	sql := "SELECT * FROM t WHERE a LIKE '%:term%' AND b = :id"
	if got := names(FindParams(sql, ParamColon, false)); !reflect.DeepEqual(got, []string{"id"}) {
		t.Fatalf("默认不算引号内: %v", got)
	}
	if got := names(FindParams(sql, ParamColon, true)); !reflect.DeepEqual(got, []string{"term", "id"}) {
		t.Fatalf("开启后算引号内: %v", got)
	}
}

func TestApplyParams(t *testing.T) {
	got := ApplyParams("SELECT * FROM t WHERE a = :id AND b = :name AND c = :id", ParamColon, false, map[string]string{"id": "7", "name": "'bob'"})
	if got != "SELECT * FROM t WHERE a = 7 AND b = 'bob' AND c = 7" {
		t.Fatal(got)
	}
	if got := ApplyParams("SELECT :a, :b", ParamColon, false, map[string]string{"a": "1"}); got != "SELECT 1, :b" {
		t.Fatalf("没填值的保持原样: %s", got)
	}
	if got := ApplyParams("WHERE a = {{x}}", ParamDoubleBrace, false, map[string]string{"x": "5"}); got != "WHERE a = 5" {
		t.Fatal(got)
	}
	if got := ParamNames([]string{"SELECT :a, :b", "SELECT :b, :c"}, ParamColon, false); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("%v", got)
	}
	if ParamFormatOf("nonsense") != ParamColon || ParamFormatOf("at") != ParamAt {
		t.Fatal("ParamFormatOf")
	}
}

func TestResultName(t *testing.T) {
	if got := ResultName("-- name: Top users\nSELECT 1"); got != "Top users" {
		t.Fatalf("%q", got)
	}
	if got := ResultName("-- 说明\n-- NAME:  订单\nSELECT 1"); got != "订单" {
		t.Fatalf("%q", got)
	}
	if got := ResultName("SELECT 1 -- name: late"); got != "" {
		t.Fatalf("语句之后的注释不算: %q", got)
	}
	if got := ResultName("SELECT 1"); got != "" {
		t.Fatal(got)
	}
}

func TestUglify(t *testing.T) {
	in := "SELECT  a,\n   b -- note\nFROM   t /* x */\nWHERE c = 'two  spaces\nkept' AND d = `a  b`;"
	want := "SELECT a, b FROM t WHERE c = 'two  spaces\nkept' AND d = `a  b`;"
	if got := Uglify(in); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if got := Uglify("  SELECT 1  "); got != "SELECT 1" {
		t.Fatalf("%q", got)
	}
	if got := Uglify("SELECT 'it''s'"); got != "SELECT 'it''s'" {
		t.Fatalf("双写引号: %q", got)
	}
}
