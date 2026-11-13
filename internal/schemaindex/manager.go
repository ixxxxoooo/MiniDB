package schemaindex

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"minidb/internal/ai"
	"minidb/internal/logger"
	"minidb/internal/storage"
)

// Manager 管理各数据库的 schema 索引：内存 + 持久化缓存，过期与 DDL 后在后台刷新，
// 并在其上提供按关键词检索表和列的能力。
//
// @author ixxxxoooo
type Manager struct {
	store           *storage.Store
	resolveConfig   ConfigResolver
	buildSchema     BuilderFunc
	refreshInterval time.Duration

	onStatus func(StatusEvent)

	mu            sync.RWMutex
	searchers     map[string]searcherEntry
	cache         map[string]*cacheEntry
	activeTargets map[string]activeTarget
	refreshGroup  singleflight.Group

	startOnce sync.Once
	stopOnce  sync.Once
	stopCh    chan struct{}

	// bgCtx 供后台刷新使用，随 Shutdown 取消，避免应用退出后仍在连接数据库
	bgCtx       context.Context
	bgCtxCancel context.CancelFunc
}

func NewManager(store *storage.Store, resolveConfig ConfigResolver, buildSchema BuilderFunc) *Manager {
	return &Manager{
		store:           store,
		resolveConfig:   resolveConfig,
		buildSchema:     buildSchema,
		refreshInterval: defaultRefreshTTL,
		cache:           make(map[string]*cacheEntry),
		searchers:       make(map[string]searcherEntry),
		activeTargets:   make(map[string]activeTarget),
	}
}

// OnStatusChanged registers the callback told when an index's status changes.
// It is called from background goroutines; the UI layer must hop to its own
// thread before touching state.
func (m *Manager) OnStatusChanged(fn func(StatusEvent)) {
	m.mu.Lock()
	m.onStatus = fn
	m.mu.Unlock()
}

func (m *Manager) Start() {
	m.startOnce.Do(func() {
		m.stopCh = make(chan struct{})
		m.bgCtx, m.bgCtxCancel = context.WithCancel(context.Background())
		go m.runTicker()
	})
}

func (m *Manager) Shutdown() {
	m.stopOnce.Do(func() {
		if m.stopCh != nil {
			close(m.stopCh)
		}
		if m.bgCtxCancel != nil {
			m.bgCtxCancel()
		}
	})
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

// getRecord 取目标库的索引记录：命中缓存直接用；DDL 之后（Dirty）同步重建；仅过期则先用旧的并后台刷新。
func (m *Manager) getRecord(ctx context.Context, connID, dbName string) (Record, error) {
	target, err := m.resolveTarget(connID, dbName)
	if err != nil {
		return Record{}, err
	}
	m.touchTarget(target)

	entry, ok, err := m.loadEntry(target.schemaKey)
	if err != nil {
		return Record{}, err
	}
	if ok && entry.hasRecord() {
		if entry.record.Dirty {
			return m.refreshTarget(ctx, target, RefreshReasonDDL)
		}
		if m.isStale(entry.record) {
			m.refreshAsync(target, RefreshReasonOnDemand)
		}
		return entry.record, nil
	}
	return m.refreshTarget(ctx, target, RefreshReasonOnDemand)
}

// GetSchema 返回目标库的表结构快照。
func (m *Manager) GetSchema(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error) {
	record, err := m.getRecord(ctx, connID, dbName)
	if err != nil {
		return nil, err
	}
	return record.Schema, nil
}

// Index 返回目标库的表结构快照和基于它的检索器。检索器按 schema 哈希缓存，结构没变就不重建。
func (m *Manager) Index(ctx context.Context, connID, dbName string) (*ai.SchemaContext, *Searcher, error) {
	record, err := m.getRecord(ctx, connID, dbName)
	if err != nil {
		return nil, nil, err
	}
	key := record.SchemaKey
	version := record.SchemaHash
	m.mu.RLock()
	cached, ok := m.searchers[key]
	m.mu.RUnlock()
	if ok && cached.version == version {
		return record.Schema, cached.searcher, nil
	}
	searcher := NewSearcher(record.Schema)
	m.mu.Lock()
	m.searchers[key] = searcherEntry{version: version, searcher: searcher}
	m.mu.Unlock()
	return record.Schema, searcher, nil
}

// searcherEntry 一个库的检索器缓存项。
type searcherEntry struct {
	version  string
	searcher *Searcher
}

// Refresh 立即重建目标库的索引并返回新状态。

// WarmAsync 在后台预热目标库的索引：没有、已过期或 DDL 后失效时才会重建。
