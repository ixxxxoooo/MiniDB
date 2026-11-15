package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// @author ixxxxoooo
//
// 表「文档」页（设计稿 doc）：标题 + 表备注标签，下面一句说明，再是三列的字段表（字段 / 类型 / 说明）。
// 字段名是代码小药丸，类型是大写灰字；AI 草拟的说明尾部带「✦ AI」。顶栏右边是「AI 补全注释」「复制 Markdown」（docActions）。

const (
	docFieldW = 220 // 字段列宽
	docTypeW  = 200 // 类型列宽；说明列吃剩下的
	docLineH  = 2   // 表头下面那条分隔线的粗细
)

// docActions 画顶栏右边的两个操作。参数：t 是表标签。
func (t *tableTab) docActions(c *ui.Context, p tokens) {
	if t.doc.generating {
		ui.Spinner(c)
		ui.Text(c, locales.DocGenerating()).FontSize(fontDense).TextColor(p.fgMuted).SingleLine()
	}
	if ghostBtn(c, icSparkle, locales.DocAIFill()).Disabled(t.doc.generating || !t.st.loaded).Clicked() {
		t.fillDoc()
	}
	if ghostBtn(c, icCopy, locales.DocCopyMarkdown()).Disabled(!t.st.loaded).Clicked() {
		t.copyDocMarkdown()
	}
}

// aiTag 是「✦ AI」小标记：label 为空写 AI，也可以写成「AI 草拟」。
func aiTag(c *ui.Context, p tokens, label string) {
	ui.Row(c).Gap(2).AlignItems(ui.Center).Shrink(0).Margin(0, 0, 0, spaceSM).Children(func() {
		ui.Icon(c, icSparkle).FontSize(fontDense).TextColor(p.accentText)
		ui.Text(c, label).FontSize(fontDense).TextColor(p.accentText).SingleLine()
	})
}

// docView 画文档页。
func (t *tableTab) docView(c *ui.Context) {
	p := baseTokens(c)
	d := &t.doc
	switch {
	case d.err != "":
		ui.Text(c, locales.TableLoadFail(d.err)).TextColor(p.danger).Padding(spaceLG).Selectable()
		return
	case t.st.err != "":
		ui.Text(c, locales.TableLoadFail(t.st.err)).TextColor(p.danger).Padding(spaceLG).Selectable()
		return
	case !d.loaded || !t.st.loaded:
		if !t.st.loaded && !t.st.loading {
			t.st.load(t)
		}
		quietLoading(c, p, &d.shownAt)
		return
	}
	scrollbars(c, ui.Scroll(c.Key("doc")).Fill()).Children(func() {
		ui.Column(c).Padding(spaceXL, spaceXL, spaceXL+spaceLG, spaceXL).Gap(spaceXL - 2).FillWidth().Children(func() {
			t.docHead(c, p)
			t.docTable(c, p)
		})
	})
}

// docHead 画标题、表备注标签和一句说明（没有说明时给一个「AI 草拟」的入口）。
func (t *tableTab) docHead(c *ui.Context, p tokens) {
	ui.Column(c.Key("doc-head")).Gap(spaceSM).FillWidth().Children(func() {
		ui.Row(c).Gap(spaceSM).AlignItems(ui.Center).Children(func() {
			ui.Text(c, t.table).FontSize(fontHeading).FontWeight(650).SingleLine().Selectable()
			if cm := t.tableComment(); cm != "" {
				sqChip(c, chipInfo, cm)
			}
		})
		desc, ai := t.docDesc()
		ui.Row(c.Key("doc-desc")).AlignItems(ui.Center).FillWidth().Children(func() {
			if desc == "" {
				ui.Text(c, locales.DocNoDesc()).FontSize(fontTitle).TextColor(p.fgMuted).SingleLine()
				if t.doc.generating {
					return
				}
				b := ui.Button(c.Key("doc-draft"), "").Label(locales.DocAIDraft()).Padding(0).Background(ui.Color{}).BorderWidth(0).Margin(0, 0, 0, spaceSM)
				b.Children(func() { aiTag(c, p, locales.DocAIDraft()) })
				if b.Clicked() {
					t.fillDoc()
				}
				return
			}
			ui.Text(c, desc).FontSize(fontTitle).TextColor(p.fgSecondary).Selectable().Shrink(1).MinWidth(0)
			if ai {
				aiTag(c, p, locales.DocAIDraft())
			}
		})
	})
}

// docTable 画字段表：表头、粗一点的分隔线，然后每个字段一行。
func (t *tableTab) docTable(c *ui.Context, p tokens) {
	cell := func(w float32, grow bool) ui.Element {
		e := ui.Row(c).AlignItems(ui.Center).MinWidth(0).Padding(0, spaceSM+2)
		if grow {
			return e.Grow(1)
		}
		return e.Width(w).Shrink(0)
	}
	ui.Column(c.Key("doc-table")).FillWidth().Children(func() {
		ui.Row(c).FillWidth().Height(sizeRow + spaceSM).AlignItems(ui.Center).Children(func() {
			for i, h := range []string{locales.DocColField(), locales.DocColType(), locales.DocColDesc()} {
				cell([]float32{docFieldW, docTypeW, 0}[i], i == 2).Children(func() {
					ui.Text(c, h).FontSize(fontCtl).FontWeight(600).TextColor(p.fgMuted).SingleLine()
				})
			}
		})
		ui.Box(c.Key("doc-th-line")).FillWidth().Height(docLineH).Background(p.border)
		for _, r := range t.docRows() {
			r := r
			ui.Row(c.Key("doc-"+r.name)).FillWidth().MinHeight(sizeRow+sizeRow/2).AlignItems(ui.Center).Padding(spaceSM, 0).Children(func() {
				cell(docFieldW, false).Children(func() {
					ui.Row(c).Padding(1, spaceSM-2).Radius(radiusSm).Background(p.segTrack).Children(func() {
						ui.Text(c, r.name).Font(mono).FontSize(fontCtl).SingleLine().Selectable()
					})
				})
				cell(docTypeW, false).Children(func() {
					ui.Text(c, strings.ToUpper(r.typ)).FontSize(fontBody).TextColor(p.fgMuted).SingleLine().TruncateMiddle()
				})
				cell(0, true).Children(func() {
					ui.Text(c, r.comment).FontSize(fontBody).Selectable().Shrink(1).MinWidth(0)
					if r.ai {
						aiTag(c, p, locales.DocAITag())
					}
				})
			})
			hairline(c, p.borderSubtle)
		}
	})
}
