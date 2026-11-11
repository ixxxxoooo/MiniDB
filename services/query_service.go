package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
)

// 查询结果错误码（供前端 i18n，不依赖后端文案）
const (
	queryErrorCodeCancelled = "cancelled"
)

// QueryService 查询执行服务，封装 SQL 执行、表数据查询、行操作等功能
type QueryService struct {
	manager *database.Manager
	schema  *schemaindex.Manager

	// 进行中的查询：requestID -> cancel，用于前端主动停止
	mu        sync.Mutex
	cancelMap map[string]context.CancelFunc
}

// NewQueryService 创建查询服务
func NewQueryService(manager *database.Manager, schema *schemaindex.Manager) *QueryService {
	return &QueryService{
		manager:   manager,
		schema:    schema,
		cancelMap: make(map[string]context.CancelFunc),
	}
}

// errReadOnlyConnection 连接处于只读模式时拒绝写操作。
var errReadOnlyConnection = errors.New("当前连接为只读模式，已拒绝写操作（可在连接设置中关闭只读模式）")

func ensureWritable(cfg *database.ConnectionConfig) error {
	if cfg != nil && cfg.ReadOnly {
		return errReadOnlyConnection
	}
	return nil
}

// readOnlyViolation 只读连接上检查一条 SQL，返回拒绝原因；允许执行返回空串。
func readOnlyViolation(sqlStr string) string {
	if check := database.CheckReadOnlySQL(sqlStr); !check.Allowed {
		return errReadOnlyConnection.Error()
	}
	return ""
}

// withSession 取出连接配置，并在一条固定连接上切到 dbName 后执行 fn。
// 所有依赖"当前库"的操作都必须走这里，不能在连接池上直接 USE（见 database.Session）。
func withSession(ctx context.Context, manager *database.Manager, connID, dbName string, fn func(session *database.Session, cfg *database.ConnectionConfig) error) error {
	db, err := manager.GetDBForDatabase(connID, dbName)
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}
	cfg, ok := manager.GetConfig(connID)
	if !ok {
		return fmt.Errorf("连接配置不存在: %s", connID)
	}
	return database.WithSession(ctx, db, cfg.Type, dbName, func(session *database.Session) error {
		return fn(session, cfg)
	})
}

func (s *QueryService) registerCancel(requestID string, cancel context.CancelFunc) {
	if requestID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if prev, ok := s.cancelMap[requestID]; ok {
		prev()
	}
	s.cancelMap[requestID] = cancel
}

func (s *QueryService) clearCancel(requestID string) {
	if requestID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cancelMap, requestID)
}

// CancelQuery 取消一次进行中的查询（对应 ExecuteSQLBatch 的 requestID）。
// 取消会中断正在执行的语句，并跳过同一批次中尚未执行的语句。
func (s *QueryService) CancelQuery(requestID string) {
	s.mu.Lock()
	cancel, ok := s.cancelMap[requestID]
	s.mu.Unlock()
	if ok {
		logger.Info("[QueryService] 取消查询: requestID=%s", requestID)
		cancel()
	}
}

// ExecuteSQL 执行 Raw SQL（首次自动分页），根据数据库类型自动切换目标库
func (s *QueryService) ExecuteSQL(connID, dbName, sqlStr string) (*database.QueryResult, error) {
	return s.ExecuteSQLPagedContext(context.Background(), connID, dbName, sqlStr, 1, database.DefaultPageSize)
}

// ExecuteSQLPaged 分页执行 Raw SQL（page=0 不分页）
func (s *QueryService) ExecuteSQLPaged(connID, dbName, sqlStr string, page, pageSize int) (*database.QueryResult, error) {
	return s.ExecuteSQLPagedContext(context.Background(), connID, dbName, sqlStr, page, pageSize)
}

// ExecuteSQLPagedContext 分页执行 Raw SQL（page=0 不分页），支持取消。
//
//wails:ignore
func (s *QueryService) ExecuteSQLPagedContext(ctx context.Context, connID, dbName, sqlStr string, page, pageSize int) (*database.QueryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	logger.Info("[QueryService] 执行 SQL: connID=%s db=%s sql_len=%d page=%d pageSize=%d", connID, dbName, len(sqlStr), page, pageSize)

	var result *database.QueryResult
	err := withSession(ctx, s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if cfg.ReadOnly {
			if msg := readOnlyViolation(sqlStr); msg != "" {
				result = &database.QueryResult{Error: msg, ErrorCode: "read_only"}
				return nil
			}
		}
		var execErr error
		result, execErr = database.ExecuteQueryPagedContext(ctx, session, sqlStr, page, pageSize, true)
		return execErr
	})
	s.afterExecute(connID, dbName, sqlStr, result, err)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ExecuteReadOnlySQLContext 在只读事务内执行单条 SQL（供 AI 自动执行通道使用）。
// 语句分类护栏之外再加一层服务端强制：即使护栏被绕过，写操作也会被数据库拒绝。
// StarRocks 不支持只读事务，SQLite 的只读事务不拦截写，这两者仅依赖语句护栏。
//
//wails:ignore
func (s *QueryService) ExecuteReadOnlySQLContext(ctx context.Context, connID, dbName, sqlStr string, page, pageSize int) (*database.QueryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	logger.Info("[QueryService] 只读执行 SQL: connID=%s db=%s sql_len=%d", connID, dbName, len(sqlStr))

	var result *database.QueryResult
	err := withSession(ctx, s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if !database.SupportsReadOnlyTx(cfg.Type) {
			var execErr error
			result, execErr = database.ExecuteQueryPagedContext(ctx, session, sqlStr, page, pageSize, true)
			return execErr
		}
		tx, err := session.BeginReadOnly()
		if err != nil {
			return fmt.Errorf("开启只读事务失败: %w", err)
		}
		defer tx.Rollback()
		var execErr error
		result, execErr = database.ExecuteQueryPagedContext(ctx, tx, sqlStr, page, pageSize, true)
		return execErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ExecuteSQLBatch 在同一条数据库连接上按顺序执行一批语句，返回逐条结果。
//
// 同一连接保证了 SET 变量、显式事务、临时表等会话状态在语句之间有效。
// 遇到第一条失败的语句即停止，后续语句不再执行（避免事务中途出错后继续提交）。
// requestID 用于 CancelQuery；被取消时最后一条结果的 errorCode 为 "cancelled"。
func (s *QueryService) ExecuteSQLBatch(requestID, connID, dbName string, statements []string, page, pageSize int) ([]*database.QueryResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.registerCancel(requestID, cancel)
	defer s.clearCancel(requestID)

	logger.Info("[QueryService] 批量执行 SQL: requestID=%s connID=%s db=%s statements=%d", requestID, connID, dbName, len(statements))

	results := make([]*database.QueryResult, 0, len(statements))
	err := withSession(ctx, s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		openedTx := false
		defer func() {
			// 批次里开过事务但没有正常结束时，归还连接前必须回滚，
			// 否则未提交的事务会随连接回到连接池，污染后续不相关的操作。
			if openedTx {
				if _, rbErr := session.ExecContext(context.Background(), "ROLLBACK"); rbErr != nil {
					logger.Warn("[QueryService] 批次结束回滚失败: %v", rbErr)
				}
			}
		}()

		for i, stmt := range statements {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if ctx.Err() != nil {
				results = append(results, cancelledResult())
				return nil
			}
			switch database.SQLLeadingVerb(stmt) {
			case "begin", "start":
				openedTx = true
			case "commit", "rollback":
				openedTx = false
			}

			// 多语句时每条都从第一页开始；只有单条语句才使用调用方指定的页码
			stmtPage := page
			if len(statements) > 1 {
				stmtPage = 1
			}
			if cfg.ReadOnly {
				if msg := readOnlyViolation(stmt); msg != "" {
					results = append(results, &database.QueryResult{Error: msg, ErrorCode: "read_only"})
					return nil
				}
			}
			result, execErr := database.ExecuteQueryPagedContext(ctx, session, stmt, stmtPage, pageSize, true)
			if execErr != nil {
				if errors.Is(execErr, context.Canceled) || ctx.Err() != nil {
					results = append(results, cancelledResult())
					return nil
				}
				results = append(results, &database.QueryResult{Error: execErr.Error()})
				logger.Error("[QueryService] 批量执行第 %d 条出错: %v", i+1, execErr)
				return nil
			}
			results = append(results, result)
			s.afterExecute(connID, dbName, stmt, result, nil)
			if result.Error != "" {
				return nil
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return append(results, cancelledResult()), nil
		}
		return results, err
	}
	return results, nil
}

func cancelledResult() *database.QueryResult {
	return &database.QueryResult{
		Error:     "query cancelled",
		ErrorCode: queryErrorCodeCancelled,
		Rows:      []map[string]interface{}{},
		Columns:   []database.ColumnMeta{},
	}
}

// afterExecute 记录执行结果，并在结构变更后触发 schema 索引刷新。
func (s *QueryService) afterExecute(connID, dbName, sqlStr string, result *database.QueryResult, err error) {
	switch {
	case err != nil:
		logger.Error("[QueryService] SQL 执行出错: %v", err)
	case result == nil:
	case result.Error != "":
		logger.Warn("[QueryService] SQL 执行返回错误: %s", result.Error)
	default:
		logger.Info("[QueryService] SQL 执行成功: 行数=%d 总数=%d 耗时=%dms autoLimited=%v", int64(len(result.Rows)), result.Total, result.Duration, result.AutoLimited)
		if s.schema != nil && database.IsSchemaChangeSQL(sqlStr) {
			s.schema.MarkDirtyAndRefreshAsync(connID, dbName)
		}
	}
}

// QueryTableData 分页查询表数据
func (s *QueryService) QueryTableData(connID, dbName, table string, page, pageSize int, filters []database.Filter, sorts []database.Sort) (*database.QueryResult, error) {
	var result *database.QueryResult
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		var queryErr error
		result, queryErr = database.QueryTableData(session, cfg.Type, dbName, table, page, pageSize, filters, sorts)
		return queryErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// QueryTableDataWithRawInput 表数据查询：rawInput 为空时走筛选分页；否则由后端判断完整 SQL 或 WHERE 片段并执行（职责从前端下沉）
func (s *QueryService) QueryTableDataWithRawInput(connID, dbName, table string, page, pageSize int, filters []database.Filter, sorts []database.Sort, rawInput string) (*database.QueryResult, error) {
	if strings.TrimSpace(rawInput) == "" {
		return s.QueryTableData(connID, dbName, table, page, pageSize, filters, sorts)
	}
	logger.Info("[QueryService] QueryTableDataWithRawInput: connID=%s table=%s page=%d raw_len=%d", connID, table, page, len(rawInput))

	var result *database.QueryResult
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		ver, verr := database.GetServerVersion(session, cfg.Type)
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
		result, queryErr = database.ExecuteQueryPaged(session, sqlStr, page, pageSize, true)
		return queryErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// withTx 在固定连接上开启事务执行 fn；fn 返回错误则回滚，否则提交。
func (s *QueryService) withTx(connID, dbName string, fn func(tx *sql.Tx, cfg *database.ConnectionConfig) error) error {
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
	n := len(deletePKs) + len(inserts) + len(updates)
	if n == 0 {
		return nil
	}
	logger.Info("[QueryService] CommitTableDataChanges: table=%s deletes=%d inserts=%d updates=%d", table, len(deletePKs), len(inserts), len(updates))
	err := s.withTx(connID, dbName, func(tx *sql.Tx, cfg *database.ConnectionConfig) error {
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
		return err
	}
	logger.Info("[QueryService] CommitTableDataChanges 成功: table=%s", table)
	return nil
}

// DeleteRow 删除行
func (s *QueryService) DeleteRow(connID, dbName, table string, primaryKey map[string]interface{}) error {
	return withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		return database.DeleteRow(session, cfg.Type, dbName, table, primaryKey)
	})
}

// UpdateRow 更新行
func (s *QueryService) UpdateRow(connID, dbName, table string, primaryKey map[string]interface{}, changes map[string]interface{}) error {
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
	logger.Info("[QueryService] 插入行: table=%s", table)
	return withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if err := ensureWritable(cfg); err != nil {
			return err
		}
		return database.InsertRow(session, cfg.Type, dbName, table, row)
	})
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

// RowUpdate 单行更新：主键定位 + 变更字段
type RowUpdate struct {
	PrimaryKey map[string]interface{} `json:"primaryKey"`
	Changes    map[string]interface{} `json:"changes"`
}

// BatchUpdateRows 批量事务更新（多行修改一次提交）
func (s *QueryService) BatchUpdateRows(connID, dbName, table string, updates []RowUpdate) error {
	logger.Info("[QueryService] 事务批量更新: table=%s count=%d", table, len(updates))
	err := s.withTx(connID, dbName, func(tx *sql.Tx, cfg *database.ConnectionConfig) error {
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
	var defaults map[string]interface{}
	err := withSession(context.Background(), s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		cols, err := database.GetColumns(session, cfg.Type, dbName, table)
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
