package ui

import (
	"crypto/sha256"
	"encoding/json"
	"sort"

	"time"

	"minidb/internal/app"
	"minidb/internal/logger"
	"minidb/internal/model"
	"minidb/locales"
)

// sessionState 是应用级的会话：关闭前依次打开的连接，以及当时在前台的那个。
// 每个连接自己的标签页、查询草稿由 wsState 另存，重新打开连接时一并恢复。
type sessionState struct {
	Conns  []string `json:"conns"`
	Active string   `json:"active,omitempty"`
}

// tabState is what is kept of an open tab.
type tabState struct {
	Kind  ObjectKind `json:"kind"` // KindTable 或 KindQuery
	Table string     `json:"table,omitempty"`
	Mode  int        `json:"mode,omitempty"`
	SQL   string     `json:"sql,omitempty"`
	// 表标签的行预览栏（空格）是不是开着
	Preview bool `json:"preview,omitempty"`
}

// wsState is a workspace's saved session: the tabs and query drafts, restored
// the next time the connection opens.
type wsState struct {
	Database string     `json:"database"`
	Tabs     []tabState `json:"tabs"`
	Active   int        `json:"active"`
	Panel    int        `json:"panel"`
	// 侧栏：哪一段（项目 / 查询 / 历史）、折叠起来的分组
	SideSeg   int      `json:"sideSeg,omitempty"`
	Collapsed []string `json:"collapsed,omitempty"`
}

func (w *Workspace) snapshot() wsState {
	st := wsState{Database: w.db, Active: w.active, Panel: w.panel, SideSeg: w.side.seg}
	for k, v := range w.side.collapsed {
		if v {
			st.Collapsed = append(st.Collapsed, k)
		}
	}
	sort.Strings(st.Collapsed)
	for _, tb := range w.tabs {
		switch t := tb.(type) {
		case *tableTab:
			if t.isNew {
				continue // 还没建出来的表不进会话
			}
			st.Tabs = append(st.Tabs, tabState{Kind: KindTable, Table: t.table, Mode: t.mode, Preview: t.showPreview})
		case *overviewTab:
			st.Tabs = append(st.Tabs, tabState{Kind: KindOverview})
		case *erTab:
			st.Tabs = append(st.Tabs, tabState{Kind: KindER, Table: t.focus})
		case *queryTab:
			st.Tabs = append(st.Tabs, tabState{Kind: KindQuery, SQL: t.ed.Buffer.String()})
		}
	}
	return st
}

// restore reopens the saved tabs. Table tabs load their data when first shown.
func (w *Workspace) restore(st wsState) {
	for _, ts := range st.Tabs {
		switch ts.Kind {
		case KindTable:
			t := newTableTab(w, ts.Table)
			t.mode = ts.Mode
			if ts.Preview {
				t.showPreview = true
				if w.sh.layout.PreviewW > 0 {
					t.previewW = w.sh.layout.PreviewW
				}
			}
			w.tabs = append(w.tabs, t)
		case KindOverview:
			w.openOverview()
		case KindER:
			w.openER(ts.Table)
		case KindQuery:
			w.newQuery(ts.SQL)
		}
	}
	w.active = min(max(st.Active, 0), max(len(w.tabs)-1, 0))
	w.panel = st.Panel
	w.side.seg = min(max(st.SideSeg, sideItems), sideHistory)
	for _, k := range st.Collapsed {
		if w.side.collapsed == nil {
			w.side.collapsed = map[string]bool{}
		}
		w.side.collapsed[k] = true
	}
}

// saveWorkspace keeps a workspace's session under its own key, and as the
// connection's last session, which the next connect restores.
func (s *Shell) saveWorkspace(w *Workspace) {
	s.flushState(s.workspaceItems(w), false)
}

// SaveState 把所有打开的工作区、布局和会话写到磁盘，写完才返回：退出前调用。
// 每个连接最后写的是前台的那个工作区，所以它就是这个连接下次打开时恢复的会话。
func (s *Shell) SaveState() {
	s.flushState(s.stateItems(), true)
}

// stateItems 在 UI 线程上取出此刻所有需要保存的状态的快照。
func (s *Shell) stateItems() []stateItem {
	items := s.layoutItems()
	for i, w := range s.spaces {
		if i != s.active {
			items = append(items, s.workspaceItems(w)...)
		}
	}
	if s.active >= 0 && s.active < len(s.spaces) {
		items = append(items, s.workspaceItems(s.spaces[s.active])...)
	}
	return append(items, s.sessionItems()...)
}

// workspaceItems 是一个工作区要保存的两份快照：自己的库键，以及连接的"上次会话"。
func (s *Shell) workspaceItems(w *Workspace) []stateItem {
	st := w.snapshot()
	return []stateItem{
		newStateItem("ws:"+w.dbKey(), st, func() error { return s.app.UIState.SaveWorkspaceState(w.dbKey(), st) }),
		newStateItem("ws:"+w.cfg.ID, st, func() error { return s.app.UIState.SaveWorkspaceState(w.cfg.ID, st) }),
	}
}

// sessionItems 记下现在打开的连接。恢复还没做完时不写，免得把上次的存档用一个空列表盖掉。
func (s *Shell) sessionItems() []stateItem {
	if !s.sessionReady {
		return nil
	}
	st := sessionState{}
	for _, w := range s.spaces {
		st.Conns = append(st.Conns, w.cfg.ID)
	}
	if s.active >= 0 && s.active < len(s.spaces) {
		st.Active = s.spaces[s.active].cfg.ID
	}
	return []stateItem{newStateItem("session", st, func() error { return s.app.UIState.SaveSession(st) })}
}

// restoreSession 在连接列表第一次读出来之后，按上次的顺序把连接逐个重新打开，
// 并回到上次在前台的那个。需要现输密码的连接不自动打开，免得一启动就弹出一串密码框。
func (s *Shell) restoreSession() {
	if s.restored {
		return
	}
	s.restored = true
	var st sessionState
	if s.prefs.NoRestore || s.app.UIState.LoadSession(&st) != nil {
		s.sessionReady = true
		return
	}
	var queue []model.ConnectionConfig
	for _, id := range st.Conns {
		for _, cfg := range s.conns {
			if cfg.ID == id && !app.NeedsPassword(cfg) {
				queue = append(queue, cfg)
			}
		}
	}
	logger.Info("[Session] 恢复上次打开的连接: %d 个", len(queue))
	s.restoreNext(queue, st.Active)
}

// restoreNext 打开队列里的下一个连接，等它完成（成功或失败）再开下一个，保证标签顺序不乱。
func (s *Shell) restoreNext(queue []model.ConnectionConfig, active string) {
	if len(queue) == 0 {
		for i, w := range s.spaces {
			if w.cfg.ID == active {
				s.active = i
			}
		}
		s.sessionReady = true
		s.redraw()
		return
	}
	s.openConnectionThen(queue[0], func() { s.restoreNext(queue[1:], active) })
}

// startAutosave 每隔几秒保存一次状态：UI 线程上只取快照，写盘交给后台；内容没变就不写。
// 返回前先停掉上一个定时器；Close 时通过 stop 通道结束。
func (s *Shell) startAutosave() {
	if s.stopSave != nil {
		return
	}
	s.stopSave = make(chan struct{})
	stop := s.stopSave
	go func() {
		t := time.NewTicker(autosaveEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				s.up.post(func() { s.flushState(s.stateItems(), false) })
			}
		}
	}()
}

// pendingEverywhere counts uncommitted changes in all workspaces.
func (s *Shell) pendingEverywhere() int {
	n := 0
	for _, w := range s.spaces {
		n += w.pendingTotal()
	}
	return n
}

func (s *Shell) quitGuard() (cancel bool) {
	n := s.pendingEverywhere()
	if n == 0 {
		return false
	}
	s.askWarn(locales.QuitTitle(), locales.QuitMsg(n), locales.QuitConfirm(), func() {
		s.quitting = true
		s.requestQuit()
	})
	s.redraw()
	return true
}

// autosaveEvery 是自动保存的间隔。
const autosaveEvery = 8 * time.Second

// stateItem 是一份待保存的状态快照：id 标识它存在哪，hash 用来发现内容没变。
type stateItem struct {
	id    string
	hash  [sha256.Size]byte
	write func() error
}

func newStateItem(id string, v any, write func() error) stateItem {
	data, _ := json.Marshal(v)
	return stateItem{id: id, hash: sha256.Sum256(data), write: write}
}

// flushState 把有变化的快照写下去。wait 为真时等写完再返回；否则交给后台写，不阻塞界面。
// 所有写盘都经过同一个串行队列，同一个键的新旧两次写不会乱序。
func (s *Shell) flushState(items []stateItem, wait bool) {
	if s.saved == nil {
		s.saved = map[string][sha256.Size]byte{}
	}
	var dirty []stateItem
	for _, it := range items {
		if h, ok := s.saved[it.id]; ok && h == it.hash {
			continue
		}
		s.saved[it.id] = it.hash
		dirty = append(dirty, it)
	}
	for _, it := range dirty {
		it := it
		s.writer.do(it.id, func() {
			if err := it.write(); err != nil {
				s.up.post(func() { delete(s.saved, it.id) }) // 没写成：下次不要当成没变而跳过
			}
		})
	}
	if wait {
		s.writer.wait()
	}
}
