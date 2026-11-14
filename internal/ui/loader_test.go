package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestLoaderDropsStaleResponse(t *testing.T) {
	var l loader[string]
	first := l.begin()  // 慢的旧请求
	second := l.begin() // 快的新请求
	if !l.finish(second, "新", nil) || l.Val != "新" || l.Loading {
		t.Fatalf("新请求先回来应生效: %+v", l)
	}
	if l.finish(first, "旧", nil) || l.Val != "新" {
		t.Fatalf("旧请求后到应被丢弃，Val = %q", l.Val)
	}
}

func TestLoaderLoadRunsDoneOnce(t *testing.T) {
	var l loader[string]
	up := uiThread{win: func() *mygo.Window { return nil }, sync: true}
	var applied []string
	l.load(up, func() (string, error) { return "旧", nil }, func(v string, _ error) { applied = append(applied, v) })
	l.load(up, func() (string, error) { return "新", nil }, func(v string, _ error) { applied = append(applied, v) })
	if l.Val != "新" || len(applied) != 2 || l.Loading {
		t.Fatalf("%+v applied=%v", l, applied)
	}
}

func TestLoaderSeqInvalidatesInFlight(t *testing.T) {
	var l loader[int]
	l.seq = 5
	mine := l.seq
	l.cancel()
	if mine == l.seq {
		t.Fatal("cancel 应使在途的加载作废")
	}
}

func TestLoaderKeepsValueOnError(t *testing.T) {
	var l loader[int]
	up := uiThread{win: func() *mygo.Window { return nil }, sync: true}
	l.load(up, func() (int, error) { return 7, nil }, nil)
	l.load(up, func() (int, error) { return 0, errors.New("boom") }, nil)
	if l.Val != 7 || l.Err != "boom" || l.Loading {
		t.Fatalf("出错时保留旧值并记下错误: %+v", l)
	}
}

// 同步模式下，别的协程 post 的任务不能直接执行（会和 UI 协程抢状态），要排队等 UI 协程 drain。
func TestSyncThreadQueuesPostsFromOtherGoroutines(t *testing.T) {
	up := newUIThread(func() *mygo.Window { return nil }, true)
	var ran []string
	up.post(func() { ran = append(ran, "own") })
	if len(ran) != 1 {
		t.Fatal("UI 协程自己 post 的任务应当立即执行")
	}
	done := make(chan struct{})
	go func() {
		up.post(func() { ran = append(ran, "other") })
		close(done)
	}()
	<-done
	if len(ran) != 1 {
		t.Fatalf("别的协程 post 的任务不应立即执行: %v", ran)
	}
	up.drain()
	if len(ran) != 2 || ran[1] != "other" {
		t.Fatalf("drain 之后应执行排队的任务: %v", ran)
	}
}

// 延迟加载：新的一次开始或 cancel 时，旧的那次的 ctx 被取消，等待中的加载不会再去取数。
func TestLoaderLoadAfterIsCancelable(t *testing.T) {
	var l loader[int]
	up := uiThread{win: func() *mygo.Window { return nil }, sync: true}
	var first, second context.Context
	l.loadAfter(up, 0, func(ctx context.Context) (int, error) { first = ctx; return 1, nil }, nil)
	if l.Val != 1 || first.Err() != nil {
		t.Fatalf("第一次加载应成功且尚未取消: %+v", l)
	}
	l.loadAfter(up, 0, func(ctx context.Context) (int, error) { second = ctx; return 2, nil }, nil)
	if first.Err() == nil || second.Err() != nil || l.Val != 2 {
		t.Fatal("新一次开始应取消旧的")
	}
	l.cancel()
	if second.Err() == nil {
		t.Fatal("cancel 应取消在途的延迟加载")
	}
}

// 等待期间被取消：waitOrCancel 立刻返回 false，work 不会执行；没取消则等满 delay 返回 true。
func TestWaitOrCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	if !waitOrCancel(ctx, 0) || !waitOrCancel(ctx, time.Millisecond) {
		t.Fatal("没取消时应放行")
	}
	cancel()
	start := time.Now()
	if waitOrCancel(ctx, time.Hour) || time.Since(start) > time.Second {
		t.Fatal("已取消应立即返回 false，不能等满 delay")
	}
}
