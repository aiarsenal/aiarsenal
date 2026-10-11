//go:build !windows

package mcpclient

import (
	"os/exec"
	"syscall"
)

// setProcGroupAttr 把 stdio 子进程放进独立进程组，便于整组回收（Windows 版见 probe_proc_windows.go）。
func setProcGroupAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree 独立进程组整组强杀（防孙进程残留）：按 pgid 杀进程组，
// Getpgid 失败（进程已退出等）无从取组，回落单杀。
func killProcessTree(pid int) {
	if pgid, err := syscall.Getpgid(pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
