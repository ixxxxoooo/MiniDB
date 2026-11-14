package ui

import (
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/logger"
)

// 界面线程的性能打点（规则见 CLAUDE.md「性能日志」，解读方法见 docs/PERFORMANCE.md）。
//
//   - ui.build            整帧的构建（Shell.view），立即模式下点击处理也在里面
//   - ui.build.<段>       帧里的一段：工具栏、侧栏、标签内容、网格、AI 面板、对话框…
//   - bg.<函数>           up.bg 的后台部分，名字取自调用 up.bg 的函数，自动打点，不用手写
//   - ui.done.<函数>      up.bg 的收尾回调，在 UI 线程上执行
//   - ui.post.<函数>      其他经 up.post 投递到 UI 线程的回调
//   - ui.queue            回调从投递到真正开始执行等了多久，长说明 UI 线程一直忙
//
// 在 UI 线程上登记的段落（PerfUI）会被看门狗盯着：停留超过 300ms 就把所有协程的堆栈写进日志。
//
// @author ixxxxoooo

// timedView 是窗口内容的入口：给整帧计时并登记，defer 保证出 panic 时也能注销。
func (s *Shell) timedView(c *ui.Context) {
	defer logger.PerfUI("ui.build")()
	s.view(c)
}

// perfBuild 给帧里的一段计时。name 要是完整的 ui.build.<段>。
// 参数 fn 是这一段的构建函数。
func perfBuild(name string, fn func()) {
	defer logger.PerfUI(name)()
	fn()
}

// tabPerfName 返回当前标签内容对应的段名；按类型取常量，免得每帧拼字符串。
func tabPerfName(t any) string {
	switch t.(type) {
	case *tableTab:
		return "ui.build.tab.table"
	case *queryTab:
		return "ui.build.tab.query"
	case *overviewTab:
		return "ui.build.tab.overview"
	case *defTab:
		return "ui.build.tab.definition"
	case *erTab:
		return "ui.build.tab.er"
	}
	return "ui.build.tab.other"
}

// perfSite 返回调用者的函数名（去掉包路径），用来给 up.bg / up.post 的打点命名。
// 参数 skip 是往上数几层：0 是 perfSite 自己，1 是它的调用者，依此类推。
func perfSite(skip int) string {
	var pc [1]uintptr
	if runtime.Callers(skip+1, pc[:]) == 0 {
		return "unknown"
	}
	f, _ := runtime.CallersFrames(pc[:]).Next()
	name := f.Function
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.TrimPrefix(name, "ui.")
	return strings.NewReplacer("(*", "", ")", "").Replace(name)
}

// queueWatch 返回一个函数：回调开始执行时调用它，记录从投递到现在排了多久。
func queueWatch(task string) func() {
	queued := time.Now()
	return func() { logger.PerfDone("ui.queue", time.Since(queued), "next", task) }
}
