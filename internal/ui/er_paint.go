package ui

import (
	"math"
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

// drawNode 画一张表：表头、列清单（主键和外键带图标）、边框。
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
		border, bw = p.accent, 2
	} else if n.Name == e.hover {
		border = p.accent
	}
	pt.Fill(box, p.surface.Alpha(a), 6)
	pt.FillRadii(ui.Rect{X: x, Y: y, W: b.W, H: erHeadH}, [4]float32{6, 6, 0, 0}, p.iconTable.Alpha(0.16*a))
	pt.Stroke(box, border.Alpha(a), 6, bw)
	pt.Text(x+erPadX, y+7, clipLabel(n.Name, int(b.W/7.5)), 13, p.fg.Alpha(a))
	rows, hidden := erRows(n)
	for i, col := range rows {
		ry := y + erHeadH + float32(i)*erRowH
		switch {
		case col.PK:
			pt.Icon(icKey, ui.Rect{X: x + 7, Y: ry + 4, W: 11, H: 11}, p.warning.Alpha(a))
		case col.FK:
			pt.Icon(icLink, ui.Rect{X: x + 7, Y: ry + 4, W: 11, H: 11}, p.accent.Alpha(a))
		}
		nameCol := p.fg
		if col.PK {
			nameCol = p.fg
		} else if !col.FK {
			nameCol = p.fgSecondary
		}
		pt.Text(x+24, ry+3, clipLabel(col.Name, 28), erFont, nameCol.Alpha(a))
		typ := clipLabel(strings.ToLower(col.Type), 16)
		pt.Text(x+b.W-erPadX-textWidth(typ, erFont-1), ry+4, typ, erFont-1, p.fgMuted.Alpha(a))
	}
	if hidden > 0 {
		pt.Text(x+24, y+erHeadH+float32(len(rows))*erRowH+3, locales.ErColumnsMore(hidden), erFont, p.fgMuted.Alpha(a))
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
	col, w := p.fgMuted, float32(1.2)
	if lit {
		col, w = p.accent, 1.8
	} else if !bright {
		col = col.Alpha(0.35)
	}
	var path ui.Path
	path.MoveTo(v.X+sx, v.Y+fy).CubeTo(v.X+sx+ds*k, v.Y+fy, v.X+ex+de*k, v.Y+ty, v.X+ex, v.Y+ty)
	pt.StrokePath(&path, w, col)
	// 引用方一端一个小圆点，被引用方一端一个箭头
	pt.Fill(ui.Rect{X: v.X + sx - 2.5, Y: v.Y + fy - 2.5, W: 5, H: 5}, col, 2.5)
	var head ui.Path
	ax, ay := v.X+ex, v.Y+ty
	head.MoveTo(ax, ay).LineTo(ax+de*8, ay-4).LineTo(ax+de*8, ay+4).Close()
	pt.FillPath(&head, col)
}

func first(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[0]
}
