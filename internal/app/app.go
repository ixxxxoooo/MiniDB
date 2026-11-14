package app

import (
	"context"

	"minidb/internal/ai"
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
	"minidb/internal/storage"
)

// Event topics published on App.Bus.
const (
	TopicSchemaIndex = "schema:index_status_changed" // schemaindex.StatusEvent
	TopicExport      = "export:progress"             // ExportProgressEvent
	TopicImport      = "import:progress"             // ImportProgressEvent
	TopicDump        = "dump:progress"               // DumpProgressEvent
	TopicAgent       = "ai:agent_events"             // agent.Event
	// EventConnectionStatus ("connection:status") carries ConnectionStatusEvent.
)

// App owns the services and the store; the UI layer talks to it only.
type App struct {
	Bus *Bus

	// 下面三个只在 app 包内使用；界面层通过各个 Service 访问。
	store   *storage.Store
	manager *database.Manager
	schema  *schemaindex.Manager

	Connections *ConnectionService
	Databases   *DatabaseService
	Queries     *QueryService
	UIState     *UIStateService
	Docs        *DocService
	Settings    *SettingsService
	AI          *AIService
	SchemaIndex *SchemaIndexService
	Exports     *ExportService
	Imports     *ImportService
	Explain     *ExplainService
	ER          *ERService
	Dumps       *DumpService
	History     *HistoryService
}

// New wires the services over store. host serves dialogs and may be nil in
// tests.
func New(store *storage.Store, host Host) *App {
	manager := database.NewManager()
	schemaMgr := schemaindex.NewManager(
		store,
		func(connID string) (*database.ConnectionConfig, bool) {
			if cfg, ok := manager.GetConfig(connID); ok && cfg != nil {
				return cfg, true
			}
			return loadPersistedConnectionConfig(store, connID)
		},
		func(ctx context.Context, connID, dbName string) (*ai.SchemaContext, error) {
			return schemaindex.BuildSchemaFromDatabaseManager(ctx, manager, connID, dbName)
		},
	)
	queries := NewQueryService(manager, schemaMgr)
	a := &App{
		Bus:     NewBus(),
		store:   store,
		manager: manager,
		schema:  schemaMgr,

		Connections: NewConnectionService(store, manager, schemaMgr),
		Databases:   NewDatabaseService(manager, schemaMgr),
		Queries:     queries,
		UIState:     NewUIStateService(store),
		Docs:        NewDocService(store),
		Settings:    NewSettingsService(store),
		AI:          NewAIService(manager, store, queries, schemaMgr),
		SchemaIndex: NewSchemaIndexService(schemaMgr),
		Exports:     NewExportService(manager),
		Imports:     NewImportService(manager),
		Explain:     NewExplainService(manager),
		ER: NewERService(manager, schemaMgr.GetSchema, func(ctx context.Context, connID, dbName string) error {
			_, err := schemaMgr.Refresh(ctx, connID, dbName, schemaindex.RefreshReasonManual)
			return err
		}),
		Dumps:   NewDumpService(manager),
		History: NewHistoryService(store),
	}
	a.SetHost(host)
	a.AI.explain = a.Explain
	schemaMgr.OnStatusChanged(func(ev schemaindex.StatusEvent) { a.Bus.Emit(TopicSchemaIndex, ev) })
	return a
}

// SetHost sets the platform adapter the services use for dialogs.
func (a *App) SetHost(host Host) {
	env := &Env{Host: host, Bus: a.Bus}
	a.Connections.SetEnv(env)
	a.AI.SetEnv(env)
	a.Exports.SetEnv(env)
	a.Imports.SetEnv(env)
	a.Dumps.SetEnv(env)
}

// Start launches background work (schema refresh, connection keepalive).
func (a *App) Start() {
	a.schema.Start()
	a.Connections.StartKeepalive()
}

// CloseConnections 关掉所有数据库连接（不动存储）；测试清理时用。
func (a *App) CloseConnections() { a.manager.CloseAll() }

// Shutdown stops background work and closes every connection and the store.
func (a *App) Shutdown() {
	a.Connections.StopKeepalive()
	a.schema.Shutdown()
	a.manager.CloseAll()
	if a.store != nil {
		a.store.Close()
	}
	logger.Info("应用服务已关闭")
}

func loadPersistedConnectionConfig(store *storage.Store, connID string) (*database.ConnectionConfig, bool) {
	var cfg database.ConnectionConfig
	if err := store.Get("connections", connID, &cfg); err != nil {
		return nil, false
	}
	password, err := store.DecryptString(cfg.Password)
	if err != nil {
		logger.Warn("读取 schema 索引连接配置失败: id=%s err=%v", connID, err)
		return nil, false
	}
	cfg.Password = password
	return &cfg, true
}
