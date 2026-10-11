package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// openclawBinary openclaw CLI 路径（npm 全局在 PATH；官方镜像软链到 /usr/local/bin/openclaw）。
const openclawBinary = "openclaw"

// openclawConfigPath openclaw 配置文件路径。桌面镜像走 /aa/.openclaw（aiarsenal 持久化目录，挂 PVC），
// 官方镜像走 /home/node/.openclaw。读环境变量 OPENCLAW_CONFIG_PATH 覆盖。
func openclawConfigPath() string {
	if p := os.Getenv("OPENCLAW_CONFIG_PATH"); p != "" {
		return p
	}
	return "/aa/.openclaw/openclaw.json"
}

// OpenclawManager 管理 openclaw 进程（容器内）。
//
// manager 自身由 webtop s6 longrun 保活（进程崩了 s6 重启 manager）。
// 这里只负责按 api 下发的命令 fork/停/重启 openclaw gateway + 写配置 + 上报状态。
// 不做"崩了自动拉起"——agent 的生命周期由 api 驱动（心跳发现 down → api 下发 restart）。
type OpenclawManager struct {
	gatewayCmd *exec.Cmd
	startTime  time.Time
	mu         mutex
}

func NewOpenclawManager() *OpenclawManager { return &OpenclawManager{} }

// Name 心跳模块 key（实现 RuntimeManager）。
func (m *OpenclawManager) Name() string { return "openclaw" }

// Start 拉起 openclaw gateway（非阻塞后台子进程）。已在跑则跳过。
// 用 manager 自己的 gatewayCmd 状态判定「是否已拉起」，不依赖 IsRunning() 的 pgrep——
// 首次 config_push 时 pgrep -f openclaw-gateway 存在假阳性（误判在跑），会让 Start 跳过 spawn，
// 导致 openclaw 不启动。gatewayCmd==nil（从未拉起）或已退出（ProcessState!=nil）才 spawn。
func (m *OpenclawManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gatewayCmd != nil && m.gatewayCmd.ProcessState == nil {
		return nil
	}
	return m.spawn()
}

// spawn 实际 fork openclaw gateway 子进程（调用方持锁）。
// 必须用 `gateway run`（前台常驻 Gateway）；裸 `gateway` 不起 gateway 只留 stale migration lock；
// `gateway start`/`restart`/`stop` 走 systemd（容器里 disabled，会卡超时）——一律不用。
func (m *OpenclawManager) spawn() error {
	cmd := exec.Command(openclawBinary, "gateway", "run")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 openclaw gateway run 失败: %w", err)
	}
	m.gatewayCmd = cmd
	m.startTime = time.Now()
	log.Printf("[agent] openclaw gateway run 已拉起 pid=%d", cmd.Process.Pid)
	// 回收子进程：Wait 防僵尸 + 退出后清句柄（让 IsRunning 落到 pgrep 判定）。不自动重启。
	go func(c *exec.Cmd) {
		_ = c.Wait()
		m.mu.Lock()
		if m.gatewayCmd == c {
			m.gatewayCmd = nil
		}
		m.mu.Unlock()
	}(cmd)
	return nil
}

// IsRunning openclaw gateway 是否在跑（子进程存活 或 pgrep 命中）。
// 真实进程名是 openclaw-gateway（node 启动后 process.title 重命名），不是 "openclaw gateway run"。
func (m *OpenclawManager) IsRunning() bool {
	if m.gatewayCmd != nil && m.gatewayCmd.ProcessState == nil {
		return true
	}
	out, err := runCmdQuick(5*time.Second, "pgrep", "-f", "openclaw-gateway")
	return err == nil && strings.TrimSpace(out) != ""
}

// Stop 停止 openclaw：SIGTERM 持有子进程（最多 5s 优雅退）→ SIGKILL 兜底。
// 不用 `gateway stop`（systemd，容器 disabled 会卡）。spawn 的 Wait 协程负责回收僵尸。
func (m *OpenclawManager) Stop() error {
	m.mu.Lock()
	cmd := m.gatewayCmd
	m.gatewayCmd = nil
	m.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	for i := 0; i < 50 && cmd.ProcessState == nil; i++ { // 最多等 5s
		time.Sleep(100 * time.Millisecond)
	}
	if cmd.ProcessState == nil {
		_ = cmd.Process.Kill()
	}
	return nil
}

// Restart 优先 SIGUSR1（openclaw 官方 in-process 重启：排空活跃任务后重启、状态不丢、无停机，
// docs/cli/gateway.md 「SIGUSR1 triggers an in-process restart」；gated by commands.restart 默认 true）。
// 仅对前台 `gateway run` 子进程有效。进程没在跑 / 发信号失败 → 回退 Stop + Start。
// 不用 `gateway restart`（管理 launchd/systemd 服务，容器里 disabled 会卡超时；且会触发
// startup migration 锁竞争）。
func (m *OpenclawManager) Restart() error {
	m.mu.Lock()
	cmd := m.gatewayCmd
	m.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.SIGUSR1); err == nil {
			return nil
		}
	}
	if err := m.Stop(); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return m.Start()
}

// PushConfig 写 openclaw.json（配置下发）。
func (m *OpenclawManager) PushConfig(content string) error {
	if content == "" {
		return fmt.Errorf("配置内容为空")
	}
	if err := os.MkdirAll(filepath.Dir(openclawConfigPath()), 0700); err != nil {
		return err
	}
	return os.WriteFile(openclawConfigPath(), []byte(content), 0600)
}

// ConfigExists openclaw.json 是否存在且非空（启动期判定是否立即拉起）。
func (m *OpenclawManager) ConfigExists() bool {
	info, err := os.Stat(openclawConfigPath())
	return err == nil && info.Size() > 0
}

// ChannelLogin 渠道扫码登录。
//
// 微信（channel 含 "weixin"）走 CLI 降级路径：openclaw 的 web.login RPC 不支持微信，
// 改跑 `openclaw channels login --channel <id>`，逐行读 stdout 抓 QR 内容 URL（微信插件 displayQRCode
// 在"你可以访问以下链接以继续："后一行打印）→ 写 pb（→ 心跳上报 progress_json → UI 渲染二维码），
// 阻塞等进程退出（扫码完成/超时）。移植自 desktop platform.rs:124-196（desktop 是非阻塞+后台线程，
// 这里阻塞版更简单：executor 本就在 goroutine 里跑，且已有 pb 进度通道）。
//
// 其它渠道（WhatsApp/Zalo 等）走 web.login.start RPC（返回 qrDataUrl）。
func (m *OpenclawManager) ChannelLogin(channel string, pb *progressBuffer) (string, error) {
	if strings.Contains(channel, "weixin") {
		return m.channelLoginWeixinCLI(channel, pb)
	}
	return m.channelLoginRPC(channel, pb)
}

// ChannelStatus 查渠道登录态：跑 `openclaw channels status --channel <id> --json`，解析
// channelAccounts[id][0] 的 configured/linked/connected，产出结构化 result 给前端（不外泄 openclaw schema）。
// bound = configured && linked && connected。--json 输出可能混日志前缀，截首个 { 到末 } 再 Unmarshal。
func (m *OpenclawManager) ChannelStatus(channel string) (string, error) {
	args := []string{"channels", "status", "--json"}
	if channel != "" {
		args = append(args, "--channel", channel)
	}
	out, err := runCmd(openclawBinary, args...)
	if err != nil {
		return out, fmt.Errorf("channels status 失败: %s", strings.TrimSpace(out))
	}
	// 实测微信渠道 channelAccounts[id][0] 字段：accountId/configured/running/enabled/lastError
	// （无 linked/connected/name，与 openclaw schema 文档不同；configured=true 即账号已配置=已绑定）。
	var st struct {
		ChannelAccounts map[string][]struct {
			AccountId  string `json:"accountId"`
			Configured *bool  `json:"configured"`
			Running    *bool  `json:"running"`
			Enabled    *bool  `json:"enabled"`
			LastError  string `json:"lastError"`
		} `json:"channelAccounts"`
	}
	bound := false
	alias := ""
	accountId := ""
	connected := false
	lastError := ""
	if json.Unmarshal([]byte(extractJSON(out)), &st) == nil {
		if accs, ok := st.ChannelAccounts[channel]; ok && len(accs) > 0 {
			a := accs[0]
			accountId = a.AccountId
			alias = a.AccountId // 该渠道 account 无 name 字段，用 accountId 作展示
			connected = a.Running != nil && *a.Running
			lastError = a.LastError
			bound = a.Configured != nil && *a.Configured
		}
	}
	res, _ := json.Marshal(map[string]any{
		"channel":   channel,
		"bound":     bound,
		"accountId": accountId,
		"alias":     alias,
		"connected": connected,
		"lastError": lastError,
	})
	return string(res), nil
}

// ChannelLogout 解绑渠道（清账号 token）：跑 `openclaw channels logout --channel <id>`。
// 成功即 done（openclaw 非零退出码 → failed）。返回合并输出便于排查。
func (m *OpenclawManager) ChannelLogout(channel string) (string, error) {
	out, err := runCmd(openclawBinary, "channels", "logout", "--channel", channel)
	if err != nil {
		return out, fmt.Errorf("channels logout 失败: %s", strings.TrimSpace(out))
	}
	return out, nil
}

// InstallSkill 把已解压的 skill 目录安装到 openclaw 共享托管目录（--global）。
// openclaw skills install 只收本地目录（不收 URL/ZIP），slug 由 --as 指定以便卸载时定位。
// 重装/升级语义：install 前先把旧版本目录挪走为备份（CLI 对已存在目录的覆盖语义未定义，
// 文档只说 Git/local 源须重装刷新），install 成功清备份、失败挪回——旧版本原样保留。
// 实现 SkillInstaller（仅 openclaw；hermes 等不实现 → executor 返回 not supported）。
// 限时 300s 兜底：openclaw 可能跑 skill 生命周期钩子，防 goroutine 泄漏。
func (m *OpenclawManager) InstallSkill(dir, slug string, pb *progressBuffer) (string, error) {
	if pb != nil {
		pb.Write(fmt.Sprintf("openclaw skills install %s --as %s --global", dir, slug))
	}
	// 托管目录解析与 UninstallSkill 同源；list 跑不动/未装过则不挪（直装，让 CLI 自己处理）。
	backup := ""
	if out, err := runCmdQuick(60*time.Second, openclawBinary, "skills", "list", "--json"); err == nil {
		if managedDir, present := parseManagedSkillsDir(extractJSON(out), slug); managedDir != "" && present {
			target := filepath.Join(managedDir, slug)
			bak := target + ".aa-installing"
			_ = os.RemoveAll(bak) // 清上次异常残留
			if err := os.Rename(target, bak); err == nil {
				backup = bak
				if pb != nil {
					pb.Write(fmt.Sprintf("已挪走旧版本目录 → %s（失败将自动挪回）", bak))
				}
			} else if pb != nil {
				pb.Write(fmt.Sprintf("挪走旧版本目录失败，继续直装: %v", err))
			}
		}
	}
	out, err := runCmdQuick(300*time.Second, openclawBinary, "skills", "install", dir, "--as", slug, "--global")
	if err != nil {
		if backup != "" {
			_ = os.Rename(backup, strings.TrimSuffix(backup, ".aa-installing")) // 挪回旧版本
		}
		return out, fmt.Errorf("openclaw skills install 失败: %s", strings.TrimSpace(out))
	}
	if backup != "" {
		_ = os.RemoveAll(backup) // 新版就位，清备份
	}
	return out, nil
}

// UninstallSkill 卸载：openclaw 无 uninstall 子命令，读 skills list --json 取 managedSkillsDir，
// 确认 slug 已装后删除其目录（<managedSkillsDir>/<slug>）；openclaw path-watch 自动刷新，无需重启。
// slug 不在列表（已删）→ 幂等成功。
func (m *OpenclawManager) UninstallSkill(slug string, pb *progressBuffer) (string, error) {
	if pb != nil {
		pb.Write(fmt.Sprintf("openclaw skills list --json → 定位 %s 安装目录", slug))
	}
	out, err := runCmdQuick(60*time.Second, openclawBinary, "skills", "list", "--json")
	if err != nil {
		return out, fmt.Errorf("openclaw skills list 失败: %s", strings.TrimSpace(out))
	}
	managedDir, present := parseManagedSkillsDir(extractJSON(out), slug)
	if managedDir == "" {
		return "", fmt.Errorf("无法从 skills list 解析 managedSkillsDir")
	}
	if !present {
		return fmt.Sprintf(`{"action":"uninstall_skill","slug":%q,"alreadyGone":true}`, slug), nil
	}
	target := filepath.Join(managedDir, slug)
	if err := os.RemoveAll(target); err != nil {
		return "", fmt.Errorf("删除 skill 目录失败: %w", err)
	}
	if pb != nil {
		pb.Write(fmt.Sprintf("已删除 %s", target))
	}
	return fmt.Sprintf(`{"action":"uninstall_skill","slug":%q,"removed":%q}`, slug, target), nil
}

// parseManagedSkillsDir 从 `openclaw skills list --json` 输出解析 managedSkillsDir，
// 并判断 slug 是否在已装列表（skills[].name）。openclaw list JSON 形如：
//
//	{"workspaceDir":"...","managedSkillsDir":"...","skills":[{"name":"<slug>",...}]}
func parseManagedSkillsDir(jsonStr, slug string) (managedDir string, slugPresent bool) {
	var st struct {
		ManagedSkillsDir string `json:"managedSkillsDir"`
		Skills           []struct {
			Name string `json:"name"`
		} `json:"skills"`
	}
	if json.Unmarshal([]byte(jsonStr), &st) != nil {
		return "", false
	}
	for _, s := range st.Skills {
		if s.Name == slug {
			slugPresent = true
			break
		}
	}
	return st.ManagedSkillsDir, slugPresent
}

// extractJSON 从可能混有日志前缀的 CLI 输出里截取首个 { 到末个 } 的子串（--json 输出容错）。
func extractJSON(s string) string {
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j < 0 || j < i {
		return s
	}
	return s[i : j+1]
}

// channelLoginRPC web.login.start RPC 路径（非微信渠道）。返回 openclaw 的 JSON 输出（含 qrDataUrl）。
func (m *OpenclawManager) channelLoginRPC(channel string, pb *progressBuffer) (string, error) {
	if pb != nil {
		pb.Write(fmt.Sprintf("发起渠道登录（RPC web.login.start）channel=%s", channel))
	}
	params := "{}"
	if channel != "" {
		params = fmt.Sprintf(`{"channel":"%s"}`, channel)
	}
	out, err := runCmd(openclawBinary, "gateway", "call", "web.login.start", "--params", params, "--json")
	if err != nil {
		return out, fmt.Errorf("web.login.start 失败: %s", strings.TrimSpace(out))
	}
	return out, nil
}

// channelLoginWeixinCLI 微信 CLI 登录：阻塞跑 `openclaw channels login --channel <id>`，
// 逐行读 stdout 抓 QR 内容 URL（"若二维码...访问以下链接以继续："后一行）写 pb（→ 心跳 progress_json → UI 渲染），
// 等进程退出（扫码完成/超时）。
//
// 前置（须在镜像/配置层就绪，不在本函数运行时装）：
//   - 插件 @tencent-weixin/openclaw-weixin 已在镜像构建时预装（Dockerfile），gateway 启动即加载；
//     运行时装会慢（~5min npm）且需重启 gateway 才加载，故不在此装。
//   - openclaw.json 声明了本渠道（channels.openclaw-weixin + plugins.entries.openclaw-weixin.enabled），
//     由 api 侧 BuildOpenclawConfig 下发。否则 channels login 走交互"Install plugin?"提示、无 TTY 直接 exit 13。
func (m *OpenclawManager) channelLoginWeixinCLI(channel string, pb *progressBuffer) (string, error) {
	writef := func(s string) {
		if pb != nil {
			pb.Write(s)
		}
	}
	log.Printf("[agent] channel_login 微信 CLI 路径 channel=%s", channel)
	writef(fmt.Sprintf("发起微信扫码登录：openclaw channels login --channel %s", channel))

	// 阻塞跑 channels login，限时 270s（命令总限时 300s，留 30s 余量收尾，防 goroutine 泄漏）。
	ctx, cancel := context.WithTimeout(context.Background(), 270*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, openclawBinary, "channels", "login", "--channel", channel)
	cmd.Stderr = os.Stderr // 诊断信息进容器日志
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[agent] channel_login stdout pipe 失败: %v", err)
		return "", fmt.Errorf("打开 channels login stdout 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[agent] channel_login 启动失败: %v", err)
		return "", fmt.Errorf("启动 channels login 失败: %w", err)
	}
	log.Printf("[agent] channel_login 已启动 pid=%d，等待 stdout", cmd.Process.Pid)

	// 逐行读 stdout，抓 QR URL。buffer 放大：URL 可能较长。
	st := &extractQrState{}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		log.Printf("[agent] channel_login stdout: %s", line)
		if qr := extractQrURL(st, line); qr != "" {
			// 只把 QR URL 写进 pb，且必须是最后写入的行：心跳取 pb.LastLine 上报 progress_json.step，
			// UI 据此渲染二维码。若在此后再写文案行，LastLine 会变成文案、UI 拿不到二维码。
			log.Printf("[agent] channel_login 抓到 QR URL（%d 字节）", len(qr))
			writef(qr)
		}
	}

	// 等退出：扫码完成 exit 0；超时/异常 → failed。
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("[agent] channel_login 扫码超时（270s）")
		return "", fmt.Errorf("扫码超时（270s 内未完成）")
	}
	if err := cmd.Wait(); err != nil {
		log.Printf("[agent] channel_login 异常退出: %v", err)
		return "", fmt.Errorf("channels login 异常退出: %w", err)
	}
	log.Printf("[agent] channel_login 微信登录成功")
	writef("微信登录成功")
	return fmt.Sprintf(`{"action":"channel_login","channel":%q,"status":"connected"}`, channel), nil
}

// extractQrState 抓 QR 的逐行状态机（纯逻辑，可单测，不依赖真实 openclaw）。
type extractQrState struct {
	nextIsUrl bool // 上一行是「访问以下链接」提示 → 本行应为 QR URL
}

// extractQrURL 按行喂入，返回非空表示该行是抓到的 QR 内容 URL。
// 微信插件在含「访问以下链接」/「以下链接以继续」的提示行之后一行打印 QR 内容 URL（对齐 desktop platform.rs）。
// 空行不重置状态（容忍提示行与 QR 间的空行）。
func extractQrURL(st *extractQrState, line string) string {
	if st.nextIsUrl && line != "" {
		st.nextIsUrl = false
		return line
	}
	if strings.Contains(line, "访问以下链接") || strings.Contains(line, "以下链接以继续") {
		st.nextIsUrl = true
	}
	return ""
}

// Status 采集 openclaw 模块状态（供心跳上报）。
func (m *OpenclawManager) Status() ModuleReport {
	r := ModuleReport{Status: "stopped"}
	m.mu.Lock()
	cmd := m.gatewayCmd
	startTime := m.startTime
	m.mu.Unlock()
	if m.IsRunning() {
		r.Status = "running"
		if cmd != nil && cmd.Process != nil {
			r.PidOrContainerId = fmt.Sprintf("%d", cmd.Process.Pid)
			r.UptimeSeconds = int64(time.Since(startTime).Seconds())
		} else if out, err := runCmdQuick(5*time.Second, "pgrep", "-f", "openclaw"); err == nil {
			r.PidOrContainerId = strings.TrimSpace(out)
		}
	}
	if ver, err := runCmdQuick(5*time.Second, openclawBinary, "--version"); err == nil {
		r.Version = strings.TrimSpace(ver)
	}
	if content, err := os.ReadFile(openclawConfigPath()); err == nil {
		r.ConfigSize = int64(len(content))
		r.ConfigMtime = fileMtime(openclawConfigPath())
		r.ConfigContent = string(content)
		sum := sha256.Sum256(content)
		r.ConfigHash = hex.EncodeToString(sum[:])
	}
	return r
}

// 注：runCmd / runCmdQuick / fileMtime / mutex 已抽到 cmdutil.go（runtime 中立，供各 Manager 复用）。
