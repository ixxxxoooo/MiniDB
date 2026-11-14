package app

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"minidb/internal/database"
	"minidb/internal/logger"
	"sync/atomic"
	"time"
)

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
	// 超时和手动取消走同一条路：到点就 cancel，再把"已取消"换成"超时"的提示。
	var timedOut atomic.Bool
	limit := time.Duration(s.timeout.Load())
	if limit > 0 {
		timer := time.AfterFunc(limit, func() { timedOut.Store(true); cancel() })
		defer timer.Stop()
	}
	results, err := s.executeBatch(ctx, connID, dbName, statements, page, pageSize)
	if timedOut.Load() && len(results) > 0 && results[len(results)-1].ErrorCode == queryErrorCodeCancelled {
		results[len(results)-1] = &database.QueryResult{
			Error:     fmt.Sprintf("query timed out after %s", limit),
			ErrorCode: queryErrorCodeTimeout,
			Rows:      []map[string]interface{}{},
			Columns:   []database.ColumnMeta{},
		}
	}
	return results, err
}

func (s *QueryService) executeBatch(ctx context.Context, connID, dbName string, statements []string, page, pageSize int) ([]*database.QueryResult, error) {
	logger.Info("[QueryService] 批量执行 SQL: connID=%s db=%s statements=%d", connID, dbName, len(statements))

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
			result, execErr := database.ExecuteQueryPagedFor(ctx, cfg.Type, session, stmt, stmtPage, pageSize, true)
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
