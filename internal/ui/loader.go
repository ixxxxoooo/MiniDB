package ui

import (
	"context"
	"time"
)

// loader 是一次异步加载的状态：加载中、错误文字、最近一次成功的结果，
// 并带序号保护——连着发起两次加载（快速切库、反复刷新）时，慢的旧响应回来会被丢掉，
// 不会盖住新的结果，也不会让界面闪回旧数据。
// 只在 UI 线程上使用；work 在后台线程跑，done 回到 UI 线程。
type loader[T any] struct {
	Loading bool
	Err     string
	Val     T
	seq     int
	stop    context.CancelFunc // 延迟加载用：新的一次开始或 cancel 时让还在路上的旧加载作废
}

// begin 标记一次加载开始，返回这次的序号；finish 凭这个序号交回结果。
func (l *loader[T]) begin() int {
	l.abort()
	l.seq++
	l.Loading, l.Err = true, ""
	return l.seq
}

// finish 交回序号为 mine 的那次加载的结果；它已不是最新一次时返回 false 并丢弃结果。
func (l *loader[T]) finish(mine int, v T, err error) bool {
	if mine != l.seq {
		return false
	}
	l.Loading = false
	if err != nil {
		l.Err = errMessage(err)
	} else {
		l.Val = v
	}
	return true
}

// load 发起一次加载。work 在后台取数据；done 在 UI 线程上收结果，
// 只有这一次仍是最新的一次时才会被调用（err 不为空时 Val 保持上一次的值）。
func (l *loader[T]) load(up uiThread, work func() (T, error), done func(v T, err error)) {
	mine := l.begin()
	var v T
	var err error
	up.bg(func() { v, err = work() }, func() {
		if l.finish(mine, v, err) && done != nil {
			done(v, err)
		}
	})
}

// cancel 让还在路上的加载结果作废（比如关闭了它所属的标签）。
func (l *loader[T]) cancel() {
	l.abort()
	l.seq++
	l.Loading = false
}

// abort 取消还没开始干活的延迟加载（已经在跑的取数靠序号丢弃结果）。
func (l *loader[T]) abort() {
	if l.stop != nil {
		l.stop()
		l.stop = nil
	}
}

// loadAfter 和 load 一样，但先等 delay 再取数据：用于"不急着要"的数据，让用户刚打开的内容先画出来。
// 等待期间如果又发起了新的加载、或被 cancel（切库、关标签页），这一次直接作废、不再取数。
// work 拿到的 ctx 在同样的时机被取消，取数应当把它传下去。
func (l *loader[T]) loadAfter(up uiThread, delay time.Duration, work func(ctx context.Context) (T, error), done func(v T, err error)) {
	mine := l.begin()
	ctx, cancel := context.WithCancel(context.Background())
	l.stop = cancel
	var v T
	var err error
	up.bg(func() {
		if !waitOrCancel(ctx, delay) {
			return
		}
		v, err = work(ctx)
	}, func() {
		if l.finish(mine, v, err) && done != nil {
			done(v, err)
		}
	})
}

// waitOrCancel 等 delay；等待中 ctx 被取消返回 false，否则返回 true。
func waitOrCancel(ctx context.Context, delay time.Duration) bool {
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return false
		}
	}
	return ctx.Err() == nil
}
