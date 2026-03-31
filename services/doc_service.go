package services

import (
	"tableplus-ai/internal/storage"
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
	key := connID + ":" + dbName + ":" + tableName
	return s.store.Put("docs", key, markdown)
}

// GetTableDoc 获取表文档
func (s *DocService) GetTableDoc(connID, dbName, tableName string) (string, error) {
	key := connID + ":" + dbName + ":" + tableName
	var doc string
	err := s.store.Get("docs", key, &doc)
	if err != nil {
		return "", nil // 文档不存在返回空字符串
	}
	return doc, nil
}
