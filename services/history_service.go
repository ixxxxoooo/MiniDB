package services

import (
	"fmt"
	"minidb/internal/logger"
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
// ID 采用纳秒精度的可排序时间格式：既保证同一连接同一秒内的多条记录不互相覆盖，
// 又与旧版秒级 key 保持字典序一致（反向游标仍能按时间倒序读取）。
func (s *HistoryService) AddHistory(connID, database, sql string, duration, rowCount int64, queryError string) error {
	item := storage.QueryHistoryItem{
		ID:        time.Now().Format("20060102150405.000000000") + "-" + connID,
		ConnID:    connID,
		Database:  database,
		SQL:       sql,
		Duration:  duration,
		RowCount:  rowCount,
		Error:     queryError,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	logger.Debug("[HistoryService] 添加历史记录: connID=%s db=%s sql_len=%d duration=%dms", connID, database, len(sql), duration)
	if err := s.store.AddHistory(item); err != nil {
		logger.Error("[HistoryService] 添加历史记录失败: %v", err)
		return fmt.Errorf("添加历史记录失败: %w", err)
	}
	return nil
}

// GetHistory 获取历史记录
func (s *HistoryService) GetHistory(limit int) ([]storage.QueryHistoryItem, error) {
	logger.Debug("[HistoryService] 获取历史记录: limit=%d", limit)
	items, err := s.store.GetHistory(limit)
	if err != nil {
		logger.Error("[HistoryService] 获取历史记录失败: %v", err)
		return nil, fmt.Errorf("获取历史记录失败: %w", err)
	}
	return items, nil
}

// GetFavorites 获取收藏查询
func (s *HistoryService) GetFavorites() ([]storage.QueryHistoryItem, error) {
	logger.Debug("[HistoryService] 获取收藏查询")
	items, err := s.store.GetFavoriteQueries()
	if err != nil {
		logger.Error("[HistoryService] 获取收藏查询失败: %v", err)
		return nil, fmt.Errorf("获取收藏查询失败: %w", err)
	}
	return items, nil
}

// ToggleFavorite 切换收藏
func (s *HistoryService) ToggleFavorite(id string) error {
	logger.Info("[HistoryService] 切换收藏: id=%s", id)
	if err := s.store.ToggleFavorite(id); err != nil {
		logger.Error("[HistoryService] 切换收藏失败: id=%s err=%v", id, err)
		return fmt.Errorf("切换收藏失败: %w", err)
	}
	return nil
}
