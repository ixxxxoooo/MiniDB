package app

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func seedHistory(t *testing.T, h *HistoryService, sqls ...string) {
	t.Helper()
	for _, q := range sqls {
		if err := h.Add("c1", "db", q); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Microsecond)
	}
}

func sqlsOf(items []HistoryItem) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.SQL)
	}
	return out
}

func TestHistorySearchOptions(t *testing.T) {
	h := newHistory(t)
	seedHistory(t, h, "SELECT id FROM users", "select * from Orders", "UPDATE users SET a = 1", "DELETE FROM user_log")
	cases := []struct {
		name string
		s    HistorySearch
		want int
	}{
		{"默认不区分大小写", HistorySearch{Text: "orders"}, 1},
		{"区分大小写", HistorySearch{Text: "orders", Case: true}, 0},
		{"区分大小写命中", HistorySearch{Text: "Orders", Case: true}, 1},
		{"整词 user 不匹配 users 和 user_log", HistorySearch{Text: "user", Word: true}, 0},
		{"整词 users", HistorySearch{Text: "users", Word: true}, 2},
		{"正则", HistorySearch{Text: `^(select|update)\b`, Regex: true}, 3},
		{"正则加整词", HistorySearch{Text: "user(s|_log)", Regex: true, Word: true}, 3},
		{"非正则时元字符按字面", HistorySearch{Text: "a = 1"}, 1},
		{"非正则的点不是通配", HistorySearch{Text: "u.ers"}, 0},
		{"空搜索词返回全部", HistorySearch{}, 4},
	}
	for _, c := range cases {
		got, err := h.Search("c1", c.s)
		if err != nil || len(got) != c.want {
			t.Errorf("%s: 条数 %d（期望 %d），err=%v，%q", c.name, len(got), c.want, err, sqlsOf(got))
		}
	}
}

func TestHistorySearchInvalidRegex(t *testing.T) {
	h := newHistory(t)
	seedHistory(t, h, "SELECT 1")
	_, err := h.Search("c1", HistorySearch{Text: "(", Regex: true})
	if !errors.Is(err, ErrHistoryPattern) {
		t.Fatalf("应报正则不合法: %v", err)
	}
	if _, err := h.Search("c1", HistorySearch{Text: "(", Regex: false}); err != nil {
		t.Fatalf("非正则模式下括号是普通字符: %v", err)
	}
}

// 收藏过滤和搜索都要扫完整个历史，不能只看最近的 Limit 条。
func TestHistorySearchScansBeyondLimit(t *testing.T) {
	h := newHistory(t)
	seedHistory(t, h, "SELECT needle")
	old, _ := h.List("c1", "", 0)
	if err := h.ToggleFavorite(old[0].ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 400; i++ {
		seedHistory(t, h, fmt.Sprintf("SELECT %d", i))
	}
	got, err := h.Search("c1", HistorySearch{Favorites: true, Limit: 300})
	if err != nil || len(got) != 1 || got[0].SQL != "SELECT needle" {
		t.Fatalf("旧收藏应能搜到: %v %q", err, sqlsOf(got))
	}
	got, _ = h.Search("c1", HistorySearch{Text: "needle", Limit: 300})
	if len(got) != 1 {
		t.Fatalf("旧语句应能搜到: %q", sqlsOf(got))
	}
}
