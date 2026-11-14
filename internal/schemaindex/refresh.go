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
	"time"

	"minidb/internal/ai"
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/storage"
)

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
		// 使用可取消的后台上下文：应用退出时立即中止后台刷新，避免访问已关闭的连接
		ctx := context.Background()
		if m.bgCtx != nil {
			ctx = m.bgCtx
		}
		if _, err := m.refreshTarget(ctx, target, reason); err != nil {
			logger.Warn("[SchemaIndex] 后台刷新失败: key=%s db=%s reason=%s err=%v", target.schemaKey, target.dbName, reason, err)
		}
	}()
}

func (m *Manager) refreshTarget(ctx context.Context, target resolvedTarget, reason RefreshReason) (Record, error) {
	defer logger.Perf("schema.refresh", "db", target.dbName, "reason", reason)() // 要读整库元数据，大库上最容易拖慢
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

		schema, buildErr := m.buildSchema(ctx, target.connID, target.dbName)
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
		Version:         recordVersion,
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

	if record.Version < recordVersion {
		record.RefreshedAtUnix = 0 // 旧格式缺少新信息：先照常使用，但视为过期，由后台刷新补全
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
	dbType := strings.ToLower(strings.TrimSpace(string(cfg.Type)))
	dbName = strings.TrimSpace(dbName)
	if database.CapsOf(cfg.Type).LocalFile() {
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
