package archtest

import (
	"fmt"
	"go/format"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// @author ixxxxoooo
//
// 界面样式的「只许减不许增」：颜色、字号、控件尺寸都要走令牌（docs/UI_TOKENS.md），
// 业务代码里不再写数字和色值。历史遗留的写死处记在 style_ceilings_test.go 里，逐步迁走。
//
// 三类：
//   - color：`ui.Hex(` / `ui.RGBA(`，只许出现在 ui/theme*.go；
//   - font：`FontSize(12)` 这类数字字号，应改用 fontDense / fontBody 等常量；
//   - size：`.Height(30)` / `.Size(24, 24)` 这类 ≥2 的数字尺寸，应改用 sizeXxx 常量
//     （1 像素的分隔线、0 不算）。
//
// 迁走之后上限必须同步下调：运行 `STYLE_CEILINGS=update go test ./internal/archtest -run TestUINoHardcodedStyle`
// 会按当前实际数量重写上限表。更新模式只会调低或删除，不会调高——写死处变多了仍然失败。

var styleRules = map[string]*regexp.Regexp{
	"color": regexp.MustCompile(`\bui\.(Hex|RGBA)\(`),
	"font":  regexp.MustCompile(`\bFontSize\(\s*[0-9]`),
	"size":  regexp.MustCompile(`\.(Height|Size)\(\s*([2-9][0-9.]*|1[0-9][0-9.]*|[0-9]{3,})`),
}

// styleCounts 统计 ui 包（非测试文件）每个文件里各类写死样式的行数，注释行不算。
func styleCounts(t *testing.T) map[string]map[string]int {
	t.Helper()
	out := map[string]map[string]int{}
	goFiles(t, func(rel, src string) {
		if !strings.HasPrefix(rel, "ui/") || strings.HasPrefix(strings.TrimPrefix(rel, "ui/"), "theme") {
			return
		}
		for _, line := range strings.Split(src, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for kind, re := range styleRules {
				if n := len(re.FindAllString(line, -1)); n > 0 {
					if out[kind] == nil {
						out[kind] = map[string]int{}
					}
					out[kind][rel] += n
				}
			}
		}
	})
	return out
}

// TestUINoHardcodedStyle 守住「颜色、字号、尺寸走令牌」。
func TestUINoHardcodedStyle(t *testing.T) {
	got := styleCounts(t)
	if os.Getenv("STYLE_CEILINGS") == "update" {
		writeStyleCeilings(t, got)
		return
	}
	for kind, files := range got {
		for rel, n := range files {
			ceiling := styleCeilings[kind][rel]
			switch {
			case n > ceiling:
				t.Errorf("%s 里有 %d 处写死的%s（上限 %d）：颜色、字号、尺寸要用令牌（docs/UI_TOKENS.md），不要在调用处写数字或色值", rel, n, kindName(kind), ceiling)
			case n < ceiling:
				t.Errorf("%s 的写死%s已减到 %d 处，请下调 style_ceilings_test.go 里的上限（当前 %d）：STYLE_CEILINGS=update go test ./internal/archtest -run TestUINoHardcodedStyle", rel, kindName(kind), n, ceiling)
			}
		}
	}
	for kind, files := range styleCeilings {
		for rel, ceiling := range files {
			if got[kind][rel] == 0 && ceiling > 0 {
				t.Errorf("style_ceilings_test.go 里 %s 的 %s 已清零，请从上限表删除这一项", rel, kind)
			}
		}
	}
}

func kindName(kind string) string {
	return map[string]string{"color": "颜色", "font": "字号", "size": "尺寸"}[kind]
}

// writeStyleCeilings 按当前数量重写上限表；只许调低，发现某项比旧上限更高就拒绝写入。
func writeStyleCeilings(t *testing.T, got map[string]map[string]int) {
	t.Helper()
	for kind, files := range got {
		for rel, n := range files {
			if old, ok := styleCeilings[kind][rel]; ok && n > old {
				t.Fatalf("%s 的写死%s从 %d 增加到 %d：更新模式只能调低，请先去掉新增的写死处", rel, kindName(kind), old, n)
			}
		}
	}
	var b strings.Builder
	b.WriteString("package archtest\n\n// 由 STYLE_CEILINGS=update 生成；只许减不许增，见 style_test.go。\nvar styleCeilings = map[string]map[string]int{\n")
	for _, kind := range []string{"color", "font", "size"} {
		files := got[kind]
		keys := make([]string, 0, len(files))
		for k := range files {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(&b, "\t%q: {\n", kind)
		for _, k := range keys {
			fmt.Fprintf(&b, "\t\t%q: %d,\n", k, files[k])
		}
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n")
	src, err := format.Source([]byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("style_ceilings_test.go", src, 0o644); err != nil {
		t.Fatal(err)
	}
}
