package ui

// @author ixxxxoooo

import (
	"fmt"
	"minidb/internal/app"
	"minidb/locales"
	"time"
)

// fmtDur 把耗时写成短文字，不足 1 毫秒的用微秒（µs）：
// 小于 1 毫秒写成 320 µs，小于 10 毫秒保留一位小数（4.2 ms），到 1 秒之前写整毫秒，
// 1 秒到 1 分钟写成 1.2s，1 分钟及以上写成 1m 23s（四舍五入到秒）。
// 不足 1 微秒写成 <1 µs，免得出现「0」让人以为没统计。
//
// 参数：d 耗时。返回值：显示用的文字。
func fmtDur(d time.Duration) string {
	switch {
	case d.Round(100*time.Millisecond) >= time.Minute:
		sec := int(d.Round(time.Second) / time.Second)
		return fmt.Sprintf("%dm %02ds", sec/60, sec%60)
	case d >= time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d >= 10*time.Millisecond:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%.1f ms", float64(d)/float64(time.Millisecond))
	case d >= time.Microsecond:
		return fmt.Sprintf("%d µs", d.Microseconds())
	}
	return "<1 µs"
}

// resultDur 是一次查询的耗时：优先用微秒，没有（旧数据）就用毫秒。
func resultDur(r *app.QueryResult) time.Duration {
	if r.DurationUs > 0 {
		return time.Duration(r.DurationUs) * time.Microsecond
	}
	return time.Duration(r.Duration) * time.Millisecond
}

// aiUsageText 是 AI 回答下面的用量文字：有 token 数时写成「40,852 tokens · 20.6s」，否则只写耗时。
//
// 参数：tokens 本轮总 token 数（0 表示没有）；ms 本轮耗时毫秒。
// 返回值：显示用的文字；没有耗时返回空串，表示不显示。
func aiUsageText[T, D int | int64](tokens T, ms D) string {
	if ms <= 0 {
		return ""
	}
	dur := fmtDur(time.Duration(ms) * time.Millisecond)
	if tokens <= 0 {
		return dur
	}
	return locales.AiUsageTokens(int(tokens), dur)
}
