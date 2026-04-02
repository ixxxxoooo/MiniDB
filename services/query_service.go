package services

import (
	"fmt"
	"tableplus-ai/internal/database"
	"tableplus-ai/internal/logger"
)

// QueryService 查询执行服务，封装 SQL 执行、表数据查询、行操作等功能
type QueryService struct {
	manager *database.Manager
}

// NewQueryService 创建查询服务
func NewQueryService(manager *database.Manager) *QueryService {
	return &QueryService{manager: manager}
}

// ExecuteSQL 执行 Raw SQL（首次自动分页），根据数据库类型自动切换目标库
func (s *QueryService) ExecuteSQL(connID, dbName, sqlStr string) (*database.QueryResult, error) {
	return s.ExecuteSQLPaged(connID, dbName, sqlStr, 1, database.DefaultPageSize)
}

// ExecuteSQLPaged 分页执行 Raw SQL（page=0 不分页）
func (s *QueryService) ExecuteSQLPaged(connID, dbName, sqlStr string, page, pageSize int) (*database.QueryResult, error) {
	logger.Info("[QueryService] 执行 SQL: connID=%s db=%s sql_len=%d page=%d pageSize=%d", connID, dbName, len(sqlStr), page, pageSize)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		logger.Error("[QueryService] 获取连接失败: %v", err)
		return nil, err
	}

	cfg, ok := s.manager.GetConfig(connID)
	if ok && database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		db.Exec("USE " + dbName)
	}

	result, err := database.ExecuteQueryPaged(db, sqlStr, page, pageSize, true)
	if err != nil {
		logger.Error("[QueryService] SQL 执行出错: %v", err)
	} else if result.Error != "" {
		logger.Warn("[QueryService] SQL 执行返回错误: %s", result.Error)
	} else {
		logger.Info("[QueryService] SQL 执行成功: 行数=%d 总数=%d 耗时=%dms autoLimited=%v", int64(len(result.Rows)), result.Total, result.Duration, result.AutoLimited)
	}
	return result, err
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

	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
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

// UpdateRow 更新行
func (s *QueryService) UpdateRow(connID, dbName, table string, primaryKey map[string]interface{}, changes map[string]interface{}) error {
	logger.Info("[QueryService] 更新行: table=%s", table)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return err
	}
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		db.Exec("USE " + dbName)
	}
	return database.UpdateRow(db, cfg.Type, dbName, table, primaryKey, changes)
}

// InsertRow 插入新行
func (s *QueryService) InsertRow(connID, dbName, table string, row map[string]interface{}) error {
	logger.Info("[QueryService] 插入行: table=%s", table)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return err
	}
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		db.Exec("USE " + dbName)
	}
	return database.InsertRow(db, cfg.Type, dbName, table, row)
}

// GenerateInsertSQL 生成 INSERT 语句
func (s *QueryService) GenerateInsertSQL(table string, row map[string]interface{}) string {
	return database.GenerateInsertSQL(table, row)
}

// BatchUpdateRow 批量事务更新（多行修改一次提交）
type RowUpdate struct {
	PrimaryKey map[string]interface{} `json:"primaryKey"`
	Changes    map[string]interface{} `json:"changes"`
}

func (s *QueryService) BatchUpdateRows(connID, dbName, table string, updates []RowUpdate) error {
	logger.Info("[QueryService] 事务批量更新: table=%s count=%d", table, len(updates))
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return fmt.Errorf("连接配置不存在")
	}
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		db.Exec("USE " + dbName)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}

	for _, u := range updates {
		err := database.UpdateRowTx(tx, cfg.Type, dbName, table, u.PrimaryKey, u.Changes)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("事务更新失败: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("事务提交失败: %w", err)
	}
	logger.Info("[QueryService] 事务批量更新成功: table=%s count=%d", table, len(updates))
	return nil
}
