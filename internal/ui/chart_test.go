package ui

import (
	"math"
	"reflect"
	"testing"

	"minidb/internal/database"
)

func cols(names ...string) []database.ColumnMeta {
	out := make([]database.ColumnMeta, len(names))
	for i, n := range names {
		out[i] = database.ColumnMeta{Name: n}
	}
	return out
}

func TestNumeric(t *testing.T) {
	for _, v := range []any{int64(3), 3, float64(3.5), "7", []byte("8.5"), uint8(2)} {
		if _, ok := numeric(v); !ok {
			t.Errorf("%T %v 应是数字", v, v)
		}
	}
	for _, v := range []any{nil, "abc", true, math.NaN(), ""} {
		if _, ok := numeric(v); ok {
			t.Errorf("%T %v 不应是数字", v, v)
		}
	}
}

func TestDefaultChartSpecPicksTextAcrossNumbersUp(t *testing.T) {
	rows := []map[string]any{{"city": "a", "n": int64(3), "m": 1.5}, {"city": "b", "n": int64(4), "m": 2.5}}
	spec := defaultChartSpec("", cols("city", "n", "m"), rows)
	if spec.Kind != chartBar || spec.X != "city" || !reflect.DeepEqual(spec.Ys, []string{"n"}) {
		t.Fatalf("%+v", spec)
	}
	// 全是数字的结果：横轴取第一列，数值取第二列
	spec = defaultChartSpec(chartLine, cols("a", "b"), []map[string]any{{"a": int64(1), "b": int64(2)}})
	if spec.X != "a" || !reflect.DeepEqual(spec.Ys, []string{"b"}) || spec.Kind != chartLine {
		t.Fatalf("%+v", spec)
	}
}

func TestChartSpecFromComment(t *testing.T) {
	rows := []map[string]any{{"month": "jan", "total": int64(10), "cost": int64(4)}}
	c := cols("month", "total", "cost")
	spec, ok := chartSpecFor("-- minidb linechart x:month, y:total, y:cost\nSELECT 1", c, rows)
	if !ok || spec.Kind != chartLine || spec.X != "month" || !reflect.DeepEqual(spec.Ys, []string{"total", "cost"}) {
		t.Fatalf("%+v %v", spec, ok)
	}
	// 指令里写错的列名退回自动挑选，不会崩
	spec, ok = chartSpecFor("-- minidb barchart x:nope, y:ghost\nSELECT 1", c, rows)
	if !ok || spec.X != "month" || len(spec.Ys) != 1 || spec.Ys[0] != "total" {
		t.Fatalf("%+v", spec)
	}
	if _, ok = chartSpecFor("SELECT 1", c, rows); ok {
		t.Fatal("没有注释不算指令")
	}
}

func TestBuildChartAndThinning(t *testing.T) {
	rows := []map[string]any{{"k": "a", "v": int64(1)}, {"k": nil, "v": nil}, {"k": "c", "v": "x"}}
	d := buildChart(rows, chartSpec{X: "k", Ys: []string{"v"}})
	if !reflect.DeepEqual(d.Labels, []string{"a", "", "c"}) || !reflect.DeepEqual(d.Series[0].Values, []float64{1, 0, 0}) {
		t.Fatalf("%+v", d)
	}
	var many []map[string]any
	for i := 0; i < 2000; i++ {
		many = append(many, map[string]any{"k": i, "v": i})
	}
	d = buildChart(many, chartSpec{X: "k", Ys: []string{"v"}})
	if !d.Thin || len(d.Labels) > maxChartPoints {
		t.Fatalf("应抽样到 %d 点以内: %d", maxChartPoints, len(d.Labels))
	}
}

func TestNiceScale(t *testing.T) {
	cases := []struct{ lo, hi, wantLo, wantHi, wantStep float64 }{
		{3, 97, 0, 100, 20},
		{0, 1, 0, 1, 0.2},
		{-5, 12, -10, 15, 5},
		{50, 50, 0, 60, 10},
	}
	for _, c := range cases {
		lo, hi, step := niceScale(c.lo, c.hi, 5)
		if lo > c.lo && c.lo < 0 || hi < c.hi || step <= 0 {
			t.Errorf("niceScale(%v,%v) = %v,%v,%v 没有盖住数据", c.lo, c.hi, lo, hi, step)
		}
	}
	if lo, hi, step := niceScale(3, 97, 5); lo != 0 || hi != 100 || step != 20 {
		t.Fatalf("%v %v %v", lo, hi, step)
	}
}

func TestPieSlices(t *testing.T) {
	d := chartData{Labels: []string{"a", "b", "c", "d"}, Series: []chartSeries{{Values: []float64{2, 6, -1, 2}}}}
	s := pieSlices(d, "其他")
	if len(s) != 3 || math.Abs(s[1].Share-0.6) > 1e-9 {
		t.Fatalf("负值跳过、份额按正数合计: %+v", s)
	}
	var labels []string
	var vals []float64
	for i := 0; i < 15; i++ {
		labels = append(labels, string(rune('a'+i)))
		vals = append(vals, float64(i+1))
	}
	s = pieSlices(chartData{Labels: labels, Series: []chartSeries{{Values: vals}}}, "其他")
	if len(s) != maxPieSlices || s[len(s)-1].Label != "其他" {
		t.Fatalf("多出的并入“其他”: %d %+v", len(s), s[len(s)-1])
	}
	sum := 0.0
	for _, x := range s {
		sum += x.Share
	}
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("份额之和应为 1: %v", sum)
	}
	if pieSlices(chartData{Labels: []string{"a"}, Series: []chartSeries{{Values: []float64{0}}}}, "x") != nil {
		t.Fatal("全是 0 画不出饼")
	}
}

func TestFormatNumber(t *testing.T) {
	for in, want := range map[float64]string{0: "0", 5: "5", 0.25: "0.25", 12500: "12.5k", 3000000: "3M", 1.5e9: "1.5B", 250.5: "250.5"} {
		if got := formatNumber(in); got != want {
			t.Errorf("formatNumber(%v) = %q, want %q", in, got, want)
		}
	}
}

// 带 -- minidb 注释的查询，运行后直接显示图表页；也能手动切换图表种类。
func TestChartPageFromComment(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("-- minidb barchart x:age, y:n\nSELECT age, COUNT(*) AS n FROM users GROUP BY age ORDER BY age")
	e.tt.Frame()
	q.run(true)
	if q.page != pageChart {
		t.Fatalf("注释应直接打开图表页: page=%d failed=%q", q.page, q.failed)
	}
	e.has("横轴")
	e.shot("27-chart-bar")
	r := q.results[0]
	if r.chart.spec.Kind != chartBar || r.chart.spec.X != "age" || len(r.chart.data.Labels) != 40 {
		t.Fatalf("%+v labels=%d", r.chart.spec, len(r.chart.data.Labels))
	}
	r.chart.spec.Kind = chartLine
	e.shot("28-chart-line")
	r.chart.spec.Kind = chartPie
	e.shot("29-chart-pie")

	// 没有注释的查询停在数据页
	q.ed.Buffer.Set("SELECT id, age FROM users")
	q.page = pageData
	q.run(true)
	if q.page != pageData {
		t.Fatal("没有注释不该跳到图表页")
	}
	q.page = pageChart
	e.has("横轴")
	if q.results[0].chart.spec.X == "" || len(q.results[0].chart.spec.Ys) == 0 {
		t.Fatalf("应自动挑列: %+v", q.results[0].chart.spec)
	}
}

// 没有数值列时给出提示，不崩。
func TestChartWithoutNumbers(t *testing.T) {
	e := newEnv(t)
	w := openWorkspace(e)
	q := w.newQuery("SELECT name FROM users LIMIT 3")
	e.tt.Frame()
	q.run(true)
	q.page = pageChart
	e.has("没有可画的数据")
}
