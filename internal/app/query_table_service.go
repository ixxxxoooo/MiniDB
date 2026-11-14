package app

import (
	"context"
	"minidb/internal/model"

	"fmt"

	"strings"

	"minidb/internal/database"
	"minidb/internal/logger"
)

// QueryTableData 分页查询表数据
func (s *QueryService) QueryTableData(connID, dbName, table string, page, pageSize int, filters []database.Filter, sorts []database.Sort) (*database.QueryResult, error) {
	defer logger.Perf("app.Queries.QueryTableData", "conn", connID, "db", dbName, "table", table)()
	var result *database.QueryResult
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		var queryErr error
		result, queryErr = database.QueryTableData(session.Context(), session, cfg.Type, dbName, table, page, pageSize, filters, sorts)
		return queryErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// QueryTableRows 分页取表数据，不统计总行数（Total 为 -1），首屏更快。
func (s *QueryService) QueryTableRows(connID, dbName, table string, page, pageSize int, filters []database.Filter, sorts []database.Sort) (*database.QueryResult, error) {
	defer logger.Perf("app.Queries.QueryTableRows", "conn", connID, "db", dbName, "table", table)()
	var result *database.QueryResult
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		var queryErr error
		result, queryErr = database.QueryTableRows(session.Context(), session, cfg.Type, dbName, table, page, pageSize, filters, sorts)
		return queryErr
	})
	return result, err
}

// CountTableData 统计满足筛选条件的总行数。
func (s *QueryService) CountTableData(connID, dbName, table string, filters []database.Filter) (int64, error) {
	defer logger.Perf("app.Queries.CountTableData", "conn", connID, "db", dbName, "table", table)()
	var total int64
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		var countErr error
		total, countErr = database.CountTableData(session.Context(), session, cfg.Type, dbName, table, filters)
		return countErr
	})
	return total, err
}

// QueryTableDataWithRawInput 表数据查询：rawInput 为空时走筛选分页；否则由后端判断完整 SQL 或 WHERE 片段并执行（职责从前端下沉）
func (s *QueryService) QueryTableDataWithRawInput(connID, dbName, table string, page, pageSize int, filters []database.Filter, sorts []database.Sort, rawInput string) (*database.QueryResult, error) {
	defer logger.Perf("app.Queries.QueryTableDataWithRawInput", "conn", connID, "db", dbName, "table", table)()
	if strings.TrimSpace(rawInput) == "" {
		return s.QueryTableData(connID, dbName, table, page, pageSize, filters, sorts)
	}
	logger.Info("[QueryService] QueryTableDataWithRawInput: connID=%s table=%s page=%d raw_len=%d", connID, table, page, len(rawInput))

	var result *database.QueryResult
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		ver, verr := database.GetServerVersion(session.Context(), session, cfg.Type)
		if verr != nil {
			logger.Warn("[QueryService] QueryTableDataWithRawInput 获取服务器版本失败，按未知版本处理: %v", verr)
			ver = ""
		}
		sqlStr, buildErr := database.BuildTableDataQuerySQL(cfg.Type, dbName, table, rawInput, page, pageSize, ver)
		if buildErr != nil {
			result = &database.QueryResult{Error: buildErr.Error()}
			return nil
		}
		var queryErr error
		result, queryErr = database.ExecuteQueryPagedFor(context.Background(), cfg.Type, session, sqlStr, page, pageSize, true)
		return queryErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// withTx 在固定连接上开启事务执行 fn；fn 返回错误则回滚，否则提交。
// n 是本次写入的行数：手动提交模式下不再自己提交，而是把改动留在用户的事务里，并累加未提交条数。
func (s *QueryService) withTx(connID, dbName string, n int, fn func(tx database.Executor, cfg *database.ConnectionConfig) error) error {
	if mt := s.manager.ManualTx(connID, dbName); mt != nil {
		cfg, ok := s.manager.GetConfig(connID)
		if !ok {
			return model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
		}
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		return mt.Use(context.Background(), func(session *database.Session) error {
			return mt.RunWrite(context.Background(), session, n, func(ex database.Executor) error { return fn(ex, cfg) })
		})
	}
	return withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		tx, err := session.Begin()
		if err != nil {
			return fmt.Errorf("开启事务失败: %w", err)
		}
		if err := fn(tx, cfg); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("事务提交失败: %w", err)
		}
		return nil
	})
}

// CommitTableDataChanges 在单事务内提交删除、新增、更新（顺序与前端原逻辑一致）
func (s *QueryService) CommitTableDataChanges(connID, dbName, table string, deletePKs []map[string]interface{}, inserts []map[string]interface{}, updates []RowUpdate) error {
	defer logger.Perf("app.Queries.CommitTableDataChanges", "conn", connID, "db", dbName, "table", table)()
	n := len(deletePKs) + len(inserts) + len(updates)
	if n == 0 {
		return nil
	}
	logger.Info("[QueryService] CommitTableDataChanges: table=%s deletes=%d inserts=%d updates=%d", table, len(deletePKs), len(inserts), len(updates))
	err := s.withTx(connID, dbName, n, func(tx database.Executor, cfg *database.ConnectionConfig) error {
		for _, pk := range deletePKs {
			if len(pk) == 0 {
				return fmt.Errorf("删除行失败: 缺少主键")
			}
			if err := database.DeleteRowTx(tx, cfg.Type, dbName, table, pk); err != nil {
				return fmt.Errorf("删除行失败: %w", err)
			}
		}
		for _, row := range inserts {
			if len(row) == 0 {
				continue
			}
			if err := database.InsertRowTx(tx, cfg.Type, dbName, table, row); err != nil {
				return fmt.Errorf("插入行失败: %w", err)
			}
		}
		for _, u := range updates {
			if err := database.UpdateRowTx(tx, cfg.Type, dbName, table, u.PrimaryKey, u.Changes); err != nil {
				return fmt.Errorf("更新行失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		// 失败原因此前只回给界面弹一下提示，日志里什么都没有，事后没法查
		logger.Warn("[QueryService] CommitTableDataChanges 失败（已回滚）: connID=%s db=%s table=%s deletes=%d inserts=%d updates=%d err=%v", connID, dbName, table, len(deletePKs), len(inserts), len(updates), err)
		return err
	}
	logger.Info("[QueryService] CommitTableDataChanges 成功: table=%s", table)
	return nil
}

// DeleteRow 删除行
func (s *QueryService) DeleteRow(connID, dbName, table string, primaryKey map[string]interface{}) error {
	defer logger.Perf("app.Queries.DeleteRow", "conn", connID, "db", dbName, "table", table)()
	return withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		return database.DeleteRow(session, cfg.Type, dbName, table, primaryKey)
	})
}

// UpdateRow 更新行
func (s *QueryService) UpdateRow(connID, dbName, table string, primaryKey map[string]interface{}, changes map[string]interface{}) error {
	defer logger.Perf("app.Queries.UpdateRow", "conn", connID, "db", dbName, "table", table)()
	logger.Info("[QueryService] 更新行: table=%s", table)
	return withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		return database.UpdateRow(session, cfg.Type, dbName, table, primaryKey, changes)
	})
}

// InsertRow 插入新行
func (s *QueryService) InsertRow(connID, dbName, table string, row map[string]interface{}) error {
	defer logger.Perf("app.Queries.InsertRow", "conn", connID, "db", dbName, "table", table)()
	logger.Info("[QueryService] 插入行: table=%s", table)
	return withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		return database.InsertRow(session, cfg.Type, dbName, table, row)
	})
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
	sqlStr := database.AppendPage(cfg.Type, "SELECT * FROM "+q, limit, -1) + ";"
	logger.Debug("[QueryService] DefaultSelectTableSQL: table=%s limit=%d", table, limit)
	return sqlStr, nil
}

// RowUpdate 单行更新：主键定位 + 变更字段
type RowUpdate struct {
	PrimaryKey map[string]interface{} `json:"primaryKey"`
	Changes    map[string]interface{} `json:"changes"`
}

// BatchUpdateRows 批量事务更新（多行修改一次提交）
func (s *QueryService) BatchUpdateRows(connID, dbName, table string, updates []RowUpdate) error {
	defer logger.Perf("app.Queries.BatchUpdateRows", "conn", connID, "db", dbName, "table", table)()
	logger.Info("[QueryService] 事务批量更新: table=%s count=%d", table, len(updates))
	err := s.withTx(connID, dbName, len(updates), func(tx database.Executor, cfg *database.ConnectionConfig) error {
		for _, u := range updates {
			if err := database.UpdateRowTx(tx, cfg.Type, dbName, table, u.PrimaryKey, u.Changes); err != nil {
				return fmt.Errorf("事务更新失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	logger.Info("[QueryService] 事务批量更新成功: table=%s count=%d", table, len(updates))
	return nil
}

// BuildNewRowDefaults 根据表结构生成新行默认值（时间型字段自动填充当前时间等）
func (s *QueryService) BuildNewRowDefaults(connID, dbName, table string) (map[string]interface{}, error) {
	defer logger.Perf("app.Queries.BuildNewRowDefaults", "conn", connID, "db", dbName, "table", table)()
	var defaults map[string]interface{}
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		cols, err := database.GetColumns(session.Context(), session, cfg.Type, dbName, table)
		if err != nil {
			return fmt.Errorf("获取列信息失败: %w", err)
		}
		defaults = database.BuildNewRowDefaults(cols)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return defaults, nil
}
