package app

import (
	"minidb/internal/logger"
	"minidb/internal/updater"
	appversion "minidb/internal/version"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func init() {
	application.RegisterEvent[updater.StatePayload](updater.EventState)
	application.RegisterEvent[updater.ProgressPayload](updater.EventProgress)
	application.RegisterEvent[updater.ReadyPayload](updater.EventReady)
	application.RegisterEvent[updater.ErrorPayload](updater.EventError)
}

func Run(resources EmbeddedResources) error {
	logger.Info("%s 启动中...", appversion.AppName)

	coreApp := newCore()
	var updateManager *updater.Manager

	var wailsApp *application.App
	wailsApp = application.New(application.Options{
		Name:        appversion.AppName,
		Description: appversion.Description,
		Services:    coreApp.services(),
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(resources.Assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: false,
		},
		OnShutdown: func() {
			if updateManager != nil {
				updateManager.Shutdown()
			}
			coreApp.shutdown(wailsApp.Context())
		},
	})

	coreApp.startup(wailsApp.Context(), wailsApp)
	updateManager = updater.NewManager(wailsApp)
	coreApp.SettingsSvc.SetUpdater(updateManager)
	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		updateManager.Start()
	})
	createMainWindow(wailsApp)

	return wailsApp.Run()
}

func createMainWindow(wailsApp *application.App) *application.WebviewWindow {
	return wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     appversion.AppName,
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
}
