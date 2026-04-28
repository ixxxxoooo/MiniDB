package main

import (
	"embed"
	"fmt"

	"tableplus-ai/internal/logger"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 最先初始化日志系统，确保后续所有操作都有日志记录
	if err := logger.Init(); err != nil {
		fmt.Printf("日志系统初始化失败: %v\n", err)
	}
	logger.Info("TablePlus AI 启动中...")

	coreApp := NewApp()

	var wailsApp *application.App
	wailsApp = application.New(application.Options{
		Name:        "TablePlus AI",
		Description: "AI 增强的数据库管理工具",
		Services: []application.Service{
			application.NewService(coreApp.ConnectionSvc),
			application.NewService(coreApp.DatabaseSvc),
			application.NewService(coreApp.QuerySvc),
			application.NewService(coreApp.DocSvc),
			application.NewService(coreApp.SettingsSvc),
			application.NewService(coreApp.AISvc),
			application.NewService(coreApp.ExportSvc),
			application.NewService(coreApp.HistorySvc),
			application.NewService(coreApp.ClipboardSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: false,
		},
		OnShutdown: func() {
			coreApp.shutdown(wailsApp.Context())
		},
	})

	coreApp.startup(wailsApp.Context(), wailsApp)

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "TablePlus AI",
		Width:     1280,
		Height:    800,
		MinWidth:  960,
		MinHeight: 600,
		URL:       "/",
		// 隐藏原生标题栏，使用前端自绘窗口控制按钮。
		Frameless:        true,
		BackgroundType:   application.BackgroundTypeTransparent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		Mac: application.MacWindow{
			Appearance: application.NSAppearanceNameAqua,
		},
		Windows: application.WindowsWindow{
			DisableFramelessWindowDecorations: false,
		},
	})

	err := wailsApp.Run()
	if err != nil {
		logger.Error("Wails 运行失败: %v", err)
		fmt.Printf("Error: %s\n", err.Error())
	}
}
