package ui

// @author ixxxxoooo

import (
	"fmt"
	"minidb/internal/app"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	appversion "minidb/internal/version"
	"minidb/locales"
)

// 「关于」页：应用图标、名称、版本和运行平台；更新状态卡（检查、下载进度、重启更新、自动下载开关）；
// 仓库、更新日志、反馈、许可的链接；数据和日志目录；版权。

// aboutLink 是「关于」页里一个外链。
type aboutLink struct {
	icon  *ui.SVG
	label string
	url   string
}

func (s *Shell) aboutSettings(c *ui.Context, p tokens) {
	repo := "https://github.com/" + appversion.Repository
	// 头部：图标 + 名称 + 版本，一眼知道这是什么、哪一版
	ui.Row(c).Gap(16).AlignItems(ui.Center).FillWidth().Padding(4, 0, 6, 0).Children(func() {
		logo(c, 72)
		ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, locales.AppName()).FontSize(22).FontWeight(700)
			ui.Text(c, appversion.Description).FontSize(font2XS + 1).TextColor(p.fgSecondary)
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				aboutChip(c, p, locales.SettingsVersion(s.appVersion()), true)
				aboutChip(c, p, runtime.GOOS+" · "+runtime.GOARCH, false)
			})
		})
	})
	s.updateCard(c, p)
	// 链接
	links := []aboutLink{
		{icLink, locales.AboutRepo(), repo},
		{icBars, locales.AboutChangelog(), repo + "/releases"},
		{icBot, locales.AboutIssues(), repo + "/issues/new"},
		{icShield, locales.AboutLicense(), repo + "/blob/main/LICENSE"},
	}
	ui.Row(c).Gap(8).FillWidth().Wrap().Children(func() {
		for _, l := range links {
			if fieldBtn(c.Key("lk-"+l.label), l.icon, l.label).Clicked() {
				_ = mygo.Shell.OpenExternal(l.url)
			}
		}
	})
	// 目录与版本信息
	info := [][2]string{
		{locales.AboutPlatform(), fmt.Sprintf("%s %s · %s", runtime.GOOS, runtime.GOARCH, runtime.Version())},
		{locales.SettingsDataDir(), app.DataDir()},
		{locales.AboutLogDir(), app.LogDir()},
	}
	ui.Column(c).FillWidth().Radius(radiusBtn).Border(1, p.borderSubtle).Children(func() {
		for i, r := range info {
			ui.Row(c.Key("ab-"+r[0])).Gap(12).Padding(8, 12).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, r[0]).FontWeight(500).Width(100).Shrink(0).SingleLine()
				ui.Text(c, r[1]).TextColor(p.fgSecondary).FontSize(font2XS).Grow(1).MinWidth(0).Selectable()
			})
			if i < len(info)-1 {
				hairline(c, p.borderSubtle)
			}
		}
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).FillWidth().Wrap().Children(func() {
		if fieldBtn(c, icFolder, locales.SettingsDataDirOpen()).Clicked() {
			_ = s.host.OpenPath(app.DataDir())
		}
		if fieldBtn(c, icTerminal, locales.SettingsLogs()).Clicked() {
			_ = s.host.OpenPath(app.LogDir())
		}
		if fieldBtn(c, icCopy, locales.AboutCopyInfo()).Clicked() {
			s.copyText(fmt.Sprintf("%s %s\n%s %s/%s\n%s", locales.AppName(), s.appVersion(), runtime.Version(), runtime.GOOS, runtime.GOARCH, repo))
		}
	})
	ui.Text(c, locales.AboutCopyright()).FontSize(font2XS).TextColor(p.fgMuted).Padding(6, 0, 0, 0)
}

// aboutChip 是头部的小标签：版本（强调色）和平台（中性）。
func aboutChip(c *ui.Context, p tokens, text string, accent bool) {
	col := p.fgSecondary
	bg := p.surfaceSecondary
	if accent {
		col, bg = p.accent, p.accent.Alpha(0.12)
	}
	ui.Text(c, text).FontSize(font2XS).TextColor(col).Padding(2, 8).Radius(999).Background(bg).SingleLine()
}

// updateCard 是更新状态卡：当前状态、对应的操作，以及「自动下载更新」开关。
func (s *Shell) updateCard(c *ui.Context, p tokens) {
	u := &s.upd
	// 更新卡：状态、进度、说明和开关在一起，是一块有内容的面板，所以保留框
	ui.Column(c).FillWidth().Gap(10).Padding(12, 14).Radius(radiusPanel-2).Background(p.surfaceSecondary.Alpha(0.55)).Border(1, p.borderSubtle).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).FillWidth().Children(func() {
			ic, tint, title, sub := icCheckC, p.success, locales.UpdUpToDate(), ""
			switch {
			case !updatesEnabled():
				ic, tint, title = icInfo, p.fgMuted, locales.SettingsUpdateDisabled()
			case u.phase == upChecking:
				ic, tint, title = icRefresh, p.accent, locales.UpdChecking()
			case u.phase == upAvailable:
				ic, tint, title = icDownload, p.accent, locales.UpdFoundTitle(u.version)
			case u.phase == upDownloading:
				ic, tint, title = icDownload, p.accent, locales.UpdDownloadingTitle(u.version)
			case u.phase == upReady:
				ic, tint, title, sub = icCheckC, p.success, locales.UpdReadyTitle(u.version), locales.UpdReadyDesc()
			case u.phase == upFailed:
				ic, tint, title, sub = icXCircle, p.danger, locales.UpdFailTitle(), u.err
			case !u.checkedAt.IsZero():
				sub = locales.UpdCheckedAt(u.checkedAt.Format("15:04"))
			}
			ui.Row(c).Size(34, 34).Shrink(0).Center().Radius(10).Background(tint.Alpha(0.13)).Children(func() {
				breathe(ui.Icon(c, ic).FontSize(17).TextColor(tint), "upd", u.phase == upChecking || u.phase == upDownloading)
			})
			ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, title).FontWeight(600)
				if sub != "" {
					ui.Text(c, sub).FontSize(font2XS).TextColor(p.fgSecondary).Selectable()
				}
			})
			switch {
			case !updatesEnabled(), u.phase == upChecking, u.phase == upDownloading:
			case u.phase == upReady:
				if fieldPrimary(c, icRefresh, locales.UpdRestartBtn()).Clicked() {
					s.applyUpdate()
				}
			case u.phase == upAvailable:
				if fieldPrimary(c, icDownload, locales.UpdDownloadBtn()).Clicked() {
					s.downloadUpdate()
				}
			case u.phase == upFailed && u.up != nil:
				if fieldBtn(c, icRefresh, locales.UpdRetryBtn()).Clicked() {
					s.downloadUpdate()
				}
			default:
				if fieldBtn(c, icRefresh, locales.SettingsCheckUpdates()).Clicked() {
					s.checkUpdates(true)
				}
			}
		})
		if u.phase == upDownloading {
			ui.Progress(c, float64(u.percent())/100).FillWidth()
		}
		if u.notes != "" && (u.phase == upAvailable || u.phase == upDownloading || u.phase == upReady) {
			ui.Text(c, headLines(u.notes, 12)).FontSize(font2XS).TextColor(p.fgSecondary).Selectable().FillWidth().
				Padding(8, 10).Radius(radiusBtn).Background(p.surface)
		}
		if updatesEnabled() {
			auto := !s.prefs.NoAutoUpdate
			ui.Row(c).Gap(14).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, locales.UpdAuto()).FontWeight(500)
					ui.Text(c, locales.UpdAutoDesc()).FontSize(font2XS).TextColor(p.fgSecondary)
				})
				if ui.Switch(c, &auto).Changed() {
					s.prefs.NoAutoUpdate = !auto
					s.savePrefs()
					if auto && s.upd.phase == upAvailable {
						s.downloadUpdate() // 刚打开自动下载：已经发现的新版本马上开始下载
					}
				}
			})
		}
	})
}
