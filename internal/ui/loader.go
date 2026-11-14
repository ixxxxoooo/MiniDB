package ui

// loader 是一次异步加载的状态：加载中、错误文字、最近一次成功的结果，
// 并带序号保护——连着发起两次加载（快速切库、反复刷新）时，慢的旧响应回来会被丢掉，
// 不会盖住新的结果，也不会让界面闪回旧数据。
// 只在 UI 线程上使用；work 在后台线程跑，done 回到 UI 线程。
type loader[T any] struct {
	Loading bool
	Err     string
	Val     T
	seq     int
}

// begin 标记一次加载开始，返回这次的序号；finish 凭这个序号交回结果。
func (l *loader[T]) begin() int {
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
	l.seq++
	l.Loading = false
}
