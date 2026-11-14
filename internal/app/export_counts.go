package app

import (
	"context"
	"fmt"
	"minidb/internal/database"

	"minidb/internal/logger"
	"minidb/internal/model"

	"strings"
)

func (s *ExportService) queryRowCount(connID, dbName, sqlStr string) int64 {
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return 0
	}
	db, err := s.manager.GetDBForDatabase(connID, dbName)
	if err != nil {
		return 0
	}
	var n int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS __export_count__", strings.TrimRight(strings.TrimSpace(sqlStr), ";"))
	err = database.WithSession(context.Background(), db, cfg.Type, dbName, func(session *database.Session) error {
		return session.QueryRow(countSQL).Scan(&n)
	})
	if err != nil {
		logger.Warn("[ExportService] 获取查询结果总行数失败: %v", err)
		return 0
	}
	return n
}

// getTableRowCount 获取表的总行数
func (s *ExportService) getTableRowCount(connID, dbName, tableName string) (int64, error) {
	db, err := s.manager.GetDBForDatabase(connID, dbName)
	if err != nil {
		return 0, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return 0, model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	session, err := database.OpenSession(context.Background(), db, cfg.Type, dbName)
	if err != nil {
		return 0, err
	}
	defer session.Close()

	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		quotedTable = fmt.Sprintf("%s.%s", database.QuoteIdent(cfg.Type, dbName), quotedTable)
	} else if cfg.Type == database.Postgres {
		quotedTable = database.QuoteTableName(cfg.Type, tableName)
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s", quotedTable)
	var count int64
	if err := session.QueryRow(countSQL).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// detectCursorColumn 探测可用于 keyset 分页的单列主键。
// 仅当表存在且只存在一个主键列时返回该列名；无主键或组合主键返回空字符串（调用方回退 OFFSET）。
func (s *ExportService) detectCursorColumn(db database.Executor, cfg *database.ConnectionConfig, dbName, tableName string) string {
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	cols, err := database.GetColumns(ctx, db, cfg.Type, dbName, tableName)
	if err != nil {
		logger.Warn("[ExportService] 探测主键失败，回退 OFFSET 分页: %v", err)
		return ""
	}
	var pk string
	for _, c := range cols {
		if !c.IsPrimary {
			continue
		}
		if pk != "" {
			return "" // 组合主键不适合 keyset，回退 OFFSET
		}
		pk = c.Name
	}
	return pk
}

// getTableColumns 获取表的列名列表
func (s *ExportService) getTableColumns(db database.Executor, cfg *database.ConnectionConfig, dbName, tableName string) ([]string, error) {
	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		quotedTable = fmt.Sprintf("%s.%s", database.QuoteIdent(cfg.Type, dbName), quotedTable)
	} else if cfg.Type == database.Postgres {
		quotedTable = database.QuoteTableName(cfg.Type, tableName)
	}

	// LIMIT 0 只取元数据，不传输任何数据行
	probeSQL := database.SelectZeroRows(cfg.Type, quotedTable)
	rows, err := db.Query(probeSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	return colNames, nil
}
