package ui

import (
	"math"

	"github.com/egoist/mygo/ui"
	"strconv"
	"strings"
)

// niceScale gives axis bounds and a tick step that land on round numbers.
func niceScale(lo, hi float64, ticks int) (float64, float64, float64) {
	if lo > 0 {
		lo = 0 // bars and lines start at zero unless there are negatives
	}
	if hi < 0 {
		hi = 0
	}
	if hi == lo {
		hi = lo + 1
	}
	raw := (hi - lo) / float64(max(ticks, 1))
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	var step float64
	switch f := raw / mag; {
	case f <= 1:
		step = mag
	case f <= 2:
		step = 2 * mag
	case f <= 5:
		step = 5 * mag
	default:
		step = 10 * mag
	}
	return math.Floor(lo/step) * step, math.Ceil(hi/step) * step, step
}

// formatNumber writes a value short: 1.2k, 3.4M, 0.25.
func formatNumber(v float64) string {
	a := math.Abs(v)
	switch {
	case a >= 1e9:
		return trimZero(v/1e9) + "B"
	case a >= 1e6:
		return trimZero(v/1e6) + "M"
	case a >= 1e4:
		return trimZero(v/1e3) + "k"
	case a == math.Trunc(a):
		return strconv.FormatFloat(v, 'f', 0, 64)
	case a >= 100:
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func trimZero(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	return strings.TrimSuffix(s, ".0")
}

// textWidth guesses a label's width: CJK characters are as wide as they are tall.
func textWidth(s string, size float32) float32 {
	var w float32
	for _, r := range s {
		if r > 0x2E80 {
			w += size
		} else {
			w += size * 0.58
		}
	}
	return w
}

func clipLabel(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

var chartPalette = []string{"#0a84ff", "#30d158", "#ff9f0a", "#bf5af2", "#ff453a", "#64d2ff", "#ffd60a", "#ac8e68", "#5e5ce6", "#ff6482", "#66d4cf"}

func chartColor(i int) ui.Color { return ui.Hex(chartPalette[i%len(chartPalette)]) }

type chartTheme struct {
	grid, axis, text, bg ui.Color
}

func (t chartTheme) fontSize() float32 { return 11 }

// paintChart draws the chart of kind into r.
func paintChart(pt *ui.Painter, r ui.Rect, kind string, d chartData, th chartTheme, other string) {
	if len(d.Labels) == 0 || len(d.Series) == 0 {
		return
	}
	switch kind {
	case chartPie:
		paintPie(pt, r, d, th, other)
	case chartLine:
		paintAxes(pt, r, d, th, false)
	default:
		paintAxes(pt, r, d, th, true)
	}
}

// paintAxes draws a line or a bar chart: grid, value axis, category labels, then the series.
func paintAxes(pt *ui.Painter, r ui.Rect, d chartData, th chartTheme, bars bool) {
	fs := th.fontSize()
	lo, hi := d.bounds()
	lo, hi, step := niceScale(lo, hi, 5)
	var tickLabels []string
	for v := lo; v <= hi+step/2; v += step {
		tickLabels = append(tickLabels, formatNumber(v))
	}
	left := float32(8)
	for _, s := range tickLabels {
		left = max(left, textWidth(s, fs)+14)
	}
	legendH := float32(0)
	if len(d.Series) > 1 {
		legendH = 22
	}
	plot := ui.Rect{X: r.X + left, Y: r.Y + 12 + legendH, W: r.W - left - 16, H: r.H - 12 - legendH - 30}
	if plot.W < 40 || plot.H < 40 {
		return
	}
	yOf := func(v float64) float32 { return plot.Y + plot.H - float32((v-lo)/(hi-lo))*plot.H }

	// grid lines and the value axis
	for v := lo; v <= hi+step/2; v += step {
		y := yOf(v)
		pt.Line(plot.X, y, plot.X+plot.W, y, 1, th.grid)
		s := formatNumber(v)
		pt.Text(plot.X-textWidth(s, fs)-6, y-fs/2-1, s, fs, th.text)
	}
	if zero := yOf(0); lo < 0 {
		pt.Line(plot.X, zero, plot.X+plot.W, zero, 1, th.axis)
	}

	n := len(d.Labels)
	catW := plot.W / float32(n)
	// category labels, thinned so they do not touch
	maxLabel := 0.0
	for _, l := range d.Labels {
		maxLabel = math.Max(maxLabel, float64(textWidth(clipLabel(l, 12), fs)))
	}
	every := max(1, int(math.Ceil((maxLabel+10)/float64(catW))))
	for i, l := range d.Labels {
		if i%every != 0 {
			continue
		}
		s := clipLabel(l, 12)
		w := textWidth(s, fs)
		x := plot.X + catW*(float32(i)+0.5) - w/2
		x = min(max(x, r.X+2), r.X+r.W-w-2)
		pt.Text(x, plot.Y+plot.H+8, s, fs, th.text)
	}

	if bars {
		group := min(catW*0.72, 48*float32(len(d.Series)))
		barW := group / float32(len(d.Series))
		for si, s := range d.Series {
			for i, v := range s.Values {
				x := plot.X + catW*(float32(i)+0.5) - group/2 + barW*float32(si)
				y0, y1 := yOf(0), yOf(v)
				top := min(y0, y1)
				h := max(float32(math.Abs(float64(y0-y1))), 1)
				pt.Fill(ui.Rect{X: x + 0.5, Y: top, W: max(barW-1, 1), H: h}, chartColor(si), min(3, barW/3))
				if barW >= 22 && n <= 24 {
					l := formatNumber(v)
					pt.Text(x+barW/2-textWidth(l, fs-1)/2, top-fs-2, l, fs-1, th.text)
				}
			}
		}
	} else {
		for si, s := range d.Series {
			pts := make([][2]float32, len(s.Values))
			for i, v := range s.Values {
				pts[i] = [2]float32{plot.X + catW*(float32(i)+0.5), yOf(v)}
			}
			if len(pts) > 1 {
				pt.StrokePolyline(pts, 2, chartColor(si))
			}
			if n <= 60 {
				for _, p := range pts {
					pt.Fill(ui.Rect{X: p[0] - 3, Y: p[1] - 3, W: 6, H: 6}, chartColor(si), 3)
				}
			}
		}
	}
	// legend, for more than one series
	if legendH > 0 {
		x := plot.X
		for si, s := range d.Series {
			pt.Fill(ui.Rect{X: x, Y: r.Y + 6, W: 10, H: 10}, chartColor(si), 2)
			name := clipLabel(s.Name, 20)
			pt.Text(x+14, r.Y+5, name, fs, th.text)
			x += 14 + textWidth(name, fs) + 16
		}
	}
}

// paintPie draws the first series as slices with a legend beside them.
func paintPie(pt *ui.Painter, r ui.Rect, d chartData, th chartTheme, other string) {
	slices := pieSlices(d, other)
	if len(slices) == 0 {
		return
	}
	fs := th.fontSize()
	legendW := float32(0)
	for _, s := range slices {
		legendW = max(legendW, textWidth(clipLabel(s.Label, 18)+"  100%", fs)+24)
	}
	area := ui.Rect{X: r.X + 16, Y: r.Y + 12, W: r.W - legendW - 40, H: r.H - 24}
	if area.W < 60 {
		area.W, legendW = r.W-32, 0
	}
	radius := min(area.W, area.H) / 2
	if radius < 20 {
		return
	}
	cx, cy := area.X+area.W/2, area.Y+area.H/2
	angle := -math.Pi / 2
	for i, s := range slices {
		sweep := s.Share * 2 * math.Pi
		var path ui.Path
		if s.Share >= 0.9999 {
			path.Circle(cx, cy, radius)
		} else {
			path.MoveTo(cx, cy)
			segs := max(2, int(sweep/(math.Pi/45)))
			for k := 0; k <= segs; k++ {
				a := angle + sweep*float64(k)/float64(segs)
				path.LineTo(cx+radius*float32(math.Cos(a)), cy+radius*float32(math.Sin(a)))
			}
			path.Close()
		}
		pt.FillPath(&path, chartColor(i))
		pt.StrokePath(&path, 1.5, th.bg)
		angle += sweep
	}
	if legendW > 0 {
		x, y := r.X+r.W-legendW-8, r.Y+16
		for i, s := range slices {
			if y+fs > r.Y+r.H-8 {
				break
			}
			pt.Fill(ui.Rect{X: x, Y: y + 1, W: 10, H: 10}, chartColor(i), 2)
			pt.Text(x+16, y, clipLabel(s.Label, 18)+"  "+strconv.Itoa(int(math.Round(s.Share*100)))+"%", fs, th.text)
			y += fs + 8
		}
	}
}
