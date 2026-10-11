package main

import (
	"testing"
	"time"
)

// TestCmdStateIsBusyOnlyRunning 验证 isBusy 只把 running 态算忙。
// 这是 refresh-key 时 restart 命令被孤儿化的根因修复：
// 服务端在「上报 done 的那次心跳」里顺带下发下一条命令，若此时 isBusy 仍为 true（旧实现
// 只判 current!=nil），管理器会静默丢弃它 → ClaimNext 不重投 → 60s 超时标 failed。
func TestCmdStateIsBusyOnlyRunning(t *testing.T) {
	s := &cmdState{notify: make(chan struct{}, 1)}

	if s.isBusy() {
		t.Fatal("初始应空闲")
	}

	// 认领一条 config_push → running → 忙
	s.begin("cmd1", "openclaw", "config_push")
	if !s.isBusy() {
		t.Fatal("running 态应判定为忙")
	}

	// config_push 完成（done）→ 不应再忙（修复点）。
	// current 仍未被 clearReported 清掉，但已能让出队列接下一条。
	s.finish("cmd1", 3, "{}", "")
	if s.isBusy() {
		t.Fatal("done 态不应忙——否则同一次心跳下发的下一条命令会被丢弃（0.8.4 restart 失败根因）")
	}

	// 立刻认领下一条 restart（模拟服务端在同一次心跳响应里塞回的新命令）
	s.begin("cmd2", "openclaw", "restart")
	if s.current.commandId != "cmd2" {
		t.Fatal("done 后应能立刻认领下一条命令")
	}
	if !s.isBusy() {
		t.Fatal("新命令 running 应判定为忙")
	}
}

// TestRunCmdQuickTimesOut 验证 runCmdQuick 限时生效：体检命令（--version/pgrep）卡死时
// 不会拖垮心跳循环（曾致实例失联 40 分钟）。sleep 10 无限时会卡 10s。
func TestRunCmdQuickTimesOut(t *testing.T) {
	start := time.Now()
	_, err := runCmdQuick(200*time.Millisecond, "sleep", "10")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("超时应返回 error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("应在 ~200ms 返回，实际 %v（限时未生效）", elapsed)
	}
}

// TestExtractQrURL 验证从 openclaw channels login 的 stdout 抓微信 QR 内容 URL 的状态机。
// 规则（对齐 desktop platform.rs）：命中含「访问以下链接」/「以下链接以继续」的行后，
// 下一非空行即 QR URL；空行不重置；可多次刷新。
func TestExtractQrURL(t *testing.T) {
	st := &extractQrState{}

	// 普通行：不抓。
	if q := extractQrURL(st, "正在启动微信渠道..."); q != "" {
		t.Fatalf("普通行不应抓到 QR，got=%q", q)
	}
	// 提示行本身：不是 QR。
	if q := extractQrURL(st, "你可以访问以下链接以继续："); q != "" {
		t.Fatalf("提示行本身不应是 QR，got=%q", q)
	}
	// 提示行的下一行 = QR URL。
	want := "https://login.weixin.qq.com/qr/AbC123"
	if q := extractQrURL(st, want); q != want {
		t.Fatalf("应抓到 QR URL，want=%q got=%q", want, q)
	}

	// 二维码过期刷新：提示行后容忍空行，再下一行抓新 QR。
	if q := extractQrURL(st, "二维码已过期，请访问以下链接以继续："); q != "" {
		t.Fatalf("刷新提示行不应是 QR，got=%q", q)
	}
	if q := extractQrURL(st, ""); q != "" { // 空行不重置状态
		t.Fatalf("空行不应返回 QR，got=%q", q)
	}
	want2 := "https://login.weixin.qq.com/qr/Def456"
	if q := extractQrURL(st, want2); q != want2 {
		t.Fatalf("刷新后应抓到新 QR URL，want=%q got=%q", want2, q)
	}

	// 抓过后回到普通行：不再误抓。
	if q := extractQrURL(st, "登录成功"); q != "" {
		t.Fatalf("普通行不应误抓，got=%q", q)
	}
}
