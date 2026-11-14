package schemaindex

import (
	"context"
	"time"

	"minidb/internal/ai"
	"minidb/internal/database"
)

const (
	bucketSchemaIndexes = "schema_indexes"
	defaultRefreshTTL   = 15 * time.Minute
	// recordVersion 索引记录的格式版本。记录里新增信息（如数据库索引）后递增：
	// 旧版本的记录仍可使用，但会被视为过期并在后台重建。
	recordVersion = 2
)

type RefreshReason string

const (
	RefreshReasonPrebuild  RefreshReason = "prebuild"
	RefreshReasonManual    RefreshReason = "manual"
	RefreshReasonDDL       RefreshReason = "ddl"
	RefreshReasonScheduled RefreshReason = "scheduled"
	RefreshReasonOnDemand  RefreshReason = "on_demand"
)

// Source 说明索引内容来自哪里。
type Source string

const (
	SourceMemory    Source = "memory"
	SourcePersisted Source = "persisted"
	SourceRebuilt   Source = "rebuilt"
)

type ConfigResolver func(connID string) (*database.ConnectionConfig, bool)
type BuilderFunc func(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error)

type Status struct {
	SchemaKey       string `json:"schemaKey,omitempty"`
	DatabaseName    string `json:"databaseName,omitempty"`
	Exists          bool   `json:"exists"`
	Refreshing      bool   `json:"refreshing"`
	Dirty           bool   `json:"dirty"`
	Stale           bool   `json:"stale"`
	LastRefreshedAt string `json:"lastRefreshedAt,omitempty"`
	LastError       string `json:"lastError,omitempty"`
	TableCount      int    `json:"tableCount"`
	Source          Source `json:"source,omitempty"`
}

type StatusEvent struct {
	Reason string `json:"reason"`
	Status Status `json:"status"`
}

type Record struct {
	Version         int               `json:"version,omitempty"`
	SchemaKey       string            `json:"schemaKey"`
	DatabaseName    string            `json:"databaseName"`
	DatabaseType    string            `json:"databaseType"`
	Schema          *ai.SchemaContext `json:"schema"`
	TableCount      int               `json:"tableCount"`
	RefreshedAtUnix int64             `json:"refreshedAtUnix"`
	SourceVersion   string            `json:"sourceVersion,omitempty"`
	SchemaHash      string            `json:"schemaHash,omitempty"`
	LastError       string            `json:"lastError,omitempty"`
	Dirty           bool              `json:"dirty"`
	RefreshReason   RefreshReason     `json:"refreshReason"`
}

type cacheEntry struct {
	record     Record
	source     Source
	refreshing bool
}

func (e cacheEntry) hasRecord() bool {
	return e.record.Schema != nil
}

type activeTarget struct {
	connID   string
	dbName   string
	lastUsed time.Time
}

type resolvedTarget struct {
	schemaKey string
	connID    string
	dbName    string
}
