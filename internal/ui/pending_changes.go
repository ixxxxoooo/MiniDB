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
	sh.dialog(c, &t.sqlOpen, dlgOpts{title: locales.TablePreviewSQL(), width: 560, extra: func() {
		if len(stmts) > 0 {
			chip(c, chipWarn, locales.TablePreviewCount(itoa(len(stmts))), false)
		}
	}}, func() {
		if len(stmts) == 0 {
			ui.Text(c, locales.StructNoChanges()).TextColor(p.fgMuted)
		}
		dlgBox(c, p).FillWidth().Children(func() {
			scrollbars(c, ui.Scroll(c.Key("sqlprev")).MaxHeight(300).FillWidth()).Children(func() {
				ui.Column(c).Padding(spaceMD, spaceMD+2).Gap(spaceXS).FillWidth().Children(func() {
					for _, st := range stmts {
						ui.RichText(c, sqlSpans(p, st, p.fg)...).Selectable().FontSize(font2XS).Font(mono)
					}
				})
			})
		})
	}, func() {
		ui.Icon(c, icLock).FontSize(btnActIcon).TextColor(p.fgMuted)
		ui.Text(c, locales.TablePreviewLogHint()).FontSize(fontCaption).TextColor(p.fgMuted).SingleLine().Grow(1).MinWidth(0)
		if footBtn(c.Key("sqlprev-copy"), locales.CommonCopy()).Disabled(len(stmts) == 0).Clicked() {
			c.Services().WriteClipboard(strings.Join(stmts, "\n"))
		}
		if footPrimary(c.Key("sqlprev-close"), locales.CommonClose()).Clicked() {
			t.sqlOpen = false
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
