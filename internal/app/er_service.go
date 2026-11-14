package app

import (
	"context"
	"strings"

	"minidb/internal/ai"
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/model"
)

// ERService 把一个库的表和外键整理成 ER 图的数据（节点与边）。
// 表和列取自表结构索引的缓存（没有就现建一次），外键用方言的批量接口一次取回。
type ERService struct {
	manager *database.Manager
	schema  func(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error)
	refresh func(ctx context.Context, connID, dbName string) error // 强制重建索引；索引落后于库时用
}

func NewERService(manager *database.Manager, schema func(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error), refresh func(ctx context.Context, connID, dbName string) error) *ERService {
	return &ERService{manager: manager, schema: schema, refresh: refresh}
}

// CanER 该库是否能画 ER 图：需要外键元数据。
func CanER(t database.DBType) bool { return database.CapsOf(t).ForeignKeys }

// Graph 返回库的完整关系图，节点和边都按名字排好序。
// 视图不画；引用了本库之外的表（如 PostgreSQL 的其他模式）的外键不返回。
func (s *ERService) Graph(ctx context.Context, connID, dbName string) (*model.ERGraph, error) {
	defer logger.Perf("app.ER.Graph", "conn", connID, "db", dbName)()
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	if !CanER(cfg.Type) {
		return nil, model.Errf(model.CodeERUnsupported, "%s 不支持外键，无法画 ER 图", DialectLabel(cfg.Type))
	}
	ctx, cancel := context.WithTimeout(ctx, statementTimeout)
	defer cancel()
	db, err := s.manager.GetDBForDatabase(connID, dbName)
	if err != nil {
		return nil, model.Wrapf(model.CodeGetConnFailed, "获取数据库连接失败: %w", err)
	}
	infos, err := database.GetTables(ctx, db, cfg.Type, dbName)
	if err != nil {
		return nil, err
	}
	isTable := map[string]bool{}
	var names []string
	for _, t := range infos {
		if t.Type != "view" {
			isTable[t.Name] = true
			names = append(names, t.Name)
		}
	}
	sc, err := s.schema(ctx, connID, dbName)
	if err != nil {
		return nil, err
	}
	if s.refresh != nil && missingTables(sc, names) { // 缓存落后于库（刚建了表）：重建一次
		if err := s.refresh(ctx, connID, dbName); err != nil {
			return nil, err
		}
		if sc, err = s.schema(ctx, connID, dbName); err != nil {
			return nil, err
		}
	}
	fks, err := database.AllForeignKeys(ctx, db, cfg.Type, dbName, names)
	if err != nil {
		return nil, err
	}
	g := buildERGraph(sc, isTable, fks)
	logger.Info("[ERService] ER 图: connID=%s db=%s 表=%d 外键=%d", connID, dbName, len(g.Nodes), len(g.Edges))
	return g, nil
}

// missingTables 报告库里有没有表是表结构快照里没有的。
func missingTables(sc *ai.SchemaContext, names []string) bool {
	have := map[string]bool{}
	for _, t := range sc.Tables {
		have[t.Name] = true
	}
	for _, n := range names {
		if !have[n] {
			return true
		}
	}
	return false
}

// buildERGraph 是纯函数：表结构快照 + 外键 → 图。isTable 限定要画的表。
func buildERGraph(sc *ai.SchemaContext, isTable map[string]bool, fks []database.TableForeignKey) *model.ERGraph {
	g := &model.ERGraph{}
	fkCols := map[string]map[string]bool{}
	for _, fk := range fks {
		if !isTable[fk.Table] || !isTable[fk.RefTable] {
			continue
		}
		g.Edges = append(g.Edges, model.EREdge{Name: fk.Name, From: fk.Table, To: fk.RefTable,
			FromCols: fk.Columns, ToCols: fk.RefColumns, OnDelete: fk.OnDelete, OnUpdate: fk.OnUpdate})
		if fkCols[fk.Table] == nil {
			fkCols[fk.Table] = map[string]bool{}
		}
		for _, c := range fk.Columns {
			fkCols[fk.Table][c] = true
		}
	}
	if sc != nil {
		for _, t := range sc.Tables {
			if !isTable[t.Name] {
				continue
			}
			n := model.ERNode{Name: t.Name, Comment: strings.TrimSpace(t.Comment)}
			for _, c := range t.Columns {
				n.Columns = append(n.Columns, model.ERColumn{Name: c.Name, Type: c.Type, PK: c.IsPrimary, FK: fkCols[t.Name][c.Name]})
			}
			g.Nodes = append(g.Nodes, n)
		}
	}
	g.Sorted()
	return g
}
