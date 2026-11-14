package app

import (
	"context"
	"fmt"
	"minidb/internal/database"
	"minidb/internal/export"
	"minidb/internal/logger"
	"minidb/internal/model"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 流式导出每批查询行数
const exportBatchSize = 5000

// ExportProgressEvent 导出进度事件（通过 Bus 推送给界面）
type ExportProgressEvent struct {
	TaskID   string `json:"taskId"`
	Status   string `json:"status"`   // "progress" | "done" | "error" | "cancelled"
	Current  int64  `json:"current"`  // 已导出行数
	Total    int64  `json:"total"`    // 总行数（可能为 0 表示未知）
	FileName string `json:"fileName"` // 文件名
	FilePath string `json:"filePath"` // 完整路径
	Error    string `json:"error"`    // 错误信息
}

// ExportRequest 描述一次导出：一张或多张表，或一条查询的结果。
type ExportRequest struct {
	ConnID   string
	Database string
	Tables   []string // 导出表；多于一张时每张表一个文件
	Query    string   // 非空时导出这条语句的结果，忽略 Tables
	Options  export.Options
	// Pattern 是多表导出的文件名模板，可用 {table} {db} {date} {time} {datetime}；空为 {table}_{datetime}。
	Pattern string
	// Limit 大于 0 时每个文件最多写这么多行（取前 N 行）。
	Limit int64
	// Path 为空时弹出保存对话框；单文件时是文件路径，多表时是目录。
	Path string
}

// ExportService 导出服务
type ExportService struct {
	env     envRef
	manager *database.Manager

	// 取消导出任务支持
	mu        sync.Mutex
	cancelMap map[string]context.CancelFunc
}

// NewExportService 创建导出服务
func NewExportService(manager *database.Manager) *ExportService {
	return &ExportService{
		manager:   manager,
		cancelMap: make(map[string]context.CancelFunc),
	}
}

// SetEnv 设置平台环境（在启动时调用）
func (s *ExportService) SetEnv(env *Env) {
	s.env.set(env)
}

// CancelExport 取消导出任务
func (s *ExportService) CancelExport(taskID string) {
	s.mu.Lock()
	cancel, ok := s.cancelMap[taskID]
	s.mu.Unlock()
	if ok {
		logger.Info("[ExportService] 取消导出任务: %s", taskID)
		cancel()
	}
}

// OpenExportedFile 打开已导出的文件
func (s *ExportService) OpenExportedFile(filePath string) error {
	defer logger.Perf("app.Exports.OpenExportedFile")()
	if strings.TrimSpace(filePath) == "" {
		return model.Errf(model.CodeFilePathEmpty, "文件路径为空")
	}
	if _, err := os.Stat(filePath); err != nil {
		logger.Warn("[ExportService] 打开导出文件失败，文件不存在: %s err=%v", filePath, err)
		return err
	}
	logger.Info("[ExportService] 打开导出文件: %s", filePath)
	if s.env.get() != nil && s.env.get().Host != nil {
		return s.env.get().Host.OpenPath(filePath)
	}
	return exec.Command("open", filePath).Start()
}

// ExpandPattern 把文件名模板里的占位符换掉，并去掉路径分隔符。
func ExpandPattern(pattern, table, db string, now time.Time) string {
	if strings.TrimSpace(pattern) == "" {
		pattern = "{table}_{datetime}"
	}
	r := strings.NewReplacer(
		"{table}", table, "{db}", db,
		"{date}", now.Format("20060102"), "{time}", now.Format("150405"),
		"{datetime}", now.Format("20060102_150405"),
	)
	name := r.Replace(pattern)
	return strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(name)
}

// StartExport 开始导出：先定下保存位置（必要时弹窗），再在后台分批查询、写入并推送进度。
// 返回任务 ID；空串表示用户取消了位置选择。
func (s *ExportService) StartExport(req ExportRequest) (string, error) {
	defer logger.Perf("app.Exports.StartExport")()
	cfg, ok := s.manager.GetConfig(req.ConnID)
	if !ok {
		return "", model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", req.ConnID)
	}
	if req.Query == "" && len(req.Tables) == 0 {
		return "", fmt.Errorf("没有要导出的内容")
	}
	if req.Options.Format == "" {
		req.Options.Format = "csv"
	}
	ext := req.Options.Extension()
	now := time.Now()

	var jobs []exportJob
	var label string
	switch {
	case req.Query != "":
		path, err := s.target(req.Path, "query_result", ext)
		if path == "" || err != nil {
			return "", err
		}
		jobs = []exportJob{{table: "query_result", query: req.Query, path: path}}
		label = filepath.Base(path)
	case len(req.Tables) == 1:
		path, err := s.target(req.Path, req.Tables[0], ext)
		if path == "" || err != nil {
			return "", err
		}
		jobs = []exportJob{{table: req.Tables[0], path: path}}
		label = filepath.Base(path)
	default:
		dir := req.Path
		if dir == "" {
			picked, err := s.getSavePath("export", ext)
			if picked == "" || err != nil {
				return "", err
			}
			dir = filepath.Dir(picked)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		used := map[string]int{}
		for _, t := range req.Tables {
			name := ExpandPattern(req.Pattern, t, req.Database, now)
			used[name]++
			if n := used[name]; n > 1 {
				name = fmt.Sprintf("%s_%d", name, n)
			}
			jobs = append(jobs, exportJob{table: t, path: filepath.Join(dir, name+"."+ext)})
		}
		label = dir
	}

	var total int64
	for _, j := range jobs {
		var n int64
		if j.query != "" {
			n = s.queryRowCount(req.ConnID, req.Database, j.query)
		} else if c, err := s.getTableRowCount(req.ConnID, req.Database, j.table); err == nil {
			n = c
		}
		if req.Limit > 0 && n > req.Limit {
			n = req.Limit
		}
		total += n
	}

	taskID := fmt.Sprintf("export_%s_%d", jobs[0].table, now.UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()
	go s.run(ctx, taskID, req, cfg, jobs, label, total)
	return taskID, nil
}

// target 返回单文件导出的保存路径：给了就用，否则弹保存对话框。
func (s *ExportService) target(path, name, ext string) (string, error) {
	if path != "" {
		return path, nil
	}
	return s.getSavePath(name, ext)
}

// emitExportProgress 向界面推送导出进度事件
func (s *ExportService) emitExportProgress(event ExportProgressEvent) {
	if s.env.get() == nil {
		logger.Warn("[ExportService] 推送进度时运行环境为空")
		return
	}
	s.env.get().Emit(TopicExport, event)
}

func (s *ExportService) getSavePath(tableName, ext string) (string, error) {
	name := fmt.Sprintf("%s_%s.%s", tableName, time.Now().Format("20060102_150405"), ext)
	if s.env.get() != nil && s.env.get().Host != nil {
		return s.env.get().Host.SaveFile(SaveOptions{Title: "导出数据", Filename: name})
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Desktop", name), nil
}
