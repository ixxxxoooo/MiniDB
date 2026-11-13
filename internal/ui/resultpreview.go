package ui

import (
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// 查询结果的行预览：和表数据页一样，选中行按空格，在网格右边打开一个窄栏，
// 每个字段一行，只读（查询结果不能直接改）；JSON 自动缩进，可以搜索字段、复制值。

// previewRows 是选中的行（升序）；一行都没选时为空。
func (r *queryResult) previewRows() []int {
	var rows []int
	for i := range r.g.chosen.All() {
		if i >= 0 && i < len(r.g.rows) {
			rows = append(rows, i)
		}
	}
	sort.Ints(rows)
	return rows
}

// togglePreview 显示/隐藏行预览；打开时如果没有选中行，就选中第一行，让栏里总有内容可看。
func (r *queryResult) togglePreview() {
	r.showPreview = !r.showPreview
	if r.previewW == 0 {
		r.previewW = 340
	}
	if r.showPreview && len(r.previewRows()) == 0 && len(r.g.rows) > 0 {
		r.g.chosen.Add(0)
	}
}

// gridKeys 是结果网格的键盘：空格切换行预览，⌘C 复制行，⌘⇧C 复制单元格。
// 只在网格拿着焦点时生效，所以在编辑器里打字不会触发。
func (r *queryResult) gridKeys(c *ui.Context, tbl ui.Element, sh *Shell) {
	switch {
	case tbl.Shortcut(0, ui.KeySpace):
		r.togglePreview()
	case tbl.Shortcut(ui.Cmd|ui.Shift, ui.KeyC):
		if r.g.copyCells(c) {
			sh.toastOK(locales.StatusCopied())
		}
	case tbl.Shortcut(ui.Cmd, ui.KeyC):
		if r.g.copyRows(c) {
			sh.toastOK(locales.StatusCopied())
		}
	}
}

// previewHandle 是网格和行预览之间的分隔线，拖动改变预览栏宽度。
func (r *queryResult) previewHandle(c *ui.Context, p tokens) {
	dx, reset := vHandle(c, p, "qphandle")
	r.previewW = min(max(r.previewW-dx, 220), 720)
	if reset {
		r.previewW = 340
	}
}

// resultPreview 画行预览栏。
func (r *queryResult) resultPreview(c *ui.Context, p tokens, sh *Shell) {
	rows := r.previewRows()
	ui.Column(c).Grow(1).FillHeight().Background(p.surface).Children(func() {
		ui.Row(c).Gap(6).Padding(0, 8, 0, 12).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 2).Shrink(0).Children(func() {
			ui.Text(c, locales.PreviewTitle()).FontSize(font2XS).FontWeight(500)
			if len(rows) > 1 {
				ui.Text(c, locales.TableRows(len(rows))).FontSize(font2XS - 1).TextColor(p.fgMuted)
			}
			ui.Spacer(c)
			if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				r.showPreview = false
			}
		})
		hairline(c, p.borderSubtle)
		if len(rows) == 0 {
			ui.Text(c, locales.PreviewNone()).FontSize(font2XS).TextColor(p.fgMuted).Padding(14)
			return
		}
		ui.Row(c).Padding(8, 10, 2, 10).FillWidth().Shrink(0).Children(func() {
			ui.SearchField(c.Key("qfieldq"), &r.pvq).Placeholder(locales.TableFieldSearch()).Height(sizeBtnSm).FontSize(font2XS).FillWidth()
		})
		q := strings.ToLower(strings.TrimSpace(r.pvq))
		scrollbars(c, ui.Scroll(c.Key("qrowpreview")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Gap(10).Padding(8, 12, 12, 12).FillWidth().Children(func() {
				for _, i := range r.g.vis {
					col := r.g.cols[i]
					if q != "" && !strings.Contains(strings.ToLower(col.Name), q) {
						continue
					}
					r.previewField(c, p, sh, rows, col.Name, col.Type)
				}
			})
		})
	})
}

// previewField 是一个字段：名称、类型、悬停出现的复制按钮，下面是只读的值。
func (r *queryResult) previewField(c *ui.Context, p tokens, sh *Shell, rows []int, name, typ string) {
	var text string
	var null, mixed bool
	for k, row := range rows {
		tx, n := cellText(r.g.rows[row][name])
		if k == 0 {
			text, null = tx, n
			continue
		}
		if tx != text || n != null {
			text, null, mixed = "", false, true
			break
		}
	}
	show := text
	if pretty, ok := prettyValue(show); ok && !sh.prefs.RawJSON && len(rows) == 1 {
		show = pretty
	}
	field := ui.Column(c.Key(name)).Gap(3).FillWidth()
	hover := field.Hovered()
	field.Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Height(20).Children(func() {
			ui.Text(c, name).FontSize(font2XS - 1).FontWeight(500).TextColor(p.fgSecondary).SingleLine()
			ui.Text(c, typ).FontSize(10).TextColor(p.fgMuted).SingleLine()
			ui.Spacer(c)
			if hover && !null && !mixed && tipBtn(c, icCopy, locales.PreviewCopy()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				sh.copyText(text)
			}
		})
		ph := ""
		switch {
		case mixed:
			ph = locales.TableMultiValue()
		case null:
			ph = locales.TableNull()
		}
		buf := show
		if strings.Contains(show, "\n") || len([]rune(show)) > 60 {
			ui.TextArea(c.Key("v"), &buf).Lines(2, 8).Placeholder(ph).FillWidth().Font(mono).FontSize(font2XS).ReadOnly(true)
		} else {
			ui.TextInput(c.Key("v"), &buf).Placeholder(ph).FillWidth().Height(sizeBtnSm).Font(mono).FontSize(font2XS).ReadOnly(true)
		}
	})
}
