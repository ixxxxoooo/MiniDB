package ui

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// ER 图画布的绘制：表、外键曲线，随视图平移缩放。

// draw 画整张图：先边后表，悬停的表和与它相连的边、表保持原色，其余变淡。
func (e *erTab) draw(pt *ui.Painter, r ui.Rect, p tokens) {
	e.vw, e.vh = r.W, r.H
	if e.needFit {
		e.needFit = false
		e.fit()
	}
	if e.shown == nil {
		return
	}
	focus := e.hover
	if focus == "" {
		focus = e.sel
	}
	var near map[string]bool
	if focus != "" {
		near = e.related(focus)
	}
	e.paintDots(pt, r, p)
	view := ui.Translation(-e.panX, -e.panY).Then(ui.Scaling(e.zoom, e.zoom))
	pt.Clip(r, 0, func() {
		pt.Transform(view, r.X, r.Y, func() {
			for _, ed := range e.shown.Edges {
				lit := focus != "" && (ed.From == focus || ed.To == focus)
				e.drawEdge(pt, r, p, ed, lit, focus == "" || lit)
			}
			for _, n := range e.shown.Nodes {
				e.drawNode(pt, r, p, n, near == nil || near[n.Name])
			}
		})
	})
}

// drawNode 画一张表（设计稿 A）：白底圆角卡片，表头是浅灰底 + 表图标 + 粗体表名 + 右侧列数，
// 下面每列一行：主键（钥匙）和外键（链接）带图标，主键名加粗，类型靠右淡色。
func (e *erTab) drawNode(pt *ui.Painter, v ui.Rect, p tokens, n model.ERNode, lit bool) {
	b := e.rects[n.Name]
	x, y := v.X+b.X, v.Y+b.Y
	box := ui.Rect{X: x, Y: y, W: b.W, H: b.H}
	a := float32(1)
	if !lit {
		a = 0.35
	}
	border, bw := p.border, float32(1)
	if n.Name == e.sel {
		border, bw = p.accent, 1.5
	} else if n.Name == e.hover {
		border = p.accent
	}
	pt.Fill(box, p.surface.Alpha(a), radiusPanel-2)
	pt.FillRadii(ui.Rect{X: x, Y: y, W: b.W, H: erHeadH}, [4]float32{radiusPanel - 2, radiusPanel - 2, 0, 0}, p.surfaceSecondary.Alpha(a))
	pt.Line(x, y+erHeadH, x+b.W, y+erHeadH, 1, p.borderSubtle.Alpha(a))
	pt.Stroke(box, border.Alpha(a), radiusPanel-2, bw)
	pt.Icon(icTable, ui.Rect{X: x + 8, Y: y + (erHeadH-erIcon)/2, W: erIcon, H: erIcon}, p.fg.Alpha(a))
	cnt := strconv.Itoa(len(n.Columns))
	pt.Text(x+b.W-erPadX+2-textWidth(cnt, fontMicro), y+(erHeadH-fontMicro)/2-1, cnt, fontMicro, p.fgFaint.Alpha(a))
	pt.Text(x+8+erIcon+6, y+(erHeadH-fontCtl)/2-1, clipLabel(n.Name, int(b.W/7.5)), fontCtl, p.fg.Alpha(a))
	rows, hidden := erRows(n)
	for i, col := range rows {
		ry := y + erHeadH + float32(i)*erRowH
		switch {
		case col.PK:
			pt.Icon(icKey, ui.Rect{X: x + 8, Y: ry + (erRowH-erIcon)/2, W: erIcon, H: erIcon}, p.fg.Alpha(a))
		case col.FK:
			pt.Icon(icLink, ui.Rect{X: x + 8, Y: ry + (erRowH-erIcon)/2, W: erIcon, H: erIcon}, p.fgSecondary.Alpha(a))
		}
		pt.Text(x+8+erIcon+6, ry+(erRowH-erFont)/2-1, clipLabel(col.Name, 28), erFont, p.fg.Alpha(a))
		typ := clipLabel(strings.ToLower(col.Type), 16)
		pt.Text(x+b.W-erPadX+2-textWidth(typ, fontMicro), ry+(erRowH-fontMicro)/2-1, typ, fontMicro, p.fgFaint.Alpha(a))
	}
	if hidden > 0 {
		pt.Text(x+8+erIcon+6, y+erHeadH+float32(len(rows))*erRowH+3, locales.ErColumnsMore(hidden), erFont, p.fgMuted.Alpha(a))
	}
}

// drawEdge 画一个外键：从引用方那一列的侧边，到被引用方那一列的侧边，三次曲线，被引用方一端有箭头。
func (e *erTab) drawEdge(pt *ui.Painter, v ui.Rect, p tokens, ed model.EREdge, lit, bright bool) {
	from, to := e.rects[ed.From], e.rects[ed.To]
	fn, _ := e.full.Node(ed.From)
	tn, _ := e.full.Node(ed.To)
	fy := from.Y + erAnchorY(fn, first(ed.FromCols))
	ty := to.Y + erAnchorY(tn, first(ed.ToCols))
	var sx, ex, ds, de float32 // 起止点的 x，和曲线离开两端的方向
	switch {
	case from.X >= to.X+to.W:
		sx, ex, ds, de = from.X, to.X+to.W, -1, 1
	case to.X >= from.X+from.W:
		sx, ex, ds, de = from.X+from.W, to.X, 1, -1
	default: // 同一层或自引用：都从右边出入，向右鼓出去
		sx, ex, ds, de = from.X+from.W, to.X+to.W, 1, 1
	}
	k := max(40, float32(math.Abs(float64(ex-sx)))/2)
	if ds == de && ds == 1 {
		k = 46
	}
	col, w := p.fgFaint, float32(1.4)
	if lit {
		col, w = p.accent, 1.8
	} else if !bright {
		col = col.Alpha(0.35)
	}
	var path ui.Path
	path.MoveTo(v.X+sx, v.Y+fy).CubeTo(v.X+sx+ds*k, v.Y+fy, v.X+ex+de*k, v.Y+ty, v.X+ex, v.Y+ty)
	pt.StrokePath(&path, w, col)
	// 引用方一端一个强调色圆点，被引用方一端一道竖线（「一」的一端）
	dot := p.accent
	if !lit && !bright {
		dot = dot.Alpha(0.35)
	}
	pt.Fill(ui.Rect{X: v.X + sx - 2.5, Y: v.Y + fy - 2.5, W: 5, H: 5}, dot, 2.5)
	ax, ay := v.X+ex+de*3, v.Y+ty
	pt.Line(ax, ay-5, ax, ay+5, w, col)
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

// paintDots 画画布的点阵底纹（设计稿 A）：间距固定，跟着平移走。
func (e *erTab) paintDots(pt *ui.Painter, r ui.Rect, p tokens) {
	ox := float32(math.Mod(float64(-e.panX*e.zoom), erDotGap))
	oy := float32(math.Mod(float64(-e.panY*e.zoom), erDotGap))
	if ox < 0 {
		ox += erDotGap
	}
	if oy < 0 {
		oy += erDotGap
	}
	for y := r.Y + oy; y < r.Y+r.H; y += erDotGap {
		for x := r.X + ox; x < r.X+r.W; x += erDotGap {
			pt.Fill(ui.Rect{X: x, Y: y, W: erDotSize, H: erDotSize}, p.border, erDotSize/2)
		}
	}
}
