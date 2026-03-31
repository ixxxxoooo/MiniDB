package services

import (
	"tableplus-ai/internal/database"
)

// QueryService 查询执行服务
type QueryService struct {
	manager *database.Manager
}

// NewQueryService 创建查询服务
func NewQueryService(manager *database.Manager) *QueryService {
	return &QueryService{manager: manager}
}

// ExecuteSQL 执行 Raw SQL
func (s *QueryService) ExecuteSQL(connID, dbName, sqlStr string) (*database.QueryResult, error) {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}

	// 如果指定了数据库名，先切换
	cfg, ok := s.manager.GetConfig(connID)
	if ok && cfg.Type == "mysql" && dbName != "" {
		db.Exec("USE " + dbName)
	}

	return database.ExecuteQuery(db, sqlStr)
}

// QueryTableData 分页查询表数据
func (s *QueryService) QueryTableData(connID, dbName, table string, page, pageSize int, filters []database.Filter, sorts []database.Sort) (*database.QueryResult, error) {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}

	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, err
	}

	if cfg.Type == "mysql" && dbName != "" {
		db.Exec("USE " + dbName)
	}

	return database.QueryTableData(db, cfg.Type, dbName, table, page, pageSize, filters, sorts)
}

// DeleteRow 删除行
func (s *QueryService) DeleteRow(connID, dbName, table string, primaryKey map[string]interface{}) error {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return err
	}
	return database.DeleteRow(db, cfg.Type, dbName, table, primaryKey)
}

// GenerateInsertSQL 生成 INSERT 语句
func (s *QueryService) GenerateInsertSQL(table string, row map[string]interface{}) string {
	return database.GenerateInsertSQL(table, row)
}
