package services

import (
	"fmt"
	"strings"
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
	if ok {
		if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
			return nil, err
		}
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
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}

	if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
		return nil, err
	}

	return database.QueryTableData(db, cfg.Type, dbName, table, page, pageSize, filters, sorts)
}

// QueryTableDataWithRawInput 表数据查询：rawInput 为空时走筛选分页；否则由后端判断完整 SQL 或 WHERE 片段并执行（职责从前端下沉）
func (s *QueryService) QueryTableDataWithRawInput(connID, dbName, table string, page, pageSize int, filters []database.Filter, sorts []database.Sort, rawInput string) (*database.QueryResult, error) {
	if strings.TrimSpace(rawInput) == "" {
		return s.QueryTableData(connID, dbName, table, page, pageSize, filters, sorts)
	}
	logger.Info("[QueryService] QueryTableDataWithRawInput: connID=%s table=%s page=%d raw_len=%d", connID, table, page, len(rawInput))
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在")
	}
	if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
		return nil, err
	}
	ver, verr := database.GetServerVersion(db, cfg.Type)
	if verr != nil {
		logger.Warn("[QueryService] QueryTableDataWithRawInput 获取服务器版本失败，按未知版本处理: %v", verr)
		ver = ""
	} else {
		logger.Debug("[QueryService] QueryTableDataWithRawInput 服务器版本: %s", ver)
	}
	sqlStr, err := database.BuildTableDataQuerySQL(cfg.Type, dbName, table, rawInput, page, pageSize, ver)
	if err != nil {
		return &database.QueryResult{Error: err.Error()}, nil
	}
	return database.ExecuteQueryPaged(db, sqlStr, page, pageSize, true)
}

// CommitTableDataChanges 在单事务内提交删除、新增、更新（顺序与前端原逻辑一致）
func (s *QueryService) CommitTableDataChanges(connID, dbName, table string, deletePKs []map[string]interface{}, inserts []map[string]interface{}, updates []RowUpdate) error {
	n := len(deletePKs) + len(inserts) + len(updates)
	if n == 0 {
		return nil
	}
	logger.Info("[QueryService] CommitTableDataChanges: table=%s deletes=%d inserts=%d updates=%d", table, len(deletePKs), len(inserts), len(updates))
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return fmt.Errorf("连接配置不存在")
	}
	if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	for _, pk := range deletePKs {
		if len(pk) == 0 {
			continue
		}
		if err := database.DeleteRowTx(tx, cfg.Type, dbName, table, pk); err != nil {
			tx.Rollback()
			return fmt.Errorf("删除行失败: %w", err)
		}
	}
	for _, row := range inserts {
		if len(row) == 0 {
			continue
		}
		if err := database.InsertRowTx(tx, cfg.Type, dbName, table, row); err != nil {
			tx.Rollback()
			return fmt.Errorf("插入行失败: %w", err)
		}
	}
	for _, u := range updates {
		if err := database.UpdateRowTx(tx, cfg.Type, dbName, table, u.PrimaryKey, u.Changes); err != nil {
			tx.Rollback()
			return fmt.Errorf("更新行失败: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("事务提交失败: %w", err)
	}
	logger.Info("[QueryService] CommitTableDataChanges 成功: table=%s", table)
	return nil
}

// DeleteRow 删除行
func (s *QueryService) DeleteRow(connID, dbName, table string, primaryKey map[string]interface{}) error {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return fmt.Errorf("连接配置不存在: %s", connID)
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
		return fmt.Errorf("连接配置不存在: %s", connID)
	}
	if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
		return err
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
		return fmt.Errorf("连接配置不存在: %s", connID)
	}
	if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
		return err
	}
	return database.InsertRow(db, cfg.Type, dbName, table, row)
}

// GenerateInsertSQL 生成 INSERT 语句
func (s *QueryService) GenerateInsertSQL(table string, row map[string]interface{}) string {
	return database.GenerateInsertSQL(table, row)
}

// DefaultSelectTableSQL 生成带 LIMIT 的默认浏览查询 SQL（表名按方言引用，避免前端拼接）
func (s *QueryService) DefaultSelectTableSQL(connID, table string, limit int) (string, error) {
	if limit < 1 {
		limit = database.DefaultPageSize
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return "", fmt.Errorf("连接配置不存在")
	}
	q := database.QuoteTableName(cfg.Type, table)
	sqlStr := fmt.Sprintf("SELECT * FROM %s LIMIT %d;", q, limit)
	logger.Debug("[QueryService] DefaultSelectTableSQL: table=%s limit=%d", table, limit)
	return sqlStr, nil
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
	if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
		return err
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
