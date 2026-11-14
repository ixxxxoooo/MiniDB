package logger

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 性能打点。目的是卡顿时能从日志里看出「哪一层、哪一处」慢：
//
//	defer logger.Perf("app.Queries.ExecuteSQL", "conn", connID)()
//
// 默认只在超过预算（perf_budget.go）时写一行 [PERF] 日志，平时不刷屏；
// 同时按名字累计次数、总耗时和最大耗时，每 5 分钟和退出时输出一次汇总。
// 环境变量：MINIDB_PERF=verbose 记录所有 ≥1ms 的；MINIDB_PERF=off 关闭；
// MINIDB_PERF_BUDGET、MINIDB_PERF_STALL 临时调整预算和卡死阈值。
//
// 打点本身很便宜（两次取时间、一次加锁累计），但不要放进循环、逐行、逐单元格的路径里。
// 日志里不许出现密码、密钥和 SQL 里的字面量（SQL 用 PerfSQL 脱敏）。
//
// @author ixxxxoooo

var (
	perfMode   atomic.Int32
	perfLogger *log.Logger // 由 openFileLocked 建立，受 mu 保护
	perfTap    func(line string)

	perfMu    sync.Mutex
	perfStats = map[string]*perfStat{}
	perfSince = time.Now()
)

type perfStat struct {
	count, slow int64
	total, max  time.Duration
}

// PerfStat 是某个名字在统计窗口里的累计。
type PerfStat struct {
	Name        string
	Count, Slow int64
	Total, Max  time.Duration
}

func noopEnd(...any) {}

// Perf 开始计时，返回的函数结束计时；结束时还能再补充属性（如行数）：
//
//	end := logger.Perf("db.query", "table", t)
//	rows := run()
//	end("rows", len(rows))
//
// 参数 name 是 层.模块.动作；kv 是成对的属性（键、值交替）。
func Perf(name string, kv ...any) func(...any) {
	if perfMode.Load() == perfModeOff {
		return noopEnd
	}
	start := time.Now()
	return func(more ...any) { PerfDone(name, time.Since(start), append(kv, more...)...) }
}

// PerfDone 记录一次已经量好的耗时（比如结果里本来就带着 Duration）。
func PerfDone(name string, d time.Duration, kv ...any) {
	mode := perfMode.Load()
	if mode == perfModeOff {
		return
	}
	limit := budgetFor(name)
	slow := limit > 0 && d >= limit
	perfMu.Lock()
	s := perfStats[name]
	if s == nil {
		s = &perfStat{}
		perfStats[name] = s
	}
	s.count++
	s.total += d
	s.max = max(s.max, d)
	if slow {
		s.slow++
	}
	perfMu.Unlock()
	switch {
	case slow:
		perfOut(fmt.Sprintf("慢 %s 耗时=%s 预算=%s%s", name, fmtDur(d), fmtDur(limit), fmtKV(kv)))
	case mode == perfModeVerbose && d >= time.Millisecond:
		perfOut(fmt.Sprintf("· %s 耗时=%s%s", name, fmtDur(d), fmtKV(kv)))
	}
}

// perfOut 写一行 [PERF] 日志。
func perfOut(line string) {
	if perfTap != nil {
		perfTap(line)
	}
	output(func() *log.Logger { return perfLogger }, "%s", line)
}

// PerfSnapshot 返回当前统计窗口的累计，按总耗时从大到小排；不清零。
func PerfSnapshot() []PerfStat {
	perfMu.Lock()
	out := make([]PerfStat, 0, len(perfStats))
	for n, s := range perfStats {
		out = append(out, PerfStat{n, s.count, s.slow, s.total, s.max})
	}
	perfMu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Total > out[j].Total })
	return out
}

// perfTake 取走并清零当前统计窗口，返回窗口长度。
func perfTake() (stats []PerfStat, window time.Duration) {
	stats = PerfSnapshot()
	perfMu.Lock()
	perfStats = map[string]*perfStat{}
	window, perfSince = time.Since(perfSince), time.Now()
	perfMu.Unlock()
	return stats, window
}

// perfSummaryTop 是汇总里列出多少个名字。
const perfSummaryTop = 12

// PerfSummary 输出一次汇总并清零窗口：总耗时最大的几个名字，以及运行时的内存、协程和 GC 情况。
func PerfSummary() {
	stats, window := perfTake()
	if len(stats) == 0 {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "汇总 近 %s，按总耗时排序（次数 / 总计 / 平均 / 最大 / 超预算次数）", window.Round(time.Second))
	for i, s := range stats {
		if i >= perfSummaryTop {
			break
		}
		fmt.Fprintf(&b, "\n    %-44s %6d  %9s  %8s  %8s  %d", s.Name, s.Count, fmtDur(s.Total), fmtDur(s.Total/time.Duration(max(s.Count, 1))), fmtDur(s.Max), s.Slow)
	}
	b.WriteString("\n    " + runtimeLine())
	perfOut(b.String())
}

// fmtDur 把耗时写成短而稳定的形式：<1s 用毫秒（带一位小数），否则用秒。
func fmtDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// fmtKV 把成对的属性写成 " k=v"；落单的键或值也照写，不丢信息。
func fmtKV(kv []any) string {
	var b strings.Builder
	for i := 0; i < len(kv); i += 2 {
		if i+1 < len(kv) {
			fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
		} else {
			fmt.Fprintf(&b, " %v", kv[i])
		}
	}
	return b.String()
}
