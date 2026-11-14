package ui

import "minidb/internal/model"

// connTracker 是 Shell 里和"连接"有关的状态：欢迎页的连接列表，以及各连接当前的连接 / 丢失 / 重连情况。
type connTracker struct {
	conns      []model.ConnectionConfig
	connsErr   string
	connsReady bool
	selected   string // connection chosen on the welcome screen

	connecting   map[string]bool
	lost         map[string]bool          // connections the keepalive lost and is retrying
	reconnecting map[string]bool          // 正在重连的连接
	reconnects   int                      // 重连触发的次数（测试用）
	passwords    map[string]typedPassword // typed for a connection that asks, until it opens
}

func newConnTracker() connTracker {
	return connTracker{connecting: map[string]bool{}, lost: map[string]bool{}, reconnecting: map[string]bool{}, passwords: map[string]typedPassword{}}
}

// sessionCtrl 是会话与状态存档：什么时候允许存、存过什么、怎么写盘。
type sessionCtrl struct {
	restored     bool                // 上次的连接已经（或不需要）恢复过了
	sessionReady bool                // 恢复完成后才允许存档，避免用空列表覆盖上次的会话
	stopSave     chan struct{}       // 关闭它就停掉自动保存
	saved        map[string][32]byte // 各份状态上次写下去的内容哈希，没变就不重写
	writer       stateWriter         // 串行、合并同键写入的写盘队列
}

// taskTray 是界面上的通知条和后台任务托盘。
type taskTray struct {
	toasts    []toastReq
	toastActs []toastAction
	tasks     []*task
}
