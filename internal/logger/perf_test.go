package logger

import (
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// capturePerf 接管 [PERF] 输出并清空统计，测试结束后还原。
func capturePerf(t *testing.T) *[]string {
	t.Helper()
	var mu sync.Mutex
	var lines []string
	perfTap = func(l string) { mu.Lock(); lines = append(lines, l); mu.Unlock() }
	perfTake()
	oldMode := perfMode.Load()
	perfMode.Store(perfModeNormal)
	t.Cleanup(func() { perfTap = nil; perfMode.Store(oldMode); perfTake() })
	return &lines
}

func TestBudgetLongestPrefixWins(t *testing.T) {
	cases := map[string]time.Duration{
		"ui.build":                   33 * time.Millisecond, // 整帧
		"ui.build.sidebar":           16 * time.Millisecond, // 帧里的一段，比整帧严
		"ui.post.Workspace.load":     20 * time.Millisecond,
		"bg.Workspace.loadKeywords":  time.Second,
		"db.connect":                 5 * time.Second, // 比 db. 更长的前缀
		"db.query":                   500 * time.Millisecond,
		"app.Queries.ExecuteSQL":     2 * time.Second, // 用户自己的查询放宽
		"app.Queries.QueryTableData": time.Second,
		"app.Databases.GetTables":    300 * time.Millisecond,
		"job.export":                 0, // 长作业不判超时
		"something.unknown":          200 * time.Millisecond,
	}
	for name, want := range cases {
		if got := budgetFor(name); got != want {
			t.Errorf("budgetFor(%q) = %v，期望 %v", name, got, want)
		}
	}
}

func TestSetBudgetOverridesAndClearsCache(t *testing.T) {
	t.Cleanup(func() { SetBudget("db.", 500*time.Millisecond) })
	_ = budgetFor("db.query") // 先进缓存
	SetBudget("db.", 10*time.Millisecond)
	if got := budgetFor("db.query"); got != 10*time.Millisecond {
		t.Fatalf("改完预算应立即生效，实际 %v", got)
	}
}

func TestPerfLogsOnlyWhenOverBudget(t *testing.T) {
	lines := capturePerf(t)
	PerfDone("app.Test.fast", 5*time.Millisecond)
	if len(*lines) != 0 {
		t.Fatalf("没超预算不该有日志: %v", *lines)
	}
	PerfDone("app.Test.slow", 450*time.Millisecond, "conn", "c1", "rows", 42)
	if len(*lines) != 1 {
		t.Fatalf("超预算应有一行日志: %v", *lines)
	}
	l := (*lines)[0]
	for _, want := range []string{"慢 app.Test.slow", "耗时=450.0ms", "预算=300.0ms", "conn=c1", "rows=42"} {
		if !strings.Contains(l, want) {
			t.Errorf("日志缺少 %q: %s", want, l)
		}
	}
}

func TestPerfZeroBudgetNeverSlow(t *testing.T) {
	lines := capturePerf(t)
	PerfDone("job.export", time.Hour)
	if len(*lines) != 0 {
		t.Fatalf("预算为 0 的名字不判超时: %v", *lines)
	}
	if s := PerfSnapshot(); len(s) != 1 || s[0].Count != 1 || s[0].Slow != 0 {
		t.Fatalf("但仍要累计统计: %+v", s)
	}
}

func TestPerfVerboseLogsEverythingOverOneMs(t *testing.T) {
	lines := capturePerf(t)
	perfMode.Store(perfModeVerbose)
	PerfDone("app.Test.a", 500*time.Microsecond)
	PerfDone("app.Test.b", 3*time.Millisecond)
	if len(*lines) != 1 || !strings.Contains((*lines)[0], "· app.Test.b") {
		t.Fatalf("verbose 只记 ≥1ms 的: %v", *lines)
	}
}

func TestPerfOffDoesNothing(t *testing.T) {
	lines := capturePerf(t)
	perfMode.Store(perfModeOff)
	Perf("app.Test.x")()
	PerfDone("app.Test.y", time.Hour)
	PerfUI("ui.build")()
	if len(*lines) != 0 || len(PerfSnapshot()) != 0 {
		t.Fatalf("off 时不记日志也不累计: %v %v", *lines, PerfSnapshot())
	}
}

func TestPerfEndAcceptsExtraAttrs(t *testing.T) {
	lines := capturePerf(t)
	SetBudget("app.Test.", time.Nanosecond)
	t.Cleanup(func() { SetBudget("app.Test.", 300*time.Millisecond) })
	end := Perf("app.Test.attrs", "table", "users")
	time.Sleep(time.Millisecond)
	end("rows", 7)
	if len(*lines) != 1 || !strings.Contains((*lines)[0], "table=users rows=7") {
		t.Fatalf("结束时补充的属性应跟在后面: %v", *lines)
	}
}

func TestPerfStatsAccumulateAndSummaryResets(t *testing.T) {
	lines := capturePerf(t)
	PerfDone("app.Test.q", 100*time.Millisecond)
	PerfDone("app.Test.q", 400*time.Millisecond) // 超预算
	PerfDone("app.Test.r", 10*time.Millisecond)
	snap := PerfSnapshot()
	if len(snap) != 2 || snap[0].Name != "app.Test.q" || snap[0].Count != 2 || snap[0].Slow != 1 || snap[0].Max != 400*time.Millisecond || snap[0].Total != 500*time.Millisecond {
		t.Fatalf("累计不对（应按总耗时降序）: %+v", snap)
	}
	*lines = nil
	PerfSummary()
	if len(*lines) != 1 || !strings.Contains((*lines)[0], "汇总") || !strings.Contains((*lines)[0], "app.Test.q") || !strings.Contains((*lines)[0], "运行时 协程=") {
		t.Fatalf("汇总内容不对: %v", *lines)
	}
	if len(PerfSnapshot()) != 0 {
		t.Fatal("汇总之后应清零窗口")
	}
	*lines = nil
	PerfSummary()
	if len(*lines) != 0 {
		t.Fatalf("没有新数据时不输出空汇总: %v", *lines)
	}
}

func TestPerfSQLRedactsLiteralsAndTruncates(t *testing.T) {
	got := PerfSQL("INSERT INTO users(name, pwd)\n  VALUES ('alice', 'p@ss''word\\'s'),\t('bob', 'x')")
	if want := "INSERT INTO users(name, pwd) VALUES ('?', '?'), ('?', '?')"; got != want {
		t.Fatalf("PerfSQL = %q，期望 %q", got, want)
	}
	if strings.Contains(got, "alice") || strings.Contains(got, "p@ss") {
		t.Fatalf("字面量不许进日志: %q", got)
	}
	long := PerfSQL("SELECT " + strings.Repeat("a, ", 200) + "b FROM t")
	if r := []rune(long); len(r) != perfSQLMax+1 || !strings.HasSuffix(long, "…") {
		t.Fatalf("应截断到 %d 个字符加省略号，实际 %d: %q", perfSQLMax, len(r), long)
	}
}

func TestStallDetectedOncePerEpisodeWithStacks(t *testing.T) {
	lines := capturePerf(t)
	stallAfter.Store(int64(50 * time.Millisecond))
	t.Cleanup(func() { stallAfter.Store(int64(300 * time.Millisecond)) })

	end := PerfUI("ui.post.Test.block")
	checkStall(time.Now()) // 刚进入，不算
	if len(*lines) != 0 {
		t.Fatalf("还没到阈值: %v", *lines)
	}
	later := time.Now().Add(80 * time.Millisecond)
	checkStall(later)
	checkStall(later.Add(time.Second)) // 同一次卡死不重复报
	if len(*lines) != 1 {
		t.Fatalf("同一次卡死只报一回，实际 %d 条", len(*lines))
	}
	for _, want := range []string{"卡死", "ui.post.Test.block", "goroutine ", "TestStallDetectedOncePerEpisodeWithStacks"} {
		if !strings.Contains((*lines)[0], want) {
			t.Errorf("卡死日志缺少 %q", want)
		}
	}
	end()
	checkStall(later.Add(time.Hour)) // 已离开，不再报
	if len(*lines) != 1 {
		t.Fatalf("离开之后不该再报: %d", len(*lines))
	}

	// 下一次卡死要能再报
	end = PerfUI("ui.post.Test.block2")
	checkStall(time.Now().Add(80 * time.Millisecond))
	end()
	if len(*lines) != 2 {
		t.Fatalf("新的一次卡死应再报: %d", len(*lines))
	}
}

func TestPerfUINestedCountsOutermostOnly(t *testing.T) {
	capturePerf(t)
	outer := PerfUI("ui.build")
	inner := PerfUI("ui.build.sidebar")
	busyMu.Lock()
	name, depth := busyName, busyDepth
	busyMu.Unlock()
	if name != "ui.build" || depth != 2 {
		t.Fatalf("嵌套时以最外层为准: name=%q depth=%d", name, depth)
	}
	inner()
	outer()
	busyMu.Lock()
	depth = busyDepth
	busyMu.Unlock()
	if depth != 0 {
		t.Fatalf("全部结束后深度应为 0: %d", depth)
	}
}

// 端到端：Init 之后，慢打点写进日志文件并带 [PERF]；标准库 log（MyGo 的帧统计走它）带 [MYGO]；
// 退出时输出一次汇总。
func TestPerfEndToEndInLogFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	capturePerf(t)
	perfTap = nil // 这里要看真正写进文件的内容
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.SetOutput(os.Stderr); log.SetPrefix("") })

	PerfDone("db.statement", 3*time.Second, "sql", SQLText("SELECT 'secret'"))
	log.Print("frame 12.3ms build=10ms")
	Close()

	b, err := os.ReadFile(CurrentPath())
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{"[PERF]", "慢 db.statement", "sql=SELECT '?'", "[MYGO]", "frame 12.3ms", "汇总 近"} {
		if !strings.Contains(text, want) {
			t.Errorf("日志文件里应有 %q", want)
		}
	}
	if strings.Contains(text, "secret") {
		t.Error("SQL 字面量不许写进日志文件")
	}
}
