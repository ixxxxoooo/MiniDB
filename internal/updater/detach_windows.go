//go:build windows

package updater

import (
	"os/exec"
	"syscall"
)

func detach(cmd *exec.Cmd) {
	// CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200 | 0x08000000, HideWindow: true}
}
