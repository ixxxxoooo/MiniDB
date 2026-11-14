package ui

import (
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"minidb/locales"
)

var requestSeq atomic.Int64

// The pages of the result area.
const (
	pageData = iota
	pageChart
	pageMessages
)

// queryTab is a SQL editor over a result area.
type queryTab struct {
	w *Workspace
	n int

	ed *editor.State

	results []*queryResult
	shown   int
	running bool
	reqID   string
	started time.Time
	failed  string

	splitPct float32
	fav      favCache // 收藏按钮针对的语句，见 favorites.go
	ia       inlineAI // 编辑器上方的内联 AI 栏，见 inlineai.go

	vars        paramDialog
	page        int  // pageData, pageChart or pageMessages
	hideResults bool // ⌃` folds the result area away

	// A split puts a second editor beside this one (⌘⇧D). The second is a
	// queryTab of its own that is not in the tab bar; parent is set on it.
	pane2  *queryTab
	parent *queryTab
	second bool      // on the root: the second pane has the keyboard
	want   *queryTab // on the root: the pane whose editor takes the focus next frame
	splitH float32
}

func (w *Workspace) newQuery(sql string) *queryTab {
	w.queryN++
	buf := editor.NewBuffer(sql)
	buf.SetLanguage(editor.LanguageNamed("SQL"))
	q := &queryTab{w: w, n: w.queryN, splitPct: 45}
	q.ed = &editor.State{Buffer: buf, SoftWrap: true}
	// Cmd+Enter runs; the editor's default (open a line below) steps aside.
	q.ed.Bindings = w.sh.queryBindings()
	q.ed.OnCompletion = q.complete
	w.tabs = append(w.tabs, q)
	w.active = len(w.tabs) - 1
	return q
}

// queryBindings steps the editor's own keys aside for the ones the tab handles.
func (s *Shell) queryBindings() []editor.Binding {
	var out []editor.Binding
	for _, k := range []chord{
		s.chordOf("run"), s.chordOf("run-all"), s.chordOf("format"), s.chordOf("inline-ai"),
		{ui.Cmd | ui.Shift, ui.KeyD}, {ui.Cmd | ui.Shift, ui.KeyI},
		{ui.Cmd | ui.Alt, ui.KeyBracketLeft}, {ui.Cmd | ui.Alt, ui.KeyBracketRight},
		{ui.Ctrl, ui.KeyBackquote},
	} {
		if k.valid() {
			out = append(out, editor.Binding{Mods: k.mods, Key: k.key, Command: editor.NoCommand})
		}
	}
	return out
}

// newPane makes the second editor of a split.
func (q *queryTab) newPane(sql string) *queryTab {
	w := q.w
	w.queryN++
	buf := editor.NewBuffer(sql)
	buf.SetLanguage(editor.LanguageNamed("SQL"))
	p := &queryTab{w: w, n: w.queryN, splitPct: 45, parent: q}
	p.ed = &editor.State{Buffer: buf, SoftWrap: true}
	p.ed.Bindings = w.sh.queryBindings()
	p.ed.OnCompletion = p.complete
	return p
}

// root is the tab in the tab bar: this one, or the one a split pane belongs to.
func (q *queryTab) root() *queryTab {
	if q.parent != nil {
		return q.parent
	}
	return q
}

// focusedPane is the pane with the keyboard: the second one after ⌘⇧D or a
// click in it, else the tab itself.
func (q *queryTab) focusedPane() *queryTab {
	if q.pane2 != nil && q.second {
		return q.pane2
	}
	return q
}

// isActive tells whether this pane takes the keyboard shortcuts.
func (q *queryTab) isActive() bool {
	r := q.root()
	return r.focusedPane() == q
}

// toggleSplit opens the second pane to the right, or closes it.
func (q *queryTab) toggleSplit() {
	if q.pane2 != nil {
		q.pane2, q.second, q.want = nil, false, q
		return
	}
	q.pane2, q.splitH = q.newPane(""), 50
	q.second = true
	q.want = q.pane2
}

// switchPane moves the keyboard to the other pane (⌘⌥[ and ⌘⌥]).
func (q *queryTab) switchPane() {
	if q.pane2 != nil {
		q.second = !q.second
		q.want = q.focusedPane()
	}
}

func (q *queryTab) title() string       { return locales.TabQuery(q.n) }
func (q *queryTab) icon() *ui.SVG       { return objectIcon(q.objKind()) }
func (q *queryTab) objKind() ObjectKind { return KindQuery }
func (q *queryTab) pending() int        { return 0 }

// view draws the tab: one editor with its results, or two side by side.
func (q *queryTab) view(c *ui.Context, p tokens) {
	if q.pane2 == nil {
		q.body(c, p)
		return
	}
	// 左栏里可能处理 ⌘⇧D 取消分屏而把 q.pane2 清空，右栏要用这一帧开头取到的那个
	pane2 := q.pane2
	ui.SplitPercent(c, &q.splitH, func() {
		ui.Column(c.Key("pane1")).Fill().Children(func() { q.body(c, p) })
	}, func() {
		ui.Column(c.Key("pane2")).Fill().Children(func() { pane2.body(c, p) })
	}).Grow(1).FillWidth()
}

func (q *queryTab) body(c *ui.Context, p tokens) {
	ui.Toolbar(c, func() {
		ui.Row(c).Gap(4).Padding(0, 8).AlignItems(ui.Center).FillWidth().Height(sizeBtn + 4).Shrink(0).Children(func() {
			if q.running {
				if actBtn(c, icStop, locales.QueryStop()).Clicked() {
					q.stop()
				}
				ui.Spinner(c)
			} else if tipOn(c, actPrimary(c, icPlay, locales.QueryRun()), locales.QueryRunCurrent()+q.w.sh.keyHint("run")).Clicked() {
				q.run(false)
			}
			if tipBtn(c, icPlayAll, locales.QueryRunAll()).Size(sizeBtnSm, sizeBtnSm).Disabled(q.running).Clicked() {
				q.run(true)
			}
			vline(c, p.border, 12)
			if tipBtn(c, icFormat, locales.QueryFormat()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.format()
			}
			if tipBtn(c, icUglify, locales.QueryUglify()+" (⌘⇧I)").Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.uglify()
			}
			// 收藏当前语句：已收藏时是实心的黄色星标
			if _, faved := q.favoriteItem(); faved {
				if favBtn(c, icStarFill, locales.QueryUnfavorite(), p.warning).Clicked() {
					q.toggleFavorite()
				}
			} else if favBtn(c, icStar, locales.QueryFavorite(), p.fgSecondary).Clicked() {
				q.toggleFavorite()
			}
			if toggleBtn(c, icSparkle, locales.QueryAiInline()+q.w.sh.keyHint("inline-ai"), q.ia.open).Clicked() {
				q.inline().toggle()
			}
			ui.Spacer(c)
			if q.running {
				ui.Text(c, locales.QueryRunning()).TextColor(p.fgMuted).FontSize(font2XS)
			}
			if q.parent == nil && tipBtn(c.Key("split"), icSplit, locales.QuerySplit()+" (⌘⇧D)").Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.toggleSplit()
			}
			if q.parent != nil && tipBtn(c.Key("unsplit"), icX, locales.QueryCloseSplit()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.parent.toggleSplit()
			}
			if tipBtn(c.Key("edopts"), icSettings, locales.EditorOptions()).Size(sizeBtnSm, sizeBtnSm).Clicked() {
				q.w.sh.edOpt = true
			}
		})
	}).Padding(0)
	hairline(c, p.border)

	// 字号、换行、行号、缩进等由设置决定，所有查询编辑器共用
	q.w.sh.applyEditorPrefs(q.ed)
	edView := func() {
		q.inline().view(c)
		editorBars(c, q.ed)
		// 编辑器要有固定的键：上面的内联 AI 栏出现或消失时它在列里的位置会变，
		// 没有键就会被当成新控件，AutoFocus 又把焦点抢回编辑器
		e := editor.View(c.Key("sql-editor"), q.ed).Grow(1).FillWidth()
		if r := q.root(); r.want == q {
			r.want = nil
			e.Focus()
		} else if q.pane2 == nil && q.parent == nil {
			e.AutoFocus()
		}
		if e.Focused() {
			q.root().second = q.parent != nil
		}
	}
	// 内联 AI 栏在编辑器上方，两者放进同一列，编辑器吃掉栏以外的高度
	edPane := func() { ui.Column(c).Fill().Children(edView) }
	if q.hideResults {
		edPane()
	} else {
		ui.SplitVerticalPercent(c, &q.splitPct, edPane, func() { q.resultsView(c) }).Grow(1).FillWidth()
	}

	q.paramsView(c)
	if !q.isActive() || q.vars.open {
		return
	}

	switch {
	case q.w.sh.hit(c, "run-all"):
		q.runDefault(true)
	case q.w.sh.hit(c, "run"):
		q.runDefault(false)
	case q.w.sh.hit(c, "format"):
		q.format()
	case q.w.sh.hit(c, "inline-ai"):
		q.inline().toggle()
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyI):
		q.uglify()
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyD):
		q.root().toggleSplit()
	case c.Shortcut(ui.Cmd|ui.Alt, ui.KeyBracketLeft), c.Shortcut(ui.Cmd|ui.Alt, ui.KeyBracketRight):
		q.root().switchPane()
	case c.Shortcut(ui.Ctrl, ui.KeyBackquote):
		q.hideResults = !q.hideResults
	}
}
