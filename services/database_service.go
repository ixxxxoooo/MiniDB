package services

import (
	"tableplus-ai/internal/database"
)

// DatabaseService 数据库元数据服务
type DatabaseService struct {
	manager *database.Manager
}

// NewDatabaseService 创建数据库服务
func NewDatabaseService(manager *database.Manager) *DatabaseService {
	return &DatabaseService{manager: manager}
}

// GetDatabases 获取数据库列表
func (s *DatabaseService) GetDatabases(connID string) ([]database.DatabaseInfo, error) {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, err
	}
	return database.GetDatabases(db, cfg.Type)
}

// GetTables 获取表列表
func (s *DatabaseService) GetTables(connID, dbName string) ([]database.TableInfo, error) {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, err
	}
	return database.GetTables(db, cfg.Type, dbName)
}

// GetColumns 获取列信息
func (s *DatabaseService) GetColumns(connID, dbName, tableName string) ([]database.ColumnInfo, error) {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, err
	}
	return database.GetColumns(db, cfg.Type, dbName, tableName)
}

// GetDDL 获取 DDL
func (s *DatabaseService) GetDDL(connID, dbName, tableName string) (string, error) {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return "", err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return "", err
	}
	return database.GetDDL(db, cfg.Type, dbName, tableName)
}
