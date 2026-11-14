package ui

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"minidb/internal/app"
	"minidb/locales"
	"strings"
)

// tableSelectSQL is the query a page of a table's data amounts to, for the log.
func (t *tableTab) tableSelectSQL() string {
	q := fmt.Sprintf("SELECT * FROM %s", app.QuoteIdent(t.w.cfg.Type, t.table))
	var where strings.Builder
	for i, f := range t.applied {
		if i > 0 {
			where.WriteString(map[bool]string{true: " OR ", false: " AND "}[f.Or])
		}
		where.WriteString(app.QuoteIdent(t.w.cfg.Type, f.Column) + " " + f.Operator + " " + app.QuoteString(t.w.cfg.Type, f.Value))
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
		order = append(order, app.QuoteIdent(t.w.cfg.Type, s.Column)+" "+s.Direction)
	}
	if len(order) > 0 {
		q += " ORDER BY " + strings.Join(order, ", ")
	}
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", q, t.size(), (t.page-1)*t.size())
}

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
		s.commitTxIfActive() // 没有表格修改可提交时，⌘S 提交手动事务
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

// pendingTotal 是工作区所有标签页里没提交的修改总数。
func (w *Workspace) pendingTotal() int {
	n := 0
	for _, t := range w.tabs {
		n += t.pending()
	}
	return n
}
