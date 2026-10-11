//go:build windows

package registry

import (
	"os/exec"
	"strconv"
	"syscall"
)

// setProcessGroup 让 stdio 子进程放进独立进程组，与宿主进程的 Ctrl+C 等信号隔离（Unix 版见 process_unix.go）。
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
}

// killProcessTree 整树强杀（防孙进程残留）：taskkill /T 按进程树递归终止。
func killProcessTree(pid int) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}
