package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"minidb/internal/database"
)

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", what)
}

// 计数器按相邻两次采样的差算速率，缓冲只保留最近的 monitorMaxPoints 个点。
func TestMonitorSessionSamplesAndTrims(t *testing.T) {
	var n atomic.Int64
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sample := func(ctx context.Context) (database.MonitorSample, error) {
		i := n.Add(1)
		return database.MonitorSample{At: base.Add(time.Duration(i) * time.Second), Values: map[string]float64{
			database.MetricConnections: float64(i), database.MetricQueries: float64(i * 10),
		}}, nil
	}
	var updates atomic.Int64
	ms := NewMonitorSession(sample, time.Millisecond, func() { updates.Add(1) })
	waitFor(t, "采满缓冲", func() bool { return n.Load() > monitorMaxPoints+20 })
	ms.Stop()
	<-ms.done
	v := ms.View()
	if !v.Loaded || v.Err != "" || len(v.Points) != monitorMaxPoints {
		t.Fatalf("view: loaded=%v err=%q points=%d", v.Loaded, v.Err, len(v.Points))
	}
	last := v.Points[len(v.Points)-1]
	if last.Values[database.MetricQueries] != 10 { // 每秒增加 10
		t.Errorf("速率 = %v", last.Values)
	}
	if updates.Load() == 0 {
		t.Error("每个点都应通知界面")
	}
	before := len(ms.View().Points)
	time.Sleep(20 * time.Millisecond)
	if len(ms.View().Points) != before {
		t.Error("Stop 之后不该再有新点")
	}
	ms.Stop() // 重复调用不能 panic
}

// 采样出错只记下错误，不丢已有的点，恢复后错误清掉。
func TestMonitorSessionKeepsPointsOnError(t *testing.T) {
	var calls atomic.Int64
	var fail atomic.Bool
	sample := func(ctx context.Context) (database.MonitorSample, error) {
		if calls.Add(1) > 1 && fail.Load() {
			return database.MonitorSample{}, errors.New("boom")
		}
		return database.MonitorSample{At: time.Now(), Values: map[string]float64{database.MetricConnections: 1}}, nil
	}
	ms := NewMonitorSession(sample, time.Millisecond, nil)
	defer ms.Stop()
	waitFor(t, "第一个点", func() bool { return len(ms.View().Points) >= 1 })
	fail.Store(true)
	waitFor(t, "出现错误", func() bool { return ms.View().Err == "boom" })
	if v := ms.View(); len(v.Points) == 0 || !v.Loaded {
		t.Errorf("出错时应保留已有的点: %+v", v)
	}
	kept := len(ms.View().Points)
	fail.Store(false)
	waitFor(t, "错误清除", func() bool { v := ms.View(); return v.Err == "" && len(v.Points) > kept })
}

func TestMonitorSessionSetInterval(t *testing.T) {
	var calls atomic.Int64
	ms := NewMonitorSession(func(ctx context.Context) (database.MonitorSample, error) {
		calls.Add(1)
		return database.MonitorSample{At: time.Now(), Values: map[string]float64{}}, nil
	}, time.Hour, nil)
	defer ms.Stop()
	waitFor(t, "第一次采样", func() bool { return calls.Load() == 1 })
	ms.SetInterval(5 * time.Millisecond) // 间隔改短后应立刻生效，而不是等完一小时
	waitFor(t, "按新间隔继续采样", func() bool { return calls.Load() >= 3 })
	if ms.Interval() != 5*time.Millisecond {
		t.Errorf("Interval = %v", ms.Interval())
	}
}

// Stop 要能取消正在进行的采样，界面线程不会被卡住。
func TestMonitorSessionStopCancelsSample(t *testing.T) {
	started := make(chan struct{})
	ms := NewMonitorSession(func(ctx context.Context) (database.MonitorSample, error) {
		close(started)
		<-ctx.Done()
		return database.MonitorSample{}, ctx.Err()
	}, time.Hour, nil)
	<-started
	ms.Stop()
	select {
	case <-ms.done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop 之后后台 goroutine 没有退出")
	}
	if v := ms.View(); len(v.Points) != 0 {
		t.Errorf("被取消的采样不该产生点: %+v", v)
	}
}
