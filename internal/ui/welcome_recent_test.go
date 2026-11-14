package ui

import (
	"testing"
	"time"
)

// @author ixxxxoooo
//
// 欢迎页的「最近使用」「已固定」视图和卡片上的使用时间。

func TestAgoText(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.Local)
	at := func(d time.Duration) int64 { return now.Add(-d).Unix() }
	for _, c := range []struct {
		ts   int64
		want string
	}{
		{0, "尚未使用"},
		{at(20 * time.Second), "刚刚"},
		{at(5 * time.Minute), "5 分钟前"},
		{at(2 * time.Hour), "2 小时前"},
		{at(30 * time.Hour), "昨天"},
		{at(72 * time.Hour), "3 天前"},
		{time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local).Unix(), "11 月 2 日"},
	} {
		if got := agoText(c.ts, now); got != c.want {
			t.Errorf("agoText(%d) = %q, want %q", c.ts, got, c.want)
		}
	}
}

func TestWelcomeRecentAndPinnedViews(t *testing.T) {
	e := welcomeEnv(t)
	s := e.sh
	now := time.Now().Unix()
	s.conns[0].Pinned = true
	s.conns[1].LastUsed = now - 3600
	s.conns[0].LastUsed = now - 7200
	e.tt.Frame()
	for _, want := range []string{"最近使用", "已固定", "1 小时前", "2 小时前"} {
		e.has(want)
	}

	e.click("已固定")
	e.has("alpha-conn")
	e.hasNot("local-sqlite")

	e.click("最近使用")
	got := s.welcomeConns()
	if len(got) != 2 || got[0].ID != s.conns[1].ID {
		t.Fatalf("最近使用应按时间倒序: %+v", got)
	}

	s.conns[0].LastUsed, s.conns[1].LastUsed = 0, 0
	e.tt.Frame()
	e.has("还没有使用过的连接")
}

func TestPinnedViewEmptyHint(t *testing.T) {
	e := welcomeEnv(t)
	e.sh.conns[0].Pinned = false
	e.tt.Frame()
	e.click("已固定")
	e.has("右键点连接")
	if n := len(e.sh.welcomeConns()); n != 0 {
		t.Fatalf("没固定任何连接，应为空: %d", n)
	}
}
