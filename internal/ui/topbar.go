package ui

import (
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/locales"
)

// indexView 是工作区当前库的 schema 索引状态，顶栏的索引芯片读它。
type indexView struct {
	db string // 状态属于哪个库；库切换后重新取
	st app.SchemaStatus
}

// subscribeIndex 订阅后台索引的状态变化：状态一变，就重取对应库的最新状态。
func (s *Shell) subscribeIndex() {
	s.subscribe(app.Subscribe(s.app.Bus, app.TopicSchemaIndex, func(ev app.SchemaStatusEvent) {
		s.up.post(func() {
			for _, w := range s.spaces {
				if w.idx.db != "" && w.idx.db == ev.Status.DatabaseName {
					w.pollIndex()
				}
			}
			s.redraw()
		})
	}))
}

// pollIndex 在后台取当前库的索引状态；还没建过索引时顺手预热一次。
func (w *Workspace) pollIndex() {
	db := w.db
	var st app.SchemaStatus
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

// indexMinSpin 是重新索引时转圈至少显示的时间。
const indexMinSpin = 700 * time.Millisecond

// indexChipMinW 是索引芯片的最小宽度：状态切换（转圈、换文案）时芯片宽度不变，右对齐的工具栏就不会跟着抖。
const indexChipMinW = 84

// reindex 手动重建索引。
func (w *Workspace) reindex() {
	db := w.db
	w.idx.st.Refreshing = true
	var st app.SchemaStatus
	var err error
	w.sh.up.bg(func() {
		start := time.Now()
		st, err = w.sh.app.SchemaIndex.RefreshSchemaIndex(w.cfg.ID, db)
		// 小库一瞬间就建完了，转圈至少停留一会儿，否则芯片一闪而过，看上去像抖了一下
		if d := indexMinSpin - time.Since(start); d > 0 {
			time.Sleep(d)
		}
	}, func() {
		if w.db != db {
			return
		}
		if err != nil {
			w.idx.st.Refreshing, w.idx.st.LastError = false, errMessage(err)
			w.sh.toastErr(errMessage(err))
			return
		}
		w.idx.st = st
		w.sh.toastOK(locales.IdxDone(st.TableCount)) // 点完要有个明确的结果反馈
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
		// 已经有索引时文案保持"N 张表"不变，只让指示点变成转圈；从没建过才显示"索引中…"
		state = locales.IdxBuilding()
		if !st.Exists {
			label = locales.IdxBuilding()
		}
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
	chip := tipOn(c, ui.Row(c.Key("index-chip")).Gap(5).AlignItems(ui.Center).Justify(ui.Center).Padding(0, 8).MinWidth(indexChipMinW).Height(sizeBtnSm).Radius(999), tip)
	if chip.Hovered() {
		chip.Background(p.hover)
	}
	chip.Children(func() {
		// 指示位固定 12×12，转圈和圆点互相替换时不改变布局
		ui.Row(c.Key("index-slot")).Size(12, 12).Shrink(0).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			if st.Refreshing {
				ui.Spinner(c).Size(10, 10)
			} else {
				ui.Box(c).Size(6, 6).Radius(3).Background(dot)
			}
		})
		ui.Text(c, label).FontSize(font2XS - 1).TextColor(p.fgSecondary).SingleLine()
	})
	if chip.Clicked() && !st.Refreshing {
		w.reindex()
	}
}

// indexWhen 是索引上次更新距今的说法。
func indexWhen(st app.SchemaStatus) string {
	t, err := time.Parse(time.RFC3339, st.LastRefreshedAt)
	if err != nil {
		return "—"
	}
	return relativeTime(t, time.Now())
}
