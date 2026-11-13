package schemaindex

import "time"

func (m *Manager) statusFromEntry(dbName string, entry cacheEntry) Status {
	if !entry.hasRecord() {
		return Status{
			SchemaKey:    entry.record.SchemaKey,
			DatabaseName: dbName,
			Refreshing:   entry.refreshing,
			Source:       entry.source,
		}
	}
	return m.statusFromRecord(entry.record, entry.source, entry.refreshing)
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
	m.mu.RLock()
	fn := m.onStatus
	m.mu.RUnlock()
	if fn == nil {
		return
	}
	fn(StatusEvent{Reason: reason, Status: status})
}
