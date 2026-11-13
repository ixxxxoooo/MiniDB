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
	Name       string    `json:"name,omitempty"`    // 收藏时起的名字，空则界面显示语句本身
	Keyword    string    `json:"keyword,omitempty"` // 收藏查询的关键字：在编辑器里输入它，补全出整条语句
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

// SaveFavorite 把 sql 收藏起来：历史里已有同样的语句就把它标成收藏（并更新名字），没有就新建一条。
// 返回收藏后的条目。
func (h *HistoryService) SaveFavorite(connID, db, name, sql string) (HistoryItem, error) {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return HistoryItem{}, fmt.Errorf("没有可收藏的语句")
	}
	name = strings.TrimSpace(name)
	if existing, ok := h.find(connID, sql); ok {
		existing.Favorite = true
		if name != "" {
			existing.Name = name
		}
		return existing, h.store.Put(historyBucket, existing.ID, existing)
	}
	at := time.Now()
	item := HistoryItem{ID: historyKey(connID, at), ConnID: connID, Database: db, SQL: sql, ExecutedAt: at, Favorite: true, Name: name}
	return item, h.store.Put(historyBucket, item.ID, item)
}

// FavoriteOf 返回连接上与 sql 相同的收藏；不是收藏时 ok 为 false。
func (h *HistoryService) FavoriteOf(connID, sql string) (HistoryItem, bool) {
	it, ok := h.find(connID, strings.TrimSpace(sql))
	return it, ok && it.Favorite
}

// Favorites 返回连接上的全部收藏，新的在前。
func (h *HistoryService) Favorites(connID string) ([]HistoryItem, error) {
	items, err := h.List(connID, "", 0)
	if err != nil {
		return nil, err
	}
	var out []HistoryItem
	for _, it := range items {
		if it.Favorite {
			out = append(out, it)
		}
	}
	return out, nil
}

// SetName 改收藏的名字，空串清掉名字。
func (h *HistoryService) SetName(id, name string) error {
	var item HistoryItem
	if err := h.store.Get(historyBucket, id, &item); err != nil {
		return err
	}
	item.Name = strings.TrimSpace(name)
	return h.store.Put(historyBucket, id, item)
}

// find 找连接上语句与 sql 相同的最新一条。
func (h *HistoryService) find(connID, sql string) (HistoryItem, bool) {
	items, err := h.List(connID, "", 0)
	if err != nil {
		return HistoryItem{}, false
	}
	for _, it := range items {
		if strings.TrimSpace(it.SQL) == sql {
			return it, true
		}
	}
	return HistoryItem{}, false
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

// SetKeyword 给收藏的查询绑定关键字，空串解除绑定。关键字在同一连接内不能重复，也不能含空白。
func (h *HistoryService) SetKeyword(id, keyword string) error {
	keyword = strings.TrimSpace(keyword)
	if strings.ContainsAny(keyword, " \t\r\n") {
		return fmt.Errorf("关键字不能含空白")
	}
	var item HistoryItem
	if err := h.store.Get(historyBucket, id, &item); err != nil {
		return err
	}
	if keyword != "" {
		others, err := h.List(item.ConnID, "", 0)
		if err != nil {
			return err
		}
		for _, o := range others {
			if o.ID != id && strings.EqualFold(o.Keyword, keyword) {
				return fmt.Errorf("关键字 %s 已被另一条查询使用", keyword)
			}
		}
		item.Favorite = true // 绑定了关键字的查询自然是收藏
	}
	item.Keyword = keyword
	return h.store.Put(historyBucket, id, item)
}

// Keywords 返回连接上绑定了关键字的查询：关键字到语句。
func (h *HistoryService) Keywords(connID string) (map[string]string, error) {
	items, err := h.List(connID, "", 0)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, it := range items {
		if it.Keyword != "" {
			out[it.Keyword] = it.SQL
		}
	}
	return out, nil
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
