package ui

import (
	"cmp"

	"slices"
	"strings"
)

// arranged 是按当前排序摆好的行（local 网格）；没有排序或不是 local 时原样返回。
func (g *resultGrid) arranged(rows []map[string]any) []map[string]any {
	if !g.local || g.sort.Column == "" || g.colIndex(g.sort.Column) < 0 {
		return rows
	}
	out := slices.Clone(rows)
	col, desc := g.sort.Column, g.sort.Descending
	slices.SortStableFunc(out, func(a, b map[string]any) int {
		x, y := a[col], b[col]
		switch { // NULL 总是排在最后，不随方向翻转
		case x == nil && y == nil:
			return 0
		case x == nil:
			return 1
		case y == nil:
			return -1
		}
		r := compareCells(x, y)
		if desc {
			r = -r
		}
		return r
	})
	return out
}

// compareCells 比较两个单元格：数字按数值，其余按文本（不分大小写）。
func compareCells(x, y any) int {
	fx, okx := cellNumber(x)
	fy, oky := cellNumber(y)
	if okx && oky {
		return cmp.Compare(fx, fy)
	}
	sx, _ := cellText(x)
	sy, _ := cellText(y)
	if c := strings.Compare(strings.ToLower(sx), strings.ToLower(sy)); c != 0 {
		return c
	}
	return strings.Compare(sx, sy)
}

func cellNumber(v any) (float64, bool) {
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
		return n, true
	}
	return 0, false
}
