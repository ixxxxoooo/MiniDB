//go:build !windows

package updater

import (
	"os/exec"
	"syscall"
)

// detach 让安装脚本脱离应用的进程组，应用退出后脚本仍能继续运行。
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
