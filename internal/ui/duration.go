package ui

// @author ixxxxoooo

import (
	"fmt"
	"time"

	"minidb/internal/database"
)

// fmtDur 把耗时写成短文字，不足 1 毫秒的也显示：
// 小于 1 毫秒写成 0.32 ms，小于 10 毫秒保留一位小数（4.2 ms），到 1 秒之前写整毫秒，1 秒及以上写成 1.2s。
// 小于 0.01 毫秒（10 微秒）写成 <0.01 ms，免得出现「0 ms」让人以为没统计。
//
// 参数：d 耗时。返回值：显示用的文字。
func fmtDur(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d >= 10*time.Millisecond:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond))
	case d >= 10*time.Microsecond:
		return fmt.Sprintf("%.2f ms", float64(d)/float64(time.Millisecond))
	}
	return "<0.01 ms"
}

// resultDur 是一次查询的耗时：优先用微秒，没有（旧数据）就用毫秒。
func resultDur(r *database.QueryResult) time.Duration {
	if r.DurationUs > 0 {
		return time.Duration(r.DurationUs) * time.Microsecond
	}
	return time.Duration(r.Duration) * time.Millisecond
}
