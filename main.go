package main

import (
	"embed"
	"fmt"
	"os"

	"minidb/internal/app"
	"minidb/internal/logger"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 最先初始化日志系统，确保后续所有操作都有日志记录
	if err := logger.Init(); err != nil {
		fmt.Printf("日志系统初始化失败: %v\n", err)
	}
	if err := app.Run(app.EmbeddedResources{Assets: assets}); err != nil {
		logger.Error("Wails 运行失败: %v", err)
		fmt.Printf("Error: %s\n", err.Error())
		os.Exit(1)
	}
}
