package app

import (
	"context"

	"tableplus-ai/internal/database"
	"tableplus-ai/internal/logger"
	"tableplus-ai/internal/storage"
	"tableplus-ai/services"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type core struct {
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
	ClipboardSvc  *services.ClipboardService
}

func newCore() *core {
	logger.Info("正在创建应用实例...")

	store, err := storage.NewStore()
	if err != nil {
		logger.Error("初始化存储失败: %v", err)
		panic("初始化存储失败: " + err.Error())
	}
	logger.Info("BoltDB 存储引擎初始化成功")

	manager := database.NewManager()
	logger.Info("数据库连接管理器初始化成功")

	querySvc := services.NewQueryService(manager)
	app := &core{
		store:   store,
		manager: manager,

		ConnectionSvc: services.NewConnectionService(store, manager),
		DatabaseSvc:   services.NewDatabaseService(manager),
		QuerySvc:      querySvc,
		DocSvc:        services.NewDocService(store),
		SettingsSvc:   services.NewSettingsService(store),
		AISvc:         services.NewAIService(manager, store, querySvc),
		ExportSvc:     services.NewExportService(manager),
		HistorySvc:    services.NewHistoryService(store),
		ClipboardSvc:  services.NewClipboardService(),
	}
	logger.Info("所有服务实例创建完成")
	return app
}

func (a *core) services() []application.Service {
	return []application.Service{
		application.NewService(a.ConnectionSvc),
		application.NewService(a.DatabaseSvc),
		application.NewService(a.QuerySvc),
		application.NewService(a.DocSvc),
		application.NewService(a.SettingsSvc),
		application.NewService(a.AISvc),
		application.NewService(a.ExportSvc),
		application.NewService(a.HistorySvc),
		application.NewService(a.ClipboardSvc),
	}
}

func (a *core) startup(ctx context.Context, wailsApp *application.App) {
	logger.Info("应用启动中... (Wails startup)")
	a.ctx = ctx
	a.ExportSvc.SetWailsApplication(wailsApp)
	a.AISvc.SetWailsApplication(wailsApp)
	logger.Info("应用启动完成，窗口即将显示")
}

func (a *core) shutdown(ctx context.Context) {
	logger.Info("应用正在关闭... (Wails shutdown)")
	a.manager.CloseAll()
	logger.Info("所有数据库连接已关闭")
	if a.store != nil {
		a.store.Close()
		logger.Info("存储引擎已关闭")
	}
	logger.Close()
}
