package ui

import (
	"sort"
	"strings"
	"time"

	"minidb/internal/model"
	"minidb/locales"
)

// @author ixxxxoooo
//
// 欢迎页左边的「最近使用」和「已固定」两个视图，以及卡片上的「N 小时前」和环境简称。
// 两个视图用 layout.connFolder 里的保留 ID 表示（文件夹 ID 是随机串，不会重名）。

const (
	viewRecent = "@recent" // 最近使用：用过的连接，按最近一次连接成功的时间倒序
	viewPinned = "@pinned" // 已固定：固定了的连接，按原来的顺序
)

// recentLimit 是「最近使用」最多列几条。
const recentLimit = 12

// isConnView 报告 id 是不是「最近使用」「已固定」这两个保留视图。
func isConnView(id string) bool { return id == viewRecent || id == viewPinned }

// viewGroup 返回保留视图 id 对应的分组（连接为空时 conns 为空）；其他 id 返回 false。
// 分组的 folder 是个只带名字和保留 ID 的占位，画标题时靠 isConnView 区分，不当成真文件夹。
func (s *Shell) viewGroup(id string) (connGroup, bool) {
	var conns []model.ConnectionConfig
	var name string
	switch id {
	case viewRecent:
		name = locales.WelcomeRecent()
		for _, cfg := range s.visibleConns() {
			if cfg.LastUsed > 0 {
				conns = append(conns, cfg)
			}
		}
		sort.SliceStable(conns, func(i, j int) bool { return conns[i].LastUsed > conns[j].LastUsed })
		if len(conns) > recentLimit {
			conns = conns[:recentLimit]
		}
	case viewPinned:
		name = locales.WelcomePinned()
		for _, cfg := range s.visibleConns() {
			if cfg.Pinned {
				conns = append(conns, cfg)
			}
		}
	default:
		return connGroup{}, false
	}
	return connGroup{folder: &model.ConnectionFolder{ID: id, Name: name}, conns: conns}, true
}

// visibleConns 是搜索框过滤后的连接（没有搜索词就是全部）。
func (s *Shell) visibleConns() []model.ConnectionConfig {
	q := strings.ToLower(strings.TrimSpace(s.dlg.search))
	if q == "" {
		return s.conns
	}
	var out []model.ConnectionConfig
	for _, cfg := range s.conns {
		if strings.Contains(strings.ToLower(cfg.Name+" "+connAddr(cfg)+" "+typeLabel(cfg.Type)), q) {
			out = append(out, cfg)
		}
	}
	return out
}

// pinnedCount 是已固定的连接数。
func (s *Shell) pinnedCount() int {
	n := 0
	for _, cfg := range s.conns {
		if cfg.Pinned {
			n++
		}
	}
	return n
}

// setPinned 固定或取消固定连接，完成后重新读取列表。
func (s *Shell) setPinned(cfg model.ConnectionConfig, pinned bool) {
	var err error
	s.up.bg(func() { err = s.app.Connections.SetPinned(cfg.ID, pinned) }, func() {
		if err != nil {
			s.toastErr(errMessage(err))
			return
		}
		s.loadConnections()
	})
}

// agoText 把最近使用时间说成「刚刚 / N 分钟前 / N 小时前 / 昨天 / N 天前」，一周以上直接写日期；ts 为 0 是还没用过。
func agoText(ts int64, now time.Time) string {
	if ts <= 0 {
		return locales.AgoNever()
	}
	d := now.Sub(time.Unix(ts, 0))
	switch {
	case d < time.Minute:
		return locales.AgoNow()
	case d < time.Hour:
		return locales.AgoMinutes(int(d / time.Minute))
	case d < 24*time.Hour:
		return locales.AgoHours(int(d / time.Hour))
	case d < 48*time.Hour:
		return locales.AgoYesterday()
	}
	if d < 7*24*time.Hour {
		return locales.AgoDays(int(d / (24 * time.Hour)))
	}
	u := time.Unix(ts, 0)
	return locales.AgoDate(itoa(int(u.Month())), itoa(u.Day()))
}

// tagShort 是环境标记的大写简称（PROD / STG / DEV…），用在卡片和弹层的小标签上。
func tagShort(tag string) string {
	switch tag {
	case "production":
		return locales.ConnTagProductionShort()
	case "test":
		return locales.ConnTagTestShort()
	case "development":
		return locales.ConnTagDevelopmentShort()
	case "staging":
		return locales.ConnTagStagingShort()
	}
	return locales.ConnTagLocalShort()
}

// addrNote 是地址后面的附注：只读、走 SSH 隧道。
func addrNote(cfg model.ConnectionConfig) string {
	note := ""
	if cfg.ReadOnly {
		note += " · " + locales.ConnReadOnlyShort()
	}
	if cfg.SSH.Enabled {
		note += " · SSH"
	}
	return note
}
