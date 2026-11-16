package ui

import (
	"github.com/egoist/mygo"
)

// 窗口与退出的关系（@author ixxxxoooo）：
// 点关闭按钮（或「关闭窗口」菜单）只是把主窗口藏起来，应用继续在 Dock 里运行，
// 点 Dock 图标再显示出来；只有 Cmd+Q / 菜单里的退出才真正结束进程。

// watchLifecycle 在主窗口创建后调用一次：登记退出前的检查、关窗时隐藏、点 Dock 图标时恢复。
func (s *Shell) watchLifecycle() {
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if s.quitting || e.SessionEnding {
			s.exiting = true
			s.SaveState()
			return
		}
		if s.quitGuard() {
			s.reveal() // 确认框在主窗口里，窗口被藏起来时要先显示出来
			e.PreventDefault()
			return
		}
		s.exiting = true
		s.SaveState()
	})
	mygo.App.OnQuitCanceled(func() { s.exiting = false })
	mygo.App.OnActivate(func(visible bool) {
		if !visible {
			s.reveal()
		}
	})
	s.win.OnClose(s.onWindowClose)
}

// onWindowClose 处理主窗口的关闭请求：不是在退出过程中，就拦下来改成隐藏。
func (s *Shell) onWindowClose(e *mygo.CloseEvent) {
	if s.exiting {
		return
	}
	e.PreventDefault()
	s.SaveState()
	if s.win != nil {
		s.win.Hide()
	}
}

// reveal 把主窗口显示出来并带到前台（可能是被关闭按钮藏起来的，也可能被最小化了）。
func (s *Shell) reveal() {
	if s.win == nil {
		return
	}
	s.win.Show()
	s.win.Restore()
	s.win.Focus()
}
