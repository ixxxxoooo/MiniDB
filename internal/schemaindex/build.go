package schemaindex

import (
	"fmt"
	"sync"

	"minidb/internal/ai"
	"minidb/internal/database"
	"minidb/internal/logger"
)

// tableFetchWorkers 构建索引时并发读取表元数据的上限，避免占满连接池。
const tableFetchWorkers = 6

// BuildSchemaFromDatabaseManager 读取一个库的表、列、外键与索引，构建 schema 快照。
func BuildSchemaFromDatabaseManager(dbManager *database.Manager, connID, dbName string) (*ai.SchemaContext, error) {
	db, err := dbManager.GetDBForDatabase(connID, dbName)
	if err != nil {
		return nil, err
	}
	cfg, ok := dbManager.GetConfig(connID)
	if !ok {
		return nil, fmt.Errorf("连接配置不存在: %s", connID)
	}

	tables, err := database.GetTables(db, cfg.Type, dbName)
	if err != nil {
		return nil, err
	}

	dbVersion := ""
	ver, verErr := database.GetServerVersion(db, cfg.Type)
	if verErr == nil {
		dbVersion = ver
	}

	schema := &ai.SchemaContext{
		DatabaseType:    cfg.Type,
		DatabaseName:    dbName,
		DatabaseVersion: dbVersion,
	}

	// 每张表的列信息各需一次查询：表多时串行会很慢，所以用有限的并发取回，
	// 结果按表原来的顺序放进 schema，输出保持确定。
	type tableResult struct {
		schema ai.TableSchema
		ok     bool
	}
	results := make([]tableResult, len(tables))
	var wg sync.WaitGroup
	sem := make(chan struct{}, tableFetchWorkers)
	for i, t := range tables {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, t database.TableInfo) {
			defer wg.Done()
			defer func() { <-sem }()
			cols, err := database.GetColumns(db, cfg.Type, dbName, t.Name)
			if err != nil {
				logger.Warn("[SchemaIndex] 获取表 %s 列信息失败: %v", t.Name, err)
				return
			}
			tableSchema := ai.TableSchema{Name: t.Name, Comment: t.Comment}
			for _, c := range cols {
				defaultVal := ""
				if c.DefaultValue != nil {
					defaultVal = *c.DefaultValue
				}
				tableSchema.Columns = append(tableSchema.Columns, ai.ColumnSchema{
					Name:         c.Name,
					Type:         c.Type,
					Nullable:     c.Nullable,
					Comment:      c.Comment,
					IsPrimary:    c.IsPrimary,
					DefaultValue: defaultVal,
					ForeignKey:   c.ForeignKey,
				})
			}
			tableSchema.Indexes = fetchIndexes(db, cfg.Type, dbName, t.Name)
			results[i] = tableResult{schema: tableSchema, ok: true}
		}(i, t)
	}
	wg.Wait()
	failed := 0
	for _, r := range results {
		if r.ok {
			schema.Tables = append(schema.Tables, r.schema)
		} else {
			failed++
		}
	}
	// 一张表都没取到（比如构建途中连接被关闭了）不是"空库"：报错，让调用方保留旧索引，
	// 否则空索引会被存下来，AI 就会以为库里没有表和字段。
	if len(tables) > 0 && len(schema.Tables) == 0 {
		return nil, fmt.Errorf("读取 %d 张表的字段信息全部失败（连接可能已关闭），本次不更新索引", len(tables))
	}
	if failed > 0 {
		logger.Warn("[SchemaIndex] %d/%d 张表读取字段失败，索引中缺少这些表", failed, len(tables))
	}

	logger.Info("[SchemaIndex] 构建 schema 完成: db=%s tables=%d", dbName, len(schema.Tables))
	return schema, nil
}

// fetchIndexes 读取一张表的索引；失败（如权限不足）只丢掉索引信息，不影响表和列。
func fetchIndexes(db database.Executor, dbType, dbName, table string) []ai.IndexSchema {
	infos, err := database.GetIndexes(db, dbType, dbName, table)
	if err != nil {
		logger.Debug("[SchemaIndex] 读取表 %s 的索引失败: %v", table, err)
		return nil
	}
	out := make([]ai.IndexSchema, 0, len(infos))
	for _, info := range infos {
		out = append(out, ai.IndexSchema{Name: info.Name, Columns: info.Columns, Unique: info.IsUnique, Primary: info.IsPrimary})
	}
	return out
}
