package services

import (
	"minidb/internal/storage"
	"time"
)

// HistoryService 查询历史服务
type HistoryService struct {
	store *storage.Store
}

// NewHistoryService 创建历史服务
func NewHistoryService(store *storage.Store) *HistoryService {
	return &HistoryService{store: store}
}

// AddHistory 添加历史记录
func (s *HistoryService) AddHistory(connID, database, sql string, duration, rowCount int64, queryError string) error {
	item := storage.QueryHistoryItem{
		ID:        time.Now().Format("20060102150405") + "-" + connID,
		ConnID:    connID,
		Database:  database,
		SQL:       sql,
		Duration:  duration,
		RowCount:  rowCount,
		Error:     queryError,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	return s.store.AddHistory(item)
}

// GetHistory 获取历史记录
func (s *HistoryService) GetHistory(limit int) ([]storage.QueryHistoryItem, error) {
	return s.store.GetHistory(limit)
}

// GetFavorites 获取收藏查询
func (s *HistoryService) GetFavorites() ([]storage.QueryHistoryItem, error) {
	return s.store.GetFavoriteQueries()
}

// ToggleFavorite 切换收藏
func (s *HistoryService) ToggleFavorite(id string) error {
	return s.store.ToggleFavorite(id)
}
