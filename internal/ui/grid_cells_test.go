package ui

import (
	"testing"

	"minidb/internal/model"
)

// @author ixxxxoooo

// enum 的类型名里带 NUM，不能被当成数字列右对齐。
func TestEnumIsNotNumeric(t *testing.T) {
	if isNumericType("enum('a','b')") {
		t.Fatal("enum 被当成了数字类型")
	}
	if !isNumericType("decimal(10,2)") {
		t.Fatal("decimal 应该是数字类型")
	}
}

// 值的画法：主键优先，其次枚举、数字、时间，其余是普通文字。
func TestKindOf(t *testing.T) {
	cases := []struct {
		typ  string
		pk   bool
		want valueKind
	}{
		{"bigint", true, valPK}, {"enum('a')", false, valEnum}, {"int", false, valNumber},
		{"datetime", false, valTime}, {"timestamp", false, valTime}, {"varchar(20)", false, valText},
	}
	for _, c := range cases {
		if got := kindOf(model.ColumnMeta{Type: c.typ}, c.pk); got != c.want {
			t.Errorf("kindOf(%q,%v)=%v 想要 %v", c.typ, c.pk, got, c.want)
		}
	}
}
