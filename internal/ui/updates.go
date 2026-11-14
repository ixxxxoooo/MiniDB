package ui

// @author ixxxxoooo

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"minidb/internal/logger"
	"minidb/locales"
)

// 自动更新：启动 10 秒后和之后每 6 小时在后台检查；发现新版本就自动下载（设置里可以关），
// 下载完成后主界面右上角出现「新版本 x · 重启更新」，点一下重启进入新版本。
// 更新用 mygo.Updater：下载后核对签名、原地替换应用，运行中的这一份不受影响，重启（或下次启动）才生效。

type updatePhase int

const (
	upIdle        updatePhase = iota
	upChecking                // 正在查
	upAvailable               // 有新版本，还没下载（自动下载关闭时停在这里）
	upDownloading             // 正在下载
	upReady                   // 下载好了，等重启
	upUpToDate                // 查过了，已是最新
	upFailed                  // 检查或下载失败
)

// updateState 是更新的状态。下载进度由下载线程写、界面读，所以用原子数。
type updateState struct {
	phase     updatePhase
	version   string
	notes     string
	err       string
	checkedAt time.Time
	up        *mygo.Update
	done      atomic.Int64 // 已下载字节
	total     atomic.Int64
}

// percent 是下载进度，0～100；总大小未知时为 0。
func (u *updateState) percent() int {
	if t := u.total.Load(); t > 0 {
		return int(min(u.done.Load()*100/t, 100))
	}
	return 0
}

// updatesEnabled 报告这个构建能不能自己更新（开发版、没配更新源、由包管理器安装的不能）。
func updatesEnabled() bool { return mygo.Updater.Enabled() }

// startUpdater 开始后台检查：启动 10 秒后第一次，之后每 6 小时一次。
func (s *Shell) startUpdater() {
	if !updatesEnabled() {
		return
	}
	go func() {
		time.Sleep(10 * time.Second)
		for {
			s.up.post(func() { s.checkUpdates(false) })
			time.Sleep(6 * time.Hour)
		}
	}()
}

// checkUpdates 检查更新。manual 为真是用户在「关于」里点的：失败要显示出来；后台检查失败只记日志。
func (s *Shell) checkUpdates(manual bool) {
	u := &s.upd
	switch u.phase {
	case upChecking, upDownloading, upReady:
		return
	}
	if !updatesEnabled() {
		return
	}
	u.phase, u.err = upChecking, ""
	var up *mygo.Update
	var err error
	s.up.bg(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		up, err = mygo.Updater.Check(ctx)
	}, func() {
		u.checkedAt = time.Now()
		switch {
		case err != nil:
			logger.Warn("[Updater] 检查更新失败: %v", err)
			u.phase = upIdle
			if manual {
				u.phase, u.err = upFailed, err.Error()
			}
		case up == nil:
			u.phase = upUpToDate
		default:
			u.phase, u.up, u.version, u.notes = upAvailable, up, up.Version, up.Notes
			logger.Info("[Updater] 发现新版本 %s", up.Version)
			if !s.prefs.NoAutoUpdate {
				s.downloadUpdate()
			}
		}
	})
}

// downloadUpdate 在后台下载并安装（替换磁盘上的应用）新版本，进度写进 u.done / u.total。
func (s *Shell) downloadUpdate() {
	u := &s.upd
	if u.up == nil || u.phase == upDownloading || u.phase == upReady {
		return
	}
	u.phase, u.err = upDownloading, ""
	u.done.Store(0)
	u.total.Store(0)
	up := u.up
	var err error
	last := -1
	s.up.bg(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		err = up.Install(ctx, func(done, total int64) {
			u.done.Store(done)
			u.total.Store(total)
			if p := u.percent(); p != last { // 百分比变了才请求重绘
				last = p
				s.up.post(s.redraw)
			}
		})
	}, func() {
		if err != nil {
			logger.Warn("[Updater] 下载更新失败: %v", err)
			u.phase, u.err = upFailed, err.Error()
			return
		}
		logger.Info("[Updater] 新版本 %s 已下载，等待重启", u.version)
		u.phase = upReady
	})
}

// applyUpdate 重启进入已经装好的新版本；有未提交的修改时退出前会先询问。
func (s *Shell) applyUpdate() {
	if s.upd.phase != upReady {
		return
	}
	mygo.App.Relaunch()
}

// updateAction 是点「更新提示」时该做的事：下载好了就重启，还没下载就下载，失败就重试下载或重新检查。
func (s *Shell) updateAction() {
	u := &s.upd
	switch u.phase {
	case upReady:
		s.applyUpdate()
	case upAvailable:
		s.downloadUpdate()
	case upFailed:
		if u.up != nil {
			s.downloadUpdate()
		} else {
			s.checkUpdates(true)
		}
	}
}

// updateChip 是主界面右上角的更新提示：下载中显示进度，下载好了「重启更新」，点一下就更新。
// 没有更新、正在检查时什么也不画。
func (s *Shell) updateChip(c *ui.Context, p tokens) {
	u := &s.upd
	switch u.phase {
	case upDownloading:
		ui.Row(c.Key("upd-dl")).Gap(6).AlignItems(ui.Center).Padding(0, 6).Children(func() {
			ui.Spinner(c).Size(11, 11)
			ui.Text(c, locales.UpdChipDownloading(u.percent())).FontSize(font2XS).TextColor(p.fgSecondary).SingleLine()
		})
	case upReady, upAvailable, upFailed:
		label, ic := locales.UpdChipReady(u.version), icDownload
		switch u.phase {
		case upAvailable:
			label = locales.UpdChipFound(u.version)
		case upFailed:
			if u.up == nil {
				return // 检查失败只在「关于」里说，不在主界面打扰
			}
			label, ic = locales.UpdChipFailed(), icRefresh
		}
		b := actPrimary(c.Key("upd-chip"), ic, label)
		if u.phase == upFailed {
			b = actBtn(c.Key("upd-chip"), ic, label)
		}
		if b.Transition(enterTransition).Clicked() {
			s.updateAction()
		}
	}
}
