package main

import (
	"context"
	"log"
	"tableplus-ai/internal/database"
	"tableplus-ai/internal/storage"
	"tableplus-ai/services"
)

// App 应用主结构体
type App struct {
	ctx context.Context

	store   *storage.Store
	manager *database.Manager

	ConnectionSvc *services.ConnectionService
	DatabaseSvc   *services.DatabaseService
	QuerySvc      *services.QueryService
	DocSvc        *services.DocService
	SettingsSvc   *services.SettingsService
	AISvc         *services.AIService
	ExportSvc     *services.ExportService
	HistorySvc    *services.HistoryService
}

// NewApp 创建应用实例
func NewApp() *App {
	store, err := storage.NewStore()
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}

	manager := database.NewManager()

	return &App{
		store:   store,
		manager: manager,

		ConnectionSvc: services.NewConnectionService(store, manager),
		DatabaseSvc:   services.NewDatabaseService(manager),
		QuerySvc:      services.NewQueryService(manager),
		DocSvc:        services.NewDocService(store),
		SettingsSvc:   services.NewSettingsService(store),
		AISvc:         services.NewAIService(manager, store),
		ExportSvc:     services.NewExportService(),
		HistorySvc:    services.NewHistoryService(store),
	}
}

// startup 应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.ExportSvc.SetContext(ctx)
}

// shutdown 应用关闭时调用
func (a *App) shutdown(ctx context.Context) {
	a.manager.CloseAll()
	if a.store != nil {
		a.store.Close()
	}
}
