package storage

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

// QueryHistoryItem 查询历史条目
type QueryHistoryItem struct {
	ID        string `json:"id"`
	ConnID    string `json:"connId"`
	Database  string `json:"database"`
	SQL       string `json:"sql"`
	Duration  int64  `json:"duration"`
	RowCount  int64  `json:"rowCount"`
	Error     string `json:"error,omitempty"`
	Favorited bool   `json:"favorited"`
	CreatedAt string `json:"createdAt"`
}

// AddHistory 添加查询历史
func (s *Store) AddHistory(item QueryHistoryItem) error {
	item.CreatedAt = time.Now().Format(time.RFC3339)
	if err := s.Put("history", item.ID, item); err != nil {
		return err
	}
	return s.trimHistory()
}

// maxHistoryEntries 查询历史保留上限，超出时删除最旧记录
const maxHistoryEntries = 2000

// trimHistory 控制历史总量：超出上限时删除最旧的记录
func (s *Store) trimHistory() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHistory)

		var count int
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
		}
		toDelete := count - maxHistoryEntries
		if toDelete <= 0 {
			return nil
		}
		// bbolt 允许在游标迭代过程中删除当前键
		del := b.Cursor()
		for k, _ := del.First(); k != nil && toDelete > 0; k, _ = del.Next() {
			if err := del.Delete(); err != nil {
				return err
			}
			toDelete--
		}
		return nil
	})
}

// GetHistory 获取查询历史列表（最近 N 条，按时间倒序）。
// bbolt 的 key 形如 "<纳秒时间戳>-<connID>"，字典序即时间序，用反向游标取最近 limit 条，避免全量加载。
func (s *Store) GetHistory(limit int) ([]QueryHistoryItem, error) {
	var items []QueryHistoryItem
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHistory)
		c := b.Cursor()
		count := 0
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var item QueryHistoryItem
			if err := json.Unmarshal(v, &item); err != nil {
				continue // 容忍单条损坏数据
			}
			items = append(items, item)
			count++
			if limit > 0 && count >= limit {
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetFavoriteQueries 获取收藏的查询
func (s *Store) GetFavoriteQueries() ([]QueryHistoryItem, error) {
	all, err := s.GetHistory(0)
	if err != nil {
		return nil, err
	}

	var favorites []QueryHistoryItem
	for _, item := range all {
		if item.Favorited {
			favorites = append(favorites, item)
		}
	}
	return favorites, nil
}

// ToggleFavorite 切换收藏状态
func (s *Store) ToggleFavorite(id string) error {
	var item QueryHistoryItem
	if err := s.Get("history", id, &item); err != nil {
		return fmt.Errorf("查询不存在: %s", id)
	}
	item.Favorited = !item.Favorited
	return s.Put("history", id, item)
}
