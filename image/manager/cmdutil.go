package main

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"time"
)

// cmdutil.go — runtime 中立的命令执行 / 文件工具，被各 Manager（openclaw/hermes/…）复用。
// 从原 openclaw.go 抽出，使各 runtime 文件彼此解耦：删/改 openclaw.go 不再连带影响 hermes.go，
// 反之亦然。新增 runtime 只实现 RuntimeManager 接口即可，无需触碰这里。

// mutex 给各 Manager 用（避免直接暴露 sync.Mutex 类型名）。
type mutex = sync.Mutex

// runCmd 执行命令并返回合并输出（无超时，用于命令协程内的长任务如 ChannelLogin）。
func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runCmdQuick 执行命令并限时 timeout（用于心跳体检：--version/pgrep）。
// 这些命令跑在心跳循环里，若被管 runtime 瞬间 wedged（migration 锁竞争）会让子进程一起卡死、
// 冻住整个心跳循环（曾致实例失联 40 分钟）。限时兜底：超时即杀，该字段本轮跳过。
func runCmdQuick(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// fileMtime 返回文件 mtime 的本地格式化串（配置变更时间用），取值失败返回 ""。
func fileMtime(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fi.ModTime().Format("2006-01-02 15:04:05")
}
