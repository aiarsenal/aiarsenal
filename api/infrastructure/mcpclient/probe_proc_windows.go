//go:build windows

package mcpclient

import (
	"os/exec"
	"strconv"
	"syscall"
)

// setProcGroupAttr 把 stdio 子进程放进独立进程组，与宿主进程的 Ctrl+C 等信号隔离（Unix 版见 probe_proc_unix.go）。
func setProcGroupAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// killProcessTree 整树强杀（防孙进程残留）：taskkill /T 按进程树递归终止。
func killProcessTree(pid int) {
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}
