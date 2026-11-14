package app

import (
	"fmt"
	"minidb/internal/model"

	"minidb/internal/database"
	"minidb/internal/logger"
)

// usersConn 取出管理用户要用的连接，并确认方言支持。
func (s *DatabaseService) usersConn(connID string) (*database.ConnectionConfig, error) {
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	if !database.SupportsUsers(cfg.Type) {
		return nil, model.Errf(model.CodeUnsupportedUsers, "%s 不支持管理用户", cfg.Type)
	}
	return cfg, nil
}

// GetUsers 列出服务器上的账号。
func (s *DatabaseService) GetUsers(connID string) ([]database.UserInfo, error) {
	cfg, err := s.usersConn(connID)
	if err != nil {
		return nil, err
	}
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, model.Wrapf(model.CodeGetConnFailed, "获取数据库连接失败: %w", err)
	}
	ctx, cancel := withTimeout(adminTimeout)
	defer cancel()
	return database.GetUsers(ctx, db, cfg.Type)
}

// GetUserGrants 读出账号现有的授权。
func (s *DatabaseService) GetUserGrants(connID, name, host string) ([]string, error) {
	cfg, err := s.usersConn(connID)
	if err != nil {
		return nil, err
	}
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return nil, model.Wrapf(model.CodeGetConnFailed, "获取数据库连接失败: %w", err)
	}
	ctx, cancel := withTimeout(adminTimeout)
	defer cancel()
	return database.GetUserGrants(ctx, db, cfg.Type, name, host)
}

// ApplyUserStatements 按顺序执行改账号和权限的语句，遇到错误就停。
// 返回已成功执行的条数；这类语句多数不能回滚，所以失败时前面的会保留。
func (s *DatabaseService) ApplyUserStatements(connID string, stmts []string) (int, error) {
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return 0, model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	if err := ensureWritable(cfg); err != nil {
		return 0, err
	}
	for i, st := range stmts {
		// 日志里只记动词：语句里可能带密码
		logger.Info("[DatabaseService] 执行账号语句: verb=%s", database.SQLLeadingVerb(st))
		if err := s.execServerLevel(connID, st); err != nil {
			return i, fmt.Errorf("第 %d 条语句失败（前面 %d 条已执行）: %w", i+1, i, err)
		}
	}
	return len(stmts), nil
}
