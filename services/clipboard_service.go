package services

import (
	"fmt"
	"os/exec"
	"runtime"
	"minidb/internal/logger"
)

// ClipboardService 提供应用内稳定的文本复制能力。
type ClipboardService struct{}

func NewClipboardService() *ClipboardService {
	return &ClipboardService{}
}

// SetText 将文本写入系统剪贴板。
func (s *ClipboardService) SetText(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("/usr/bin/pbcopy")
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "$input | Set-Clipboard")
	case "linux":
		cmd = exec.Command("sh", "-c", "command -v wl-copy >/dev/null 2>&1 && wl-copy || xclip -selection clipboard")
	default:
		return fmt.Errorf("unsupported clipboard platform: %s", runtime.GOOS)
	}

	logger.Debug("[ClipboardService] 写入剪贴板: bytes=%d", len([]byte(text)))
	in, err := cmd.StdinPipe()
	if err != nil {
		logger.Error("[ClipboardService] 获取剪贴板写入管道失败: %v", err)
		return err
	}
	if err := cmd.Start(); err != nil {
		logger.Error("[ClipboardService] 启动剪贴板命令失败: %v", err)
		return err
	}
	if _, err := in.Write([]byte(text)); err != nil {
		_ = in.Close()
		_ = cmd.Wait()
		logger.Error("[ClipboardService] 写入剪贴板命令失败: %v", err)
		return err
	}
	if err := in.Close(); err != nil {
		_ = cmd.Wait()
		logger.Error("[ClipboardService] 关闭剪贴板写入管道失败: %v", err)
		return err
	}
	if err := cmd.Wait(); err != nil {
		logger.Error("[ClipboardService] 剪贴板命令执行失败: %v", err)
		return err
	}
	logger.Debug("[ClipboardService] 剪贴板写入完成")
	return nil
}

// GetText 读取系统剪贴板中的文本，用于复制后的校验。
func (s *ClipboardService) GetText() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("/usr/bin/pbpaste")
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw")
	case "linux":
		cmd = exec.Command("sh", "-c", "command -v wl-paste >/dev/null 2>&1 && wl-paste --no-newline || xclip -selection clipboard -o")
	default:
		return "", fmt.Errorf("unsupported clipboard platform: %s", runtime.GOOS)
	}

	out, err := cmd.Output()
	if err != nil {
		logger.Error("[ClipboardService] 读取剪贴板失败: %v", err)
		return "", err
	}
	return string(out), nil
}
