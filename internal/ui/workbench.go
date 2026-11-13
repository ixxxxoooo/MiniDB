package ui

import (
	"fmt"
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
	h := ui.Box(c.Key(key)).Width(5).FillHeight().Shrink(0).Cursor(ui.CursorResizeEW).Background(ui.Transparent)
	if x, _, ok := h.Dragged(); ok {
		dx = x
	}
	reset = h.DoubleClicked()
	line := p.border
	if h.Hovered() {
		line = p.accent
	}
	h.Draw(func(pt *ui.Painter, r ui.Rect) { pt.Fill(ui.Rect{X: r.X + r.W/2 - 0.5, Y: r.Y, W: 1, H: r.H}, line, 0) })
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
}

type consoleEntry struct {
	at   time.Time
	db   string
	sql  string
	dur  time.Duration
	rows int
	err  string
}

// logSQL adds a statement to the console log.
func (s *Shell) logSQL(db, sql string, dur time.Duration, rows int, err string) {
	s.console = append(s.console, consoleEntry{at: time.Now(), db: db, sql: strings.TrimSpace(sql), dur: dur, rows: rows, err: err})
	if len(s.console) > 500 {
		s.console = s.console[len(s.console)-500:]
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
	var conds []string
	for _, f := range t.applied {
		conds = append(conds, fmt.Sprintf("%s %s '%s'", f.Column, f.Operator, f.Value))
	}
	if t.appliedRaw != "" {
		conds = append(conds, "("+t.appliedRaw+")")
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	if t.sorted && t.sortOrder.Column != "" {
		dir := "ASC"
		if t.sortOrder.Descending {
			dir = "DESC"
		}
		q += fmt.Sprintf(" ORDER BY %s %s", database.QuoteIdent(t.w.cfg.Type, t.sortOrder.Column), dir)
	}
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", q, t.size(), (t.page-1)*t.size())
}

func (s *Shell) notesDialog(c *ui.Context) {
	if !s.notesOpen {
		return
	}
	p := s.tokens(c)
	s.dialog(c, &s.notesOpen, dlgOpts{title: locales.NotesTitle(), width: 460, height: 420, extra: func() {
		if ui.Button(c, locales.NotesClear()).Height(sizeBtnSm).FontSize(font2XS).Disabled(len(s.notes) == 0).Clicked() {
			s.notes = nil
		}
	}}, func() {
		if len(s.notes) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Gap(8).Children(func() {
				ui.Icon(c, icBell).FontSize(26).TextColor(p.fgMuted)
				ui.Text(c, locales.NotesEmpty()).TextColor(p.fgMuted)
			})
			return
		}
		ui.Scroll(c.Key("notes")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).Gap(1).FillWidth().Children(func() {
				for i := len(s.notes) - 1; i >= 0; i-- {
					n := s.notes[i]
					ui.Row(c.Key(fmt.Sprint("n", i))).Gap(10).Padding(7, 10).AlignItems(ui.Start).FillWidth().Children(func() {
						ui.Text(c, n.at.Format("15:04:05")).Font(mono).FontSize(font2XS - 1).TextColor(p.fgMuted).Shrink(0)
						ui.Text(c, n.text).FontSize(font2XS + 1).Grow(1).Selectable()
					})
				}
			})
		})
	}, nil)
}

// consolePanel is the log of executed SQL at the foot of the window (⌃⌘C).
func (s *Shell) consolePanel(c *ui.Context, p tokens) {
	if !s.consoleOpen {
		return
	}
	hairline(c, p.border)
	ui.Column(c.Key("console")).Height(190).FillWidth().Shrink(0).Background(p.surface).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(0, 8, 0, 12).Height(26).FillWidth().Shrink(0).Background(p.toolbar).Children(func() {
			ui.Icon(c, icConsole).FontSize(13).TextColor(p.fgSecondary)
			ui.Text(c, locales.ConsoleTitle()).FontSize(font2XS).FontWeight(600)
			ui.Spacer(c)
			if tipBtn(c, icTrash, locales.NotesClear()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				s.console = nil
			}
			if tipBtn(c, icX, locales.CommonClose()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				s.consoleOpen = false
			}
		})
		hairline(c, p.borderSubtle)
		if len(s.console) == 0 {
			ui.Text(c, locales.ConsoleEmpty()).FontSize(font2XS).TextColor(p.fgMuted).Padding(12)
			return
		}
		ui.Scroll(c.Key("consolelog")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).Padding(4, 0).FillWidth().Children(func() {
				for i, e := range s.console {
					e := e
					ui.Row(c.Key(fmt.Sprint("l", i))).Gap(10).Padding(2, 12).AlignItems(ui.Start).FillWidth().Children(func() {
						ui.Text(c, e.at.Format("15:04:05")).Font(mono).FontSize(font2XS - 1).TextColor(p.fgMuted).Shrink(0)
						ui.Text(c, e.sql).Font(mono).FontSize(font2XS).TextColor(map[bool]ui.Color{true: p.danger, false: p.fg}[e.err != ""]).Grow(1).Selectable()
						meta := fmt.Sprintf("%dms", e.dur.Milliseconds())
						if e.err != "" {
							meta = e.err
						} else if e.rows >= 0 {
							meta += fmt.Sprintf(" · %d", e.rows)
						}
						ui.Text(c, meta).FontSize(font2XS - 1).TextColor(p.fgMuted).MaxWidth(240).SingleLine().Shrink(0)
					})
				}
			})
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
			b := ui.Button(c.Key(key), "").Label(tip).Tooltip(tip).Height(sizeBtnSm).Padding(0, 7).BorderWidth(0).Background(ui.Color{}).Disabled(!on)
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
			ui.Scroll(c.Key("sqlprev")).Height(300).FillWidth().Children(func() {
				ui.Column(c).Padding(10).Gap(6).FillWidth().Children(func() {
					for _, st := range stmts {
						ui.Text(c, st).Selectable().FontSize(font2XS).Font(mono)
					}
				})
			})
		})
	}, func() {
		if ui.Button(c, locales.CommonClose()).Height(sizeBtn).Clicked() {
			t.sqlOpen = false
		}
		if ui.PrimaryButton(c, locales.TableCommit()+" (⌘S)").Height(sizeBtn).Disabled(len(stmts) == 0).Clicked() {
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
				ui.Text(c, w.db).FontSize(font2XS - 1).TextColor(p.fgMuted).SingleLine()
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
