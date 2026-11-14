package app

import (
	"context"
	"errors"

	"fmt"
	"minidb/internal/model"

	"sync"
	"sync/atomic"
	"time"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
)

// 查询结果错误码（供前端 i18n，不依赖后端文案）
const (
	queryErrorCodeCancelled = "cancelled"
	queryErrorCodeTimeout   = "timeout"
)

// QueryService 查询执行服务，封装 SQL 执行、表数据查询、行操作等功能
type QueryService struct {
	manager *database.Manager
	schema  *schemaindex.Manager

	// 进行中的查询：requestID -> cancel，用于前端主动停止
	mu        sync.Mutex
	cancelMap map[string]context.CancelFunc

	timeout atomic.Int64 // 手写查询的超时（纳秒），0 不限
}

// SetQueryTimeout 设置编辑器里批量执行的超时；d<=0 表示不限。
func (s *QueryService) SetQueryTimeout(d time.Duration) {
	if d < 0 {
		d = 0
	}
	s.timeout.Store(int64(d))
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
var errReadOnlyConnection = model.Errf(model.CodeReadOnlyConn, "当前连接为只读模式，已拒绝写操作（可在连接设置中关闭只读模式）")

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
// 该库开启了"手动提交"时，复用它独占的那条会话，这样才能看到并延续未提交的更改。
func withSession(ctx context.Context, manager *database.Manager, connID, dbName string, fn func(session *database.Session, cfg *database.ConnectionConfig) error) error {
	if mt := manager.ManualTx(connID, dbName); mt != nil {
		cfg, ok := manager.GetConfig(connID)
		if !ok {
			return model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
		}
		return mt.Use(ctx, func(session *database.Session) error { return fn(session, cfg) })
	}
	return withPooledSession(ctx, manager, connID, dbName, fn)
}

// withPooledSession 总是从连接池取一条新连接，不参与手动提交。
// 只读通道（AI 自动执行）要自己开只读事务，且只应看到已提交的数据，必须走这里。
func withPooledSession(ctx context.Context, manager *database.Manager, connID, dbName string, fn func(session *database.Session, cfg *database.ConnectionConfig) error) error {
	db, err := manager.GetDBForDatabase(connID, dbName)
	if err != nil {
		return model.Wrapf(model.CodeGetConnFailed, "获取数据库连接失败: %w", err)
	}
	cfg, ok := manager.GetConfig(connID)
	if !ok {
		return model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	return database.WithSession(ctx, db, cfg.Type, dbName, func(session *database.Session) error {
		return fn(session, cfg)
	})
}

// execStatement 在 session 上执行一条语句；手动提交模式（mt 非空）下写语句执行前会先开启事务，
// 执行后同步未提交条数与事务状态。
func execStatement(ctx context.Context, mt *database.ManualTx, session *database.Session, cfg *database.ConnectionConfig, stmt string, page, pageSize int) (*database.QueryResult, error) {
	if mt != nil {
		if err := mt.PrepareStatement(ctx, session, stmt); err != nil {
			return nil, err
		}
	}
	result, err := database.ExecuteQueryPagedFor(ctx, cfg.Type, session, stmt, page, pageSize, true)
	if mt != nil {
		failed := err
		if failed == nil && result != nil && result.Error != "" {
			failed = errors.New(result.Error)
		}
		mt.FinishStatement(stmt, failed)
	}
	return result, err
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
		result, execErr = execStatement(ctx, s.manager.ManualTx(connID, dbName), session, cfg, sqlStr, page, pageSize)
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
// SQLite 的只读事务不拦截写，改用 PRAGMA query_only；StarRocks 不支持只读事务，仅依赖语句护栏。
func (s *QueryService) ExecuteReadOnlySQLContext(ctx context.Context, connID, dbName, sqlStr string, page, pageSize int) (*database.QueryResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	logger.Info("[QueryService] 只读执行 SQL: connID=%s db=%s sql_len=%d", connID, dbName, len(sqlStr))

	var result *database.QueryResult
	err := withPooledSession(ctx, s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		if !database.SupportsReadOnlyTx(cfg.Type) {
			run := func() (err error) {
				result, err = database.ExecuteQueryPagedFor(ctx, cfg.Type, session, sqlStr, page, pageSize, true)
				return err
			}
			if database.CapsOf(cfg.Type).QueryOnlyPragma {
				return session.WithQueryOnly(run)
			}
			return run()
		}
		tx, err := session.BeginReadOnly()
		if err != nil {
			return fmt.Errorf("开启只读事务失败: %w", err)
		}
		defer tx.Rollback()
		var execErr error
		result, execErr = database.ExecuteQueryPagedFor(ctx, cfg.Type, tx, sqlStr, page, pageSize, true)
		return execErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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

// SetManualTransaction 开启或关闭 connID/dbName 上的手动提交模式。
// 关闭时若还有未提交的更改会返回错误，调用方应先提交或回滚。
func (s *QueryService) SetManualTransaction(connID, dbName string, on bool) error {
	if on {
		return s.manager.EnableManualTx(connID, dbName)
	}
	return s.manager.DisableManualTx(connID, dbName)
}

// TransactionStatus 返回手动提交状态快照（纯内存读取，可在界面线程调用）。
func (s *QueryService) TransactionStatus(connID, dbName string) model.TxStatus {
	return s.manager.ManualTxStatus(connID, dbName)
}

// CommitTransaction 提交手动提交模式下的当前事务。
func (s *QueryService) CommitTransaction(connID, dbName string) error {
	mt := s.manager.ManualTx(connID, dbName)
	if mt == nil {
		return nil
	}
	if err := mt.Commit(context.Background()); err != nil {
		return err
	}
	s.refreshAfterTx(connID, dbName)
	return nil
}

// RollbackTransaction 回滚手动提交模式下的当前事务。
func (s *QueryService) RollbackTransaction(connID, dbName string) error {
	mt := s.manager.ManualTx(connID, dbName)
	if mt == nil {
		return nil
	}
	if err := mt.Rollback(context.Background()); err != nil {
		return err
	}
	s.refreshAfterTx(connID, dbName)
	return nil
}

// refreshAfterTx 事务结束后让结构缓存重新对齐（回滚可能撤销了 DDL）。
func (s *QueryService) refreshAfterTx(connID, dbName string) {
	if s.schema != nil {
		s.schema.MarkDirtyAndRefreshAsync(connID, dbName)
	}
}
