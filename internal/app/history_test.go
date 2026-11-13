package app

import (
	"path/filepath"
	"testing"
	"time"

	"minidb/internal/storage"
)

func newHistory(t *testing.T) *HistoryService {
	t.Helper()
	store, err := storage.OpenStore(filepath.Join(t.TempDir(), "h.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return NewHistoryService(store)
}

func TestHistoryAddListSearch(t *testing.T) {
	h := newHistory(t)
	for _, q := range []string{"SELECT 1", "SELECT 1", "SELECT * FROM users", "  "} {
		if err := h.Add("c1", "db", q); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Microsecond)
	}
	h.Add("c2", "db", "SELECT other")
	got, _ := h.List("c1", "", 0)
	if len(got) != 2 || got[0].SQL != "SELECT * FROM users" || got[1].SQL != "SELECT 1" {
		t.Fatalf("history = %+v", got)
	}
	if got, _ := h.List("c1", "USERS", 0); len(got) != 1 {
		t.Fatalf("search = %+v", got)
	}
	if got, _ := h.List("c1", "", 1); len(got) != 1 {
		t.Fatal("limit")
	}
	if got, _ := h.List("c2", "", 0); len(got) != 1 || got[0].SQL != "SELECT other" {
		t.Fatalf("other connection leaked: %+v", got)
	}
}

func TestHistoryFavoritesSurviveClear(t *testing.T) {
	h := newHistory(t)
	h.Add("c1", "db", "SELECT 1")
	time.Sleep(time.Microsecond)
	h.Add("c1", "db", "SELECT 2")
	items, _ := h.List("c1", "", 0)
	if err := h.ToggleFavorite(items[1].ID); err != nil { // SELECT 1
		t.Fatal(err)
	}
	h.Clear("c1")
	got, _ := h.List("c1", "", 0)
	if len(got) != 1 || got[0].SQL != "SELECT 1" || !got[0].Favorite {
		t.Fatalf("after clear: %+v", got)
	}
	if err := h.Remove(got[0].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := h.List("c1", "", 0); len(got) != 0 {
		t.Fatal("remove")
	}
}

func TestHistoryKeywords(t *testing.T) {
	h := newHistory(t)
	h.Add("c1", "db", "SELECT 1")
	time.Sleep(time.Microsecond)
	h.Add("c1", "db", "SELECT 2")
	items, _ := h.List("c1", "", 0)
	if err := h.SetKeyword(items[0].ID, "two"); err != nil {
		t.Fatal(err)
	}
	if err := h.SetKeyword(items[1].ID, "TWO"); err == nil {
		t.Fatal("同一连接内关键字不能重复（不分大小写）")
	}
	if err := h.SetKeyword(items[1].ID, "a b"); err == nil {
		t.Fatal("关键字不能含空白")
	}
	kw, _ := h.Keywords("c1")
	if len(kw) != 1 || kw["two"] != "SELECT 2" {
		t.Fatalf("keywords=%v", kw)
	}
	got, _ := h.List("c1", "", 0)
	if !got[0].Favorite {
		t.Fatal("绑定关键字的查询应自动收藏")
	}
	h.Clear("c1")
	if kw, _ = h.Keywords("c1"); len(kw) != 1 {
		t.Fatalf("清空历史不应丢掉绑定了关键字的收藏: %v", kw)
	}
	if err := h.SetKeyword(items[0].ID, ""); err != nil {
		t.Fatal(err)
	}
	if kw, _ = h.Keywords("c1"); len(kw) != 0 {
		t.Fatal("空串应解除绑定")
	}
}
