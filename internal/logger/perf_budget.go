package logger

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 性能打点的耗时预算：按名字前缀匹配，最长前缀优先，0 表示不判超时（只累计统计）。
// 名字的约定是 层.模块.动作，如 ui.build、db.query、app.Queries.ExecuteSQL、bg.Workspace.loadKeywords。
// 新增一类打点时，如果默认的 200ms 不合适，就在这里加一条，并写明为什么是这个数。

type budgetRule struct {
	prefix string
	limit  time.Duration
}

var defaultBudgets = []budgetRule{
	{"", 200 * time.Millisecond},             // 没有专门规则的，200ms 以上算慢
	{"ui.build", 33 * time.Millisecond},      // 整帧的构建：超过两帧（60Hz 下 33ms）就能看出掉帧
	{"ui.build.", 16 * time.Millisecond},     // 帧里的某一段（侧栏、标签内容、网格…）：一段就吃掉一帧说明有问题
	{"ui.post.", 20 * time.Millisecond},      // 在 UI 线程上执行的回调
	{"ui.done.", 20 * time.Millisecond},      // 后台任务完成后，在 UI 线程上执行的收尾
	{"ui.queue", 100 * time.Millisecond},     // 回调排队等 UI 线程的时间：长说明 UI 线程一直忙
	{"bg.", time.Second},                     // 后台任务本身
	{"db.connect", 5 * time.Second},          // 建连：含 SSH 隧道、TLS、网络
	{"db.ping", 3 * time.Second},             // 保活探测
	{"db.statement", 2 * time.Second},        // 一条语句从发出到读完结果；用户自己的查询，2 秒以上才值得注意
	{"db.", 500 * time.Millisecond},          // 会话里的一次数据库往返
	{"schema.", 3 * time.Second},             // 表结构索引刷新：要读整库元数据
	{"ai.", 30 * time.Second},                // AI 请求的一轮（流式，本来就慢）
	{"app.AI.", time.Minute},                 // AI 对话整体
	{"app.Queries.Execute", 2 * time.Second}, // 用户自己写的查询，几百毫秒很常见，2 秒以上才值得注意
	{"app.Queries.QueryTable", time.Second},  // 浏览表的一页
	{"app.bus.", 50 * time.Millisecond},      // 总线订阅回调要「立即返回」，一个慢回调会拖住发布者
	{"app.", 300 * time.Millisecond},         // 其他服务方法
	{"job.", 0},                              // 导出、导入、备份这类长作业：本来就长，由作业自己记完成日志
}

var (
	budgetMu    sync.RWMutex
	budgets     = append([]budgetRule(nil), defaultBudgets...)
	budgetCache sync.Map // 名字 → 预算，规则变了就清空

	stallAfter atomic.Int64 // UI 线程在同一段逻辑里停多久算卡死（纳秒）
)

const (
	perfModeNormal  int32 = iota // 默认：只记超预算的
	perfModeVerbose              // MINIDB_PERF=verbose：≥1ms 的都记
	perfModeOff                  // MINIDB_PERF=off：什么都不做
)

func init() {
	stallAfter.Store(int64(300 * time.Millisecond))
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MINIDB_PERF"))) {
	case "1", "on", "true", "verbose", "all":
		perfMode.Store(perfModeVerbose)
	case "0", "off", "false", "no":
		perfMode.Store(perfModeOff)
	}
	// MINIDB_PERF_BUDGET="db.=300ms,ui.build=20ms"：临时调整预算，排查时用
	for _, kv := range strings.Split(os.Getenv("MINIDB_PERF_BUDGET"), ",") {
		if p, v, ok := strings.Cut(strings.TrimSpace(kv), "="); ok {
			if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil && d >= 0 {
				SetBudget(strings.TrimSpace(p), d)
			}
		}
	}
	if d, err := time.ParseDuration(os.Getenv("MINIDB_PERF_STALL")); err == nil && d > 0 {
		stallAfter.Store(int64(d))
	}
}

// SetBudget 设置或替换一个前缀的预算；d 为 0 表示该前缀不判超时。
func SetBudget(prefix string, d time.Duration) {
	budgetMu.Lock()
	defer budgetMu.Unlock()
	for i := range budgets {
		if budgets[i].prefix == prefix {
			budgets[i].limit = d
			budgetCache.Clear()
			return
		}
	}
	budgets = append(budgets, budgetRule{prefix, d})
	budgetCache.Clear()
}

// budgetFor 返回名字对应的预算（最长前缀优先）。
func budgetFor(name string) time.Duration {
	if v, ok := budgetCache.Load(name); ok {
		return v.(time.Duration)
	}
	budgetMu.RLock()
	best, limit := -1, time.Duration(0)
	for _, r := range budgets {
		if len(r.prefix) > best && strings.HasPrefix(name, r.prefix) {
			best, limit = len(r.prefix), r.limit
		}
	}
	budgetMu.RUnlock()
	budgetCache.Store(name, limit)
	return limit
}
