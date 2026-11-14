package app

import (
	"compress/gzip"
	"context"

	"errors"
	"fmt"
	"io"
	"minidb/internal/model"

	"os"
	"path/filepath"
	"strings"

	"minidb/internal/database"
	"minidb/internal/logger"
	"time"
)

// RestoreRequest 开始一次还原。
type RestoreRequest struct {
	ConnID   string
	Database string // 还原到的库（可以是刚新建的库）
	FilePath string
	Tool     string
	// ExtraArgs 同 DumpOptions.ExtraArgs。
	ExtraArgs string
}

// restorePlan 是还原前检查出来的结果。
type restorePlan struct {
	tool string
	cmd  *dumpCommand
	size int64
}

// prepareRestore 检查还原能不能做：连接可写、类型支持、文件在、工具找得到。
func (s *DumpService) prepareRestore(req RestoreRequest) (*restorePlan, error) {
	cfg, ok := s.manager.GetConfig(req.ConnID)
	if !ok {
		return nil, model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", req.ConnID)
	}
	if err := ensureWritable(cfg); err != nil {
		return nil, err
	}
	if database.CapsOf(cfg.Type).LocalFile() {
		return nil, errors.New("SQLite 的备份就是数据库文件：把它添加为新连接即可打开；要导入 SQL 文件请用“导入文件”")
	}
	if req.Database == "" {
		return nil, model.Errf(model.CodeNoDBSelected, "没有选中的数据库")
	}
	st, err := os.Stat(req.FilePath)
	if err != nil {
		return nil, err
	}
	host, port, _ := s.manager.Endpoint(req.ConnID)
	cmd, err := buildRestoreCommand(cfg, host, port, req.Database, req.ExtraArgs)
	if err != nil {
		return nil, err
	}
	tool, err := findTool(cmd.names, req.Tool)
	if err != nil {
		return nil, err
	}
	return &restorePlan{tool: tool, cmd: cmd, size: st.Size()}, nil
}

// CheckRestore 只做还原前的检查，不动数据库；界面用它在新建数据库之前先发现问题。
func (s *DumpService) CheckRestore(req RestoreRequest) error {
	_, err := s.prepareRestore(req)
	return err
}

// StartRestore 开始还原：把 SQL 文件（可以是 .gz）交给 mysql 或 psql 执行。
func (s *DumpService) StartRestore(req RestoreRequest) (string, error) {
	plan, err := s.prepareRestore(req)
	if err != nil {
		return "", err
	}
	taskID := fmt.Sprintf("restore_%d", time.Now().UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelMap[taskID] = cancel
	s.mu.Unlock()
	go s.runRestore(ctx, taskID, req, plan.tool, plan.cmd, plan.size)
	return taskID, nil
}

func (s *DumpService) runRestore(ctx context.Context, taskID string, req RestoreRequest, tool string, cmd *dumpCommand, size int64) {
	defer s.finish(taskID)
	name := filepath.Base(req.FilePath)
	ev := func(status string, n int64, errMsg string) {
		s.emit(DumpProgressEvent{TaskID: taskID, Kind: "restore", Status: status, Current: n, Total: size, FileName: name, FilePath: req.FilePath, Error: errMsg})
	}
	ev("progress", 0, "")
	f, err := os.Open(req.FilePath)
	if err != nil {
		ev("error", 0, err.Error())
		return
	}
	defer f.Close()
	cr := &countingReader{r: f}
	var in io.Reader = cr
	if strings.HasSuffix(strings.ToLower(req.FilePath), ".gz") {
		gz, err := gzip.NewReader(cr)
		if err != nil {
			ev("error", 0, fmt.Sprintf("读取压缩文件失败: %v", err))
			return
		}
		defer gz.Close()
		in = gz
	}
	stop := make(chan struct{})
	go watch(stop, cr.n.Load, func(n int64) { ev("progress", n, "") })
	err = runTool(ctx, tool, cmd, in, io.Discard)
	close(stop)
	switch {
	case err == nil:
		logger.Info("[DumpService] 还原完成: %s", name)
		ev("done", size, "")
	case ctx.Err() != nil:
		ev("cancelled", cr.n.Load(), "")
	default:
		logger.Warn("[DumpService] 还原失败: %v", err)
		ev("error", cr.n.Load(), err.Error())
	}
}
