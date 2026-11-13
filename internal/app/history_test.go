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
