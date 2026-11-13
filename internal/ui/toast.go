package ui

import (
	"time"

	"github.com/egoist/mygo/ui"
	"minidb/locales"
)

// 提示的种类。右上角的提示卡和通知中心都按它配色、配图标。
const (
	kindInfo    = "info"
	kindSuccess = "success"
	kindWarning = "warning"
	kindError   = "error"
)

// toastReq 是等着在下一帧弹出的一条提示。
type toastReq struct {
	msg, kind string
}

var (
	icAlertTri = svg(`<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3"/><path d="M12 9v4"/><path d="M12 17h.01"/>`)
	icXCircle  = svg(`<circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/>`)
)

// toast 弹一条中性提示（也记进通知中心）。
func (s *Shell) toast(msg string) { s.pushToast(msg, kindInfo) }

// toastOK 弹一条"成功"提示。
func (s *Shell) toastOK(msg string) { s.pushToast(msg, kindSuccess) }

// toastWarn 弹一条"请注意"提示：操作没做成，但不算出错。
func (s *Shell) toastWarn(msg string) { s.pushToast(msg, kindWarning) }

// toastErr 弹一条"失败"提示，停留得更久。
func (s *Shell) toastErr(msg string) { s.pushToast(msg, kindError) }

func (s *Shell) pushToast(msg, kind string) {
	s.toasts = append(s.toasts, toastReq{msg, kind})
	s.notes = append(s.notes, note{at: time.Now(), text: msg, kind: kind})
	if len(s.notes) > 100 {
		s.notes = s.notes[len(s.notes)-100:]
		s.notesSeen = max(s.notesSeen-1, 0)
	}
	if s.win != nil {
		s.win.Invalidate()
	}
}

// kindColor 是一种提示的代表色。
func kindColor(p tokens, kind string) ui.Color {
	switch kind {
	case kindSuccess:
		return p.success
	case kindWarning:
		return p.warning
	case kindError:
		return p.danger
	}
	return p.accent
}

// kindIcon 是一种提示的图标。
func kindIcon(kind string) *ui.SVG {
	switch kind {
	case kindSuccess:
		return icCheck
	case kindWarning:
		return icAlertTri
	case kindError:
		return icXCircle
	}
	return icInfo
}

// toastTimeout 是一种提示停留多久：失败的要让人看清，所以更长。
func toastTimeout(kind string, hasAction bool) time.Duration {
	switch {
	case kind == kindError:
		return 9 * time.Second
	case hasAction:
		return 7 * time.Second
	case kind == kindWarning:
		return 5 * time.Second
	}
	return 3500 * time.Millisecond
}

// flushToasts 把攒下的提示交给窗口，在每一帧开头调用。
func (s *Shell) flushToasts(c *ui.Context) {
	for _, m := range s.toasts {
		c.AddToast(ui.Toast{ID: m.kind + "|" + m.msg, Title: m.msg, Type: m.kind, Timeout: toastTimeout(m.kind, false)})
	}
	s.toasts = s.toasts[:0]
	for _, a := range s.toastActs {
		c.AddToast(ui.Toast{ID: kindSuccess + "|" + a.msg, Title: a.msg, Type: kindSuccess, Timeout: toastTimeout(kindSuccess, true), Action: a.label, OnAction: a.fn})
	}
	s.toastActs = s.toastActs[:0]
}

// toastViewport 在窗口右上角（工具栏下方）画提示卡：新的在上，最多三张，
// 指针停在上面时计时暂停，底部一条细线倒数剩余时间。
// 它取了通知栏（短暂浮现）与通知中心（铃铛里留底）两种常见做法的长处：
// 浮层只负责"刚刚发生了什么"，想回看就点铃铛。
func (s *Shell) toastViewport(c *ui.Context, p tokens) {
	ui.ToastViewportBase(c, func(vp ui.Element, toasts []ui.Toast) {
		vp.Padding(sizeToolbar+10, 14, 0, 0).AlignItems(ui.End).Justify(ui.Start).Gap(8)
		for i := len(toasts) - 1; i >= 0; i-- {
			s.toastCard(c, p, toasts[i])
		}
	})
}

func (s *Shell) toastCard(c *ui.Context, p tokens, tt ui.Toast) {
	t := ui.ToastBase(c, tt)
	col := kindColor(p, tt.Type)
	t.Root.Row().Width(348).AlignItems(ui.Start).Gap(10).Padding(11, 8, 11, 12).Radius(radiusPanel).
		Background(p.surfaceElev).Border(1, p.border).Shadow(0, 6, 22, 0, ui.RGBA(0, 0, 0, 0.16)).
		Children(func() {
			ui.Row(c).Size(22, 22).Shrink(0).Center().Radius(11).Background(col.Alpha(0.15)).Children(func() {
				ui.Icon(c, kindIcon(tt.Type)).FontSize(13).TextColor(col)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(6).Padding(2, 0, 0, 0).Children(func() {
				ui.Text(c, tt.Title).FontSize(fontXS).TextColor(p.fg).Selectable()
				if tt.Action != "" {
					a := t.ActionButton().Height(22).Padding(0, 8).Radius(radiusBtn - 2).Background(col.Alpha(0.12)).AlignSelf(ui.Start)
					if a.Hovered() {
						a.Background(col.Alpha(0.22))
					}
					a.Children(func() { ui.Text(c, tt.Action).FontSize(font2XS).FontWeight(600).TextColor(col) })
				}
			})
			x := t.CloseButton().Label(locales.CommonClose()).Size(20, 20).Shrink(0).Radius(5).Center()
			if x.Hovered() {
				x.Background(p.hover)
			}
			x.Children(func() { ui.Icon(c, icX).FontSize(12).TextColor(p.fgMuted) })
		})
	// 倒计时：底部一条随剩余时间缩短的细线
	if left, total := t.Left(), toastTimeout(tt.Type, tt.Action != ""); left > 0 && total > 0 {
		frac := float32(left) / float32(total)
		t.Root.DrawOver(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X + 12, Y: r.Y + r.H - 2.5, W: (r.W - 24) * min(frac, 1), H: 1.5}, col.Alpha(0.55), 0)
		})
		c.AnimationFrame()
	}
}
