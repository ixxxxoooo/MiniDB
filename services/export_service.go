package services

import (
	"fmt"
	"os"
	"path/filepath"
	"tableplus-ai/internal/export"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"context"
)

// ExportService 导出服务
type ExportService struct {
	ctx context.Context
}

// NewExportService 创建导出服务
func NewExportService() *ExportService {
	return &ExportService{}
}

// SetContext 设置上下文（在 startup 时调用）
func (s *ExportService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// ExportCSV 导出为 CSV
func (s *ExportService) ExportCSV(tableName string, columns []string, rows []map[string]interface{}) (string, error) {
	filePath, err := s.getSavePath(tableName, "csv")
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil // 用户取消
	}
	return filePath, export.ToCSV(filePath, columns, rows)
}

// ExportJSON 导出为 JSON
func (s *ExportService) ExportJSON(tableName string, rows []map[string]interface{}) (string, error) {
	filePath, err := s.getSavePath(tableName, "json")
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}
	return filePath, export.ToJSON(filePath, rows)
}

// ExportSQL 导出为 SQL INSERT
func (s *ExportService) ExportSQL(tableName string, columns []string, rows []map[string]interface{}) (string, error) {
	filePath, err := s.getSavePath(tableName, "sql")
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}
	return filePath, export.ToSQL(filePath, tableName, columns, rows)
}

func (s *ExportService) getSavePath(tableName, ext string) (string, error) {
	if s.ctx != nil {
		path, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
			DefaultFilename: fmt.Sprintf("%s_%s.%s", tableName, time.Now().Format("20060102_150405"), ext),
			Title:           "导出数据",
		})
		return path, err
	}

	// 回退到桌面
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Desktop",
		fmt.Sprintf("%s_%s.%s", tableName, time.Now().Format("20060102_150405"), ext)), nil
}
