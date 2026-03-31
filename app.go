package main

import (
	"context"
	"tableplus-ai/internal/database"
	"tableplus-ai/internal/logger"
	"tableplus-ai/internal/storage"
	"tableplus-ai/services"
)

// App 应用主结构体，持有所有核心依赖和服务实例
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

// NewApp 创建应用实例，初始化存储引擎、连接管理器和所有服务
func NewApp() *App {
	logger.Info("正在创建应用实例...")

	store, err := storage.NewStore()
	if err != nil {
		logger.Error("初始化存储失败: %v", err)
		panic("初始化存储失败: " + err.Error())
	}
	logger.Info("BoltDB 存储引擎初始化成功")

	manager := database.NewManager()
	logger.Info("数据库连接管理器初始化成功")

	app := &App{
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
	logger.Info("所有服务实例创建完成")
	return app
}

// startup 应用启动时调用，由 Wails 框架触发
func (a *App) startup(ctx context.Context) {
	logger.Info("应用启动中... (Wails OnStartup)")
	a.ctx = ctx
	a.ExportSvc.SetContext(ctx)
	logger.Info("应用启动完成，窗口即将显示")
}

// shutdown 应用关闭时调用，清理资源
func (a *App) shutdown(ctx context.Context) {
	logger.Info("应用正在关闭... (Wails OnShutdown)")
	a.manager.CloseAll()
	logger.Info("所有数据库连接已关闭")
	if a.store != nil {
		a.store.Close()
		logger.Info("存储引擎已关闭")
	}
	logger.Close()
}
