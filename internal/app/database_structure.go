package app

import (
	"context"
	"fmt"
	"minidb/internal/model"

	"minidb/internal/database"
	"minidb/internal/logger"
)

// 表结构变更、索引、清空/删除表、建表，以及服务端会话（进程）管理。
// 所有 DDL 的生成在 database 包（纯函数），这里负责取连接、判只读、执行与刷新 schema 索引。

// TruncateTable 清空表（不带可选项）。
func (s *DatabaseService) TruncateTable(connID, dbName, tableName string) error {
	return s.TruncateTableWithOptions(connID, dbName, tableName, database.TableOpOptions{})
}

// TruncateTableWithOptions 清空表；opts 里的 Cascade、Restart identity 只对支持的库生效。
func (s *DatabaseService) TruncateTableWithOptions(connID, dbName, tableName string, opts database.TableOpOptions) error {
	logger.Warn("[DatabaseService] TRUNCATE 表: db=%s table=%s", dbName, tableName)
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		_, execErr := session.Exec(database.BuildTruncateTableSQL(cfg.Type, tableName, opts))
		return execErr
	})
	if err != nil {
		logger.Error("[DatabaseService] TRUNCATE 失败: %v", err)
		return fmt.Errorf("TRUNCATE 表失败: %w", err)
	}
	return nil
}

// DropTable 删除表（不带可选项）。
func (s *DatabaseService) DropTable(connID, dbName, tableName string) error {
	return s.DropTableWithOptions(connID, dbName, tableName, database.TableOpOptions{})
}

// DropTableWithOptions 删除表；opts.Cascade 只对 PostgreSQL 生效。
func (s *DatabaseService) DropTableWithOptions(connID, dbName, tableName string, opts database.TableOpOptions) error {
	logger.Warn("[DatabaseService] DROP 表: db=%s table=%s", dbName, tableName)
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		_, execErr := session.Exec(database.BuildDropTableSQL(cfg.Type, tableName, opts))
		return execErr
	})
	if err != nil {
		logger.Error("[DatabaseService] DROP 失败: %v", err)
		return fmt.Errorf("DROP 表失败: %w", err)
	}
	if s.schema != nil {
		s.schema.MarkDirtyAndRefreshAsync(connID, dbName)
	}
	return nil
}

// serverVersion 读取服务器版本，失败时返回空串（生成 DDL 时按未知版本处理）。
func (s *DatabaseService) serverVersion(c connCtx) string {
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	ver, err := database.GetServerVersion(ctx, c.DB, c.Cfg.Type)
	if err != nil {
		logger.Warn("[DatabaseService] 获取服务器版本失败: %v", err)
		return ""
	}
	logger.Info("[DatabaseService] 服务器版本: %s", ver)
	return ver
}

// ApplyTableStructureChanges 根据列/索引编辑意图在后端生成并执行 ALTER TABLE（DDL 拼接职责从前端下沉）
func (s *DatabaseService) ApplyTableStructureChanges(connID, dbName, tableName string, workingCols, originalCols []database.StructureColumnEdit, workingIndexes []database.StructureIndexEdit, objs database.StructureObjectEdits) error {
	logger.Info("[DatabaseService] ApplyTableStructureChanges: table=%s cols=%d orig=%d idx=%d", tableName, len(workingCols), len(originalCols), len(workingIndexes))
	c, err := s.with(connID, "")
	if err != nil {
		return err
	}
	ver := s.serverVersion(c)
	stmts, err := database.BuildFullStructureStatements(c.Cfg.Type, ver, tableName, workingCols, originalCols, workingIndexes, objs)
	if err != nil {
		logger.Error("[DatabaseService] 生成结构变更 DDL 失败: %v", err)
		return fmt.Errorf("生成结构变更 DDL 失败: %w", err)
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

// PreviewTableStructureChanges returns the DDL statements ApplyTableStructureChanges
// would run, without running them.
func (s *DatabaseService) PreviewTableStructureChanges(connID, tableName string, workingCols, originalCols []database.StructureColumnEdit, workingIndexes []database.StructureIndexEdit, objs database.StructureObjectEdits) ([]string, error) {
	c, err := s.with(connID, "")
	if err != nil {
		return nil, err
	}
	ver := s.serverVersion(c)
	return database.BuildFullStructureStatements(c.Cfg.Type, ver, tableName, workingCols, originalCols, workingIndexes, objs)
}

// AddTableIndex 添加索引（后端生成 DDL）
func (s *DatabaseService) AddTableIndex(connID, dbName, tableName, indexName string, columns []string, unique bool) error {
	logger.Info("[DatabaseService] AddTableIndex: table=%s index=%s unique=%v cols=%d", tableName, indexName, unique, len(columns))
	cfg, err := s.configOf(connID)
	if err != nil {
		return err
	}
	sqlStr, err := database.BuildAddIndexSQL(cfg.Type, tableName, indexName, columns, unique)
	if err != nil {
		return fmt.Errorf("生成添加索引 DDL 失败: %w", err)
	}
	return s.ExecuteRawSQL(connID, dbName, sqlStr)
}

// DropTableIndex 删除索引（后端生成 DDL）
func (s *DatabaseService) DropTableIndex(connID, dbName, tableName, indexName string) error {
	logger.Info("[DatabaseService] DropTableIndex: table=%s index=%s", tableName, indexName)
	cfg, err := s.configOf(connID)
	if err != nil {
		return err
	}
	sqlStr, err := database.BuildDropIndexSQL(cfg.Type, tableName, indexName)
	if err != nil {
		return fmt.Errorf("生成删除索引 DDL 失败: %w", err)
	}
	return s.ExecuteRawSQL(connID, dbName, sqlStr)
}

// configOf 确认连接存在并返回其配置。
func (s *DatabaseService) configOf(connID string) (*database.ConnectionConfig, error) {
	if _, err := s.manager.GetDB(connID); err != nil {
		return nil, model.Wrapf(model.CodeGetConnFailed, "获取数据库连接失败: %w", err)
	}
	return resolveConfig(s.manager, connID)
}

// PreviewCreateTable 返回新建表会执行的语句，不执行。
func (s *DatabaseService) PreviewCreateTable(connID, tableName string, cols []database.StructureColumnEdit, idx []database.StructureIndexEdit) ([]string, error) {
	cfg, err := resolveConfig(s.manager, connID)
	if err != nil {
		return nil, err
	}
	return database.BuildCreateTableStatements(cfg.Type, tableName, cols, idx)
}

// CreateTable 新建表：依次执行 CREATE TABLE、列注释和索引。
func (s *DatabaseService) CreateTable(connID, dbName, tableName string, cols []database.StructureColumnEdit, idx []database.StructureIndexEdit) error {
	logger.Info("[DatabaseService] CreateTable: table=%s cols=%d idx=%d", tableName, len(cols), len(idx))
	stmts, err := s.PreviewCreateTable(connID, tableName, cols, idx)
	if err != nil {
		return err
	}
	for _, sqlStr := range stmts {
		if execErr := s.ExecuteRawSQL(connID, dbName, sqlStr); execErr != nil {
			return execErr
		}
	}
	return nil
}

// GetProcessList 获取服务端当前会话列表（MySQL/TiDB/StarRocks/PostgreSQL）。
func (s *DatabaseService) GetProcessList(connID string) ([]database.ProcessInfo, error) {
	c, err := s.with(connID, "")
	if err != nil {
		return nil, err
	}
	if !c.Dialect.Caps().ProcessList {
		return nil, model.Errf(model.CodeUnsupportedProc, "%s 不支持查看进程列表", c.Cfg.Type)
	}
	ctx, cancel := withTimeout(adminTimeout)
	defer cancel()
	return database.GetProcessList(ctx, c.DB, c.Cfg.Type)
}

// KillProcess 终止一个会话。queryOnly=true 只中断正在执行的语句，保留连接。
func (s *DatabaseService) KillProcess(connID, processID string, queryOnly bool) error {
	c, err := s.with(connID, "")
	if err != nil {
		return err
	}
	if err := ensureWritable(c.Cfg); err != nil {
		return err
	}
	logger.Warn("[DatabaseService] 终止进程: connID=%s pid=%s queryOnly=%v", connID, processID, queryOnly)
	ctx, cancel := context.WithTimeout(context.Background(), adminTimeout)
	defer cancel()
	return database.KillProcess(ctx, c.DB, c.Cfg.Type, processID, queryOnly)
}
