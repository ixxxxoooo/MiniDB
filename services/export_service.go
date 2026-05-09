package services

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"minidb/internal/database"
	"minidb/internal/export"
	"minidb/internal/logger"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// 流式导出每批查询行数
const exportBatchSize = 5000

// ExportProgressEvent 导出进度事件（通过 Wails EventsEmit 推送给前端）
type ExportProgressEvent struct {
	TaskID   string `json:"taskId"`
	Status   string `json:"status"`   // "progress" | "done" | "error" | "cancelled"
	Current  int64  `json:"current"`  // 已导出行数
	Total    int64  `json:"total"`    // 总行数（可能为 0 表示未知）
	FileName string `json:"fileName"` // 文件名
	FilePath string `json:"filePath"` // 完整路径
	Error    string `json:"error"`    // 错误信息
}

// ExportService 导出服务
type ExportService struct {
	app     *application.App
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

// SetWailsApplication 设置 Wails 应用实例（在 startup 时调用）
//
//wails:ignore
func (s *ExportService) SetWailsApplication(app *application.App) {
	s.app = app
}

// --- 新版流式导出接口 ---

// ExportTableStream 流式导出表数据，先弹窗选路径，再后台分批查询+写入+推送进度
// 返回 taskID 供前端取消使用；空字符串表示用户取消了路径选择
func (s *ExportService) ExportTableStream(connID, dbName, tableName, format string) (string, error) {
	logger.Info("[ExportService] 流式导出开始: connID=%s db=%s table=%s format=%s", connID, dbName, tableName, format)

	// 1. 先弹窗让用户选择保存路径
	filePath, err := s.getSavePath(tableName, format)
	if err != nil {
		return "", err
	}
	if filePath == "" {
		logger.Info("[ExportService] 用户取消了路径选择")
		return "", nil
	}

	// 2. 获取总行数（用于进度计算）
	totalRows, err := s.getTableRowCount(connID, dbName, tableName)
	if err != nil {
		logger.Warn("[ExportService] 获取总行数失败，将使用未知总数: %v", err)
		totalRows = 0
	}
	logger.Info("[ExportService] 表总行数: %d", totalRows)

	// 3. 生成任务 ID
	taskID := fmt.Sprintf("export_%s_%d", tableName, time.Now().UnixMilli())

	// 4. 创建可取消的上下文
	taskCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()

	// 5. 立即返回 taskID，后台 goroutine 执行导出
	go s.runStreamExport(taskCtx, taskID, connID, dbName, tableName, format, filePath, totalRows)

	return taskID, nil
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
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("文件路径为空")
	}
	if _, err := os.Stat(filePath); err != nil {
		logger.Warn("[ExportService] 打开导出文件失败，文件不存在: %s err=%v", filePath, err)
		return err
	}
	logger.Info("[ExportService] 打开导出文件: %s", filePath)
	cmd := exec.Command("open", filePath)
	if err := cmd.Start(); err != nil {
		logger.Error("[ExportService] 打开导出文件失败: %v", err)
		return err
	}
	return nil
}

// runStreamExport 后台执行流式导出
func (s *ExportService) runStreamExport(ctx context.Context, taskID, connID, dbName, tableName, format, filePath string, totalRows int64) {
	defer func() {
		s.mu.Lock()
		delete(s.cancelMap, taskID)
		s.mu.Unlock()
	}()

	fileName := filepath.Base(filePath)

	// 推送初始进度
	s.emitExportProgress(ExportProgressEvent{
		TaskID: taskID, Status: "progress", Current: 0, Total: totalRows,
		FileName: fileName, FilePath: filePath,
	})

	db, err := s.manager.GetDB(connID)
	if err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("获取连接失败: %v", err), FileName: fileName})
		return
	}

	cfg, ok := s.manager.GetConfig(connID)
	if ok {
		if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: err.Error(), FileName: fileName})
			return
		}
	}

	// 获取列信息（用第一批数据的列）
	columns, err := s.getTableColumns(db, cfg, dbName, tableName)
	if err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("获取列信息失败: %v", err), FileName: fileName})
		return
	}

	// 创建文件并初始化写入器
	file, err := os.Create(filePath)
	if err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("创建文件失败: %v", err), FileName: fileName})
		return
	}
	defer file.Close()

	writer, err := s.initWriter(file, format, columns, tableName)
	if err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: err.Error(), FileName: fileName})
		return
	}

	// 分批查询 + 写入
	var exported int64
	offset := 0
	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		quotedTable = fmt.Sprintf("%s.%s", database.QuoteIdent(cfg.Type, dbName), quotedTable)
	} else if cfg.Type == "postgres" {
		quotedTable = database.QuoteTableName(cfg.Type, tableName)
	}

	for {
		// 检查是否被取消
		select {
		case <-ctx.Done():
			logger.Info("[ExportService] 导出被取消: taskID=%s exported=%d", taskID, exported)
			s.emitExportProgress(ExportProgressEvent{
				TaskID: taskID, Status: "cancelled", Current: exported, Total: totalRows,
				FileName: fileName, FilePath: filePath,
			})
			return
		default:
		}

		batchSQL := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", quotedTable, exportBatchSize, offset)
		result, err := database.ExecuteQueryRaw(db, batchSQL)
		if err != nil {
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("查询失败(offset=%d): %v", offset, err), FileName: fileName})
			return
		}
		if result.Error != "" {
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("查询错误: %s", result.Error), FileName: fileName})
			return
		}

		if len(result.Rows) == 0 {
			break
		}

		// 写入这一批
		if err := s.writeBatch(writer, format, columns, tableName, result.Rows); err != nil {
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("写入失败: %v", err), FileName: fileName})
			return
		}

		exported += int64(len(result.Rows))
		offset += exportBatchSize

		// 推送进度
		s.emitExportProgress(ExportProgressEvent{
			TaskID: taskID, Status: "progress", Current: exported, Total: totalRows,
			FileName: fileName, FilePath: filePath,
		})

		logger.Info("[ExportService] 导出进度: taskID=%s exported=%d/%d", taskID, exported, totalRows)

		// 如果本批不满，说明已到结尾
		if len(result.Rows) < exportBatchSize {
			break
		}
	}

	// 完成写入（JSON 需要写结束括号等）
	if err := s.finalizeWriter(writer, format); err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("文件收尾失败: %v", err), FileName: fileName})
		return
	}

	logger.Info("[ExportService] 流式导出完成: taskID=%s path=%s rows=%d", taskID, filePath, exported)
	s.emitExportProgress(ExportProgressEvent{
		TaskID: taskID, Status: "done", Current: exported, Total: exported,
		FileName: fileName, FilePath: filePath,
	})
}

// getTableRowCount 获取表的总行数
func (s *ExportService) getTableRowCount(connID, dbName, tableName string) (int64, error) {
	db, err := s.manager.GetDB(connID)
	if err != nil {
		return 0, err
	}
	cfg, ok := s.manager.GetConfig(connID)
	if ok {
		if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
			return 0, err
		}
	}

	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		quotedTable = fmt.Sprintf("%s.%s", database.QuoteIdent(cfg.Type, dbName), quotedTable)
	} else if cfg.Type == "postgres" {
		quotedTable = database.QuoteTableName(cfg.Type, tableName)
	}

	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s", quotedTable)
	var count int64
	if err := db.QueryRow(countSQL).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// getTableColumns 获取表的列名列表
func (s *ExportService) getTableColumns(db *sql.DB, cfg *database.ConnectionConfig, dbName, tableName string) ([]string, error) {
	quotedTable := database.QuoteTableName(cfg.Type, tableName)
	if database.IsMySQLCompatible(cfg.Type) && dbName != "" {
		quotedTable = fmt.Sprintf("%s.%s", database.QuoteIdent(cfg.Type, dbName), quotedTable)
	} else if cfg.Type == "postgres" {
		quotedTable = database.QuoteTableName(cfg.Type, tableName)
	}

	probeSQL := fmt.Sprintf("SELECT * FROM %s LIMIT 1", quotedTable)
	rows, err := db.Query(probeSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	return colNames, nil
}

// --- 流式写入器接口 ---

// streamWriter 抽象写入接口
type streamWriter interface{}

// csvStreamWriter CSV 流式写入器
type csvStreamWriter struct {
	w *csv.Writer
}

// jsonStreamWriter JSON 流式写入器（手动拼接 JSON 数组）
type jsonStreamWriter struct {
	file     *os.File
	firstRow bool
	columns  []string
}

// sqlStreamWriter SQL INSERT 流式写入器
type sqlStreamWriter struct {
	file *os.File
}

func (s *ExportService) initWriter(file *os.File, format string, columns []string, tableName string) (streamWriter, error) {
	switch format {
	case "csv":
		// UTF-8 BOM
		file.Write([]byte{0xEF, 0xBB, 0xBF})
		w := csv.NewWriter(file)
		if err := w.Write(columns); err != nil {
			return nil, fmt.Errorf("CSV 写表头失败: %w", err)
		}
		w.Flush()
		return &csvStreamWriter{w: w}, nil
	case "json":
		// 写 JSON 数组开头
		file.WriteString("[\n")
		return &jsonStreamWriter{file: file, firstRow: true, columns: columns}, nil
	case "sql":
		return &sqlStreamWriter{file: file}, nil
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

func (s *ExportService) writeBatch(w streamWriter, format string, columns []string, tableName string, rows []map[string]interface{}) error {
	switch format {
	case "csv":
		cw := w.(*csvStreamWriter)
		for _, row := range rows {
			var record []string
			for _, col := range columns {
				val := row[col]
				if val == nil {
					record = append(record, "")
				} else {
					record = append(record, fmt.Sprintf("%v", val))
				}
			}
			if err := cw.w.Write(record); err != nil {
				return err
			}
		}
		cw.w.Flush()
		return cw.w.Error()
	case "json":
		jw := w.(*jsonStreamWriter)
		for _, row := range rows {
			if !jw.firstRow {
				jw.file.WriteString(",\n")
			}
			jw.firstRow = false
			b, err := json.Marshal(row)
			if err != nil {
				return err
			}
			jw.file.Write(b)
		}
		return nil
	case "sql":
		sw := w.(*sqlStreamWriter)
		for _, row := range rows {
			var values []string
			for _, col := range columns {
				values = append(values, escapeExportValue(row[col]))
			}
			line := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);\n",
				tableName, strings.Join(columns, ", "), strings.Join(values, ", "))
			if _, err := sw.file.WriteString(line); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

func (s *ExportService) finalizeWriter(w streamWriter, format string) error {
	switch format {
	case "csv":
		cw := w.(*csvStreamWriter)
		cw.w.Flush()
		return cw.w.Error()
	case "json":
		jw := w.(*jsonStreamWriter)
		_, err := jw.file.WriteString("\n]")
		return err
	}
	return nil
}

// escapeExportValue SQL 导出时转义值
func escapeExportValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}
	switch v := val.(type) {
	case float64:
		return fmt.Sprintf("%v", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	default:
		return fmt.Sprintf("'%v'", v)
	}
}

// emitExportProgress 向前端推送导出进度事件
func (s *ExportService) emitExportProgress(event ExportProgressEvent) {
	if s.app == nil {
		logger.Warn("[ExportService] 推送进度时 Wails 应用实例为空")
		return
	}
	s.app.Event.Emit("export:progress", event)
}

// ExportSQLResultStream 流式导出任意 SQL 查询结果
// 和 ExportTableStream 类似：先弹窗选路径 → 后台分批执行 → 推送进度
func (s *ExportService) ExportSQLResultStream(connID, dbName, sqlStr, format string) (string, error) {
	logger.Info("[ExportService] 流式导出 SQL 结果: connID=%s db=%s format=%s sql_len=%d", connID, dbName, format, len(sqlStr))

	filePath, err := s.getSavePath("query_result", format)
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}

	// 获取总行数
	var totalRows int64
	dbConn, err := s.manager.GetDB(connID)
	if err == nil {
		cfg, ok := s.manager.GetConfig(connID)
		if ok {
			if err := database.UseDatabase(dbConn, cfg.Type, dbName); err != nil {
				logger.Warn("[ExportService] 切换数据库失败，跳过总行数统计: %v", err)
				return "", err
			}
		}
		cleanSQL := strings.TrimRight(strings.TrimSpace(sqlStr), ";")
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS __export_count__", cleanSQL)
		if err := dbConn.QueryRow(countSQL).Scan(&totalRows); err != nil {
			logger.Warn("[ExportService] 获取查询结果总行数失败: %v", err)
			totalRows = 0
		}
	}

	taskID := fmt.Sprintf("export_query_%d", time.Now().UnixMilli())
	taskCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()

	go s.runSQLStreamExport(taskCtx, taskID, connID, dbName, sqlStr, format, filePath, totalRows)
	return taskID, nil
}

// runSQLStreamExport 后台执行 SQL 结果流式导出
func (s *ExportService) runSQLStreamExport(ctx context.Context, taskID, connID, dbName, sqlStr, format, filePath string, totalRows int64) {
	defer func() {
		s.mu.Lock()
		delete(s.cancelMap, taskID)
		s.mu.Unlock()
	}()

	fileName := filepath.Base(filePath)
	s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "progress", Current: 0, Total: totalRows, FileName: fileName, FilePath: filePath})

	db, err := s.manager.GetDB(connID)
	if err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("获取连接失败: %v", err), FileName: fileName})
		return
	}

	cfg, ok := s.manager.GetConfig(connID)
	if ok {
		if err := database.UseDatabase(db, cfg.Type, dbName); err != nil {
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: err.Error(), FileName: fileName})
			return
		}
	}

	// 先探测列名
	cleanSQL := strings.TrimRight(strings.TrimSpace(sqlStr), ";")
	probeSQL := fmt.Sprintf("SELECT * FROM (%s) AS __probe__ LIMIT 1", cleanSQL)
	probeRows, err := db.Query(probeSQL)
	if err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("探测列失败: %v", err), FileName: fileName})
		return
	}
	columns, _ := probeRows.Columns()
	probeRows.Close()

	if len(columns) == 0 {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: "查询无返回列", FileName: fileName})
		return
	}

	file, err := os.Create(filePath)
	if err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("创建文件失败: %v", err), FileName: fileName})
		return
	}
	defer file.Close()

	writer, err := s.initWriter(file, format, columns, "query_result")
	if err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: err.Error(), FileName: fileName})
		return
	}

	var exported int64
	offset := 0
	for {
		select {
		case <-ctx.Done():
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "cancelled", Current: exported, Total: totalRows, FileName: fileName, FilePath: filePath})
			return
		default:
		}

		batchSQL := fmt.Sprintf("SELECT * FROM (%s) AS __batch__ LIMIT %d OFFSET %d", cleanSQL, exportBatchSize, offset)
		result, err := database.ExecuteQueryRaw(db, batchSQL)
		if err != nil {
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("查询失败: %v", err), FileName: fileName})
			return
		}
		if result.Error != "" {
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: result.Error, FileName: fileName})
			return
		}
		if len(result.Rows) == 0 {
			break
		}

		if err := s.writeBatch(writer, format, columns, "query_result", result.Rows); err != nil {
			s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("写入失败: %v", err), FileName: fileName})
			return
		}

		exported += int64(len(result.Rows))
		offset += exportBatchSize
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "progress", Current: exported, Total: totalRows, FileName: fileName, FilePath: filePath})

		if len(result.Rows) < exportBatchSize {
			break
		}
	}

	if err := s.finalizeWriter(writer, format); err != nil {
		s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "error", Error: fmt.Sprintf("文件收尾失败: %v", err), FileName: fileName})
		return
	}

	logger.Info("[ExportService] SQL结果流式导出完成: taskID=%s path=%s rows=%d", taskID, filePath, exported)
	s.emitExportProgress(ExportProgressEvent{TaskID: taskID, Status: "done", Current: exported, Total: exported, FileName: fileName, FilePath: filePath})
}

// --- 兼容旧接口（前端传数据导出） ---

// ExportCSV 导出前端传入数据为 CSV（兼容旧接口）
func (s *ExportService) ExportCSV(tableName string, columns []string, rows []map[string]interface{}) (string, error) {
	filePath, err := s.getSavePath(tableName, "csv")
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}
	return filePath, export.ToCSV(filePath, columns, rows)
}

// ExportJSON 导出前端传入数据为 JSON（兼容旧接口）
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

// ExportSQL 导出前端传入数据为 SQL INSERT（兼容旧接口）
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
	if s.app != nil {
		return s.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
			Title:    "导出数据",
			Filename: fmt.Sprintf("%s_%s.%s", tableName, time.Now().Format("20060102_150405"), ext),
		}).PromptForSingleSelection()
	}

	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Desktop",
		fmt.Sprintf("%s_%s.%s", tableName, time.Now().Format("20060102_150405"), ext)), nil
}
