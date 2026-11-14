package ui

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"minidb/internal/model"
	"minidb/locales"
)

// ER 图标签：一个库的表是节点、外键是边。库大时默认只显示一张表和它周围几层的表，可切到整库。
// 数据与布局的逻辑在这里，绘制和鼠标在 er_view.go，纯布局算法在 er_layout.go。

// 默认的显示范围：不超过 erAllMax 张表就整库显示；超过时显示度数最大的表和它周围一层。
const (
	erAllMax  = 60  // 整库显示的上限
	erWarnMax = 300 // 整库显示超过它时给出提示
)

// erSaved 是一个库的 ER 图布局：用户拖过的表的位置。
type erSaved struct {
	Pos map[string][2]float32 `json:"pos,omitempty"`
}

// erTab 是 ER 图标签；每个工作区最多一个。
type erTab struct {
	w       *Workspace
	full    *model.ERGraph
	shown   *model.ERGraph
	rects   map[string]erRect // 显示的表的世界坐标
	manual  map[string][2]float32
	focus   string // 中心表；空表示整库
	depth   int    // 向外几层，-1 是整库
	q       string
	notice  string // 为什么只显示了一部分
	loading bool
	err     string
	shownAt time.Time

	panX, panY, zoom float32 // 视图：屏幕 = (世界 - pan) * zoom
	vw, vh           float32 // 画布大小，绘制时更新
	needFit          bool
	hover, sel       string
	press            erPress
}

// erPress 是鼠标按下之后的拖动状态：按在表上就是拖表，按在空白处就是平移画布。
type erPress struct {
	down, moved bool
	node        string
	x, y        float32
}

func (e *erTab) title() string       { return locales.ErTitle() }
func (e *erTab) icon() *ui.SVG       { return icLink }
func (e *erTab) objKind() ObjectKind { return KindTable }
func (e *erTab) pending() int        { return 0 }

// openER 打开 ER 图标签，已经开着就切过去。focus 不为空时以这张表为中心显示它周围一层。
func (w *Workspace) openER(focus string) {
	for i, t := range w.tabs {
		if e, ok := t.(*erTab); ok {
			w.active = i
			if focus != "" && e.full != nil {
				e.setFocus(focus, 1)
			}
			return
		}
	}
	e := &erTab{w: w, zoom: 1, depth: -1, focus: focus}
	if focus != "" {
		e.depth = 1
	}
	w.tabs = append(w.tabs, e)
	w.active = len(w.tabs) - 1
	e.load()
}

// load 在后台读整库的关系图和保存的表位置。
func (e *erTab) load() {
	w, db := e.w, e.w.db
	var g *model.ERGraph
	var err error
	var saved erSaved
	key := w.dbKey()
	e.loading, e.err = true, ""
	w.sh.up.bg(func() {
		g, err = w.sh.app.ER.Graph(context.Background(), w.cfg.ID, db)
		if w.sh.app.UIState.LoadER(key, &saved) != nil {
			saved = erSaved{}
		}
	}, func() {
		e.loading = false
		if err != nil {
			e.err = errMessage(err)
			return
		}
		e.full, e.manual = g, saved.Pos
		e.chooseScope()
		e.rebuild()
	})
}

// chooseScope 定下一开始显示哪些表：指定了中心表就用它；否则表不多就整库，多了取度数最大的表。
func (e *erTab) chooseScope() {
	n := len(e.full.Nodes)
	switch {
	case e.focus != "" && e.hasNode(e.focus):
		return
	case n <= erAllMax:
		e.focus, e.depth = "", -1
	default:
		deg, best := e.full.Degree(), ""
		for _, nd := range e.full.Nodes { // 节点已排序，度数相同取名字靠前的
			if best == "" || deg[nd.Name] > deg[best] {
				best = nd.Name
			}
		}
		e.focus, e.depth = best, 1
	}
}

func (e *erTab) hasNode(name string) bool {
	_, ok := e.full.Node(name)
	return ok
}

// setFocus 以某张表为中心显示它向外 depth 层（-1 为整库）。
func (e *erTab) setFocus(name string, depth int) {
	e.focus, e.depth, e.sel = name, depth, name
	if depth < 0 {
		e.focus = ""
	}
	e.rebuild()
}

// rebuild 按当前范围取子图、布局，并让视图适应它。
func (e *erTab) rebuild() {
	if e.full == nil {
		return
	}
	e.shown = e.full.Neighborhood(e.focus, e.depth)
	e.rects = erLayout(e.shown)
	for name, p := range e.manual { // 用户拖过的位置盖过自动布局
		if r, ok := e.rects[name]; ok {
			r.X, r.Y = p[0], p[1]
			e.rects[name] = r
		}
	}
	e.notice = ""
	switch {
	case e.focus != "" && len(e.full.Nodes) > erAllMax:
		e.notice = locales.ErNoticeScope(len(e.full.Nodes), e.focus)
	case e.focus == "" && len(e.full.Nodes) > erWarnMax:
		e.notice = locales.ErNoticeBig(len(e.full.Nodes))
	}
	e.hover = ""
	if e.sel != "" && e.rects[e.sel] == (erRect{}) {
		e.sel = ""
	}
	e.needFit = true
}

// relayout 丢掉用户拖过的位置，回到自动布局。
func (e *erTab) relayout() {
	e.manual = nil
	e.saveLayout()
	e.rebuild()
}

// saveLayout 后台保存用户拖过的位置。
func (e *erTab) saveLayout() {
	w, key := e.w, e.w.dbKey()
	st := erSaved{Pos: map[string][2]float32{}}
	for k, v := range e.manual {
		st.Pos[k] = v
	}
	w.sh.up.bg(func() { _ = w.sh.app.UIState.SaveER(key, st) }, func() {})
}

// search 按输入找一张表，把视图中心对过去；它不在当前范围里就以它为中心重取。
func (e *erTab) search() {
	q := strings.ToLower(strings.TrimSpace(e.q))
	if q == "" || e.full == nil {
		return
	}
	best := ""
	for _, n := range e.full.Nodes {
		name := strings.ToLower(n.Name)
		if name == q {
			best = n.Name
			break
		}
		if best == "" && strings.Contains(name, q) {
			best = n.Name
		}
	}
	if best == "" {
		return
	}
	if _, in := e.rects[best]; !in {
		d := e.depth
		if d < 0 {
			d = 1
		}
		e.setFocus(best, d)
		return
	}
	e.sel = best
	e.centerOn(best)
}

// centerOn 把视图中心移到某张表上，缩放不变。
func (e *erTab) centerOn(name string) {
	if r, ok := e.rects[name]; ok && e.vw > 0 {
		e.panX, e.panY = r.X+r.W/2-e.vw/(2*e.zoom), r.Y+r.H/2-e.vh/(2*e.zoom)
	}
}

// fit 让所有显示的表恰好放进画布（放大不超过 1.2 倍）。
func (e *erTab) fit() {
	b, ok := erBounds(e.rects)
	if !ok || e.vw <= 0 || e.vh <= 0 {
		return
	}
	const pad = 40
	z := min((e.vw-2*pad)/b.W, (e.vh-2*pad)/b.H)
	e.zoom = min(max(z, 0.05), 1.2)
	e.panX = b.X + b.W/2 - e.vw/(2*e.zoom)
	e.panY = b.Y + b.H/2 - e.vh/(2*e.zoom)
}

// zoomAt 以画布上的一点为中心缩放，那一点下面的内容不动。
func (e *erTab) zoomAt(x, y, factor float32) {
	nz := min(max(e.zoom*factor, 0.05), 4)
	wx, wy := x/e.zoom+e.panX, y/e.zoom+e.panY
	e.zoom = nz
	e.panX, e.panY = wx-x/nz, wy-y/nz
}

// nodeAt 是世界坐标 (x, y) 处的表，没有返回空；后画的在上面，所以从后往前找。
func (e *erTab) nodeAt(x, y float32) string {
	if e.shown == nil {
		return ""
	}
	for i := len(e.shown.Nodes) - 1; i >= 0; i-- {
		name := e.shown.Nodes[i].Name
		r := e.rects[name]
		if x >= r.X && x <= r.X+r.W && y >= r.Y && y <= r.Y+r.H {
			return name
		}
	}
	return ""
}

// related 是和某张表直接相连的表（含它自己），悬停时用来高亮。
func (e *erTab) related(name string) map[string]bool {
	out := map[string]bool{name: true}
	if e.shown == nil {
		return out
	}
	for _, ed := range e.shown.Edges {
		switch name {
		case ed.From:
			out[ed.To] = true
		case ed.To:
			out[ed.From] = true
		}
	}
	return out
}
