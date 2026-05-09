package schemaindex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"minidb/internal/ai"
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/storage"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sync/singleflight"
)

const (
	bucketSchemaIndexes = "schema_indexes"
	defaultRefreshTTL   = 15 * time.Minute
)

type RefreshReason string

const (
	RefreshReasonPrebuild  RefreshReason = "prebuild"
	RefreshReasonManual    RefreshReason = "manual"
	RefreshReasonDDL       RefreshReason = "ddl"
	RefreshReasonScheduled RefreshReason = "scheduled"
	RefreshReasonOnDemand  RefreshReason = "on_demand"
)

type Source string

const (
	SourceMemory    Source = "memory"
	SourcePersisted Source = "persisted"
	SourceRebuilt   Source = "rebuilt"
)

type ConfigResolver func(connID string) (*database.ConnectionConfig, bool)
type BuilderFunc func(connID, dbName string) (*ai.SchemaContext, error)

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

type Manager struct {
	store           *storage.Store
	resolveConfig   ConfigResolver
	buildSchema     BuilderFunc
	refreshInterval time.Duration

	app *application.App

	mu            sync.RWMutex
	cache         map[string]*cacheEntry
	activeTargets map[string]activeTarget
	refreshGroup  singleflight.Group

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}
}

func NewManager(store *storage.Store, resolveConfig ConfigResolver, buildSchema BuilderFunc) *Manager {
	return &Manager{
		store:           store,
		resolveConfig:   resolveConfig,
		buildSchema:     buildSchema,
		refreshInterval: defaultRefreshTTL,
		cache:           make(map[string]*cacheEntry),
		activeTargets:   make(map[string]activeTarget),
	}
}

// BuildSchemaFromDatabaseManager 使用数据库管理器构建 schema。
func BuildSchemaFromDatabaseManager(dbManager *database.Manager, connID, dbName string) (*ai.SchemaContext, error) {
	db, err := dbManager.GetDB(connID)
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

	for _, t := range tables {
		cols, err := database.GetColumns(db, cfg.Type, dbName, t.Name)
		if err != nil {
			logger.Warn("[SchemaIndex] 获取表 %s 列信息失败: %v", t.Name, err)
			continue
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
		schema.Tables = append(schema.Tables, tableSchema)
	}

	logger.Info("[SchemaIndex] 构建 schema 完成: db=%s tables=%d", dbName, len(schema.Tables))
	return schema, nil
}

func (m *Manager) SetWailsApplication(app *application.App) {
	m.app = app
}

func (m *Manager) Start() {
	m.startOnce.Do(func() {
		m.stopCh = make(chan struct{})
		go m.runTicker()
	})
}

func (m *Manager) Shutdown() {
	m.stopOnce.Do(func() {
		if m.stopCh != nil {
			close(m.stopCh)
		}
	})
}

func (m *Manager) GetSchema(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error) {
	target, err := m.resolveTarget(connID, dbName)
	if err != nil {
		return nil, err
	}
	m.touchTarget(target)

	entry, ok, err := m.loadEntry(target.schemaKey)
	if err != nil {
		return nil, err
	}
	if ok && entry.hasRecord() {
		if entry.record.Dirty {
			record, err := m.refreshTarget(ctx, target, RefreshReasonDDL)
			if err != nil {
				return nil, err
			}
			return record.Schema, nil
		}
		if m.isStale(entry.record) {
			m.refreshAsync(target, RefreshReasonOnDemand)
		}
		return entry.record.Schema, nil
	}

	record, err := m.refreshTarget(ctx, target, RefreshReasonOnDemand)
	if err != nil {
		return nil, err
	}
	return record.Schema, nil
}

func (m *Manager) GetStatus(connID, dbName string) (Status, error) {
	target, err := m.resolveTarget(connID, dbName)
	if err != nil {
		return Status{}, err
	}
	m.touchTarget(target)

	entry, ok, err := m.loadEntry(target.schemaKey)
	if err != nil {
		return Status{}, err
	}
	if !ok {
		return Status{
			SchemaKey:    target.schemaKey,
			DatabaseName: target.dbName,
		}, nil
	}
	return m.statusFromEntry(target.dbName, entry), nil
}

func (m *Manager) Refresh(ctx context.Context, connID, dbName string, reason RefreshReason) (Status, error) {
	target, err := m.resolveTarget(connID, dbName)
	if err != nil {
		return Status{}, err
	}
	m.touchTarget(target)

	record, err := m.refreshTarget(ctx, target, reason)
	if err != nil {
		return Status{}, err
	}
	entry, _, loadErr := m.loadEntry(target.schemaKey)
	if loadErr != nil {
		return Status{}, loadErr
	}
	if entry.hasRecord() {
		return m.statusFromEntry(target.dbName, entry), nil
	}
	return m.statusFromRecord(record, SourceRebuilt, false), nil
}

func (m *Manager) WarmAsync(connID, dbName string) {
	target, err := m.resolveTarget(connID, dbName)
	if err != nil {
		logger.Warn("[SchemaIndex] WarmAsync resolve target failed: %v", err)
		return
	}
	m.touchTarget(target)

	entry, ok, err := m.loadEntry(target.schemaKey)
	if err != nil {
		logger.Warn("[SchemaIndex] WarmAsync load entry failed: %v", err)
		return
	}
	if ok && entry.refreshing {
		return
	}
	if !ok || !entry.hasRecord() || entry.record.Dirty || m.isStale(entry.record) {
		m.refreshAsync(target, RefreshReasonPrebuild)
	}
}

func (m *Manager) MarkDirtyAndRefreshAsync(connID, dbName string) {
	target, err := m.resolveTarget(connID, dbName)
	if err != nil {
		logger.Warn("[SchemaIndex] MarkDirtyAndRefreshAsync resolve target failed: %v", err)
		return
	}
	m.touchTarget(target)

	entry, ok, err := m.loadEntry(target.schemaKey)
	if err != nil {
		logger.Warn("[SchemaIndex] MarkDirtyAndRefreshAsync load entry failed: %v", err)
		return
	}
	if ok && entry.hasRecord() {
		record := entry.record
		record.Dirty = true
		record.RefreshReason = RefreshReasonDDL
		if err := m.persistRecord(record); err != nil {
			logger.Warn("[SchemaIndex] 持久化 dirty 状态失败: %v", err)
		}
		m.setEntry(record, entry.source, entry.refreshing)
	}
	m.refreshAsync(target, RefreshReasonDDL)
}

func (m *Manager) ForgetConnection(connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for schemaKey, target := range m.activeTargets {
		if target.connID == connID {
			delete(m.activeTargets, schemaKey)
		}
	}
}

func (m *Manager) runTicker() {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.runScheduledRefresh()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) runScheduledRefresh() {
	targets := m.snapshotActiveTargets()
	for _, target := range targets {
		entry, ok, err := m.loadEntry(target.schemaKey)
		if err != nil {
			logger.Warn("[SchemaIndex] 定时刷新读取索引失败: %v", err)
			continue
		}
		if !ok || !entry.hasRecord() {
			continue
		}
		if entry.record.Dirty || m.isStale(entry.record) {
			m.refreshAsync(target, RefreshReasonScheduled)
		}
	}
}

func (m *Manager) snapshotActiveTargets() []resolvedTarget {
	m.mu.RLock()
	targets := make([]resolvedTarget, 0, len(m.activeTargets))
	lastUsed := make(map[string]time.Time, len(m.activeTargets))
	for schemaKey, target := range m.activeTargets {
		targets = append(targets, resolvedTarget{
			schemaKey: schemaKey,
			connID:    target.connID,
			dbName:    target.dbName,
		})
		lastUsed[schemaKey] = target.lastUsed
	}
	m.mu.RUnlock()

	sort.SliceStable(targets, func(i, j int) bool {
		return lastUsed[targets[i].schemaKey].After(lastUsed[targets[j].schemaKey])
	})
	return targets
}

func (m *Manager) refreshAsync(target resolvedTarget, reason RefreshReason) {
	go func() {
		if _, err := m.refreshTarget(context.Background(), target, reason); err != nil {
			logger.Warn("[SchemaIndex] 后台刷新失败: key=%s db=%s reason=%s err=%v", target.schemaKey, target.dbName, reason, err)
		}
	}()
}

func (m *Manager) refreshTarget(ctx context.Context, target resolvedTarget, reason RefreshReason) (Record, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}

	result, err, _ := m.refreshGroup.Do(target.schemaKey, func() (interface{}, error) {
		m.setRefreshing(target.schemaKey, target.dbName, true, string(reason))
		defer m.setRefreshing(target.schemaKey, target.dbName, false, string(reason))

		existing, hasExisting, loadErr := m.loadEntry(target.schemaKey)
		if loadErr != nil {
			return Record{}, loadErr
		}

		schema, buildErr := m.buildSchema(target.connID, target.dbName)
		if buildErr != nil {
			if hasExisting && existing.hasRecord() {
				record := existing.record
				record.LastError = buildErr.Error()
				record.Dirty = record.Dirty || reason == RefreshReasonDDL
				record.RefreshReason = reason
				if err := m.persistRecord(record); err != nil {
					return Record{}, err
				}
				m.setEntry(record, existing.source, false)
			}
			return Record{}, buildErr
		}

		record, err := m.recordFromSchema(target, schema, reason)
		if err != nil {
			return Record{}, err
		}
		if err := m.persistRecord(record); err != nil {
			return Record{}, err
		}
		m.setEntry(record, SourceRebuilt, false)
		return record, nil
	})
	if err != nil {
		return Record{}, err
	}
	return result.(Record), nil
}

func (m *Manager) recordFromSchema(target resolvedTarget, schema *ai.SchemaContext, reason RefreshReason) (Record, error) {
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return Record{}, err
	}
	hash := sha256.Sum256(schemaBytes)
	return Record{
		SchemaKey:       target.schemaKey,
		DatabaseName:    target.dbName,
		DatabaseType:    schema.DatabaseType,
		Schema:          schema,
		TableCount:      len(schema.Tables),
		RefreshedAtUnix: time.Now().Unix(),
		SourceVersion:   schema.DatabaseVersion,
		SchemaHash:      hex.EncodeToString(hash[:]),
		LastError:       "",
		Dirty:           false,
		RefreshReason:   reason,
	}, nil
}

func (m *Manager) persistRecord(record Record) error {
	return m.store.Put(bucketSchemaIndexes, record.SchemaKey, record)
}

func (m *Manager) loadEntry(schemaKey string) (cacheEntry, bool, error) {
	m.mu.RLock()
	if entry, ok := m.cache[schemaKey]; ok {
		copyEntry := *entry
		m.mu.RUnlock()
		if copyEntry.hasRecord() {
			copyEntry.source = SourceMemory
		}
		return copyEntry, true, nil
	}
	m.mu.RUnlock()

	var record Record
	if err := m.store.Get(bucketSchemaIndexes, schemaKey, &record); err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return cacheEntry{}, false, nil
		}
		return cacheEntry{}, false, err
	}

	entry := cacheEntry{record: record, source: SourcePersisted}
	m.mu.Lock()
	if existing, ok := m.cache[schemaKey]; ok {
		entry = *existing
	} else {
		m.cache[schemaKey] = &cacheEntry{record: record, source: SourcePersisted}
	}
	m.mu.Unlock()
	return entry, true, nil
}

func (m *Manager) setEntry(record Record, source Source, refreshing bool) {
	m.mu.Lock()
	entry, ok := m.cache[record.SchemaKey]
	if !ok {
		entry = &cacheEntry{}
		m.cache[record.SchemaKey] = entry
	}
	entry.record = record
	entry.source = source
	entry.refreshing = refreshing
	status := m.statusFromEntry(record.DatabaseName, *entry)
	m.mu.Unlock()

	m.emitStatusChanged(status, string(record.RefreshReason))
}

func (m *Manager) setRefreshing(schemaKey, dbName string, refreshing bool, reason string) {
	m.mu.Lock()
	entry, ok := m.cache[schemaKey]
	if !ok {
		entry = &cacheEntry{record: Record{SchemaKey: schemaKey, DatabaseName: dbName}}
		m.cache[schemaKey] = entry
	}
	entry.refreshing = refreshing
	status := m.statusFromEntry(dbName, *entry)
	m.mu.Unlock()

	m.emitStatusChanged(status, reason)
}

func (m *Manager) touchTarget(target resolvedTarget) {
	now := time.Now()
	m.mu.Lock()
	m.activeTargets[target.schemaKey] = activeTarget{
		connID:   target.connID,
		dbName:   target.dbName,
		lastUsed: now,
	}
	m.mu.Unlock()
}

func (m *Manager) resolveTarget(connID, dbName string) (resolvedTarget, error) {
	if strings.TrimSpace(connID) == "" {
		return resolvedTarget{}, fmt.Errorf("连接不存在: %s", connID)
	}
	cfg, ok := m.resolveConfig(connID)
	if !ok || cfg == nil {
		return resolvedTarget{}, fmt.Errorf("连接配置不存在: %s", connID)
	}
	if strings.TrimSpace(dbName) == "" {
		dbName = strings.TrimSpace(cfg.Database)
	}
	if strings.TrimSpace(dbName) == "" {
		return resolvedTarget{}, fmt.Errorf("数据库名为空")
	}
	schemaKey, err := stableSchemaKey(cfg, dbName)
	if err != nil {
		return resolvedTarget{}, err
	}
	return resolvedTarget{
		schemaKey: schemaKey,
		connID:    connID,
		dbName:    dbName,
	}, nil
}

func stableSchemaKey(cfg *database.ConnectionConfig, dbName string) (string, error) {
	dbType := strings.ToLower(strings.TrimSpace(cfg.Type))
	dbName = strings.TrimSpace(dbName)
	if dbType == "sqlite" {
		absPath, err := filepath.Abs(strings.TrimSpace(cfg.Database))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s|%s", dbType, filepath.Clean(absPath)), nil
	}
	host := strings.ToLower(strings.TrimSpace(cfg.Host))
	user := strings.TrimSpace(cfg.User)
	return fmt.Sprintf("%s|%s|%d|%s|%s", dbType, host, cfg.Port, user, dbName), nil
}

func (m *Manager) statusFromEntry(dbName string, entry cacheEntry) Status {
	if !entry.hasRecord() {
		return Status{
			SchemaKey:    entry.record.SchemaKey,
			DatabaseName: dbName,
			Refreshing:   entry.refreshing,
			Source:       entry.source,
		}
	}
	source := entry.source
	if source == SourcePersisted {
		source = SourcePersisted
	} else {
		source = SourceMemory
	}
	if entry.source == SourceRebuilt {
		source = SourceRebuilt
	}
	return m.statusFromRecord(entry.record, source, entry.refreshing)
}

func (m *Manager) statusFromRecord(record Record, source Source, refreshing bool) Status {
	status := Status{
		SchemaKey:    record.SchemaKey,
		DatabaseName: record.DatabaseName,
		Exists:       record.Schema != nil,
		Refreshing:   refreshing,
		Dirty:        record.Dirty,
		TableCount:   record.TableCount,
		LastError:    record.LastError,
		Source:       source,
	}
	if record.RefreshedAtUnix > 0 {
		status.LastRefreshedAt = time.Unix(record.RefreshedAtUnix, 0).Format(time.RFC3339)
	}
	status.Stale = record.Schema != nil && m.isStale(record)
	return status
}

func (m *Manager) isStale(record Record) bool {
	if record.Dirty {
		return true
	}
	if record.RefreshedAtUnix == 0 {
		return true
	}
	return time.Since(time.Unix(record.RefreshedAtUnix, 0)) > m.refreshInterval
}

func (m *Manager) emitStatusChanged(status Status, reason string) {
	if m.app == nil {
		return
	}
	m.app.Event.Emit("schema:index_status_changed", StatusEvent{
		Reason: reason,
		Status: status,
	})
}
