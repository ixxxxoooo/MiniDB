package services

import (
	"fmt"
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
)

// DatabaseService 数据库元数据服务，提供数据库列表、表列表、列信息、DDL 等查询
type DatabaseService struct {
	manager *database.Manager
	schema  *schemaindex.Manager
}

// NewDatabaseService 创建数据库服务
func NewDatabaseService(manager *database.Manager, schema *schemaindex.Manager) *DatabaseService {
	return &DatabaseService{manager: manager, schema: schema}
}

// GetServerVersion 获取数据库服务器版本号
func (s *DatabaseService) GetServerVersion(connID string) (string, error) {
	logger.Info("[DatabaseService] 获取服务器版本: connID=%s", connID)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return "", err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return "", fmt.Errorf("连接配置不存在: %s", connID)
	}
	version, err := database.GetServerVersion(db, cfg.Type)
	if err != nil {
		logger.Error("[DatabaseService] 获取服务器版本失败: %v", err)
	} else {
		logger.Info("[DatabaseService] 服务器版本: %s", version)
	}
	return version, err
}

// GetDatabases 获取数据库列表。如果连接配置中指定了具体数据库名，则只返回该库。
func (s *DatabaseService) GetDatabases(connID string) ([]database.DatabaseInfo, error) {
	logger.Info("[DatabaseService] 获取数据库列表: connID=%s", connID)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}

	// 如果连接配置中指定了数据库名，只返回该库
	if cfg.Database != "" && cfg.Type != "sqlite" {
		logger.Info("[DatabaseService] 连接已指定数据库 '%s'，只返回该库", cfg.Database)
		tables, _ := database.GetTables(db, cfg.Type, cfg.Database)
		return []database.DatabaseInfo{{
			Name:       cfg.Database,
			TableCount: len(tables),
		}}, nil
	}

	dbs, err := database.GetDatabases(db, cfg.Type)
	if err != nil {
		logger.Error("[DatabaseService] 获取数据库列表失败: %v", err)
	} else {
		logger.Info("[DatabaseService] 获取到 %d 个数据库", len(dbs))
	}
	return dbs, err
}

// GetAllDatabases 快速获取所有数据库列表（不统计表数量，不受连接配置限制，用于切换器）
func (s *DatabaseService) GetAllDatabases(connID string) ([]database.DatabaseInfo, error) {
	logger.Info("[DatabaseService] 快速获取所有数据库名称: connID=%s", connID)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}
	return database.GetDatabaseNames(db, cfg.Type)
}

// GetTables 获取表列表
func (s *DatabaseService) GetTables(connID, dbName string) ([]database.TableInfo, error) {
	logger.Info("[DatabaseService] 获取表列表: connID=%s db=%s", connID, dbName)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}
	tables, err := database.GetTables(db, cfg.Type, dbName)
	if err != nil {
		logger.Error("[DatabaseService] 获取表列表失败: %v", err)
	} else {
		logger.Info("[DatabaseService] 获取到 %d 张表", len(tables))
	}
	return tables, err
}

// GetColumns 获取列信息
func (s *DatabaseService) GetColumns(connID, dbName, tableName string) ([]database.ColumnInfo, error) {
	logger.Debug("[DatabaseService] 获取列信息: table=%s", tableName)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}
	return database.GetColumns(db, cfg.Type, dbName, tableName)
}

// GetDDL 获取表的 DDL 建表语句
func (s *DatabaseService) GetDDL(connID, dbName, tableName string) (string, error) {
	logger.Debug("[DatabaseService] 获取 DDL: table=%s", tableName)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return "", err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return "", fmt.Errorf("连接配置不存在: %s", connID)
	}
	return database.GetDDL(db, cfg.Type, dbName, tableName)
}

// GetTableStats 获取表统计信息（行数、大小、引擎等）
func (s *DatabaseService) GetTableStats(connID, dbName, tableName string) (*database.TableStats, error) {
	logger.Debug("[DatabaseService] 获取表统计: table=%s", tableName)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}
	return database.GetTableStats(db, cfg.Type, dbName, tableName)
}

// TruncateTable 清空表数据
func (s *DatabaseService) TruncateTable(connID, dbName, tableName string) error {
	logger.Warn("[DatabaseService] TRUNCATE 表: db=%s table=%s", dbName, tableName)
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

	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	_, err = db.Exec("TRUNCATE TABLE " + quotedTable)
	if err != nil {
		logger.Error("[DatabaseService] TRUNCATE 失败: %v", err)
	}
	return err
}

// GetIndexes 获取表的索引信息
func (s *DatabaseService) GetIndexes(connID, dbName, tableName string) ([]database.IndexInfo, error) {
	logger.Debug("[DatabaseService] 获取索引信息: table=%s", tableName)
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}
	return database.GetIndexes(db, cfg.Type, dbName, tableName)
}

// ExecuteRawSQL 执行原始 SQL（用于 ALTER TABLE 等 DDL 操作）
func (s *DatabaseService) ExecuteRawSQL(connID, dbName, sql string) error {
	logger.Info("[DatabaseService] 执行原始 SQL: db=%s sql=%s", dbName, sql)
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

	_, err = db.Exec(sql)
	if err != nil {
		logger.Error("[DatabaseService] SQL 执行失败: %v", err)
		return err
	}
	if s.schema != nil && database.IsSchemaChangeSQL(sql) {
		s.schema.MarkDirtyAndRefreshAsync(connID, dbName)
	}
	return err
}

// ApplyTableStructureChanges 根据列/索引编辑意图在后端生成并执行 ALTER TABLE（DDL 拼接职责从前端下沉）
func (s *DatabaseService) ApplyTableStructureChanges(connID, dbName, tableName string, workingCols, originalCols []database.StructureColumnEdit, workingIndexes []database.StructureIndexEdit) error {
	logger.Info("[DatabaseService] ApplyTableStructureChanges: table=%s cols=%d orig=%d idx=%d", tableName, len(workingCols), len(originalCols), len(workingIndexes))
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return fmt.Errorf("连接配置不存在: %s", connID)
	}
	ver, verr := database.GetServerVersion(db, cfg.Type)
	if verr != nil {
		logger.Warn("[DatabaseService] ApplyTableStructureChanges 获取服务器版本失败: %v", verr)
		ver = ""
	} else {
		logger.Info("[DatabaseService] ApplyTableStructureChanges 服务器版本: %s", ver)
	}
	stmts, err := database.BuildStructureAlterDDLStatements(cfg.Type, ver, tableName, workingCols, originalCols, workingIndexes)
	if err != nil {
		logger.Error("[DatabaseService] 生成结构变更 DDL 失败: %v", err)
		return err
	}
	if len(stmts) == 0 {
		logger.Debug("[DatabaseService] 无结构变更，跳过执行")
		return nil
	}
	for i, sqlStr := range stmts {
		logger.Info("[DatabaseService] 执行结构变更 DDL 片段 %d/%d sql_len=%d", i+1, len(stmts), len(sqlStr))
		if execErr := s.ExecuteRawSQL(connID, dbName, sqlStr); execErr != nil {
			return execErr
		}
	}
	return nil
}

// AddTableIndex 添加索引（后端生成 DDL）
func (s *DatabaseService) AddTableIndex(connID, dbName, tableName, indexName string, columns []string, unique bool) error {
	logger.Info("[DatabaseService] AddTableIndex: table=%s index=%s unique=%v cols=%d", tableName, indexName, unique, len(columns))
	if _, err := s.manager.GetDB(connID); err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return fmt.Errorf("连接配置不存在: %s", connID)
	}
	sqlStr, err := database.BuildAddIndexSQL(cfg.Type, tableName, indexName, columns, unique)
	if err != nil {
		return err
	}
	return s.ExecuteRawSQL(connID, dbName, sqlStr)
}

// DropTableIndex 删除索引（后端生成 DDL）
func (s *DatabaseService) DropTableIndex(connID, dbName, tableName, indexName string) error {
	logger.Info("[DatabaseService] DropTableIndex: table=%s index=%s", tableName, indexName)
	if _, err := s.manager.GetDB(connID); err != nil {
		return err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return fmt.Errorf("连接配置不存在: %s", connID)
	}
	sqlStr, err := database.BuildDropIndexSQL(cfg.Type, tableName, indexName)
	if err != nil {
		return err
	}
	return s.ExecuteRawSQL(connID, dbName, sqlStr)
}

// DropTable 删除表
func (s *DatabaseService) DropTable(connID, dbName, tableName string) error {
	logger.Warn("[DatabaseService] DROP 表: db=%s table=%s", dbName, tableName)
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

	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	_, err = db.Exec("DROP TABLE " + quotedTable)
	if err != nil {
		logger.Error("[DatabaseService] DROP 失败: %v", err)
		return err
	}
	if s.schema != nil {
		s.schema.MarkDirtyAndRefreshAsync(connID, dbName)
	}
	return err
}
