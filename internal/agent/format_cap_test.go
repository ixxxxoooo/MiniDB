package agent

import (
	"strings"
	"testing"
)

// 工具自带的 ModelText 也要封顶，不能绕过行数与单元格上限。
func TestModelTextIsCapped(t *testing.T) {
	r := &ToolResult{OK: true, ModelText: strings.Repeat("表", ModelTextMaxRunes*2)}
	out := r.FormatForModel(ModelRowsDefault, ModelCellDefault)
	if n := len([]rune(out)); n > ModelTextMaxRunes+100 {
		t.Fatalf("ModelText 未封顶: %d", n)
	}
	if !strings.Contains(out, "截断") {
		t.Fatal("被截断时要告诉模型")
	}
}
