package sqltext

import "strings"

// ChartDirective 是语句里 “-- minidb barchart x:列, y:列, y:列” 这样的注释：
// 不用在界面里点选，运行后直接按它画图。
type ChartDirective struct {
	Kind string   // line、bar、pie
	X    string   // 横轴（饼图为扇区名）的列
	Ys   []string // 数值列
}

// ParseChartDirective 从语句的注释里读出图表指令。没有指令时 ok 为假。
// 写法不分大小写：kind 可写成 linechart、barchart、piechart；x: 与 y: 都可省略，
// 省略的列由界面按结果的列自动挑选。
func ParseChartDirective(stmt string) (d ChartDirective, ok bool) {
	for _, line := range strings.Split(stmt, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "--") {
			continue
		}
		t = strings.TrimSpace(strings.TrimPrefix(t, "--"))
		lower := strings.ToLower(t)
		if !strings.HasPrefix(lower, "minidb ") {
			continue
		}
		rest := strings.TrimSpace(t[len("minidb "):])
		head, args, _ := strings.Cut(rest, " ")
		switch strings.ToLower(head) {
		case "linechart", "line":
			d.Kind = "line"
		case "barchart", "bar":
			d.Kind = "bar"
		case "piechart", "pie":
			d.Kind = "pie"
		default:
			continue
		}
		for _, part := range strings.Split(args, ",") {
			part = strings.TrimSpace(part)
			key, val, found := strings.Cut(part, ":")
			if !found {
				continue
			}
			val = strings.Trim(strings.TrimSpace(val), "`\"'")
			if val == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "x":
				d.X = val
			case "y":
				d.Ys = append(d.Ys, val)
			}
		}
		return d, true
	}
	return d, false
}
