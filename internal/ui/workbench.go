package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/database"
	"minidb/locales"
)

// ---- resizable panes ----

// vHandle is the thin line between two side-by-side panes. It reports how far
// it was dragged this frame, and a double click.
func vHandle(c *ui.Context, p tokens, key string) (dx float32, reset bool) {
	// 布局里只占 1px 的分割线，两侧不留空隙；真正用来抓取的 7px 宽区域浮在线的上方。
	hover := false
	line := ui.Box(c.Key(key)).Width(1).FillHeight().Shrink(0).Background(p.border)
	line.Children(func() {
		grab := ui.Box(c.Key(key + "-grab")).Absolute().Top(0).Bottom(0).Left(-3).Width(7).Cursor(ui.CursorResizeEW).Background(ui.Transparent)
		if x, _, ok := grab.Dragged(); ok {
			dx = x
		}
		reset = grab.DoubleClicked()
		hover = grab.Hovered()
	})
	line.Draw(func(pt *ui.Painter, r ui.Rect) {
		if hover {
			pt.Fill(r, p.accent, 0)
		}
	})
	return dx, reset
}

// view lays out a workspace: the sidebar (which ⌘0 hides), the content, and
// the AI panel when open. Every pane keeps its place in the tree whether or
// not its neighbours are shown, so opening one never resets another's scroll.
func (w *Workspace) view(c *ui.Context, p tokens) {
	w.trackTabs()
	ui.Row(c).Grow(1).FillWidth().FillHeight().MinWidth(0).Children(func() {
		side := ui.Column(c.Key("sidebar")).FillHeight().Shrink(0)
		if w.sidebarHidden {
			side.Width(0).Children(func() {})
		} else {
			side.Width(w.sidebarW).Children(func() { w.sidebar(c, p) })
			dx, reset := vHandle(c, p, "sidebar-handle")
			w.sidebarW = min(max(w.sidebarW+dx, 160), 520)
			if reset {
				w.sidebarW = 260
			}
		}
		ui.Column(c.Key("content")).Grow(1).MinWidth(0).FillHeight().Children(func() { w.content(c, p) })
	})
}

// content is the tabs, with the AI panel beside them when it is open.
func (w *Workspace) content(c *ui.Context, p tokens) {
	ui.Row(c).Fill().MinWidth(0).Children(func() {
		ui.Column(c.Key("main")).Grow(1).MinWidth(0).FillHeight().Children(func() { w.main(c, p) })
		if w.panel == 2 {
			dx, reset := vHandle(c, p, "ai-handle")
			w.panelW = min(max(w.panelW-dx, 280), 720)
			if reset {
				w.panelW = 340
			}
			ui.Column(c.Key("ai")).Width(w.panelW).FillHeight().Shrink(0).Children(func() { w.ai.view(c, p) })
		}
	})
}

// ---- tab history (⌘⌃← and ⌘⌃→) ----

// trackTabs remembers the tabs the user came from, for back and forward.
func (w *Workspace) trackTabs() {
	if w.active == w.lastActive {
		return
	}
	if !w.navigating {
		w.back = append(w.back, w.lastActive)
		w.fwd = nil
	}
	w.navigating = false
	w.lastActive = w.active
}

// goBack and goForward move through the tabs visited.
func (w *Workspace) goBack()    { w.go1(&w.back, &w.fwd) }
func (w *Workspace) goForward() { w.go1(&w.fwd, &w.back) }

func (w *Workspace) go1(from, to *[]int) {
	for len(*from) > 0 {
		i := (*from)[len(*from)-1]
		*from = (*from)[:len(*from)-1]
		if i >= 0 && i < len(w.tabs) && i != w.active {
			*to = append(*to, w.active)
			w.navigating = true
			w.active = i
			return
		}
	}
}

// ---- notifications and the console ----

type note struct {
	at   time.Time
	text string
	kind string // kindInfo、kindSuccess、kindWarning、kindError
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
	switch database.SQLLeadingVerb(sql) {
	case "insert", "update", "delete", "replace", "merge", "commit":
		return p.warning
	case "drop", "truncate", "alter", "create", "rename", "grant", "revoke":
		return p.danger
	}
	return p.fg
}

// logSQL adds a statement to the console log.
func (s *Shell) logSQL(db, sql string, dur time.Duration, rows int, err string) {
	s.console = append(s.console, consoleEntry{at: time.Now(), db: db, sql: strings.TrimSpace(sql), dur: dur, rows: rows, err: err})
	if len(s.console) > consoleMax {
		s.console = s.console[len(s.console)-consoleMax:]
	}
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

// tableSelectSQL is the query a page of a table's data amounts to, for the log.
func (t *tableTab) tableSelectSQL() string {
	q := fmt.Sprintf("SELECT * FROM %s", database.QuoteIdent(t.w.cfg.Type, t.table))
	var where strings.Builder
	for i, f := range t.applied {
		if i > 0 {
			where.WriteString(map[bool]string{true: " OR ", false: " AND "}[f.Or])
		}
		where.WriteString(fmt.Sprintf("%s %s '%s'", f.Column, f.Operator, f.Value))
	}
	if t.appliedRaw != "" {
		if where.Len() > 0 {
			where.WriteString(" AND ")
		}
		where.WriteString("(" + t.appliedRaw + ")")
	}
	if where.Len() > 0 {
		q += " WHERE " + where.String()
	}
	var order []string
	for _, s := range t.sortList() {
		order = append(order, database.QuoteIdent(t.w.cfg.Type, s.Column)+" "+s.Direction)
	}
	if len(order) > 0 {
		q += " ORDER BY " + strings.Join(order, ", ")
	}
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", q, t.size(), (t.page-1)*t.size())
}

// notesPopover 是通知中心：从铃铛按钮下面弹出的一张小面板（不是居中的对话框），
// 点面板外或按 Esc 收起；最新的通知在最上面。
func (s *Shell) notesPopover(c *ui.Context, p tokens, anchor ui.Element) {
	if !s.notesOpen {
		return
	}
	s.notesSeen = len(s.notes) // 面板开着时新来的通知也算看过
	ui.PopoverBase(c, anchor, &s.notesOpen, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomRight, ui.AnchorTopRight).Margin(6, 0, 0, 0).
			Width(380).MaxHeight(460).Background(p.surfaceElev).Border(1, p.border).Radius(10).
			Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.18)).Clip()
		ui.Column(c).FillWidth().Children(func() {
			ui.Row(c).Gap(6).Padding(7, 8, 6, 12).AlignItems(ui.Center).FillWidth().Children(func() {
				ui.Text(c, locales.NotesTitle()).FontSize(font2XS).FontWeight(600)
				if n := len(s.notes); n > 0 {
					ui.Text(c, fmt.Sprint(n)).FontSize(font2XS - 1).TextColor(p.fgMuted)
				}
				ui.Spacer(c)
				if actBtn(c, icTrash, locales.NotesClear()).Disabled(len(s.notes) == 0).Clicked() {
					s.notes = nil
				}
			})
			ui.Box(c).Height(1).FillWidth().Background(p.borderSubtle)
			if len(s.notes) == 0 {
				ui.Column(c).FillWidth().Padding(28, 12).Center().Gap(8).Children(func() {
					ui.Icon(c, icBell).FontSize(22).TextColor(p.fgMuted)
					ui.Text(c, locales.NotesEmpty()).TextColor(p.fgMuted).FontSize(font2XS)
				})
				return
			}
			scrollbars(c, ui.Scroll(c.Key("notes")).FillWidth().MaxHeight(410)).Children(func() {
				ui.Column(c).Gap(1).FillWidth().Padding(4).Children(func() {
					for i := len(s.notes) - 1; i >= 0; i-- {
						n := s.notes[i]
						kc := kindColor(p, n.kind)
						ui.Row(c.Key(fmt.Sprint("n", i))).Gap(10).Padding(7, 8).Radius(radiusBtn + 1).AlignItems(ui.Start).FillWidth().Children(func() {
							ui.Row(c).Size(20, 20).Shrink(0).Center().Radius(10).Background(kc.Alpha(0.14)).Children(func() {
								ui.Icon(c, kindIcon(n.kind)).FontSize(12).TextColor(kc)
							})
							ui.Text(c, n.text).FontSize(font2XS).Grow(1).MinWidth(0).Selectable()
							ui.Text(c, n.at.Format("15:04:05")).Font(mono).FontSize(font2XS - 1).TextColor(p.fgMuted).Shrink(0)
						})
					}
				})
			})
		})
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
			if ui.Segmented(c.Key("console-filter"), &mode, locales.ConsoleFilterAll(), locales.ConsoleFilterErrors()).FontSize(font2XS - 1).Changed() {
				s.consoleErrOnly = mode == 1
			}
			if tipBtn(c, icCopy, locales.ConsoleCopyAll()).Size(sizeBtnSm, sizeBtnSm).Disabled(len(shown) == 0).Clicked() {
				var b strings.Builder
				for _, i := range shown {
					e := s.console[i]
					fmt.Fprintf(&b, "-- %s %dms\n%s;\n", e.at.Format("15:04:05"), e.dur.Milliseconds(), strings.TrimRight(e.sql, "; \n"))
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
		meta := fmt.Sprintf("%dms", e.dur.Milliseconds())
		if e.dur >= time.Second {
			meta = fmt.Sprintf("%.1fs", e.dur.Seconds())
		}
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

// ---- pending changes: Discard / Preview / Commit ----

// pendingTable is the active table tab and how many changes it holds, data
// or structure.
func (s *Shell) pendingTable() (t *tableTab, n int) {
	t = s.activeTable()
	if t == nil {
		return nil, 0
	}
	switch t.mode {
	case 0:
		return t, t.pendingCount()
	case 1:
		return t, t.st.changeCount()
	}
	return t, 0
}

func (s *Shell) discardPending() {
	t, n := s.pendingTable()
	if t == nil || n == 0 {
		return
	}
	if t.mode == 0 {
		t.revert()
	} else {
		t.st.reset()
	}
}

func (s *Shell) previewPending() {
	t, n := s.pendingTable()
	if t == nil || n == 0 {
		return
	}
	if t.mode == 0 {
		t.buildPreview()
	} else {
		t.buildStructurePreview()
	}
	t.sqlOpen = true
}

func (s *Shell) commitPending() {
	t, n := s.pendingTable()
	// 正在输入的单元格还没算进待提交数，⌘S 要把它一并提交
	typing := t != nil && t.mode == 0 && t.es.editing != nil
	if t == nil || (n == 0 && !typing) {
		return
	}
	if t.mode == 0 {
		t.commitNow()
	} else {
		t.commitStructure()
	}
}

// pendingGroup is the toolbar's three-part control: Discard, Preview SQL and
// Commit, grey when nothing is pending.
func (s *Shell) pendingGroup(c *ui.Context, p tokens) {
	t, n := s.pendingTable()
	if t == nil {
		return
	}
	on := n > 0
	ui.Row(c).Gap(0).AlignItems(ui.Center).Height(sizeBtnSm+2).Radius(radiusBtn).Border(1, p.borderSubtle).Margin(0, 6, 0, 0).Children(func() {
		seg := func(key string, ic *ui.SVG, tip string, col ui.Color, label string) bool {
			b := tipOn(c, ui.Button(c.Key(key), "").Label(tip), tip).Height(sizeBtnSm).Padding(0, 7).BorderWidth(0).Background(ui.Color{}).Disabled(!on)
			if b.Hovered() && on {
				b.Background(p.hover)
			}
			b.Children(func() {
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, ic).FontSize(14).TextColor(col)
					if label != "" {
						ui.Text(c, label).FontSize(font2XS).FontWeight(600).TextColor(col)
					}
				})
			})
			return b.Clicked()
		}
		if seg("pg-discard", icUndo, locales.TableDiscard()+" (⌘⇧⌫)", p.fgSecondary, "") {
			s.discardPending()
		}
		ui.Box(c).Width(1).Height(14).Background(p.borderSubtle)
		if seg("pg-preview", icView, locales.TablePreviewSQL()+" (⌘⇧P)", p.fgSecondary, "") {
			s.previewPending()
		}
		ui.Box(c).Width(1).Height(14).Background(p.borderSubtle)
		label := ""
		if on {
			label = fmt.Sprint(n)
		}
		if seg("pg-commit", icCheck, locales.TableCommit()+" (⌘S)", p.accent, label) {
			s.commitPending()
		}
	})
}

// sqlPreviewDialog shows the SQL the pending changes would run (⌘⇧P); it only
// previews, committing stays with the button and ⌘S.
func (t *tableTab) sqlPreviewDialog(c *ui.Context) {
	if !t.sqlOpen {
		return
	}
	sh := t.w.sh
	p := sh.tokens(c)
	stmts := t.es.preview.stmts
	if t.mode == 1 {
		stmts = t.st.preview.stmts
	}
	sh.dialog(c, &t.sqlOpen, dlgOpts{title: locales.TablePreviewSQL(), sub: t.table, width: 720}, func() {
		if len(stmts) == 0 {
			ui.Text(c, locales.StructNoChanges()).TextColor(p.fgMuted)
		}
		dlgBox(c, p).FillWidth().Children(func() {
			scrollbars(c, ui.Scroll(c.Key("sqlprev")).Height(300).FillWidth()).Children(func() {
				ui.Column(c).Padding(10).Gap(6).FillWidth().Children(func() {
					for _, st := range stmts {
						ui.Text(c, st).Selectable().FontSize(font2XS).Font(mono)
					}
				})
			})
		})
	}, func() {
		if footBtn(c, locales.CommonClose()).Clicked() {
			t.sqlOpen = false
		}
		if footPrimary(c, locales.TableCommit()+" (⌘S)").Disabled(len(stmts) == 0).Clicked() {
			t.sqlOpen = false
			sh.commitPending()
		}
	})
}

// ---- the list of workspaces (⌘⇧\) ----

type wsPopup struct {
	open  bool
	query string
	sel   int
}

func (s *Shell) workspacesPopup(c *ui.Context) {
	d := &s.wsPop
	if !d.open {
		return
	}
	p := s.tokens(c)
	q := strings.ToLower(strings.TrimSpace(d.query))
	var idx []int
	for i, w := range s.spaces {
		if q == "" || strings.Contains(strings.ToLower(w.cfg.Name+" "+w.db), q) {
			idx = append(idx, i)
		}
	}
	d.sel = min(max(d.sel, 0), max(len(idx)-1, 0))
	choose := func() {
		if d.sel < len(idx) {
			s.active = idx[d.sel]
			d.open = false
		}
	}
	s.popup(c, &d.open, 460, &d.query, locales.WsAllTitle(), func() { d.sel = 0 }, choose, func(n int) { d.sel = min(max(d.sel+n, 0), max(len(idx)-1, 0)) }, func() {
		for k, i := range idx {
			w := s.spaces[i]
			row := ui.Row(c.Key(w.id())).Gap(10).Padding(7, 10).Radius(radiusBtn + 1).AlignItems(ui.Center).FillWidth()
			switch {
			case k == d.sel:
				row.Background(p.accent.Alpha(0.12))
			case row.Hovered():
				row.Background(p.hover)
			}
			row.Children(func() {
				dbTile(c, w.cfg.Type, 20, connColor(c, w.cfg.Color))
				ui.Text(c, w.cfg.Name).SingleLine().Grow(1)
				dbName := w.db
				if n := w.ordinal(); n > 1 {
					dbName = fmt.Sprintf("%s #%d", dbName, n)
				}
				ui.Text(c, dbName).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine()
				if i == s.active {
					ui.Icon(c, icCheck).FontSize(14).TextColor(p.accent)
				}
			})
			if row.Clicked() {
				d.sel = k
				choose()
			}
		}
	})
}

// closeOtherSpaces closes every workspace but w, asking first about changes.
func (s *Shell) closeOtherSpaces(keep *Workspace) {
	n := 0
	for _, w := range s.spaces {
		if w != keep {
			n += w.pendingTotal()
		}
	}
	s.discardGuard(n, func() {
		for i := len(s.spaces) - 1; i >= 0; i-- {
			if s.spaces[i] != keep {
				s.closeSpace(i)
			}
		}
		for i, w := range s.spaces {
			if w == keep {
				s.active = i
			}
		}
	})
}
