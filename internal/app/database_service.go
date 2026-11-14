package app

import (
	"context"
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/model"
	"minidb/internal/schemaindex"
	"strings"
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
	c, err := s.with(connID, "")
	if err != nil {
		return "", err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	version, err := database.GetServerVersion(ctx, c.DB, c.Cfg.Type)
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
	c, err := s.with(connID, "")
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()

	// 如果连接配置中指定了数据库名，只返回该库
	if c.Cfg.Database != "" && c.Dialect.Caps().MultiDatabase {
		logger.Info("[DatabaseService] 连接已指定数据库 '%s'，只返回该库", c.Cfg.Database)
		tables, _ := database.GetTables(ctx, c.DB, c.Cfg.Type, c.Cfg.Database)
		return []database.DatabaseInfo{{
			Name:       c.Cfg.Database,
			TableCount: len(tables),
		}}, nil
	}

	dbs, err := database.GetDatabases(ctx, c.DB, c.Cfg.Type)
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
	c, err := s.with(connID, "")
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	dbs, err := database.GetDatabaseNames(ctx, c.DB, c.Cfg.Type)
	if err != nil {
		logger.Error("[DatabaseService] 快速获取数据库名称失败: %v", err)
		return nil, err
	}
	if c.Cfg.Database != "" && c.Dialect.Caps().MultiDatabase {
		dbs = ensureDatabaseListed(dbs, c.Cfg.Database)
	}
	logger.Info("[DatabaseService] 切换器获取到 %d 个数据库", len(dbs))
	return dbs, nil
}

func ensureDatabaseListed(dbs []database.DatabaseInfo, name string) []database.DatabaseInfo {
	if name == "" {
		return dbs
	}
	for _, db := range dbs {
		if strings.EqualFold(db.Name, name) {
			return dbs
		}
	}
	return append([]database.DatabaseInfo{{Name: name}}, dbs...)
}

// GetTables 获取表列表
func (s *DatabaseService) GetTables(connID, dbName string) ([]database.TableInfo, error) {
	logger.Info("[DatabaseService] 获取表列表: connID=%s db=%s", connID, dbName)
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	tables, err := database.GetTables(ctx, c.DB, c.Cfg.Type, dbName)
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
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetColumns(ctx, c.DB, c.Cfg.Type, dbName, tableName)
}

// GetDDL 获取表的 DDL 建表语句
func (s *DatabaseService) GetDDL(connID, dbName, tableName string) (string, error) {
	logger.Debug("[DatabaseService] 获取 DDL: table=%s", tableName)
	c, err := s.with(connID, dbName)
	if err != nil {
		return "", err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetDDL(ctx, c.DB, c.Cfg.Type, dbName, tableName)
}

// GetTableStats 获取表统计信息（行数、大小、引擎等）
func (s *DatabaseService) GetTableStats(connID, dbName, tableName string) (*database.TableStats, error) {
	logger.Debug("[DatabaseService] 获取表统计: table=%s", tableName)
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetTableStats(ctx, c.DB, c.Cfg.Type, dbName, tableName)
}

// GetIndexes 获取表的索引信息
func (s *DatabaseService) GetIndexes(connID, dbName, tableName string) ([]database.IndexInfo, error) {
	logger.Debug("[DatabaseService] 获取索引信息: table=%s", tableName)
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetIndexes(ctx, c.DB, c.Cfg.Type, dbName, tableName)
}

// GetTableOverview 返回当前库所有表和视图的概览（名称、引擎、行数、大小、备注等），给「项目概览」页用。
func (s *DatabaseService) GetTableOverview(connID, dbName string) ([]database.TableOverview, error) {
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetTableOverview(ctx, c.DB, c.Cfg.Type, dbName)
}

// GetColumnTypes 返回指定数据库引擎支持的字段类型列表（供前端结构编辑使用）
func (s *DatabaseService) GetColumnTypes(connID string) []string {
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return database.DefaultColumnTypes()
	}
	return database.GetColumnTypes(cfg.Type)
}

// ExecuteRawSQL 执行原始 SQL（用于 ALTER TABLE 等 DDL 操作）
func (s *DatabaseService) ExecuteRawSQL(connID, dbName, sql string) error {
	// 只记录动词与长度：语句里可能带业务数据，不整条写入日志
	logger.Info("[DatabaseService] 执行原始 SQL: db=%s verb=%s sql_len=%d", dbName, database.SQLLeadingVerb(sql), len(sql))
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if cfg.ReadOnly {
			if msg := readOnlyViolation(sql); msg != "" {
				return errReadOnlyConnection
			}
		}
		_, execErr := session.Exec(sql)
		return execErr
	})
	if err != nil {
		logger.Error("[DatabaseService] SQL 执行失败: %v", err)
		return model.Wrapf(model.CodeSQLExecFailed, "执行 SQL 失败: %w", err)
	}
	if s.schema != nil && database.IsSchemaChangeSQL(sql) {
		s.schema.MarkDirtyAndRefreshAsync(connID, dbName)
	}
	return nil
}
