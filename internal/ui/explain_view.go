package ui

import "strconv"

// planNum 把行数、代价格式成简短的数字；0 表示数据库没给，留空。
func planNum(f float64) string {
	switch {
	case f <= 0:
		return ""
	case f >= 1e9:
		return strconv.FormatFloat(f/1e9, 'f', 1, 64) + "G"
	case f >= 1e6:
		return strconv.FormatFloat(f/1e6, 'f', 1, 64) + "M"
	case f >= 1e4:
		return strconv.FormatFloat(f/1e3, 'f', 1, 64) + "K"
	case f == float64(int64(f)):
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// planMs 把实际耗时（毫秒）格式成 "1.23 ms"；0 留空。
func planMs(ms float64) string {
	if ms <= 0 {
		return ""
	}
	if ms >= 1000 {
		return strconv.FormatFloat(ms/1000, 'f', 2, 64) + " s"
	}
	return strconv.FormatFloat(ms, 'f', 2, 64) + " ms"
}
