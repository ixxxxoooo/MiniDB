package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/internal/app"
	"minidb/internal/sqltext"
	"minidb/locales"
)

// ---- 收藏查询的关键字 ----

// loadKeywords 读取收藏：绑了关键字的收藏供补全，全部收藏供编辑器判断当前语句是否已收藏。
func (w *Workspace) loadKeywords() {
	var kw map[string]string
	var favs []app.HistoryItem
	w.sh.up.bg(func() {
		kw, _ = w.sh.app.History.Keywords(w.cfg.ID)
		favs, _ = w.sh.app.History.Favorites(w.cfg.ID)
	}, func() {
		w.keywords = kw
		w.favs = make(map[string]app.HistoryItem, len(favs))
		for _, it := range favs {
			if _, dup := w.favs[strings.TrimSpace(it.SQL)]; !dup { // 新的在前，同样的语句留最新一条
				w.favs[strings.TrimSpace(it.SQL)] = it
			}
		}
	})
}

// bindKeyword asks for the word that stands for a favorite query.
func (w *Workspace) bindKeyword(it app.HistoryItem) {
	w.sh.prompt(promptDialog{title: locales.QueryBindKeyword(), sub: firstLines(it.SQL, 1), label: locales.QueryKeyword(), value: it.Keyword,
		ok: locales.CommonConfirm(), action: func(kw string, _ bool, _ string) { w.setKeyword(it.ID, kw) }})
}

func (w *Workspace) setKeyword(id, kw string) {
	var err error
	w.sh.up.bg(func() { err = w.sh.app.History.SetKeyword(id, kw) }, func() {
		if err != nil {
			w.sh.toastErr(err.Error())
			return
		}
		w.loadKeywords()
		w.loadSideQueries()
	})
}

// ---- 编辑器选项 ----

var rowLimits = []int{0, 100, 500, 1000, 5000, 10000}

func rowLimitLabel(n int) string {
	if n == 0 {
		return locales.EditorRowLimitDefault()
	}
	return strconv.Itoa(n)
}

func rowLimitLabels() []string {
	out := make([]string, len(rowLimits))
	for i, n := range rowLimits {
		out[i] = rowLimitLabel(n)
	}
	return out
}

func paramFormatLabels() []string {
	out := make([]string, len(sqltext.ParamFormats))
	for i, f := range sqltext.ParamFormats {
		out[i] = f.Example()
	}
	return out
}

// tabSizes 是可选的缩进宽度。
var tabSizes = []int{2, 4, 6, 8}

// tabSize 是生效的缩进宽度，默认 2（和"格式化"的缩进一致）。
func (u uiPrefs) tabSize() int {
	for _, n := range tabSizes {
		if u.TabSize == n {
			return n
		}
	}
	return 2
}

// formatOptions 是"格式化"用的选项：缩进跟编辑器的缩进设置一致（制表符或空格、宽度），
// 关键字大小写看"格式化时关键字小写"。
func (u uiPrefs) formatOptions() sqltext.FormatOptions {
	unit := strings.Repeat(" ", u.tabSize())
	if u.IndentTabs {
		unit = "\t"
	}
	return sqltext.FormatOptions{Indent: unit, LowerKeywords: u.FormatLower}
}

// applyEditorPrefs 把编辑器相关的偏好应用到一个 SQL 编辑器上，每帧调用。
func (s *Shell) applyEditorPrefs(ed *editor.State) {
	pf := s.prefs
	ed.SoftWrap = !pf.NoWrap
	ed.LineNumbers = editor.LineNumbersOn
	if pf.NoLineNumbers {
		ed.LineNumbers = editor.LineNumbersOff
	}
	ed.NoActiveLine, ed.NoFolding, ed.NoIndentGuides = pf.NoActiveLine, pf.NoFolding, pf.NoIndentGuides
	ed.Whitespace = editor.WhitespaceSelection
	if pf.ShowWhitespace {
		ed.Whitespace = editor.WhitespaceAll
	}
	ed.TabSize = pf.tabSize()
	ed.Indent = editor.IndentSpaces
	if pf.IndentTabs {
		ed.Indent = editor.IndentTabs
	}
	ed.NoAutoClose = pf.NoAutoClose
	if sz := pf.EditorFont; sz > 0 {
		ed.Font.Size = sz
	}
}

// optGroup 画一组设置：上面是带图标的小标题，下面是一张卡片，卡片里每项占一行、项与项之间有分隔线。
// body 拿到一个 row 函数，每调用一次加一行：左边是标题和说明，右边是 control 画的控件。
func optGroup(c *ui.Context, p tokens, title string, icon *ui.SVG, body func(row func(label, help string, control func()))) {
	ui.Row(c.Key("og:"+title)).Gap(6).AlignItems(ui.Center).Padding(8, 2, 0, 2).Children(func() {
		ui.Icon(c, icon).FontSize(12).TextColor(p.fgSecondary)
		ui.Text(c, title).FontSize(font2XS).FontWeight(600).TextColor(p.fgSecondary)
	})
	ui.Column(c.Key("oc:"+title)).FillWidth().Radius(radiusPanel-2).Background(p.surfaceSecondary.Alpha(0.55)).Border(1, p.borderSubtle).Clip().Children(func() {
		n := 0
		body(func(label, help string, control func()) {
			if n > 0 {
				hairline(c, p.borderSubtle)
			}
			n++
			ui.Row(c.Key(fmt.Sprint("or:", title, n))).Gap(14).AlignItems(ui.Center).FillWidth().Padding(9, 14).Children(func() {
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, label).FontWeight(500).TextColor(p.fg)
					if help != "" {
						ui.Text(c, help).FontSize(font2XS).TextColor(p.fgSecondary)
					}
				})
				control()
			})
		})
	})
}

// editorOptions 是查询编辑器的全部选项，工具栏的齿轮和"设置 → 编辑器"显示的是同一份。
func (s *Shell) editorOptions(c *ui.Context, p tokens) {
	pf := &s.prefs
	changed := false
	// toggle 是一行开关；on 为 false 时开关关着。invert 为 true 表示偏好存的是"不……"，界面要反过来。
	toggle := func(row func(string, string, func()), key, label, help string, field *bool, invert bool) {
		row(label, help, func() {
			v := *field != invert
			if ui.Switch(c.Key(key), &v).Changed() {
				*field, changed = v != invert, true
			}
		})
	}

	optGroup(c, p, locales.EditorSectionRun(), icPlay, func(row func(string, string, func())) {
		row(locales.EditorRunDefault(), locales.EditorRunDefaultHelp(), func() {
			seg := 0
			if pf.RunAll {
				seg = 1
			}
			if ui.Segmented(c.Key("ed-run"), &seg, locales.EditorRunCurrent(), locales.EditorRunAll()).FontSize(font2XS).Changed() {
				pf.RunAll, changed = seg == 1, true
			}
		})
		row(locales.EditorRowLimit(), locales.EditorRowLimitHelp(), func() {
			limit := rowLimitLabel(pf.RowLimit)
			if ui.Select(c.Key("ed-limit"), &limit, rowLimitLabels()).Width(120).Height(sizeBtn).Changed() {
				for _, n := range rowLimits {
					if rowLimitLabel(n) == limit {
						pf.RowLimit, changed = n, true
					}
				}
			}
		})
	})

	optGroup(c, p, locales.EditorSectionAppearance(), icCode, func(row func(string, string, func())) {
		row(locales.EditorFont(), locales.EditorFontHelp(), func() {
			cur := int(pf.EditorFont)
			if cur == 0 {
				cur = editorFontDefault
			}
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				if tipBtn(c, icMinus, locales.MenuFontDown()).Disabled(cur <= editorFontMin).Clicked() {
					pf.EditorFont, changed = float32(cur-1), true
				}
				ui.Text(c, strconv.Itoa(cur)).Width(28).TextAlign(ui.Center).Font(mono).FontWeight(500)
				if tipBtn(c, icPlus, locales.MenuFontUp()).Disabled(cur >= editorFontMax).Clicked() {
					pf.EditorFont, changed = float32(cur+1), true
				}
			})
		})
		toggle(row, "ed-wrap", locales.EditorWrap(), locales.EditorWrapHelp(), &pf.NoWrap, true)
		toggle(row, "ed-ln", locales.EditorLineNumbers(), locales.EditorLineNumbersHelp(), &pf.NoLineNumbers, true)
		toggle(row, "ed-al", locales.EditorActiveLine(), locales.EditorActiveLineHelp(), &pf.NoActiveLine, true)
		toggle(row, "ed-fold", locales.EditorFolding(), locales.EditorFoldingHelp(), &pf.NoFolding, true)
		toggle(row, "ed-guides", locales.EditorIndentGuides(), locales.EditorIndentGuidesHelp(), &pf.NoIndentGuides, true)
		toggle(row, "ed-ws", locales.EditorWhitespace(), locales.EditorWhitespaceHelp(), &pf.ShowWhitespace, false)
	})

	optGroup(c, p, locales.EditorSectionIndent(), icFormat, func(row func(string, string, func())) {
		row(locales.EditorTabSize(), locales.EditorTabSizeHelp(), func() {
			seg := 0
			for i, n := range tabSizes {
				if pf.tabSize() == n {
					seg = i
				}
			}
			labels := make([]string, len(tabSizes))
			for i, n := range tabSizes {
				labels[i] = strconv.Itoa(n)
			}
			if ui.Segmented(c.Key("ed-tab"), &seg, labels...).FontSize(font2XS).Changed() {
				pf.TabSize, changed = tabSizes[seg], true
			}
		})
		toggle(row, "ed-spaces", locales.EditorIndentSpaces(), locales.EditorIndentSpacesHelp(), &pf.IndentTabs, true)
		toggle(row, "ed-fmtlower", locales.EditorFormatLower(), locales.EditorFormatLowerHelp(), &pf.FormatLower, false)
	})

	optGroup(c, p, locales.EditorSectionInput(), icKeyboard, func(row func(string, string, func())) {
		toggle(row, "ed-up", locales.EditorCompUpper(), locales.EditorCompUpperHelp(), &pf.CompUpper, false)
		toggle(row, "ed-sp", locales.EditorCompSpace(), locales.EditorCompSpaceHelp(), &pf.CompSpace, false)
		toggle(row, "ed-ac", locales.EditorAutoClose(), locales.EditorAutoCloseHelp(), &pf.NoAutoClose, true)
	})

	optGroup(c, p, locales.EditorSectionParams(), icBulb, func(row func(string, string, func())) {
		toggle(row, "ed-params", locales.EditorParams(), locales.EditorParamsHelp(), &pf.Params, false)
		if !pf.Params {
			return
		}
		row(locales.EditorParamFormat(), locales.EditorParamFormatHelp(), func() {
			format := sqltext.ParamFormatOf(pf.ParamFormat).Example()
			if ui.Select(c.Key("ed-pfmt"), &format, paramFormatLabels()).Width(140).Height(sizeBtn).Changed() {
				for _, f := range sqltext.ParamFormats {
					if f.Example() == format {
						pf.ParamFormat, changed = string(f), true
					}
				}
			}
		})
		toggle(row, "ed-pq", locales.EditorParamQuoted(), locales.EditorParamQuotedHelp(), &pf.ParamQuoted, false)
		row(locales.EditorParamValues(len(pf.ParamValues)), "", func() {
			if actBtn(c.Key("ed-pclear"), icX, locales.EditorParamClear()).Disabled(len(pf.ParamValues) == 0).Clicked() {
				pf.ParamValues, changed = nil, true
			}
		})
	})

	if changed {
		s.savePrefs()
	}
}

// resetEditorPrefs 把编辑器的选项恢复成默认值（不动记住的参数值）。
func (s *Shell) resetEditorPrefs() {
	pf := &s.prefs
	vals := pf.ParamValues
	pf.RunAll, pf.RowLimit, pf.Params, pf.ParamFormat, pf.ParamQuoted = false, 0, false, "", false
	pf.CompUpper, pf.CompSpace, pf.NoAutoClose, pf.EditorFont, pf.FormatLower = false, false, false, 0, false
	pf.NoWrap, pf.NoLineNumbers, pf.NoActiveLine, pf.NoFolding, pf.NoIndentGuides = false, false, false, false, false
	pf.ShowWhitespace, pf.TabSize, pf.IndentTabs = false, 0, false
	pf.ParamValues = vals
	s.savePrefs()
	s.toastOK(locales.EditorResetDone())
}

func (s *Shell) editorOptionsDialog(c *ui.Context) {
	if !s.edOpt {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &s.edOpt, dlgOpts{title: locales.EditorOptions(), sub: locales.SettingsEditorDesc(), width: 540, height: 620, lead: func() {
		ui.Row(c).Size(28, 28).Shrink(0).Center().Radius(8).Background(p.accent.Alpha(0.14)).Children(func() {
			ui.Icon(c, icSettings).FontSize(15).TextColor(p.accent)
		})
	}}, func() {
		scrollbars(c, ui.Scroll(c.Key("edopts-body")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Gap(6).FillWidth().Padding(0, 0, 6, 0).Children(func() { s.editorOptions(c, p) })
		})
	}, func() {
		ui.Spacer(c)
		if footBtn(c, locales.EditorReset()).Clicked() {
			s.resetEditorPrefs()
		}
		if footPrimary(c, locales.CommonClose()).Clicked() {
			s.edOpt = false
		}
	})
}
