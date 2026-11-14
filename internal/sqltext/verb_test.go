package sqltext

import "testing"

func TestLeadingVerb(t *testing.T) {
	cases := map[string]string{
		"SELECT 1":                           "select",
		"  /* 说明 */ INSERT INTO t":           "insert",
		"-- 注释\n-- 又一行\nDelete from t":       "delete",
		"":                                   "",
		"/* 只有注释 */":                         "",
		"(select 1)":                         "",
		"create or replace view v as select": "create",
	}
	for in, want := range cases {
		if got := LeadingVerb(in); got != want {
			t.Errorf("LeadingVerb(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVerbSets(t *testing.T) {
	if !IsReadOnlyVerb("select") || IsReadOnlyVerb("delete") {
		t.Error("只读动词集合不对")
	}
	if !IsRiskyVerb("drop") || IsRiskyVerb("select") {
		t.Error("风险动词集合不对")
	}
	if !IsSchemaChange("/* x */ ALTER TABLE t") || IsSchemaChange("insert into t") {
		t.Error("结构变更判断不对")
	}
}
