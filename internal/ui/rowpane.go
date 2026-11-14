package ui

// @author ixxxxoooo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/internal/model"
	"minidb/locales"
)

// 行预览栏：表数据页和查询结果页共用这一份。选中行按空格，在网格右边打开一个窄栏，
// 每个字段一行，字段名在左、类型在右，下面是值（JSON 用带语法高亮的编辑器）。
// 两边的差别只有 paneSource 里那几项：表数据页能就地改值、字段右端有下拉菜单；
// 查询结果页是只读的，没有下拉箭头。

// rowPane 是行预览栏的状态：字段搜索，以及各字段的输入缓冲、编辑器、菜单展开状态。
type rowPane struct {
	q        string
	buf, sig map[string]string
	menu     map[string]*bool // 各字段的下拉菜单是否展开
	json     map[string]bool  // 各字段当前用 JSON 编辑器
	ed       map[string]*editor.State
	own      map[string]string // 各字段刚由用户输入提交的文字：数据随之更新时不要反过来改写输入框
}

// paneSource 是行预览栏的数据来源和可做的操作。
type paneSource struct {
	g       *resultGrid
	ro      bool // 只读：值不能改
	rawJSON bool // 不自动缩进 JSON
	// commit 把用户输入的新值写回选中的行，返回是否接受；nil 表示不能改。
	commit func(rows []int, col, v string) bool
	// menu 画字段右端下拉箭头里的菜单；nil 表示没有下拉箭头。
	menu    func(m *ui.Menu, rows []int, col model.ColumnMeta, text string)
	onClose func() // 点右上角的关闭
}

// chosenRows 是网格里选中的有效行（升序）。
func (g *resultGrid) chosenRows() []int {
	var rows []int
	for r := range g.chosen.All() {
		if r >= 0 && r < len(g.rows) {
			rows = append(rows, r)
		}
	}
	sort.Ints(rows)
	return rows
}

// fieldValue 是一个字段在多行里的文字：几行一致就是这个值，否则算「多个值」。
func (g *resultGrid) fieldValue(rows []int, col string) (text string, null, mixed bool) {
	for i, r := range rows {
		tx, n := cellDisplay(g.colMeta(col), g.rows[r][col])
		if i == 0 {
			text, null = tx, n
			continue
		}
		if tx != text || n != null {
			return "", false, true
		}
	}
	return text, null, false
}

// view 画整个预览栏：标题和关闭、字段搜索、字段列表。
func (pp *rowPane) view(c *ui.Context, p tokens, src paneSource) {
	rows := src.g.chosenRows()
	if pp.buf == nil {
		pp.buf, pp.sig = map[string]string{}, map[string]string{}
	}
	ui.Column(c).Grow(1).FillHeight().Background(p.surface).Children(func() {
		ui.Row(c).Gap(6).Padding(0, 8, 0, 12).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 2).Shrink(0).Children(func() {
			ui.Text(c, locales.PreviewTitle()).FontSize(font2XS).FontWeight(500)
			if len(rows) > 1 {
				ui.Text(c, locales.TableRows(len(rows))).FontSize(font2XS - 1).TextColor(p.fgMuted)
			}
			ui.Spacer(c)
			if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				src.onClose()
			}
		})
		hairline(c, p.borderSubtle)
		if len(rows) == 0 {
			ui.Text(c, locales.PreviewNone()).FontSize(font2XS).TextColor(p.fgMuted).Padding(14)
			return
		}
		ui.Row(c).Padding(8, 10, 2, 10).FillWidth().Shrink(0).Children(func() {
			ui.SearchField(c.Key("fieldq"), &pp.q).Placeholder(locales.TableFieldSearch()).Height(sizeBtnSm).FontSize(font2XS).FillWidth()
		})
		q := strings.ToLower(strings.TrimSpace(pp.q))
		scrollbars(c, ui.Scroll(c.Key("rowpreview")).Grow(1).FillWidth()).Children(func() {
			ui.Column(c).Gap(10).Padding(8, 12, 12, 12).FillWidth().Children(func() {
				for _, i := range src.g.vis {
					col := src.g.cols[i]
					if q != "" && !strings.Contains(strings.ToLower(col.Name), q) {
						continue
					}
					pp.field(c, p, src, rows, col)
				}
			})
		})
	})
}

// noBracketMatch 关掉编辑器里光标靠近括号时的配对描边：行预览和 Quick Look 里的括号照常显示，不加着重。
//
// 参数：ed 编辑器状态。无返回值。
func noBracketMatch(ed *editor.State) {
	// 编辑器把全 0 的颜色当作「用默认」，所以用一个看不见的颜色（透明度几乎为 0）
	ed.Theme = &editor.Theme{MatchingBracket: ui.RGBA(0, 0, 0, 0.004)}
}

// field 画一个字段：名称、类型，下面是值。
func (pp *rowPane) field(c *ui.Context, p tokens, src paneSource, rows []int, col model.ColumnMeta) {
	text, null, mixed := src.g.fieldValue(rows, col.Name)
	show := text
	if null || mixed {
		show = ""
	}
	if pretty, ok := prettyValue(show); ok && !src.rawJSON && len(rows) == 1 {
		show = pretty // JSON 缩进后好读；只有真的改了才会改到数据
	}
	if pp.json == nil {
		pp.json, pp.ed, pp.own = map[string]bool{}, map[string]*editor.State{}, map[string]string{}
	}
	// 输入框的内容跟着数据走，用户正在输入时除外。
	// 用哪种输入控件（单行、多行、JSON 编辑器）也在这时定下来，输入过程中不换，免得中途丢了焦点
	sig := fmt.Sprint(rows, "\x00", show, null, mixed)
	if pp.sig[col.Name] != sig {
		pp.sig[col.Name] = sig
		if own, ok := pp.own[col.Name]; ok && own == text {
			// 数据是随用户刚输入的内容更新的：输入框里已经是它，不要再改写（比如把 JSON 重新缩进）
		} else {
			pp.buf[col.Name] = show
			pp.json[col.Name] = isJSONText(show)
			delete(pp.own, col.Name)
			if ed := pp.ed[col.Name]; ed != nil {
				ed.Buffer.Set(show)
			}
		}
	}
	buf := pp.buf[col.Name]
	if pp.menu == nil {
		pp.menu = map[string]*bool{}
	}
	if pp.menu[col.Name] == nil {
		pp.menu[col.Name] = new(bool)
	}
	menuOpen := pp.menu[col.Name]
	ph := ""
	switch {
	case mixed:
		ph = locales.TableMultiValue()
	case null:
		ph = locales.TableNull()
	}
	// 二进制值在这里只读（<BLOB N 字节>），要改去 Quick Look 的十六进制编辑
	ro := src.ro || src.commit == nil || (isBlobType(col.Type) && strings.HasPrefix(text, "<BLOB "))
	commit := func(v string) {
		if ro || !src.commit(rows, col.Name, v) {
			return
		}
		pp.buf[col.Name], pp.own[col.Name] = v, v
	}
	ui.Column(c.Key(col.Name)).Gap(3).FillWidth().Children(func() {
		// 字段名在左，类型在右
		ui.Row(c).Gap(6).AlignItems(ui.Center).FillWidth().Height(20).Children(func() {
			ui.Text(c, col.Name).FontSize(font2XS - 1).FontWeight(500).TextColor(p.fgSecondary).SingleLine().Shrink(1).MinWidth(0)
			ui.Spacer(c)
			ui.Text(c, col.Type).FontSize(10).TextColor(p.fgMuted).SingleLine().Shrink(0)
		})
		// 输入框和它右端的下拉箭头叠在一起：箭头点开就是字段菜单（只读的预览没有箭头）
		ui.Column(c.Key("box")).FillWidth().Children(func() {
			switch {
			case pp.json[col.Name]:
				// JSON 值：和 Quick Look 一样用带语法高亮的编辑器
				ed := pp.ed[col.Name]
				if ed == nil {
					b := editor.NewBuffer(buf)
					b.SetLanguage(editor.LanguageNamed("JSON"))
					ed = &editor.State{Buffer: b, SoftWrap: true, LineNumbers: editor.LineNumbersOff, NoFolding: true, NoActiveLine: true}
					ed.Font.Size = font2XS
					noBracketMatch(ed)
					pp.ed[col.Name] = ed
				}
				ed.ReadOnly = ro
				editorBars(c, ed)
				lines := min(max(strings.Count(ed.Buffer.String(), "\n")+1, 2), 10)
				ui.Column(c.Key("val-j")).FillWidth().Height(float32(lines)*17+10).Clip().Radius(radiusBtn).
					Border(1, p.border).Background(p.surfaceSecondary).Children(func() {
					editor.View(c.Key("val-je"), ed).Grow(1).FillWidth()
				})
				if cur := ed.Buffer.String(); cur != buf {
					commit(cur)
				}
			case strings.Contains(text, "\n") || len([]rune(text)) > 60 || strings.Contains(buf, "\n"):
				in := ui.TextArea(c.Key("val-a"), &buf).Lines(2, 8).Placeholder(ph).FillWidth().Font(mono).FontSize(font2XS).ReadOnly(ro)
				if in.Changed() {
					commit(buf)
				}
			default:
				in := ui.TextInput(c.Key("val-i"), &buf).Placeholder(ph).FillWidth().Height(sizeBtnSm).Font(mono).FontSize(font2XS).ReadOnly(ro)
				if in.Changed() {
					commit(buf)
				}
			}
			if src.menu != nil {
				tipBtn(c.Key("menu"), icChevronD, locales.ConnMore()).Size(20, 20).Absolute().Top(2).Right(4).PopoverMenuButton(func(m *ui.Menu) {
					src.menu(m, rows, col, text)
				}, ui.PopoverMenuButtonOptions{Open: menuOpen})
			}
		})
	})
}
