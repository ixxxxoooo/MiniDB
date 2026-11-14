package ui

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/internal/sqltext"
	"minidb/locales"
)

// Charts of a query result: line, bar and pie, drawn with the Painter. The
// model below (chartSpec, chartData, niceScale, pieSlices) knows nothing of
// drawing, so it can be tested on its own.

const (
	chartLine = "line"
	chartBar  = "bar"
	chartPie  = "pie"

	maxChartPoints = 500 // more rows than this are thinned out
	maxPieSlices   = 10  // the rest become "Other"
)

// chartSpec says what to draw.
type chartSpec struct {
	Kind string
	X    string
	Ys   []string
}

type chartSeries struct {
	Name   string
	Values []float64
}

// chartData is the result's rows turned into labels and series.
type chartData struct {
	Labels []string
	Series []chartSeries
	Thin   bool // rows were left out
}

// numeric reads a cell as a number.
func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	case []byte:
		return numeric(string(n))
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

// numericColumns lists the columns whose first filled cells are all numbers.
func numericColumns(cols []model.ColumnMeta, rows []map[string]any) []string {
	var out []string
	for _, c := range cols {
		seen, ok := 0, true
		for _, r := range rows {
			if r[c.Name] == nil {
				continue
			}
			if _, isNum := numeric(r[c.Name]); !isNum {
				ok = false
				break
			}
			if seen++; seen >= 20 {
				break
			}
		}
		if ok && seen > 0 {
			out = append(out, c.Name)
		}
	}
	return out
}

// defaultChartSpec picks the columns for a result: the first text column (or
// the first column) across, the numeric ones up.
func defaultChartSpec(kind string, cols []model.ColumnMeta, rows []map[string]any) chartSpec {
	nums := numericColumns(cols, rows)
	spec := chartSpec{Kind: kind}
	if spec.Kind == "" {
		spec.Kind = chartBar
	}
	for _, c := range cols {
		if !containsStr(nums, c.Name) {
			spec.X = c.Name
			break
		}
	}
	if spec.X == "" && len(cols) > 0 {
		spec.X = cols[0].Name
	}
	for _, n := range nums {
		if n != spec.X {
			spec.Ys = []string{n}
			break
		}
	}
	return spec
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// chartSpecFor is the spec of a result: its "-- minidb …chart" comment, with
// the columns it leaves out chosen automatically.
func chartSpecFor(stmt string, cols []model.ColumnMeta, rows []map[string]any) (spec chartSpec, fromComment bool) {
	def := defaultChartSpec(chartBar, cols, rows)
	d, ok := sqltext.ParseChartDirective(stmt)
	if !ok {
		return def, false
	}
	spec = chartSpec{Kind: d.Kind, X: d.X, Ys: d.Ys}
	has := func(name string) bool {
		for _, c := range cols {
			if c.Name == name {
				return true
			}
		}
		return false
	}
	if !has(spec.X) {
		spec.X = def.X
	}
	var ys []string
	for _, y := range spec.Ys {
		if has(y) {
			ys = append(ys, y)
		}
	}
	if len(ys) == 0 {
		ys = def.Ys
	}
	spec.Ys = ys
	return spec, true
}

// buildChart turns rows into what the chart draws. Cells that are not
// numbers count as 0 (NULL) or are skipped (anything else) per series.
func buildChart(rows []map[string]any, spec chartSpec) chartData {
	step := 1
	if len(rows) > maxChartPoints {
		step = (len(rows) + maxChartPoints - 1) / maxChartPoints
	}
	var d chartData
	d.Thin = step > 1
	for _, y := range spec.Ys {
		d.Series = append(d.Series, chartSeries{Name: y})
	}
	for i := 0; i < len(rows); i += step {
		r := rows[i]
		label := ""
		if v := r[spec.X]; v != nil {
			label = fmt.Sprint(v)
			if b, ok := v.([]byte); ok {
				label = string(b)
			}
		}
		d.Labels = append(d.Labels, label)
		for k, y := range spec.Ys {
			v, _ := numeric(r[y])
			d.Series[k].Values = append(d.Series[k].Values, v)
		}
	}
	return d
}

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

func (d chartData) bounds() (lo, hi float64) {
	first := true
	for _, s := range d.Series {
		for _, v := range s.Values {
			if first {
				lo, hi, first = v, v, false
				continue
			}
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	return
}

type pieSlice struct {
	Label string
	Value float64
	Share float64 // 0..1
}

// pieSlices is the first series as shares of its positive total, the smallest
// folded into "Other" past maxPieSlices.
func pieSlices(d chartData, other string) []pieSlice {
	if len(d.Series) == 0 {
		return nil
	}
	var all []pieSlice
	total := 0.0
	for i, v := range d.Series[0].Values {
		if v > 0 {
			all = append(all, pieSlice{Label: d.Labels[i], Value: v})
			total += v
		}
	}
	if total == 0 {
		return nil
	}
	if len(all) > maxPieSlices {
		// keep the biggest maxPieSlices-1, in their original order; ties go to the earlier row
		order := make([]int, len(all))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool { return all[order[a]].Value > all[order[b]].Value })
		keep := map[int]bool{}
		for _, i := range order[:maxPieSlices-1] {
			keep[i] = true
		}
		var kept []pieSlice
		rest := 0.0
		for i, sl := range all {
			if keep[i] {
				kept = append(kept, sl)
			} else {
				rest += sl.Value
			}
		}
		all = append(kept, pieSlice{Label: other, Value: rest})
	}
	for i := range all {
		all[i].Share = all[i].Value / total
	}
	return all
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

// ---- 绘制 ----

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

// ---- 结果区里的图表页 ----

// chartState is a result's chart choice, made on the first look.
type chartState struct {
	ready bool
	spec  chartSpec
	data  chartData
	sig   string // the spec the data was built for
}

func chartKindLabels() []string {
	return []string{locales.ChartLine(), locales.ChartBar(), locales.ChartPie()}
}

var chartKinds = []string{chartLine, chartBar, chartPie}

func chartKindIndex(kind string) int {
	for i, k := range chartKinds {
		if k == kind {
			return i
		}
	}
	return 1
}

// ensureChart makes the chart state of a result on first use.
func (r *queryResult) ensureChart() {
	if r.chart.ready {
		return
	}
	spec, _ := chartSpecFor(r.stmt, r.res.Columns, r.res.Rows)
	r.chart.spec, r.chart.ready = spec, true
}

// chartView draws the picker and the chart of a result.
func (q *queryTab) chartView(c *ui.Context, r *queryResult, p tokens) {
	r.ensureChart()
	st := &r.chart
	var names, nums []string
	for _, col := range r.res.Columns {
		names = append(names, col.Name)
	}
	nums = numericColumns(r.res.Columns, r.res.Rows)
	ui.Row(c).Gap(10).Padding(6, 10).AlignItems(ui.Center).FillWidth().Shrink(0).Children(func() {
		kind := chartKindIndex(st.spec.Kind)
		if ui.Segmented(c.Key("ckind"), &kind, chartKindLabels()...).FontSize(font2XS).Changed() {
			st.spec.Kind = chartKinds[kind]
		}
		ui.Text(c, locales.ChartX()).FontSize(font2XS).TextColor(p.fgMuted)
		ui.Select(c.Key("cx"), &st.spec.X, names).Width(150).Height(sizeBtnSm)
		ui.Text(c, locales.ChartY()).FontSize(font2XS).TextColor(p.fgMuted)
		ui.TokenField(c.Key("cy"), &st.spec.Ys, nums).Grow(1).MinWidth(120)
	})
	hairline(c, p.borderSubtle)
	sig := fmt.Sprint(st.spec.X, st.spec.Ys)
	if st.sig != sig {
		st.sig, st.data = sig, buildChart(r.res.Rows, st.spec)
	}
	if len(st.spec.Ys) == 0 || len(st.data.Labels) == 0 {
		ui.Text(c, locales.ChartNoData()).TextColor(p.fgMuted).Padding(24).TextAlign(ui.Center).FillWidth()
		return
	}
	th := chartTheme{grid: p.borderSubtle, axis: p.border, text: p.fgSecondary, bg: p.surface}
	kind, data, other := st.spec.Kind, st.data, locales.ChartOther()
	ui.Box(c.Key("chart")).Grow(1).FillWidth().MinHeight(120).Draw(func(pt *ui.Painter, rect ui.Rect) {
		paintChart(pt, rect, kind, data, th, other)
	})
	if st.data.Thin {
		ui.Text(c, locales.ChartThinned(maxChartPoints)).FontSize(font2XS).TextColor(p.fgMuted).Padding(2, 12)
	}
}
