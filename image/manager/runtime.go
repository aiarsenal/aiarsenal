package main

import (
	"log"
	"sync"
)

// runtime.go — runtime-agnostic 管理接口。
//
// aiarsenal-manager 是「唯一 manager」：无论实例里跑的是 openclaw 还是 hermes（或后续
// 其他 agent），都由本二进制 fork 子进程、上报、按 api 命令管理。具体 runtime 的差异
// （二进制名、启动参数、配置路径、模块 key）封装在各 Manager 实现里，main.go 按
// AIARSENAL_RUNTIME 选一个构造。api/心跳/认领全运行时无关。
// manager 自身由 webtop s6 longrun 保活；agent 不自动重启（崩了由 api 下发 restart）。
//
// 设计原则：加法式接口。OpenclawManager 行为零改动，仅多实现本接口。

// RuntimeManager 所有 runtime 必须实现的生命周期接口。
type RuntimeManager interface {
	// Name 模块 key（心跳 Modules map 的键，如 "openclaw"/"hermes"）。
	Name() string
	// Start 拉起被管 agent 子进程（非阻塞）+ 起守护协程。已在跑则跳过。
	Start() error
	// IsRunning 被管 agent 是否在跑（子进程存活 或 pgrep 命中）。
	IsRunning() bool
	// Stop 停止被管 agent（关守护 + 优雅停子进程）。
	Stop() error
	// Restart 重启被管 agent。
	Restart() error
	// Status 采集状态快照（供心跳上报）。
	Status() ModuleReport
	// PushConfig 写入配置文件（配置下发）。
	PushConfig(content string) error
	// ConfigExists 配置文件是否就绪（存在且非空）。
	// 启动期据此决定是否立即拉起：就绪（容器重启/存量卷）→ 立即 Start；
	// 未就绪（首次创建、卷空、配置待 config_push 下发）→ 不启动，等 executor 写盘后拉起（先配置后启动）。
	ConfigExists() bool
}

// ChannelLoginer 可选：支持渠道扫码登录的 runtime 实现（目前仅 openclaw）。
// executor 对 channel_login 命令做类型断言探测；不实现的 runtime 返回 not supported。
// pb 让 impl 边跑边写进度（微信 CLI 路径把抓到的 QR URL 写进 pb → 心跳上报 progress_json → UI 渲染二维码）。
type ChannelLoginer interface {
	ChannelLogin(channel string, pb *progressBuffer) (string, error)
}

// ChannelStatuser 可选：支持查询渠道登录态的 runtime（openclaw channels status）。
type ChannelStatuser interface {
	ChannelStatus(channel string) (string, error)
}

// ChannelLogouter 可选：支持渠道解绑（清账号 token）的 runtime（openclaw channels logout）。
type ChannelLogouter interface {
	ChannelLogout(channel string) (string, error)
}

// ChannelNotifier 可选：支持把「给人看的」通知文本发到渠道会话的 runtime（目前仅 hermes）。
// executor 对 channel_notify 命令做类型断言探测；不实现的 runtime 返回 not supported。
// 文本是平台侧拼好的人读内容（如临时预算审批结果）——数字人只是信使，把文本发到
// 与人对话的会话里，不是发给 runtime 的机器消息。
type ChannelNotifier interface {
	ChannelNotify(title, content, link string, pb *progressBuffer) (string, error)
}

// SkillInstaller 可选：支持 skill 安装/卸载的 runtime（openclaw skills install/list）。
// executor 对 install_skill/uninstall_skill 命令做类型断言探测；不实现的 runtime 返回 not supported。
// dir 是已解压的 skill 根（含 SKILL.md），slug 用作安装名与卸载定位键。
type SkillInstaller interface {
	InstallSkill(dir, slug string, pb *progressBuffer) (string, error)
	UninstallSkill(slug string, pb *progressBuffer) (string, error)
}

// SoulWriter 可选：支持平台 SOUL 铁律写入的 runtime（目前仅 hermes，$HERMES_HOME/SOUL.md）。
// executor 对 soul_push 命令做类型断言探测；不实现的 runtime 返回 not supported。
// content 由 api 侧拥有（DefaultHermesSoulMandate），manager 只负责幂等落盘，不解析内容。
type SoulWriter interface {
	PushSoul(content string) error
}

// newRuntimeManagerByName 按 runtime 名构造 Manager（差异封装在各 Manager，调用方只认名字）。
// 加新 agent 在此扩 switch（默认兜底 openclaw）。
func newRuntimeManagerByName(name string) RuntimeManager {
	switch name {
	case "hermes":
		return NewHermesManager()
	default:
		return NewOpenclawManager()
	}
}

// managerSet 被管 agent 集合（按 supportAgents）。
// 单 agent pod（openclaw/hermes）集合里 1 个；common pod（未来）多个；空=无被管 agent（common 启动期，待服务端下发）。
// mutex 保护：heartbeat 循环 sync 增删，命令 goroutine get 读，并发安全。
type managerSet struct {
	mu sync.Mutex
	m  map[string]RuntimeManager
}

func newManagerSet(names []string) *managerSet {
	ms := &managerSet{m: map[string]RuntimeManager{}}
	for _, n := range names {
		ms.m[n] = newRuntimeManagerByName(n)
	}
	return ms
}

func (s *managerSet) names() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.m))
	for k := range s.m {
		out = append(out, k)
	}
	return out
}

// all 返回当前 managers 的快照（副本），供上报遍历（遍历期间不持锁）。
func (s *managerSet) all() map[string]RuntimeManager {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]RuntimeManager, len(s.m))
	for k, v := range s.m {
		out[k] = v
	}
	return out
}

func (s *managerSet) get(name string) RuntimeManager {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[name]
}

func (s *managerSet) startAll() {
	for _, m := range s.all() {
		if err := m.Start(); err != nil {
			log.Printf("[agent] 初始启动 %s 失败: %v（继续心跳，等命令重试）", m.Name(), err)
		}
	}
}

// startAllIfReady 仅启动配置已就绪的 agent（容器重启/存量卷场景）。
// 配置未就绪（首次创建、卷空、配置待 config_push 下发）的跳过——等 executor 的 config_push
// 写盘后拉起（先配置后启动，避免 openclaw 用空配置起几秒再被 restart 纠正）。
func (s *managerSet) startAllIfReady() {
	for name, m := range s.all() {
		if !m.ConfigExists() {
			log.Printf("[agent] %s 配置未就绪，跳过初始启动（等 config_push 下发后拉起）", name)
			continue
		}
		if err := m.Start(); err != nil {
			log.Printf("[agent] 初始启动 %s 失败: %v（继续心跳，等命令重试）", name, err)
		}
	}
}

func (s *managerSet) stopAll() {
	for _, m := range s.all() {
		_ = m.Stop()
	}
}

// sync 按 supportAgents（[agent,version]）增删 Manager：新增的构造+Start；移除的 Stop+删。
// supportAgents 稳定（单 agent pod）时无操作；common pod 服务端下发变化时增删。
// Stop/Start 是进程操作较慢，放锁外执行避免长时间阻塞 get。
func (s *managerSet) sync(want [][2]string) {
	wantSet := map[string]bool{}
	for _, p := range want {
		wantSet[p[0]] = true
	}
	s.mu.Lock()
	for name, m := range s.m {
		if !wantSet[name] {
			delete(s.m, name)
			s.mu.Unlock()
			_ = m.Stop()
			log.Printf("[agent] 移除 agent %s（不在 supportAgents）", name)
			s.mu.Lock()
		}
	}
	for _, p := range want {
		name := p[0]
		if _, ok := s.m[name]; !ok {
			m := newRuntimeManagerByName(name)
			s.m[name] = m
			s.mu.Unlock()
			if err := m.Start(); err != nil {
				log.Printf("[agent] 新增 agent %s 启动失败: %v", name, err)
			} else {
				log.Printf("[agent] 新增 agent %s (%s)", name, p[1])
			}
			s.mu.Lock()
		}
	}
	s.mu.Unlock()
}
