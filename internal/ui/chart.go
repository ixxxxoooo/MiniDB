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

// ---- 绘制 ----

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
