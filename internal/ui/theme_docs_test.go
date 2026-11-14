package ui

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// @author ixxxxoooo
//
// 令牌的「规范即代码」：docs/UI_TOKENS.md 里的色值与尺寸要和 theme_*.go 逐项一致。
// 改了一边没改另一边，这里会失败，并指出是哪一项。

var (
	docRowRe  = regexp.MustCompile("^\\| `([a-z0-9-]+)` \\| `([^`]+)` \\| `([^`]+)` \\|")
	docSizeRe = regexp.MustCompile("^\\| `([A-Za-z0-9]+)` \\| `([0-9.]+)` \\|")
	rgbaRe    = regexp.MustCompile(`^rgba\((\d+),(\d+),(\d+),([0-9.]+)\)$`)
)

// parseDocColor 解析文档里的 #rrggbb 或 rgba(r,g,b,a)。
func parseDocColor(t *testing.T, s string) ui.Color {
	t.Helper()
	if m := rgbaRe.FindStringSubmatch(s); m != nil {
		r, _ := strconv.Atoi(m[1])
		g, _ := strconv.Atoi(m[2])
		b, _ := strconv.Atoi(m[3])
		a, _ := strconv.ParseFloat(m[4], 32)
		return ui.RGBA(uint8(r), uint8(g), uint8(b), float32(a))
	}
	if strings.HasPrefix(s, "#") && len(s) == 7 {
		return ui.Hex(s)
	}
	t.Fatalf("UI_TOKENS.md 里的颜色写法认不出：%q", s)
	return ui.Color{}
}

// schemeFields 是「文档令牌名 → 令牌字段」，用于随配色变化的那部分。
func schemeFields(p tokens) map[string]ui.Color {
	return map[string]ui.Color{
		"bg": p.surface, "bg2": p.surfaceSecondary, "bg3": p.surfaceTertiary, "elev": p.surfaceElev,
		"seg-on": p.segThumb, "fg": p.fg, "fg2": p.fgSecondary, "fg3": p.fgMuted,
		"line": p.border, "line2": p.borderSubtle, "stripe": p.stripe,
		"acc": p.accent, "acc-t": p.accentText, "acc-fg": p.accentFg,
		"acc-bg": p.accentSoft, "acc-bg2": p.accentSoft2,
	}
}

// sharedFields 是与配色无关的那部分。
func sharedFields(p tokens) map[string]ui.Color {
	return map[string]ui.Color{
		"ok": p.success, "ok-bg": p.successBg, "warn": p.warning, "warn-bg": p.warningBg,
		"err": p.danger, "err-bg": p.dangerBg, "info": p.info, "info-bg": p.infoBg,
		"new": p.rowNew, "del": p.rowDelete, "edit": p.cellEdit,
		"ic-t": p.iconTable, "ic-v": p.iconView, "ic-q": p.iconQuery, "ic-f": p.iconRoutine,
		"v-num": p.valNum, "v-time": p.valTime,
		"sx-kw": p.sxKeyword, "sx-fn": p.sxFunc, "sx-str": p.sxString,
		"sx-num": p.sxNumber, "sx-cm": p.sxComment, "sx-op": p.sxOp,
		"c1": p.chart[0], "c2": p.chart[1], "c3": p.chart[2], "c4": p.chart[3], "c5": p.chart[4],
		"sh": p.shadow, "scrim": p.scrim,
	}
}

// metricValues 是「常量名 → 值」，和文档的尺寸表对账。新增尺寸常量时在这里加一行。
var metricValues = map[string]float64{
	"sizeToolbar": sizeToolbar, "sizeRail": sizeRail, "sizeRailBtn": sizeRailBtn, "sizeRailIcon": sizeRailIcon, "sizeSide": sizeSide,
	"sizeStatus": sizeStatus, "sizeBar": sizeBar, "sizeTab": sizeTab, "sizeRowPane": sizeRowPane,
	"sizeSideSub": sizeSideSub, "sizeSearch": sizeSearch, "sizeAvatar": sizeAvatar, "sizeSideSel": sizeSideSel, "sizeBtnSm": sizeBtnSm, "sizeBtn": sizeBtn, "sizeBtnLg": sizeBtnLg,
	"sizeInput": sizeInput, "sizeInputLg": sizeInputLg, "sizeRow": sizeRow, "sizeHead": sizeHead, "sizeChip": sizeChip, "sizeChipSq": sizeChipSq, "sizeUnder": sizeUnder, "sizeDot": sizeDot,
	"fontCaption": fontCaption, "fontDense": fontDense, "fontCtl": fontCtl, "fontBody": fontBody,
	"fontTitle": fontTitle, "fontHeading": fontHeading,
	"radiusSm": radiusSm, "radiusBtn": radiusBtn, "radiusSeg": radiusSeg, "radiusRail": radiusRail, "radiusMenu": radiusMenu, "radiusPanel": radiusPanel, "radiusDialog": radiusDialog,
	"spaceXS": spaceXS, "spaceSM": spaceSM, "spaceMD": spaceMD, "spaceLG": spaceLG, "spaceXL": spaceXL,
	"envBandH": envBandH,
}

// TestTokensMatchDocs 逐项对账 UI_TOKENS.md 与代码里的令牌。
func TestTokensMatchDocs(t *testing.T) {
	raw, err := os.ReadFile("../../docs/UI_TOKENS.md")
	if err != nil {
		t.Fatal(err)
	}
	section := ""
	seen := map[string]map[string]bool{}
	sizes := map[string]float64{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "### ") {
			section = strings.TrimSpace(strings.TrimPrefix(line, "### "))
			seen[section] = map[string]bool{}
			continue
		}
		if section == "metrics" {
			if m := docSizeRe.FindStringSubmatch(line); m != nil {
				v, _ := strconv.ParseFloat(m[2], 64)
				sizes[m[1]] = v
			}
			continue
		}
		m := docRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, light, dark := m[1], parseDocColor(t, m[2]), parseDocColor(t, m[3])
		seen[section][name] = true
		for i, want := range []ui.Color{light, dark} {
			isDark := i == 1
			var got ui.Color
			var ok bool
			if section == "shared" {
				got, ok = sharedFields(tokensFor(schemeGraphite, isDark))[name]
			} else {
				sc := schemeByName(section)
				if schemeNames[sc] != section {
					t.Errorf("UI_TOKENS.md 里有未知的配色方案 %q", section)
					break
				}
				got, ok = schemeFields(tokensFor(sc, isDark))[name]
			}
			if !ok {
				t.Errorf("[%s] 文档里的令牌 %s 在代码里没有对应字段", section, name)
				break
			}
			if got != want {
				t.Errorf("[%s] %s（%s）：代码是 %v，文档是 %v", section, name, map[bool]string{false: "浅色", true: "深色"}[isDark], got, want)
			}
		}
	}
	// 反向：代码里的每个令牌都要写进文档。
	for _, name := range schemeNames {
		for tok := range schemeFields(tokensFor(schemeGraphite, false)) {
			if !seen[name][tok] {
				t.Errorf("配色 %s 的令牌 %s 没有写进 UI_TOKENS.md", name, tok)
			}
		}
	}
	for tok := range sharedFields(tokensFor(schemeGraphite, false)) {
		if !seen["shared"][tok] {
			t.Errorf("共用令牌 %s 没有写进 UI_TOKENS.md", tok)
		}
	}
	for name, want := range metricValues {
		got, ok := sizes[name]
		switch {
		case !ok:
			t.Errorf("尺寸常量 %s 没有写进 UI_TOKENS.md", name)
		case got != want:
			t.Errorf("尺寸 %s：代码是 %v，文档是 %v", name, want, got)
		}
	}
	for name := range sizes {
		if _, ok := metricValues[name]; !ok {
			t.Errorf("UI_TOKENS.md 的尺寸 %s 在代码对账表 metricValues 里没有", name)
		}
	}
}
