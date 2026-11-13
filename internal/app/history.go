package app

import (
	"fmt"
	"strings"
	"time"

	"minidb/internal/storage"
)

const historyBucket = "sql_history"

// maxHistoryPerConn bounds unfavorited entries kept per connection.
const maxHistoryPerConn = 500

// HistoryItem is an executed (or favorited) statement.
type HistoryItem struct {
	ID         string    `json:"id"`
	ConnID     string    `json:"connId"`
	Database   string    `json:"database"`
	SQL        string    `json:"sql"`
	ExecutedAt time.Time `json:"executedAt"`
	Favorite   bool      `json:"favorite"`
}

// HistoryService keeps the SQL history and favorites per connection.
type HistoryService struct{ store *storage.Store }

func NewHistoryService(store *storage.Store) *HistoryService { return &HistoryService{store: store} }

// key sorts a connection's entries by time: <conn>\x00<unix nanos, 20 digits>.
func historyKey(connID string, at time.Time) string {
	return fmt.Sprintf("%s\x00%020d", connID, at.UnixNano())
}

// Add records sql, skipping an immediate repeat of the latest entry.
func (h *HistoryService) Add(connID, db, sql string) error {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return nil
	}
	if recent, _ := h.List(connID, "", 1); len(recent) == 1 && strings.TrimSpace(recent[0].SQL) == sql {
		return nil
	}
	at := time.Now()
	item := HistoryItem{ID: historyKey(connID, at), ConnID: connID, Database: db, SQL: sql, ExecutedAt: at}
	if err := h.store.Put(historyBucket, item.ID, item); err != nil {
		return err
	}
	return h.trim(connID)
}

// List returns a connection's entries newest first, those containing query
// (case-insensitive) when it is not empty, at most limit (<=0: all).
func (h *HistoryService) List(connID, query string, limit int) ([]HistoryItem, error) {
	items, err := h.store.ListPrefix(historyBucket, connID+"\x00", true, func() interface{} { return &HistoryItem{} })
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var out []HistoryItem
	for _, it := range items {
		item := *it.(*HistoryItem)
		if q != "" && !strings.Contains(strings.ToLower(item.SQL), q) {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// ToggleFavorite flips an entry's favorite flag.
func (h *HistoryService) ToggleFavorite(id string) error {
	var item HistoryItem
	if err := h.store.Get(historyBucket, id, &item); err != nil {
		return err
	}
	item.Favorite = !item.Favorite
	return h.store.Put(historyBucket, id, item)
}

// Remove deletes an entry.
func (h *HistoryService) Remove(id string) error { return h.store.Delete(historyBucket, id) }

// Clear deletes a connection's entries except favorites.
func (h *HistoryService) Clear(connID string) error {
	items, err := h.List(connID, "", 0)
	if err != nil {
		return err
	}
	for _, it := range items {
		if !it.Favorite {
			if err := h.store.Delete(historyBucket, it.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *HistoryService) trim(connID string) error {
	items, err := h.List(connID, "", 0)
	if err != nil {
		return err
	}
	kept := 0
	for _, it := range items { // newest first
		if it.Favorite {
			continue
		}
		kept++
		if kept > maxHistoryPerConn {
			if err := h.store.Delete(historyBucket, it.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
