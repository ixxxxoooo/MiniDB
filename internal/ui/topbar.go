package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/schemaindex"
	"minidb/locales"
)

// indexView 是工作区当前库的 schema 索引状态，顶栏的索引芯片读它。
type indexView struct {
	db string // 状态属于哪个库；库切换后重新取
	st schemaindex.Status
}

// subscribeIndex 订阅后台索引的状态变化：状态一变，就重取对应库的最新状态。
func (s *Shell) subscribeIndex() {
	s.app.Bus.On(app.TopicSchemaIndex, func(p any) {
		ev, ok := p.(schemaindex.StatusEvent)
		if !ok {
			return
		}
		s.up.post(func() {
			for _, w := range s.spaces {
				if w.idx.db != "" && w.idx.db == ev.Status.DatabaseName {
					w.pollIndex()
				}
			}
			s.redraw()
		})
	})
}

// pollIndex 在后台取当前库的索引状态；还没建过索引时顺手预热一次。
func (w *Workspace) pollIndex() {
	db := w.db
	var st schemaindex.Status
	var err error
	w.sh.up.bg(func() { st, err = w.sh.app.SchemaIndex.GetSchemaIndexStatus(w.cfg.ID, db) }, func() {
		if err != nil || w.db != db {
			return
		}
		w.idx.db, w.idx.st = db, st
		if !st.Exists && !st.Refreshing {
			w.sh.app.SchemaIndex.WarmSchemaIndex(w.cfg.ID, db)
		}
	})
}

// reindex 手动重建索引。
func (w *Workspace) reindex() {
	db := w.db
	w.idx.st.Refreshing = true
	var st schemaindex.Status
	var err error
	w.sh.up.bg(func() { st, err = w.sh.app.SchemaIndex.RefreshSchemaIndex(w.cfg.ID, db) }, func() {
		if w.db != db {
			return
		}
		if err != nil {
			w.idx.st.Refreshing, w.idx.st.LastError = false, err.Error()
			return
		}
		w.idx.st = st
	})
}

// indexChip 是顶栏上的索引芯片：一个状态点加表数，悬停看详情，点一下重新索引。
func (s *Shell) indexChip(c *ui.Context, p tokens, w *Workspace) {
	if w.db == "" {
		return
	}
	if w.idx.db != w.db { // 换了库：清掉旧状态并重新取
		w.idx = indexView{db: w.db}
		w.pollIndex()
	}
	st := w.idx.st
	dot, label := p.success, locales.IdxTables(st.TableCount)
	state := ""
	switch {
	case st.Refreshing:
		label = locales.IdxBuilding()
	case st.LastError != "":
		dot, state = p.danger, locales.IdxStateError(st.LastError)
	case !st.Exists:
		dot, label, state = p.fgMuted, locales.IdxNone(), locales.IdxStateNone()
	case st.Stale || st.Dirty:
		dot, state = p.warning, locales.IdxStateStale(indexWhen(st))
	default:
		state = locales.IdxStateFresh(indexWhen(st))
	}
	tip := locales.IdxTitle() + "\n" + state + "\n" + locales.IdxHint()
	chip := tipOn(c, ui.Row(c.Key("index-chip")).Gap(5).AlignItems(ui.Center).Padding(0, 8).Height(sizeBtnSm).Radius(999), tip)
	if chip.Hovered() {
		chip.Background(p.hover)
	}
	chip.Children(func() {
		if st.Refreshing {
			ui.Spinner(c).Size(10, 10)
		} else {
			ui.Box(c).Size(6, 6).Radius(3).Background(dot)
		}
		ui.Text(c, label).FontSize(font2XS - 1).TextColor(p.fgSecondary).SingleLine()
	})
	if chip.Clicked() && !st.Refreshing {
		w.reindex()
	}
}

// indexWhen 是索引上次更新距今的说法。
func indexWhen(st schemaindex.Status) string {
	t, err := time.Parse(time.RFC3339, st.LastRefreshedAt)
	if err != nil {
		return "—"
	}
	return relativeTime(t, time.Now())
}
