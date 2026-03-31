package main

import (
	"embed"
	"fmt"

	"tableplus-ai/internal/logger"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 最先初始化日志系统，确保后续所有操作都有日志记录
	if err := logger.Init(); err != nil {
		fmt.Printf("日志系统初始化失败: %v\n", err)
	}
	logger.Info("TablePlus AI 启动中...")

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "TablePlus AI",
		Width:     1280,
		Height:    800,
		MinWidth:  960,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app.ConnectionSvc,
			app.DatabaseSvc,
			app.QuerySvc,
			app.DocSvc,
			app.SettingsSvc,
			app.AISvc,
			app.ExportSvc,
			app.HistorySvc,
		},
		// 隐藏原生标题栏，使用前端自绘窗口控制按钮
		Frameless: true,
		// 启用 CSS 拖拽属性 --wails-draggable: drag
		CSSDragProperty: "--wails-draggable",
		CSSDragValue:    "drag",
		Mac: &mac.Options{
			About: &mac.AboutInfo{
				Title:   "TablePlus AI",
				Message: "AI 增强的数据库管理工具",
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
	})

	if err != nil {
		logger.Error("Wails 运行失败: %v", err)
		fmt.Printf("Error: %s\n", err.Error())
	}
}
