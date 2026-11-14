package app

import (
	"context"

	"minidb/internal/ai"
	"minidb/internal/schemaindex"
)

// SchemaIndexService 对前端暴露 schema 索引状态与刷新能力。
type SchemaIndexService struct {
	manager *schemaindex.Manager
}

func NewSchemaIndexService(manager *schemaindex.Manager) *SchemaIndexService {
	return &SchemaIndexService{manager: manager}
}

func (s *SchemaIndexService) GetSchemaIndexStatus(connID, dbName string) (schemaindex.Status, error) {
	return s.manager.GetStatus(connID, dbName)
}

func (s *SchemaIndexService) RefreshSchemaIndex(connID, dbName string) (schemaindex.Status, error) {
	return s.manager.Refresh(context.Background(), connID, dbName, schemaindex.RefreshReasonManual)
}

// WarmSchemaIndex 在后台预热当前库 schema 索引。
func (s *SchemaIndexService) WarmSchemaIndex(connID, dbName string) {
	s.manager.WarmAsync(connID, dbName)
}

// GetSchema 返回库的结构快照（表、列、索引），供补全等只读用途；索引未建好时会同步建一次。
func (s *SchemaIndexService) GetSchema(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error) {
	return s.manager.GetSchema(ctx, connID, dbName)
}
