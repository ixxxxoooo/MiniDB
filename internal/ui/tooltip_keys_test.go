package ui

import (
	"reflect"
	"testing"
)

func TestSplitTipKeys(t *testing.T) {
	for _, c := range []struct {
		in, label string
		keys      []string
	}{
		{"新建查询 (⌘T)", "新建查询", []string{"⌘", "T"}},
		{"导出表… ⇧⌘E", "导出表…", []string{"⇧", "⌘", "E"}},
		{"行预览 Space", "行预览", []string{"Space"}},
		{"刷新", "刷新", nil},
		{"⌘K", "⌘K", nil}, // 整句就是快捷键：不拆
		{"设置 (⌘,)", "设置", []string{"⌘", ","}},
	} {
		l, k := splitTipKeys(c.in)
		if l != c.label || !reflect.DeepEqual(k, c.keys) {
			t.Errorf("splitTipKeys(%q) = %q %v，应为 %q %v", c.in, l, k, c.label, c.keys)
		}
	}
}
