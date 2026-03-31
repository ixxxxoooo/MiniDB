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
	return s.Put("history", item.ID, item)
}

// GetHistory 获取查询历史列表（最近 N 条）
func (s *Store) GetHistory(limit int) ([]QueryHistoryItem, error) {
	var items []QueryHistoryItem
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketHistory)
		return b.ForEach(func(k, v []byte) error {
			var item QueryHistoryItem
			if err := json.Unmarshal(v, &item); err != nil {
				return nil
			}
			items = append(items, item)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	// 按时间倒序
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}

	if limit > 0 && len(items) > limit {
		items = items[:limit]
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
