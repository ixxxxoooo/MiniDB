package app

import (
	"context"
	"fmt"
	"minidb/internal/model"
	"strings"

	"minidb/internal/database"
	"minidb/internal/logger"
)

// 表之外的对象（外键、触发器、函数与存储过程）、库级操作、表克隆和库内搜索。
// 读取走连接的元数据查询；写入一律经 ExecuteRawSQL，只读连接与 schema 刷新都在那里处理。

// GetForeignKeys 读取表上的外键约束。
func (s *DatabaseService) GetForeignKeys(connID, dbName, table string) ([]database.ForeignKeyInfo, error) {
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetForeignKeys(ctx, c.DB, c.Cfg.Type, dbName, table)
}

// GetTriggers 读取表上的触发器。
func (s *DatabaseService) GetTriggers(connID, dbName, table string) ([]database.TriggerInfo, error) {
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetTriggers(ctx, c.DB, c.Cfg.Type, dbName, table)
}

// GetRoutines 列出库里的函数与存储过程。
func (s *DatabaseService) GetRoutines(connID, dbName string) ([]database.RoutineInfo, error) {
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetRoutines(ctx, c.DB, c.Cfg.Type, dbName)
}

// GetRoutineDDL 读取函数、存储过程（kind 为 function / procedure）或视图（view）的定义。
func (s *DatabaseService) GetRoutineDDL(connID, dbName, kind, name string) (string, error) {
	if kind == "view" {
		return s.GetDDL(connID, dbName, name)
	}
	c, err := s.with(connID, dbName)
	if err != nil {
		return "", err
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	return database.GetRoutineDDL(ctx, c.DB, c.Cfg.Type, dbName, kind, name)
}

// ApplyRoutineDDL 用新的定义替换函数、存储过程或视图。MySQL 系与 SQLite 没有统一的 CREATE OR REPLACE，
// 先删后建；建失败时把旧定义建回去，不留下空缺。PostgreSQL 的定义本身就是 CREATE OR REPLACE。
func (s *DatabaseService) ApplyRoutineDDL(connID, dbName, kind, name, ddl string) error {
	c, err := s.with(connID, dbName)
	if err != nil {
		return err
	}
	cfg := c.Cfg
	if strings.TrimSpace(ddl) == "" {
		return fmt.Errorf("定义不能为空")
	}
	if database.CanReplaceInPlace(cfg.Type, kind, ddl) {
		return s.ExecuteRawSQL(connID, dbName, ddl)
	}
	old, oldErr := s.GetRoutineDDL(connID, dbName, kind, name)
	var drop string
	if kind == "view" {
		drop = database.BuildDropViewSQL(cfg.Type, name)
	} else if drop, err = database.BuildDropRoutineSQL(cfg.Type, kind, name); err != nil {
		return err
	}
	if err := s.ExecuteRawSQL(connID, dbName, drop); err != nil {
		return err
	}
	if err := s.ExecuteRawSQL(connID, dbName, ddl); err != nil {
		if oldErr == nil {
			if restoreErr := s.ExecuteRawSQL(connID, dbName, old); restoreErr != nil {
				logger.Error("[DatabaseService] 还原例程 %s 失败: %v", name, restoreErr)
				return fmt.Errorf("%w（原定义也未能还原: %v）", err, restoreErr)
			}
		}
		return err
	}
	return nil
}

// DropView 删除视图。
func (s *DatabaseService) DropView(connID, dbName, name string) error {
	c, err := s.with(connID, dbName)
	if err != nil {
		return err
	}
	return s.ExecuteRawSQL(connID, dbName, database.BuildDropViewSQL(c.Cfg.Type, name))
}

// DropRoutine 删除函数或存储过程。
func (s *DatabaseService) DropRoutine(connID, dbName, kind, name string) error {
	c, err := s.with(connID, dbName)
	if err != nil {
		return err
	}
	stmt, err := database.BuildDropRoutineSQL(c.Cfg.Type, kind, name)
	if err != nil {
		return err
	}
	return s.ExecuteRawSQL(connID, dbName, stmt)
}

// CloneTable 复制表结构，withData 时连数据一起复制。
func (s *DatabaseService) CloneTable(connID, dbName, src, dst string, withData bool) error {
	c, err := s.with(connID, dbName)
	if err != nil {
		return err
	}
	cfg := c.Cfg
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	stmts, err := database.CloneTableStatements(ctx, c.DB, cfg.Type, dbName, src, dst, withData)
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		if err := s.ExecuteRawSQL(connID, dbName, stmt); err != nil {
			return err
		}
	}
	return nil
}

// CreateDatabase 新建数据库。
func (s *DatabaseService) CreateDatabase(connID, name, charset string) error {
	cfg, err := resolveConfig(s.manager, connID)
	if err != nil {
		return err
	}
	stmt, err := database.BuildCreateDatabaseSQL(cfg.Type, name, charset)
	if err != nil {
		return err
	}
	return s.execServerLevel(connID, stmt)
}

// DropDatabase 删除数据库。
func (s *DatabaseService) DropDatabase(connID, name string) error {
	cfg, err := resolveConfig(s.manager, connID)
	if err != nil {
		return err
	}
	stmt, err := database.BuildDropDatabaseSQL(cfg.Type, name)
	if err != nil {
		return err
	}
	logger.Warn("[DatabaseService] DROP 数据库: %s", name)
	return s.execServerLevel(connID, stmt)
}

// RenameDatabase 重命名数据库。MySQL 与 TiDB 没有这个语句：新建库、把表搬过去，再删旧库；
// 旧库里还有视图等其它对象时拒绝，免得丢东西。
func (s *DatabaseService) RenameDatabase(connID, oldName, newName string) error {
	c, err := s.with(connID, oldName)
	if err != nil {
		return err
	}
	cfg := c.Cfg
	stmt, err := database.BuildRenameDatabaseSQL(cfg.Type, oldName, newName)
	if err != nil {
		return err
	}
	if stmt != "" {
		return s.execServerLevel(connID, stmt)
	}
	ctx, cancel := withTimeout(metaTimeout)
	defer cancel()
	tables, err := database.GetTables(ctx, c.DB, cfg.Type, oldName)
	if err != nil {
		return err
	}
	var base []string
	for _, t := range tables {
		if t.Type != "table" {
			return fmt.Errorf("库里有视图 %s，重命名会丢失它，请先处理", t.Name)
		}
		base = append(base, t.Name)
	}
	if rs, _ := database.GetRoutines(ctx, c.DB, cfg.Type, oldName); len(rs) > 0 {
		return fmt.Errorf("库里有函数或存储过程 %s，重命名会丢失它，请先处理", rs[0].Name)
	}
	create, err := database.BuildCreateDatabaseSQL(cfg.Type, newName, "")
	if err != nil {
		return err
	}
	if err := s.execServerLevel(connID, create); err != nil {
		return err
	}
	if move := database.BuildMoveTablesSQL(cfg.Type, oldName, newName, base); move != "" {
		if err := s.execServerLevel(connID, move); err != nil {
			return err
		}
	}
	drop, _ := database.BuildDropDatabaseSQL(cfg.Type, oldName)
	return s.execServerLevel(connID, drop)
}

// execServerLevel 执行不属于任何库的语句（建库、删库…），只读连接拒绝。
func (s *DatabaseService) execServerLevel(connID, stmt string) error {
	c, err := s.with(connID, "")
	if err != nil {
		return err
	}
	if err := ensureWritable(c.Cfg); err != nil {
		return err
	}
	logger.Info("[DatabaseService] 执行库级语句: verb=%s", database.SQLLeadingVerb(stmt))
	if _, err := c.DB.ExecContext(context.Background(), stmt); err != nil {
		return model.Wrapf(model.CodeSQLExecFailed, "执行 SQL 失败: %w", err)
	}
	return nil
}

// SearchHit 是库内搜索里一张命中的表。
type SearchHit struct {
	Table string `json:"table"`
	Count int64  `json:"count"`
}

// SearchDatabase 在库的每张表的所有列里找包含 term 的行，返回有命中的表与命中行数。
// 单张表出错（没权限、视图不可查…）时跳过，不中断整个搜索。
func (s *DatabaseService) SearchDatabase(connID, dbName, term string) ([]SearchHit, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, fmt.Errorf("搜索内容不能为空")
	}
	c, err := s.with(connID, dbName)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout(longMetaWindow)
	defer cancel()
	tables, err := database.GetTables(ctx, c.DB, c.Cfg.Type, dbName)
	if err != nil {
		return nil, err
	}
	var hits []SearchHit
	for _, t := range tables {
		if ctx.Err() != nil {
			return hits, ctx.Err()
		}
		n, err := database.CountTableData(ctx, c.DB, c.Cfg.Type, dbName, t.Name, []database.Filter{{Column: "__any", Operator: "LIKE", Value: term}})
		if err != nil {
			logger.Debug("[DatabaseService] 库内搜索跳过 %s: %v", t.Name, err)
			continue
		}
		if n > 0 {
			hits = append(hits, SearchHit{Table: t.Name, Count: n})
		}
	}
	return hits, nil
}
