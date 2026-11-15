package ui

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"minidb/internal/app"
	"minidb/locales"
	"strings"
	"time"
)

type note struct {
	at   time.Time
	text string
	kind string // kindInfo、kindSuccess、kindWarning、kindError
}

// notesState 是通知面板的状态。嵌在 Shell 里，字段名保持不变。
type notesState struct {
	notes     []note
	notesOpen bool
	notesSeen int // 用户已经看过多少条
}

// consoleState 是日志（控制台）面板的状态。嵌在 Shell 里，字段名保持不变。
type consoleState struct {
	console        []consoleEntry
	consoleOpen    bool
	consoleErrOnly bool           // 日志只看失败的
	consoleScroll  ui.ScrollState // 日志的滚动位置（要自己控制跟随末尾）
	consoleSeen    int            // 上次画日志时有多少条，用来发现新语句
	consolePct     float32        // 主区域占窗口的百分比，其余是日志；拖动分隔条改变
}

// addEntry 追加一条日志，超过 consoleMax 时丢掉最旧的。
func (c *consoleState) addEntry(e consoleEntry) {
	c.console = append(c.console, e)
	if len(c.console) > consoleMax {
		c.console = c.console[len(c.console)-consoleMax:]
	}
}

type consoleEntry struct {
	at   time.Time
	db   string
	sql  string
	dur  time.Duration
	rows int
	err  string
}

// consoleMax is how many statements the console keeps.
const consoleMax = 1000

// sqlTint colours a logged statement by what it does: reads in the text colour,
// writes in the warning colour, and what destroys or reshapes data in red.
func sqlTint(p tokens, sql string) ui.Color {
	switch app.SQLLeadingVerb(sql) {
	case "insert", "update", "delete", "replace", "merge", "commit":
		return p.warning
	case "drop", "truncate", "alter", "create", "rename", "grant", "revoke":
		return p.danger
	}
	return p.fg
}

// logSQL adds a statement to the console log.
func (s *Shell) logSQL(db, sql string, dur time.Duration, rows int, err string) {
	s.addEntry(consoleEntry{at: time.Now(), db: db, sql: strings.TrimSpace(sql), dur: dur, rows: rows, err: err})
	s.redraw()
}

// logCommit 记录一次提交：有预览出的语句就逐条记，没有就记一行说明。
func (s *Shell) logCommit(db string, stmts []string, n int, dur time.Duration, err string) {
	if len(stmts) == 0 {
		s.logSQL(db, fmt.Sprintf("-- COMMIT %d changes", n), dur, -1, err)
		return
	}
	for _, st := range stmts {
		s.logSQL(db, st, dur, -1, err)
	}
}

// notesPopover 是通知中心：从铃铛按钮下面弹出的一张小面板（不是居中的对话框），
// 点面板外或按 Esc 收起；最新的通知在最上面。
func (s *Shell) notesPopover(c *ui.Context, p tokens, anchor ui.Element) {
	if !s.notesOpen {
		return
	}
	s.notesSeen = len(s.notes) // 面板开着时新来的通知也算看过
	ui.PopoverBase(c, anchor, &s.notesOpen, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(spaceXS, 0, 0, 0).
			Width(sizeNotes).MaxHeight(460).Background(p.surfaceElev).Border(1, p.border).Radius(radiusDialog).
			Shadow(0, shadowY, shadowBlur, 0, p.shadow).Clip()
		ui.Column(c).FillWidth().Children(func() {
			ui.Row(c).Gap(spaceSM).Padding(spaceSM, spaceSM, spaceSM, spaceLG).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, locales.NotesTitle()).FontSize(fontCtl).FontWeight(600)
				ui.Spacer(c)
				if actBtn(c, nil, locales.NotesClear()).Disabled(len(s.notes) == 0).Clicked() {
					s.notes = nil
				}
			})
			if len(s.notes) == 0 {
				hairline(c, p.borderSubtle)
				ui.Column(c).FillWidth().Padding(28, 12).Center().Gap(8).Children(func() {
					ui.Icon(c, icBell).FontSize(22).TextColor(p.fgMuted)
					ui.Text(c, locales.NotesEmpty()).TextColor(p.fgMuted).FontSize(fontDense)
				})
				return
			}
			now := time.Now()
			scrollbars(c, ui.Scroll(c.Key("notes")).FillWidth().MaxHeight(410)).Children(func() {
				ui.Column(c).FillWidth().Children(func() {
					for i := len(s.notes) - 1; i >= 0; i-- {
						s.noteRow(c, p, i, now)
					}
				})
			})
		})
	})
}

// noteRow 画通知列表的一行（设计稿 A）：24px 圆形图标（按类型配淡底）、标题（文字的第一行）、
// 说明（其余行，淡色小字）、右端相对时间；行与行之间一条细线。
func (s *Shell) noteRow(c *ui.Context, p tokens, i int, now time.Time) {
	n := s.notes[i]
	kc, bg := kindColor(p, n.kind), p.accentSoft
	switch n.kind {
	case kindSuccess:
		bg = p.successBg
	case kindWarning:
		bg = p.warningBg
	case kindError:
		bg = p.dangerBg
	default:
		kc = p.accentText
	}
	title, detail, _ := strings.Cut(n.text, "\n")
	hairline(c, p.borderSubtle)
	ui.Row(c.Key(fmt.Sprint("n", i))).Gap(spaceMD).Padding(spaceSM+1, spaceLG).AlignItems(ui.Start).FillWidth().Children(func() {
		ui.Row(c).Size(sizeBtnSm-2, sizeBtnSm-2).Shrink(0).Center().Radius((sizeBtnSm - 2) / 2).Background(bg).Children(func() {
			ui.Icon(c, kindIcon(n.kind)).FontSize(fontDense).TextColor(kc)
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, title).FontSize(fontCtl).FontWeight(600).Selectable()
			if detail != "" {
				ui.Text(c, detail).FontSize(fontCaption).TextColor(p.fgMuted).Selectable()
			}
		})
		ui.Text(c, agoText(n.at.Unix(), now)).FontSize(fontCaption - 0.5).TextColor(p.fgMuted).Shrink(0)
	})
}

// consolePanel 是窗口底部的 SQL 日志（⌃⌘C）：语句带语法高亮，失败的整行泛红，
// 新语句出现时若已滚到底就自动跟到底；面板上沿可以拖动改变高度。
func (s *Shell) consolePanel(c *ui.Context, p tokens) {
	ui.Column(c.Key("console")).Fill().Background(p.surface).Children(func() {
		// 过滤后的条目，下标仍指向 s.console
		var shown []int
		failed := 0
		for i, e := range s.console {
			if e.err != "" {
				failed++
			}
			if s.consoleErrOnly && e.err == "" {
				continue
			}
			shown = append(shown, i)
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 8, 0, 12).Height(30).FillWidth().Shrink(0).Background(p.toolbar).Children(func() {
			ui.Icon(c, icConsole).FontSize(13).TextColor(p.fgSecondary)
			ui.Text(c, locales.ConsoleTitle()).FontSize(font2XS).FontWeight(600)
			ui.Text(c, locales.ConsoleCount(len(s.console))).FontSize(font2XS - 1).TextColor(p.fgMuted)
			if failed > 0 {
				ui.Text(c, fmt.Sprint(failed)).FontSize(font2XS-1).FontWeight(600).TextColor(p.danger).Padding(1, 6).Radius(8).Background(p.danger.Alpha(0.12))
			}
			ui.Spacer(c)
			mode := 0
			if s.consoleErrOnly {
				mode = 1
			}
			if segmented(c.Key("console-filter"), &mode, locales.ConsoleFilterAll(), locales.ConsoleFilterErrors()) {
				s.consoleErrOnly = mode == 1
			}
			if tipBtn(c, icCopy, locales.ConsoleCopyAll()).Size(sizeBtnSm, sizeBtnSm).Disabled(len(shown) == 0).Clicked() {
				var b strings.Builder
				for _, i := range shown {
					e := s.console[i]
					fmt.Fprintf(&b, "-- %s %s\n%s;\n", e.at.Format("15:04:05"), fmtDur(e.dur), strings.TrimRight(e.sql, "; \n"))
				}
				s.clip = b.String()
				s.toastOK(locales.StatusCopied())
			}
			if tipBtn(c, icTrash, locales.NotesClear()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				s.console = nil
			}
			if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				s.consoleOpen = false
			}
		})
		hairline(c, p.borderSubtle)
		if len(shown) == 0 {
			msg := locales.ConsoleEmpty()
			if len(s.console) > 0 {
				msg = locales.ConsoleNoMatch()
			}
			ui.Text(c, msg).FontSize(font2XS).TextColor(p.fgMuted).Padding(12)
			return
		}
		// 新语句进来时，原本就在底部的话继续跟着滚
		sc := &s.consoleScroll
		if len(s.console) != s.consoleSeen {
			if sc.Y >= sc.MaxY-4 {
				sc.Y = math.MaxFloat32
			}
			s.consoleSeen = len(s.console)
		}
		scrollbarsOn(c, ui.Scroll(c.Key("consolelog")).Grow(1).FillWidth(), sc).Children(func() {
			ui.Column(c).Padding(2, 0).FillWidth().Children(func() {
				for _, i := range shown {
					s.consoleRow(c, p, i)
				}
			})
		})
	})
}

// consoleRow 画日志里的一条：时间、状态点、高亮后的语句、耗时与行数。
func (s *Shell) consoleRow(c *ui.Context, p tokens, i int) {
	e := s.console[i]
	row := ui.Row(c.Key(fmt.Sprint("l", i))).Gap(10).Padding(3, 12).AlignItems(ui.Start).FillWidth()
	hovered := row.Hovered()
	switch {
	case e.err != "":
		row.Background(p.danger.Alpha(0.07))
	case hovered:
		row.Background(p.hover)
	}
	row.Children(func() {
		ui.Text(c, e.at.Format("15:04:05")).Font(mono).FontSize(font2XS-1).TextColor(p.fgMuted).Shrink(0).Padding(1, 0, 0, 0)
		dot := p.success
		if e.err != "" {
			dot = p.danger
		}
		ui.Box(c).Size(6, 6).Radius(3).Background(dot).Shrink(0).Margin(5, 0, 0, 0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			base := sqlTint(p, e.sql)
			if e.err != "" {
				base = p.fg
			}
			ui.RichText(c, sqlSpans(p, e.sql, base)...).Font(mono).FontSize(font2XS).Selectable()
			if e.err != "" {
				ui.Text(c, e.err).FontSize(font2XS - 1).TextColor(p.danger).Selectable()
			}
		})
		meta := fmtDur(e.dur)
		if e.rows >= 0 && e.err == "" {
			meta += fmt.Sprintf(" · %d", e.rows)
		}
		col := p.fgMuted
		if e.dur >= time.Second { // 慢语句用橙色提醒
			col = p.warning
		}
		ui.Text(c, meta).Font(mono).FontSize(font2XS-1).TextColor(col).SingleLine().Shrink(0).Padding(1, 0, 0, 0)
		// 悬停时右侧出现复制按钮；位置始终预留，免得鼠标一过整行跳动
		ui.Box(c.Key("cp-slot")).Size(sizeBtnSm, sizeBtnSm).Shrink(0).Margin(-3, 0, -3, 0).Children(func() {
			if hovered && tipBtn(c, icCopy, locales.CommonCopy()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				s.copyText(e.sql)
			}
		})
	})
}
