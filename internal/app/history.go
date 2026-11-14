package app

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"minidb/internal/logger"
	"minidb/internal/storage"
)

const historyBucket = "sql_history"

// maxHistoryPerConn bounds unfavorited entries kept per connection.
const maxHistoryPerConn = 500

// reSensitiveSQL 命中的语句会带明文口令或密钥：IDENTIFIED BY、SET/WITH PASSWORD、
// PASSWORD(...)、password = '...'、CREATE/ALTER USER|ROLE ... PASSWORD。
// 这是尽力而为的启发式，宁可少记一条历史，也不把口令落进本地库。
var reSensitiveSQL = regexp.MustCompile(`(?is)\bidentified\s+(with\s+\w+\s+)?by\b|\b(set|with|encrypted|unencrypted)\s+password\b|\bpassword\s*\(|\bpassword\s*=\s*['"]|\b(create|alter)\s+(user|role)\b.*\bpassword\b|\b(api[_-]?key|secret[_-]?key|access[_-]?token)\s*=\s*['"]`)

// IsSensitiveSQL 判断一条语句是否可能含有明文口令或密钥，这类语句不写进历史。
func IsSensitiveSQL(sql string) bool { return reSensitiveSQL.MatchString(sql) }

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
	defer logger.Perf("app.History.Add", "conn", connID)()
	sql = strings.TrimSpace(sql)
	if sql == "" || IsSensitiveSQL(sql) {
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
	defer logger.Perf("app.History.List", "conn", connID)()
	return h.Search(connID, HistorySearch{Text: query, Limit: limit})
}

// SaveFavorite 把 sql 收藏起来：历史里已有同样的语句就把它标成收藏（并更新名字），没有就新建一条。
// 返回收藏后的条目。
func (h *HistoryService) SaveFavorite(connID, db, name, sql string) (HistoryItem, error) {
	defer logger.Perf("app.History.SaveFavorite", "conn", connID)()
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
	defer logger.Perf("app.History.FavoriteOf", "conn", connID)()
	it, ok := h.find(connID, strings.TrimSpace(sql))
	return it, ok && it.Favorite
}

// Favorites 返回连接上的全部收藏，新的在前。
func (h *HistoryService) Favorites(connID string) ([]HistoryItem, error) {
	defer logger.Perf("app.History.Favorites", "conn", connID)()
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
	defer logger.Perf("app.History.SetName")()
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
	defer logger.Perf("app.History.ToggleFavorite")()
	var item HistoryItem
	if err := h.store.Get(historyBucket, id, &item); err != nil {
		return err
	}
	item.Favorite = !item.Favorite
	return h.store.Put(historyBucket, id, item)
}

// SetKeyword 给收藏的查询绑定关键字，空串解除绑定。关键字在同一连接内不能重复，也不能含空白。
func (h *HistoryService) SetKeyword(id, keyword string) error {
	defer logger.Perf("app.History.SetKeyword")()
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
	defer logger.Perf("app.History.Keywords", "conn", connID)()
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
func (h *HistoryService) Remove(id string) error {
	defer logger.Perf("app.History.Remove")()
	return h.store.Delete(historyBucket, id)
}

// Clear deletes a connection's entries except favorites.
func (h *HistoryService) Clear(connID string) error {
	defer logger.Perf("app.History.Clear", "conn", connID)()
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

// ClearAll 清空所有连接的历史（收藏的语句保留），返回删除的条数。
func (h *HistoryService) ClearAll() (int, error) {
	defer logger.Perf("app.History.ClearAll")()
	items, err := h.store.ListPrefix(historyBucket, "", false, func() interface{} { return &HistoryItem{} })
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, it := range items {
		item := it.(*HistoryItem)
		if item.Favorite {
			continue
		}
		if err := h.store.Delete(historyBucket, item.ID); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
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
