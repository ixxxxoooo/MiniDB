package logger

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// UI 线程的卡死检测与定时汇总。
//
// 界面是立即模式，空闲时不出帧，所以不能靠「多久没出帧」判断卡死。
// 做法是：UI 线程进入一段逻辑（构建一帧、执行回调）时用 PerfUI 登记，离开时注销；
// 看门狗每 200ms 看一眼，发现同一段逻辑停留超过阈值（默认 300ms），
// 就把所有协程的堆栈写进日志，一眼就能看到 UI 线程卡在哪一行、在等什么。

var (
	busyMu    sync.Mutex
	busyDepth int       // 嵌套深度；只有最外层算数
	busyName  string    // 最外层那段逻辑的名字
	busySince time.Time // 最外层进入的时间
	busyID    uint64    // 每次进入加一，用来保证同一次卡死只报一回
	busySeen  uint64    // 已经报过的 busyID

	watchOnce sync.Once
	watchStop = make(chan struct{})
)

// PerfUI 是给 UI 线程用的 Perf：除了计时，还登记「UI 线程正忙于 name」，供看门狗检测卡死。
// 只能在 UI 线程上调用；后台协程请用 Perf。
func PerfUI(name string, kv ...any) func(...any) {
	if perfMode.Load() == perfModeOff {
		return noopEnd
	}
	busyMu.Lock()
	if busyDepth == 0 {
		busyName, busySince = name, time.Now()
		busyID++
	}
	busyDepth++
	busyMu.Unlock()
	end := Perf(name, kv...)
	return func(more ...any) {
		busyMu.Lock()
		busyDepth--
		busyMu.Unlock()
		end(more...)
	}
}

// startPerf 启动看门狗和定时汇总；Init 里调用，重复调用无副作用。
func startPerf() {
	watchOnce.Do(func() {
		go watchLoop()
		go reportLoop()
	})
}

// stopPerf 停止后台协程并输出最后一次汇总。
func stopPerf() {
	select {
	case <-watchStop:
	default:
		close(watchStop)
	}
	PerfSummary()
}

func watchLoop() {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-watchStop:
			return
		case <-t.C:
			checkStall(time.Now())
		}
	}
}

// checkStall 检查 UI 线程是否在同一段逻辑里停得太久；是就报一次（同一次卡死不重复报）。
func checkStall(now time.Time) {
	limit := time.Duration(stallAfter.Load())
	busyMu.Lock()
	stalled := busyDepth > 0 && now.Sub(busySince) >= limit && busyID != busySeen
	name, since := busyName, busySince
	if stalled {
		busySeen = busyID
	}
	busyMu.Unlock()
	if !stalled {
		return
	}
	stats := PerfSnapshot()
	perfOut(fmt.Sprintf("卡死 UI 线程停在 %s 已 %s（阈值 %s），所有协程堆栈如下；最上面通常是 UI 线程\n%s%s",
		name, fmtDur(now.Sub(since)), fmtDur(limit), goroutineStacks(), recentHint(stats)))
}

// goroutineStacks 返回所有协程的堆栈，最多 48KB，避免一次卡死把日志撑爆。
func goroutineStacks() string {
	buf := make([]byte, 48<<10)
	n := runtime.Stack(buf, true)
	s := string(buf[:n])
	if n == len(buf) {
		s += "\n…（堆栈过长，已截断）"
	}
	return s
}

// recentHint 列出累计耗时最大的几个名字，帮助判断卡死前在忙什么。
func recentHint(stats []PerfStat) string {
	if len(stats) == 0 {
		return ""
	}
	s := "\n    本窗口累计耗时最大的："
	for i, st := range stats {
		if i >= 3 {
			break
		}
		s += fmt.Sprintf(" %s(%s/%d次)", st.Name, fmtDur(st.Total), st.Count)
	}
	return s
}

// reportInterval 是定时汇总的间隔。
const reportInterval = 5 * time.Minute

func reportLoop() {
	t := time.NewTicker(reportInterval)
	defer t.Stop()
	for {
		select {
		case <-watchStop:
			return
		case <-t.C:
			PerfSummary()
		}
	}
}

var lastGC struct {
	mu    sync.Mutex
	num   uint32
	pause uint64
}

// runtimeLine 返回运行时概况：协程数、堆大小，以及自上次汇总以来的 GC 次数和总暂停。
// 频繁 GC 或很长的暂停本身就会让界面卡顿，所以放进汇总里一起看。
func runtimeLine() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	lastGC.mu.Lock()
	dn, dp := m.NumGC-lastGC.num, time.Duration(m.PauseTotalNs-lastGC.pause)
	lastGC.num, lastGC.pause = m.NumGC, m.PauseTotalNs
	lastGC.mu.Unlock()
	return fmt.Sprintf("运行时 协程=%d 堆=%.0fMB 系统=%.0fMB GC=%d次 暂停合计=%s",
		runtime.NumGoroutine(), float64(m.HeapAlloc)/(1<<20), float64(m.Sys)/(1<<20), dn, fmtDur(dp))
}
