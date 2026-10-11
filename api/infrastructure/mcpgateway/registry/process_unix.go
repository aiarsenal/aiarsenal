//go:build !windows

package registry

import (
	"os/exec"
	"syscall"
)

// setProcessGroup 让 stdio 子进程放进独立进程组，便于整组回收（Windows 版见 process_windows.go）。
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessTree 独立进程组整组强杀（防孙进程残留）：按 pgid 杀进程组，
// Getpgid 失败（进程已退出等）无从取组，回落单杀。
func killProcessTree(pid int) error {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return syscall.Kill(pid, syscall.SIGKILL)
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}
