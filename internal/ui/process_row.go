package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 进程页的一行（设计稿「监控 · 进程」）：复选框、ID、用户、库、状态芯片、耗时、语句、行内两个终止按钮。
// 勾选的行淡强调底；跑得久的耗时标红加粗；「终止」是红淡底按钮，「终止查询」是普通按钮。

const (
	slowProcessSec = 10 // 耗时达到这么多秒就标红
	procStateW     = 64 // 「状态」列宽
)

// processRow 画进程列表里的一行。参数：c 上下文，p 主题色板，it 是这条进程，i 是行号（用来画斑马纹）。
func (s *Shell) processRow(c *ui.Context, p tokens, it app.ProcessInfo, i int) {
	d := &s.proc
	picked := d.sel[it.ID]
	row := ui.Row(c.Key(it.ID)).Gap(10).Padding(0, 10).Height(sizeProcRow).AlignItems(ui.Center).FillWidth()
	switch {
	case picked:
		row.Background(p.accentSoft)
	case i%2 == 1:
		row.Background(p.surface)
	}
	row.Children(func() {
		on := picked
		if ui.Checkbox(c.Key("sel"), &on, "").Changed() {
			d.sel[it.ID] = on
		}
		ui.Text(c, it.ID).Width(60).FontSize(font2XS).Font(mono)
		ui.Text(c, it.User).Width(90).FontSize(font2XS).SingleLine()
		ui.Text(c, it.Database).Width(90).FontSize(font2XS).SingleLine()
		ui.Row(c).Width(procStateW).Shrink(0).Children(func() {
			if it.Command != "" {
				kind := chipNeutral
				if strings.EqualFold(it.Command, "query") { // 正在执行语句的是绿色，休眠等是灰色
					kind = chipOK
				}
				chip(c, kind, it.Command, false)
			}
		})
		tm := ui.Text(c, fmt.Sprint(it.TimeSec, " s")).Width(56).FontSize(font2XS).Font(mono)
		if it.TimeSec >= slowProcessSec {
			tm.TextColor(p.danger).FontWeight(600)
		}
		ui.RichText(c, sqlLineSpans(p, it.Info, p.fgSecondary)...).Grow(1).FontSize(font2XS).Font(mono).SingleLine()
		if actBtn(c, icStop, locales.ProcessKillQuery()).Clicked() {
			s.killProcess(it.ID, true)
		}
		if dangerSoftBtn(c, icUnplug, locales.ProcessKill()).Clicked() {
			s.killProcess(it.ID, false)
		}
	})
}
