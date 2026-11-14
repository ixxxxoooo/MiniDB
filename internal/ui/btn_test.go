package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// @author ixxxxoooo

// TestButtonsGoThroughTheKit 带文字的按钮必须走 btn.go 里的统一按钮（actBtn / fieldBtn / footBtn 及其主按钮形态），
// 不要在调用处直接写 ui.Button(c, "文字") 再单独设高度、字号：那正是各处按钮大小、风格不一的来源。
// 只有图标的按钮（标签为空串）和圆角胶囊的建议问题不在此列。
func TestButtonsGoThroughTheKit(t *testing.T) {
	raw := regexp.MustCompile(`ui\.(Primary)?Button\([^,()]+(\([^)]*\))?, [^"]`)
	rawText := regexp.MustCompile(`ui\.(Primary)?Button\([^,()]+(\([^)]*\))?, "[^"]`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "btn.go" {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !(raw.MatchString(line) || rawText.MatchString(line)) || strings.Contains(line, "Radius(999)") {
				continue
			}
			t.Errorf("%s:%d 带文字的按钮请用 actBtn / fieldBtn / footBtn（btn.go）: %s", f, i+1, strings.TrimSpace(line))
		}
	}
}
