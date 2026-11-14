package app

import (
	"context"
	"sync"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/model"
)

// 监控面板的后台采样。面板打开时 Start 一个会话，定时取服务端指标放进环形缓冲，面板关闭就 Stop，
// 不打开面板时不对服务端产生任何额外查询。

// MonitorIntervals 是界面上可选的采样间隔。
var MonitorIntervals = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}

// monitorMaxPoints 是缓冲保留的点数：2 秒一点约 5 分钟，10 秒一点约 25 分钟。
const monitorMaxPoints = 150

// MonitorPoint 是画图用的一个点：仪表是当前值，计数器已经换算成每秒速率。
type MonitorPoint = database.MonitorPoint

// MonitorSample 是某一刻的一次采样。
type MonitorSample = database.MonitorSample

// SlowQuery 是一条慢查询。
type SlowQuery = database.SlowQuery

// 指标的键与定义，界面只依赖 app。
type MetricDef = database.MetricDef

var MetricDefs = database.MetricDefs

const (
	MetricConnections = database.MetricConnections
	MetricRunning     = database.MetricRunning
	MetricCacheHit    = database.MetricCacheHit
	MetricQueries     = database.MetricQueries
	MetricSlow        = database.MetricSlow
	MetricBytesIn     = database.MetricBytesIn
	MetricBytesOut    = database.MetricBytesOut
)

// MonitorView 是会话当前状态的一份拷贝，界面每帧读取。
type MonitorView struct {
	Points []MonitorPoint
	Err    string // 最近一次采样的错误，成功后清空
	Loaded bool   // 至少成功采样过一次
}

// MonitorSession 是一次打开面板期间的采样会话。
type MonitorSession struct {
	sample   func(ctx context.Context) (database.MonitorSample, error)
	onUpdate func()

	mu       sync.Mutex
	interval time.Duration
	points   []MonitorPoint
	prev     *database.MonitorSample
	cancel   context.CancelFunc // 正在进行的那次采样，Stop 时取消
	err      error
	loaded   bool

	wake chan struct{}
	stop chan struct{}
	done chan struct{}
}

// MonitorService 创建监控会话、读取慢查询。
type MonitorService struct{ manager *database.Manager }

func NewMonitorService(manager *database.Manager) *MonitorService {
	return &MonitorService{manager: manager}
}

// Supported 报告连接是否有监控概览与慢查询。
func (s *MonitorService) Supported(connID string) bool {
	cfg, ok := s.manager.GetConfig(connID)
	return ok && database.SupportsMonitor(cfg.Type)
}

// Start 开始对 connID 采样，每次有新点就调用 onUpdate（在后台 goroutine 里，只应做重绘通知）。
func (s *MonitorService) Start(connID string, interval time.Duration, onUpdate func()) (*MonitorSession, error) {
	defer logger.Perf("app.Monitor.Start", "conn", connID)()
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	if !database.SupportsMonitor(cfg.Type) {
		return nil, model.Errf(model.CodeMonitorUnsupported, "%s 不支持监控概览", DialectLabel(cfg.Type))
	}
	sample := func(ctx context.Context) (database.MonitorSample, error) {
		c, err := resolveConn(s.manager, connID, "")
		if err != nil {
			return database.MonitorSample{}, err
		}
		return database.GetMonitorSample(ctx, c.DB, c.Cfg.Type)
	}
	return NewMonitorSession(sample, interval, onUpdate), nil
}

// SlowQueries 返回最慢的至多 limit 条语句。
func (s *MonitorService) SlowQueries(connID string, limit int) ([]SlowQuery, error) {
	defer logger.Perf("app.Monitor.SlowQueries", "conn", connID)()
	c, err := resolveConn(s.manager, connID, "")
	if err != nil {
		return nil, err
	}
	if !c.Dialect.Caps().Monitor {
		return nil, model.Errf(model.CodeMonitorUnsupported, "%s 不支持慢查询", DialectLabel(c.Cfg.Type))
	}
	ctx, cancel := withTimeout(adminTimeout)
	defer cancel()
	return database.GetSlowQueries(ctx, c.DB, c.Cfg.Type, limit)
}

// NewMonitorSession 用任意的采样函数开始一个会话（MonitorService.Start 用连接的方言采样；测试可以传入假数据）。
func NewMonitorSession(sample func(ctx context.Context) (database.MonitorSample, error), interval time.Duration, onUpdate func()) *MonitorSession {
	if interval <= 0 {
		interval = MonitorIntervals[1]
	}
	ms := &MonitorSession{
		sample: sample, onUpdate: onUpdate, interval: interval,
		wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
	}
	go ms.loop()
	return ms
}

func (ms *MonitorSession) loop() {
	defer close(ms.done)
	for {
		ms.tick()
		ms.mu.Lock()
		wait := ms.interval
		ms.mu.Unlock()
		timer := time.NewTimer(wait)
		select {
		case <-ms.stop:
			timer.Stop()
			return
		case <-ms.wake: // 改了间隔：立刻按新间隔重新计时
			timer.Stop()
		case <-timer.C:
		}
	}
}

// tick 采一次样。采样超时取间隔与管理类超时中较短的，避免一次卡住的查询拖住后面的点。
func (ms *MonitorSession) tick() {
	ms.mu.Lock()
	timeout := min(ms.interval*2, adminTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	ms.cancel = cancel
	ms.mu.Unlock()
	defer cancel()
	endSample := logger.Perf("app.Monitor.sample") // 每 2~10 秒一次，采样慢会让整个监控页变钝
	cur, err := ms.sample(ctx)
	endSample()
	select {
	case <-ms.stop: // 采样期间面板已经关了：丢弃结果
		return
	default:
	}
	ms.mu.Lock()
	ms.err = err
	if err == nil {
		pt := database.MonitorPointOf(ms.prev, cur)
		ms.prev = &cur
		ms.loaded = true
		ms.points = append(ms.points, pt)
		if over := len(ms.points) - monitorMaxPoints; over > 0 {
			ms.points = append([]MonitorPoint(nil), ms.points[over:]...)
		}
	}
	ms.mu.Unlock()
	if ms.onUpdate != nil {
		ms.onUpdate()
	}
}

// SetInterval 改采样间隔，立即生效。
func (ms *MonitorSession) SetInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	ms.mu.Lock()
	changed := ms.interval != d
	ms.interval = d
	ms.mu.Unlock()
	if changed {
		select {
		case ms.wake <- struct{}{}:
		default:
		}
	}
}

// Interval 是当前的采样间隔。
func (ms *MonitorSession) Interval() time.Duration {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.interval
}

// View 返回当前状态的拷贝。
func (ms *MonitorSession) View() MonitorView {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	v := MonitorView{Points: append([]MonitorPoint(nil), ms.points...), Loaded: ms.loaded}
	if ms.err != nil {
		v.Err = ms.err.Error()
	}
	return v
}

// Stop 停止采样并取消正在进行的那次查询，不等后台 goroutine 退出（界面线程调用，不能阻塞）；可以重复调用。
func (ms *MonitorSession) Stop() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	select {
	case <-ms.stop:
		return
	default:
		close(ms.stop)
	}
	if ms.cancel != nil {
		ms.cancel()
	}
}
