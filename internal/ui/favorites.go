package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"minidb/internal/app"
	"minidb/internal/sqltext"
	"minidb/locales"
)

// favKey 标识"编辑器此刻的状态"：缓冲区版本、光标与选区。三者都没变，收藏目标就没变。
type favKey struct {
	ver        uint64
	cur        int
	start, end int
}

// favCache 缓存当前收藏目标，避免每帧都把整个脚本重新切分一遍。
type favCache struct {
	key favKey
	sql string
	ok  bool
}

// favoriteTarget 是收藏按钮针对的语句：选中的文字；没有选中就取光标所在的那一条；
// 光标不在任何语句上（比如空行）就取整个编辑器内容。
func (q *queryTab) favoriteTarget() string {
	sel := q.ed.Selection()
	key := favKey{ver: q.ed.Buffer.Version(), cur: q.ed.Cursor(), start: sel.Start(), end: sel.End()}
	if q.fav.ok && q.fav.key == key {
		return q.fav.sql
	}
	sql := strings.TrimSpace(q.ed.SelectedText())
	if sql == "" {
		src := q.ed.Buffer.String()
		if st, ok := sqltext.At(src, q.ed.Cursor()); ok {
			sql = strings.TrimSpace(st.Text)
		} else {
			sql = strings.TrimSpace(src)
		}
	}
	q.fav = favCache{key: key, sql: sql, ok: true}
	return sql
}

// favoriteItem 返回收藏目标对应的收藏条目；它不是收藏时 ok 为 false。
func (q *queryTab) favoriteItem() (app.HistoryItem, bool) {
	sql := q.favoriteTarget()
	if sql == "" {
		return app.HistoryItem{}, false
	}
	it, ok := q.w.favs[sql]
	return it, ok
}

// toggleFavorite 收藏或取消收藏目标语句：已收藏则取消；否则问一个名字再收藏。
func (q *queryTab) toggleFavorite() {
	w := q.w
	sql := q.favoriteTarget()
	if sql == "" {
		w.sh.toastWarn(locales.QueryFavoriteEmpty())
		return
	}
	if it, ok := w.favs[sql]; ok {
		var err error
		w.sh.up.bg(func() { err = w.sh.app.History.ToggleFavorite(it.ID) }, func() {
			if err != nil {
				w.sh.toastErr(errMessage(err))
				return
			}
			w.sh.toastOK(locales.QueryUnfavorited())
			w.favoritesChanged()
		})
		return
	}
	dbName := w.db
	w.sh.prompt(promptDialog{title: locales.QueryFavoriteTitle(), sub: firstLines(sql, 1), label: locales.QueryFavoriteName(),
		value: defaultFavoriteName(sql), ok: locales.CommonConfirm(),
		action: func(name string, _ bool, _ string) {
			var err error
			w.sh.up.bg(func() { _, err = w.sh.app.History.SaveFavorite(w.cfg.ID, dbName, name, sql) }, func() {
				if err != nil {
					w.sh.toastErr(errMessage(err))
					return
				}
				w.sh.toastOK(locales.QueryFavorited(name))
				w.favoritesChanged()
			})
		}})
}

// favoritesChanged 在收藏有增减后刷新补全关键字、收藏表与侧栏。
func (w *Workspace) favoritesChanged() {
	w.loadKeywords()
	w.loadSideQueries()
	for _, t := range w.tabs {
		if qt, ok := t.(*queryTab); ok {
			qt.fav = favCache{}
		}
	}
}

// defaultFavoriteName 给收藏起个默认名：优先用语句上方 "-- name: xxx" 注释里的名字，
// 否则取第一行压成一行后的前 40 个字。
func defaultFavoriteName(sql string) string {
	if n := sqltext.ResultName(sql); n != "" {
		return n
	}
	name := strings.Join(strings.Fields(strings.SplitN(strings.TrimSpace(sql), "\n", 2)[0]), " ")
	if utf8.RuneCountInString(name) > 40 {
		name = string([]rune(name)[:40]) + "…"
	}
	return name
}

// favBtn 是工具栏上的收藏按钮：与 tipBtn 同尺寸，图标颜色由调用方决定（已收藏为黄色）。
func favBtn(c *ui.Context, ic *ui.SVG, tip string, col ui.Color) ui.Element {
	b := tipOn(c, ui.Button(c.Key("fav"), "").Label(tip), tip).Size(sizeBtnSm, sizeBtnSm).Padding(0).BorderWidth(0).Background(ui.Color{})
	if b.Hovered() {
		b.Background(baseTokens(c).hover)
	}
	return b.Children(func() { ui.Icon(c, ic).FontSize(14).TextColor(col) })
}
