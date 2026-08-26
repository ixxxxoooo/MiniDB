package services

import (
	"errors"
	"fmt"
	"minidb/internal/logger"
	"minidb/internal/storage"
)

// DocService 文档管理服务
type DocService struct {
	store *storage.Store
}

// NewDocService 创建文档服务
func NewDocService(store *storage.Store) *DocService {
	return &DocService{store: store}
}

// SaveTableDoc 保存表文档
func (s *DocService) SaveTableDoc(connID, dbName, tableName, markdown string) error {
	if connID == "" || tableName == "" {
		return fmt.Errorf("connID 和 tableName 不能为空")
	}
	key := connID + ":" + dbName + ":" + tableName
	logger.Info("[DocService] 保存表文档: key=%s len=%d", key, len(markdown))
	if err := s.store.Put("docs", key, markdown); err != nil {
		return fmt.Errorf("保存表文档失败: %w", err)
	}
	return nil
}

// GetTableDoc 获取表文档
func (s *DocService) GetTableDoc(connID, dbName, tableName string) (string, error) {
	key := connID + ":" + dbName + ":" + tableName
	var doc string
	err := s.store.Get("docs", key, &doc)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return "", nil
		}
		logger.Error("[DocService] 获取表文档失败: key=%s err=%v", key, err)
		return "", fmt.Errorf("获取表文档失败: %w", err)
	}
	return doc, nil
}
