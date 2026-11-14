package ui

import (
	"math"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// ER 图标签的界面：顶栏（搜索、范围、缩放）、画布（自绘，平移缩放拖表）、底栏（选中的表）。

func (e *erTab) view(c *ui.Context, p tokens) {
	ui.Toolbar(c, func() { e.toolbar(c, p) }).Padding(0)
	hairline(c, p.border)
	switch {
	case e.err != "":
		ui.Text(c, locales.TableLoadFail(e.err)).TextColor(p.danger).FontSize(font2XS).Padding(12).Selectable()
		return
	case e.loading && e.full == nil:
		quietLoading(c, p, &e.shownAt)
		return
	case e.full == nil || len(e.full.Nodes) == 0:
		ui.Text(c, locales.ErEmpty()).FontSize(font2XS).TextColor(p.fgMuted).Padding(24).TextAlign(ui.Center).FillWidth()
		return
	}
	if e.notice != "" {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 12).FillWidth().Shrink(0).Background(p.warning.Alpha(0.12)).Children(func() {
			ui.Text(c, e.notice).FontSize(font2XS).Grow(1).MinWidth(0).SingleLine()
			if e.focus != "" && actBtn(c.Key("er-all"), icTable, locales.ErShowAll()).Clicked() {
				e.setFocus("", -1)
			}
		})
	}
	e.canvas(c, p)
}

// toolbar 是顶栏（设计稿 A）：左边「N 张表 · M 个外键」和层数分段，右边定位搜索、缩放、适应窗口、重新布局。
func (e *erTab) toolbar(c *ui.Context, p tokens) {
	ui.Row(c).Gap(spaceSM).Padding(0, spaceSM+2, 0, spaceLG).AlignItems(ui.Center).FillWidth().Height(sizeBar).Shrink(0).Background(p.surface).Children(func() {
		if e.full != nil {
			ui.Text(c, locales.ErStats(len(e.full.Nodes), len(e.full.Edges))).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		}
		if e.loading {
			ui.Spinner(c)
		}
		if e.full == nil {
			return
		}
		idx := e.depth
		if idx < 0 {
			idx = 3
		} else {
			idx = min(idx, 3) - 1
		}
		if e.focus != "" || e.depth < 0 {
			if segmented(c.Key("er-depth"), &idx, locales.ErDepth1(), locales.ErDepth2(), locales.ErDepth3(), locales.ErDepthAll()) {
				focus := e.focus
				if focus == "" {
					focus = e.sel
				}
				if idx == 3 || focus == "" {
					e.setFocus("", -1)
				} else {
					e.setFocus(focus, idx+1)
				}
			}
		}
		ui.Spacer(c)
		in := ui.SearchField(c.Key("erq"), &e.q).Placeholder(locales.ErSearch()).Height(sizeBtn).FontSize(fontCtl).Width(sizeSearch + 10)
		if in.Submitted() {
			e.search()
		}
		if tipBtn(c.Key("er-out"), icMinus, locales.ErZoomOut()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			e.zoomAt(e.vw/2, e.vh/2, 1/1.25)
		}
		ui.Text(c, itoa(int(math.Round(float64(e.zoom)*100)))+"%").FontSize(fontCaption).TextColor(p.fgSecondary).Width(sizeRowNo).TextAlign(ui.Center)
		if tipBtn(c.Key("er-in"), icPlus, locales.ErZoomIn()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			e.zoomAt(e.vw/2, e.vh/2, 1.25)
		}
		if tipBtn(c.Key("er-fit"), icSplit, locales.ErFit()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			e.fit()
		}
		if tipBtn(c.Key("er-relayout"), icRefresh, locales.ErRelayout()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
			e.relayout()
		}
	})
}

// canvas 是画布元素：自绘，输入由 HandleInput 接；右下角浮着一张操作提示（选中表时换成表的情况和两个动作）。
func (e *erTab) canvas(c *ui.Context, p tokens) {
	ui.Box(c.Key("er-canvas")).Grow(1).FillWidth().MinHeight(0).Clip().Background(p.surface).
		Draw(func(pt *ui.Painter, r ui.Rect) { e.draw(pt, r, p) }).
		HandleInput(e.handle).Children(func() {
		e.hintCard(c, p)
	})
}

// handle 处理画布上的鼠标：按在表上拖表、按在空白处平移，双击表打开它，滚轮平移，⌘ 加滚轮或捏合缩放。
func (e *erTab) handle(ev ui.InputEvent) bool {
	wx, wy := ev.X/e.zoom+e.panX, ev.Y/e.zoom+e.panY
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		hit := e.nodeAt(wx, wy)
		e.press = erPress{down: true, node: hit, x: ev.X, y: ev.Y}
		e.sel = hit
		if hit != "" && ev.Clicks >= 2 {
			e.press = erPress{}
			e.w.openTable(hit)
		}
		return true
	case ui.InputPointerMove:
		if e.press.down && ev.Button == 0 {
			dx, dy := (ev.X-e.press.x)/e.zoom, (ev.Y-e.press.y)/e.zoom
			e.press.x, e.press.y = ev.X, ev.Y
			if e.press.node != "" {
				r := e.rects[e.press.node]
				r.X, r.Y = r.X+dx, r.Y+dy
				e.rects[e.press.node] = r
				e.press.moved = true
			} else {
				e.panX, e.panY = e.panX-dx, e.panY-dy
			}
			return true
		}
		if h := e.nodeAt(wx, wy); h != e.hover {
			e.hover = h
			return true
		}
		return false
	case ui.InputPointerUp:
		if e.press.down && e.press.moved && e.press.node != "" {
			if e.manual == nil {
				e.manual = map[string][2]float32{}
			}
			r := e.rects[e.press.node]
			e.manual[e.press.node] = [2]float32{r.X, r.Y}
			e.saveLayout()
		}
		e.press = erPress{}
		return true
	case ui.InputScroll:
		if ev.Mods&(ui.Cmd|ui.Ctrl) != 0 {
			e.zoomAt(ev.X, ev.Y, float32(math.Exp(float64(-ev.DY)*0.01)))
		} else {
			e.panX, e.panY = e.panX+ev.DX/e.zoom, e.panY+ev.DY/e.zoom
		}
		return true
	case ui.InputZoom:
		e.zoomAt(ev.X, ev.Y, ev.Scale)
		return true
	}
	return false
}

// hintCard 是画布右下角的浮层卡片：选中的表的情况和两个动作；没有选中就提示操作方法。
func (e *erTab) hintCard(c *ui.Context, p tokens) {
	ui.Row(c.Key("er-hint")).Absolute().Right(spaceLG).Bottom(spaceLG).Gap(spaceSM).AlignItems(ui.Center).Padding(0, spaceSM+2).Height(sizeBtn+2).
		Background(p.surfaceElev).Border(1, p.border).Radius(radiusMenu - 1).Children(func() {
		n, ok := e.full.Node(e.sel)
		if !ok {
			ui.Text(c, locales.ErHint()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
			return
		}
		out, in := 0, 0
		for _, ed := range e.full.Edges {
			if ed.From == n.Name && ed.To != n.Name {
				out++
			}
			if ed.To == n.Name && ed.From != n.Name {
				in++
			}
		}
		ui.Text(c, n.Name).FontSize(fontCtl).FontWeight(600).SingleLine().Shrink(1)
		ui.Text(c, locales.ErSelected(len(n.Columns), out, in)).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine()
		if n.Comment != "" {
			ui.Text(c, n.Comment).FontSize(fontCaption).TextColor(p.fgSecondary).SingleLine().Grow(1).MinWidth(0)
		} else {
			ui.Spacer(c)
		}
		if actBtn(c.Key("er-open"), icTable, locales.ErOpenTable()).Clicked() {
			e.w.openTable(n.Name)
		}
		if actBtn(c.Key("er-center"), icSearch, locales.ErFocusHere()).Clicked() {
			d := e.depth
			if d < 0 {
				d = 1
			}
			e.setFocus(n.Name, d)
		}
	})
}
