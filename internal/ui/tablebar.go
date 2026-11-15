package ui

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/locales"
)

// baseTokens 是当前配色与明暗下的令牌，给没拿到 Shell.tokens 的角落用（编辑框描边、开关按钮的选中态等）。
// 强调色固定，不跟随连接颜色，所以它和 Shell.tokens 取到的完全一致。
func baseTokens(c *ui.Context) tokens {
	return tokensFor(activeScheme, c.Theme().Dark)
}

func (t *tableTab) view(c *ui.Context, p tokens) {
	if t.preview && (t.pending() > 0 || t.mode != 0) {
		t.keep() // editing or looking at the structure makes it a real tab
	}
	t.topBar(c, p)
	// 内容区单独一层：待提交悬浮条相对它定位，不会压到下面的状态栏
	paneIn(ui.Column(c.Key(fmt.Sprint("tabcontent", t.mode)))).Grow(1).FillWidth().MinWidth(0).MinHeight(0).Children(func() { // 换视图时 key 跟着换，新内容淡入
		switch t.mode {
		case 1:
			t.structureView(c)
		case 2:
			t.ddlView(c)
		case 3:
			t.docView(c)
		default:
			t.dataView(c, p)
		}
		t.pendingFloat(c, p)
	})
}

func (t *tableTab) modeChanged() {
	switch t.mode {
	case 1:
		if !t.st.loaded && !t.st.loading {
			t.st.load(t)
		}
	case 2:
		if !t.ddl.loaded && !t.ddl.loading {
			t.ddl.load(t)
		}
	case 3:
		if !t.doc.loaded {
			t.doc.load(t)
		}
	}
}

func (t *tableTab) dataView(c *ui.Context, p tokens) {
	if !t.started { // restored tabs load when first shown
		t.load()
	}
	switch {
	case t.err != "":
		ui.Text(c, locales.TableLoadFail(t.err)).TextColor(p.danger).Padding(16).Selectable()
	case len(t.g.cols) == 0:
		// First load: keep the area still; nothing flashes for a quick load.
		quietLoading(c, p, &t.loadStart)
	default:
		ui.Column(c).Grow(1).FillWidth().MinWidth(0).Children(func() {
			// The grid keeps its place in the tree whether or not the row pane
			// is open, so its scroll position survives the pane opening: the
			// pane is only added at the right.
			ui.Row(c).Grow(1).FillWidth().MinWidth(0).Children(func() {
				ui.Column(c.Key("grid")).Grow(1).MinWidth(0).FillHeight().Children(func() { t.gridView(c) })
				if t.showPreview {
					t.previewHandle(c, p)
					ui.Column(c.Key("preview")).Width(t.previewW).FillHeight().Shrink(0).Children(func() { t.rowPreview(c, p) })
				}
			})
			// A reload keeps the rows in place; a thin bar over the top edge
			// shows it, after a moment, without moving anything.
			if t.loading && delayed(c, t.loadStart, 500*time.Millisecond) {
				ui.Progress(c, -1).Absolute().Top(0).Left(0).Right(0).Height(2)
			}
		})
	}
	t.columnsDialog(c)
	t.jumpView(c)
	t.quickDialog(c)
	t.quickLookDialog(c)
	t.sqlPreviewDialog(c)
	if c.Shortcut(0, ui.KeySpace) && t.es.editing == nil {
		t.togglePreview()
	}
}

// togglePreview shows or hides the row pane; opening it with no row chosen
// chooses the first, so the pane always has something to show.
func (t *tableTab) togglePreview() {
	t.showPreview = !t.showPreview
	if t.previewW == 0 {
		t.previewW = sizeRowPane
	}
	if t.showPreview && t.previewRow() < 0 && len(t.g.rows) > 0 {
		t.g.chosen.Add(0)
	}
}

// toggleFilter 打开或收起筛选浮层（⌘F）。收起只是收起：已生效的条件和没应用的草稿都留着，
// 要清掉用浮层里的「取消筛选」或摘掉工具条上的条件芯片。
func (t *tableTab) toggleFilter() {
	t.showFilter = !t.showFilter
	t.focusFilter = t.showFilter
}
