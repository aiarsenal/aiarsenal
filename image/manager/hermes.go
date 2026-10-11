package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// hermes.go — HermesManager：把 hermes gateway 当作「被管 agent 子进程」托管。
//
// 与 OpenclawManager 同构（fork 子进程 + 配置下发 + 状态上报），差异仅在二进制/参数/配置路径/模块 key。
// 复用 cmdutil.go 的 runCmd / fileMtime / mutex（runtime 中立工具）。
//
// manager 自身由 webtop s6 longrun 保活。这里只按 api 命令 fork/停/重启 hermes gateway + 写配置 + 上报。
// 不自动重启——agent 生命周期由 api 驱动（心跳发现 down → api 下发 restart）。
//
// 启动：`<hermes> gateway run --external-supervisor`，前台阻塞。
//   --external-supervisor：声明外部进程管理器（即本 manager）拥有此前台 gateway，
//   hermes 内部的 restart/update 会退出回到本 manager，而不是自己 fork 脱离子进程。
// 停止：`<hermes> gateway stop`（读 {HERMES_HOME}/gateway.pid 发 SIGTERM）+ 回退 kill 子进程。
// 配置：{HERMES_HOME}/config.yaml。HERMES_HOME 默认 /aa/.hermes（随 PVC 持久化）。

// hermesBinary hermes CLI 路径。镜像里 hermes venv 装在 /opt/hermes/.venv；
// 直接用绝对路径调 venv 二进制，绕开 hermes 自带的 root→hermes 降权 shim
// （本 manager 以 aa 身份跑，非 root，shim 本就短路；用绝对路径更确定）。HERMES_BINARY 可覆盖。
func hermesBinary() string {
	if p := os.Getenv("HERMES_BINARY"); p != "" {
		return p
	}
	return "/opt/hermes/.venv/bin/hermes"
}

// hermesConfigPath 配置文件路径：$HERMES_HOME/config.yaml（HERMES_HOME 默认 /aa/.hermes）。
func hermesConfigPath() string {
	home := os.Getenv("HERMES_HOME")
	if home == "" {
		home = "/aa/.hermes"
	}
	return filepath.Join(home, "config.yaml")
}

// hermesBaseConfigPath API 下发的原始（model-only）base 配置：$HERMES_HOME/config.base.yaml。
// 实际 config.yaml 由 writeEffectiveConfig 按 weixin 绑定态在 base 上叠加生成（重启不丢）。
func hermesBaseConfigPath() string {
	home := os.Getenv("HERMES_HOME")
	if home == "" {
		home = "/aa/.hermes"
	}
	return filepath.Join(home, "config.base.yaml")
}

// weixinOpenPlatformYAML 绑定后追加到 base 的微信平台声明（open 策略，配合 .env 的
// GATEWAY_ALLOW_ALL_USERS=true 放行所有发件人，对齐 openclaw，免配对码）。
const weixinOpenPlatformYAML = `gateway:
  platforms:
    weixin:
      enabled: true
      extra:
        dm_policy: "open"
        group_policy: "disabled"
`

// approvalsOffYAML 无条件追加的审批策略块：容器即沙箱，而消息端用户（微信）看不懂英文审批卡、
// 不会回 /approve——弹卡只会挂满审批超时（约 5 分钟）后 BLOCKED，整轮对话白等。mode=off 关掉
// 弹卡；毁灭性命令仍被 hermes hardline 底线直接拒（任何模式含 yolo 都不可绕过，不走弹卡）。
const approvalsOffYAML = `approvals:
  mode: "off"
`

// onboardingOffYAML 关掉首条消息的「档案建立」提议：hermes 首条消息 onboarding 默认
// profile_build=ask，会往当轮注入 sidecar 让 agent 自我介绍 + 提议「花一分钟了解你」（问称呼/
// 用途）——微信终端用户看不懂也不会答，白占一轮。off 后仅剩一句简短介绍（/help 提及由 SOUL
// 沟通铁律压制）。"off" 必须带引号：YAML 1.1 裸 off 解析为 bool，hermes 侧只认字符串 "off"。
const onboardingOffYAML = `onboarding:
  profile_build: "off"
`

// weixinBound .env 是否已有 WEIXIN_TOKEN（=已绑定微信）。
func weixinBound() bool { return readDotEnv(hermesEnvFile())["WEIXIN_TOKEN"] != "" }

// writeEffectiveConfig 生成实际 config.yaml：base + 审批关闭块 + onboarding 关闭块 +（已绑定时）weixin open 平台块。
// 未绑定 → base + 两策略块（gateway 干净启动）；已绑定 → 再叠 weixin（+ .env 的 GATEWAY_ALLOW_ALL_USERS）放行所有人。
// base 显式声明同名顶级键时尊重 base（不重复叠加），控制面将来想按实例覆盖策略留了口子。
func writeEffectiveConfig() error {
	base, _ := os.ReadFile(hermesBaseConfigPath())
	out := string(base)
	if !strings.Contains(out, "approvals:") {
		out = strings.TrimRight(out, "\n") + "\n" + approvalsOffYAML
	}
	if !strings.Contains(out, "onboarding:") {
		out = strings.TrimRight(out, "\n") + "\n" + onboardingOffYAML
	}
	if weixinBound() && !strings.Contains(out, "platforms:") {
		out = strings.TrimRight(out, "\n") + "\n" + weixinOpenPlatformYAML
	}
	return os.WriteFile(hermesConfigPath(), []byte(out), 0o600)
}

// HermesManager 管理 hermes gateway 进程（容器内）。
type HermesManager struct {
	gatewayCmd *exec.Cmd
	startTime  time.Time
	mu         mutex
}

func NewHermesManager() *HermesManager { return &HermesManager{} }

// Name 心跳模块 key（实现 RuntimeManager）。
func (m *HermesManager) Name() string { return "hermes" }

// regenEffectiveConfigIfProvisioned 启动期折叠：实例已下发过配置（config.yaml 与 base 均在）才
// 重生成 config.yaml，让存量实例重启即升级到当前 manager 逻辑（审批关闭块等）——writeEffectiveConfig
// 平时只在 config_push / 微信绑解绑时触发，老 pod 单纯重启拿不到新增的叠加块。
// base 缺失（早于 base 机制的存量实例）不动 config.yaml，防误清模型配置；config.yaml 缺失/空
// （未下发配置的新实例）也不生成，避免只有叠加块的配置把 ConfigExists 误判成就绪。
func regenEffectiveConfigIfProvisioned() {
	if _, err := os.Stat(hermesBaseConfigPath()); err != nil {
		return
	}
	if info, err := os.Stat(hermesConfigPath()); err != nil || info.Size() == 0 {
		return
	}
	_ = writeEffectiveConfig()
}

// Start 拉起 hermes gateway（非阻塞后台子进程）。已在跑则跳过。
// 用 gatewayCmd 状态判定「是否已拉起」，不依赖 IsRunning() 的 pgrep（与 openclaw 同：pgrep 假阳性会漏拉起）。
func (m *HermesManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gatewayCmd != nil && m.gatewayCmd.ProcessState == nil {
		return nil
	}
	regenEffectiveConfigIfProvisioned()
	return m.spawn()
}

// spawn 实际 fork hermes gateway 子进程（调用方持锁）。
func (m *HermesManager) spawn() error {
	// --no-supervise：webtop 是 s6-overlay 镜像，`hermes gateway run` 默认重定向到 s6 托管服务
	// （未安装 → "no such gateway 'default'" 直接退出）。manager 自己 fork 守护，须显式跳过 s6 重定向，
	// 让 gateway 以前台子进程跑。--external-supervisor 保留：声明本 manager 拥有此前台 gateway
	// （in-chat restart/update 退出回到 manager 而非自行 fork 脱离）。
	cmd := exec.Command(hermesBinary(), "gateway", "run", "--no-supervise", "--external-supervisor")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 hermes gateway 失败: %w", err)
	}
	m.gatewayCmd = cmd
	m.startTime = time.Now()
	log.Printf("[agent] hermes gateway 已拉起 pid=%d", cmd.Process.Pid)
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

// IsRunning hermes gateway 是否在跑（子进程存活 或 pgrep 命中）。
func (m *HermesManager) IsRunning() bool {
	if m.gatewayCmd != nil && m.gatewayCmd.ProcessState == nil {
		return true
	}
	out, err := runCmd("pgrep", "-f", "hermes gateway")
	return err == nil && strings.TrimSpace(out) != ""
}

// Stop 停止 hermes：优先 gateway stop + 回退 kill 持有子进程。
func (m *HermesManager) Stop() error {
	if out, err := runCmd(hermesBinary(), "gateway", "stop"); err != nil {
		_ = out
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.gatewayCmd != nil && m.gatewayCmd.Process != nil {
		_ = m.gatewayCmd.Process.Kill()
	}
	m.gatewayCmd = nil
	return nil
}

// Restart 停 + 重起 hermes gateway（external-supervisor 模式下不自行 fork 脱离）。
func (m *HermesManager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return m.Start()
}

// PushConfig 写 config.yaml（配置下发）。
// 存 API 下发的原始（model-only）base 到 config.base.yaml，再按 weixin 绑定态生成实际 config.yaml
// （writeEffectiveConfig）——已绑定时叠加 weixin open 平台块，保证配置重推不覆盖掉绑定后的微信声明。
func (m *HermesManager) PushConfig(content string) error {
	if content == "" {
		return fmt.Errorf("配置内容为空")
	}
	if err := os.MkdirAll(filepath.Dir(hermesConfigPath()), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(hermesBaseConfigPath(), []byte(content), 0600); err != nil {
		return err
	}
	return writeEffectiveConfig()
}

// PushSoul 幂等追加平台铁律到 $HERMES_HOME/SOUL.md（executor 对 soul_push 类型断言探测）。
// 实例 SOUL.md 已由镜像 stage2-hook 种入上游人格；此处只追加不覆盖：内容首行（铁律标题）已存在
// 即跳过（重发不重复追加），否则空一行接在既有内容后（保住人格与用户手改）。实现 SoulWriter。
func (m *HermesManager) PushSoul(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("SOUL 内容为空")
	}
	marker := content // 单行内容整行即标记；多行取首行（铁律标题）
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		marker = content[:i]
	}
	path := filepath.Join(hermesHome(), "SOUL.md")
	existing, _ := os.ReadFile(path)
	if strings.Contains(string(existing), marker) {
		return nil // 铁律已在（幂等）
	}
	out := content + "\n"
	if base := strings.TrimRight(string(existing), "\n"); base != "" {
		out = base + "\n\n" + out
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// ConfigExists config.yaml 是否存在且非空（启动期判定是否立即拉起）。
func (m *HermesManager) ConfigExists() bool {
	info, err := os.Stat(hermesConfigPath())
	return err == nil && info.Size() > 0
}

// Status 采集 hermes 模块状态（供心跳上报）。
func (m *HermesManager) Status() ModuleReport {
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
		} else if out, err := runCmd("pgrep", "-f", "hermes gateway"); err == nil {
			r.PidOrContainerId = strings.TrimSpace(out)
		}
	}
	// 只取首行：hermes --version 输出多行（版本/安装目录/Python/SDK），整段塞进
	// 服务端 agent_proc_version varchar(100) 会 SQLSTATE 22001 → 心跳 Upsert 失败 →
	// 响应 Fail 无 supportAgents → sync 误移除 hermes → 移除/拉起死循环（踩过）。
	if ver, err := runCmd(hermesBinary(), "--version"); err == nil {
		if i := strings.IndexByte(ver, '\n'); i >= 0 {
			ver = ver[:i]
		}
		r.Version = strings.TrimSpace(ver)
	}
	if content, err := os.ReadFile(hermesConfigPath()); err == nil {
		r.ConfigSize = int64(len(content))
		r.ConfigMtime = fileMtime(hermesConfigPath())
		sum := sha256.Sum256(content)
		r.ConfigHash = hex.EncodeToString(sum[:])
	}
	// 技能清单随心跳上行（漂移巡检数据源）：扫一遍 skills 目录的 stat，代价可忽略。
	r.Skills = listHermesSkills()
	return r
}

// listHermesSkills 列 $HERMES_HOME/skills 下的技能目录（相对路径，含 SKILL.md 判定）。
// 两层结构：顶层目录直接是技能（skills/<slug>/SKILL.md）；顶层若是命名空间目录
// （如 @user_xxx，自身无 SKILL.md）则其每个子目录是一个技能（skills/@user_xxx/<slug>/）。
// 隐藏项（.curator_ledger.jsonl 等）忽略。结果排序保证心跳间可比（服务端按集合 diff）。
func listHermesSkills() []string {
	root := hermesSkillsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		top := filepath.Join(root, e.Name())
		if skillMdExists(filepath.Join(top, "SKILL.md")) {
			names = append(names, e.Name())
			continue
		}
		// 命名空间目录：枚举其下的技能子目录
		subs, err := os.ReadDir(top)
		if err != nil {
			continue
		}
		for _, se := range subs {
			if se.IsDir() && !strings.HasPrefix(se.Name(), ".") &&
				skillMdExists(filepath.Join(top, se.Name(), "SKILL.md")) {
				names = append(names, e.Name()+"/"+se.Name())
			}
		}
	}
	sort.Strings(names)
	return names
}

// skillMdExists SKILL.md 是否存在（命名空间判定的轻量探测）。
func skillMdExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ===== hermesHome 等路径辅助（skill 安装 / 渠道登录复用） =====

// hermesHome $HERMES_HOME（默认 /aa/.hermes，随 PVC 持久化）。
func hermesHome() string {
	if h := os.Getenv("HERMES_HOME"); h != "" {
		return h
	}
	return "/aa/.hermes"
}

// hermesSkillsDir skill 托管目录：$HERMES_HOME/skills。hermes 启动/每轮扫描此目录加载 skill
// （_find_all_skills 递归找 SKILL.md），故安装=放目录、卸载=删目录，无需 hermes skills install
// （其只收 registry id / URL，不收本地目录）。
func hermesSkillsDir() string { return filepath.Join(hermesHome(), "skills") }

// hermesEnvFile $HERMES_HOME/.env：微信凭据（WEIXIN_ACCOUNT_ID/WEIXIN_TOKEN/...）落这里，
// gateway 启动经 get_secret 解析 + load_weixin_account 加载。
func hermesEnvFile() string { return filepath.Join(hermesHome(), ".env") }

// hermesWeixinAccountsDir $HERMES_HOME/weixin/accounts：qr_login 的 save_weixin_account 落账号 json 处。
func hermesWeixinAccountsDir() string { return filepath.Join(hermesHome(), "weixin", "accounts") }

// venvPython hermes venv 的 python：与 hermes 二进制同目录（/aa/hermes/.venv/bin/python）。
func venvPython() string { return filepath.Join(filepath.Dir(hermesBinary()), "python") }

// hermesSourceDir hermes 源码根（含 gateway/ 包，供 shim import）。
// venv 在 <src>/.venv/bin/hermes → 上溯三级得 <src>；不含 gateway/ 时回退 /aa/hermes。
func hermesSourceDir() string {
	d := filepath.Dir(filepath.Dir(filepath.Dir(hermesBinary())))
	if _, err := os.Stat(filepath.Join(d, "gateway")); err == nil {
		return d
	}
	return "/aa/hermes"
}

// ===== SkillInstaller（install_skill / uninstall_skill 命令） =====

// InstallSkill 把已解压 skill 目录复制到 hermes 托管 skills 目录（hermes 自动扫描发现）。
// 与 openclaw 的 `skills install --as --global` 不同：hermes CLI 不收本地目录，但运行时扫描
// $HERMES_HOME/skills/<slug>/SKILL.md 加载，故直接放目录即等效安装。幂等替换：已存在先挪走
// 为备份再拷，拷贝失败挪回——旧版本原样保留（与 openclaw 侧重装语义一致）。
// 实现 SkillInstaller（executor 对 install_skill/uninstall_skill 类型断言探测）。
func (m *HermesManager) InstallSkill(dir, slug string, pb *progressBuffer) (string, error) {
	if pb != nil {
		pb.Write(fmt.Sprintf("hermes skill 安装：复制 %s → skills/%s", dir, slug))
	}
	dst := filepath.Join(hermesSkillsDir(), slug)
	backup := "" // 旧版本备份（.aa-installing 后缀）：成功清掉、失败挪回
	if _, err := os.Stat(dst); err == nil {
		bak := dst + ".aa-installing"
		_ = os.RemoveAll(bak) // 清上次异常残留
		if err := os.Rename(dst, bak); err == nil {
			backup = bak
		}
	}
	restore := func() {
		if backup != "" {
			_ = os.Rename(backup, dst)
		}
	}
	if err := os.MkdirAll(hermesSkillsDir(), 0o755); err != nil {
		restore()
		return "", fmt.Errorf("建 skills 目录失败: %w", err)
	}
	if err := copyDir(dir, dst); err != nil {
		restore()
		return "", fmt.Errorf("复制 skill 目录失败: %w", err)
	}
	if backup != "" {
		_ = os.RemoveAll(backup) // 新版就位，清备份
	}
	if pb != nil {
		pb.Write(fmt.Sprintf("已安装到 %s（hermes 自动扫描发现，无需重启）", dst))
	}
	return fmt.Sprintf(`{"action":"install_skill","slug":%q,"dest":%q}`, slug, dst), nil
}

// UninstallSkill 删除托管 skills 目录下该 slug 目录。hermes 下轮扫描即不再加载。
// slug 不存在（已删）→ 幂等成功。实现 SkillInstaller。
func (m *HermesManager) UninstallSkill(slug string, pb *progressBuffer) (string, error) {
	dst := filepath.Join(hermesSkillsDir(), slug)
	if _, err := os.Stat(dst); err != nil {
		return fmt.Sprintf(`{"action":"uninstall_skill","slug":%q,"alreadyGone":true}`, slug), nil
	}
	if err := os.RemoveAll(dst); err != nil {
		return "", fmt.Errorf("删除 skill 目录失败: %w", err)
	}
	if pb != nil {
		pb.Write(fmt.Sprintf("已删除 %s", dst))
	}
	return fmt.Sprintf(`{"action":"uninstall_skill","slug":%q,"removed":%q}`, slug, dst), nil
}

// copyDir 递归复制目录树（文件逐个 io.Copy，保留可执行/普通文件权限位）。
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst, info.Mode())
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyDir(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyFile 复制单个文件（按 mode 创建）。
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// ===== 渠道登录 / 状态 / 解绑（channel_login / channel_status / channel_logout） =====
//
// hermes 无非交互微信登录 CLI（仅交互式 `gateway setup` 向导）。这里走 Python shim 调 hermes
// 内部库函数 gateway.platforms.weixin.qr_login：它把二维码 URL 打印到 stdout + 自持久化账号文件。
// manager 仿 openclaw 的 channelLoginWeixinCLI：阻塞跑 shim、逐行抓 stdout 的 QR URL 写 pb
// （→ 心跳 progress_json → UI 渲染二维码），完成哨兵 AA_WEIXIN_LOGIN_OK 判成功，成功后 shim 已把
// 凭据写入 .env，gateway 重启即加载微信。
// 前置：镜像须装 messaging extra（aiohttp/qrcode），否则 qr_login import/运行即 raise。

// ChannelLogin 渠道扫码登录。hermes 仅支持微信（weixin）；其它渠道不支持。
// 实现 ChannelLoginer（executor 对 channel_login 类型断言探测）。
func (m *HermesManager) ChannelLogin(channel string, pb *progressBuffer) (string, error) {
	if !strings.Contains(channel, "weixin") {
		return "", fmt.Errorf("hermes 仅支持 weixin 渠道登录，收到: %s", channel)
	}
	out, err := m.channelLoginWeixin(pb)
	if err != nil {
		return out, err
	}
	// 放行所有发件人（对齐 openclaw，免配对码）：.env 加 GATEWAY_ALLOW_ALL_USERS=true，
	// config.yaml 叠加 weixin open 平台块（writeEffectiveConfig 按 .env 绑定态生成）。
	// hermes 适配器仅启动期实例化，须 restart 才加载 weixin。
	ensureEnvKey(hermesEnvFile(), "GATEWAY_ALLOW_ALL_USERS", "true")
	if e := writeEffectiveConfig(); e != nil && pb != nil {
		pb.Write(fmt.Sprintf("写有效配置失败: %v", e))
	}
	if pb != nil {
		pb.Write("重启 gateway 以加载微信（放行所有发件人）…")
	}
	_ = m.Restart()
	return out, nil
}

// ChannelStatus 查微信登录态：读 .env 是否有 WEIXIN_TOKEN（=已绑定）。
// 产出与 openclaw 同 schema 的结构化结果（UI 通用，不外泄 hermes 细节）。实现 ChannelStatuser。
func (m *HermesManager) ChannelStatus(channel string) (string, error) {
	env := readDotEnv(hermesEnvFile())
	accountID := env["WEIXIN_ACCOUNT_ID"]
	token := env["WEIXIN_TOKEN"]
	bound := token != ""
	res, _ := json.Marshal(map[string]any{
		"channel":   "weixin",
		"bound":     bound,
		"accountId": accountID,
		"alias":     accountID, // hermes 无昵称字段，用 accountId 作展示
		"connected": bound,
		"lastError": "",
	})
	return string(res), nil
}

// ChannelLogout 解绑微信：从 .env 删 WEIXIN_* + GATEWAY_ALLOW_ALL_USERS、删账号 json（qr_login 持久化文件），
// 重生成 config.yaml（无 WEIXIN_TOKEN → 不含 weixin 平台块），再重启 gateway 丢弃 weixin。实现 ChannelLogouter。
func (m *HermesManager) ChannelLogout(channel string) (string, error) {
	removeEnvKeys(hermesEnvFile(), func(k string) bool {
		return strings.HasPrefix(k, "WEIXIN_") || k == "GATEWAY_ALLOW_ALL_USERS"
	})
	if d := hermesWeixinAccountsDir(); d != "" {
		if entries, e := os.ReadDir(d); e == nil {
			for _, en := range entries {
				if !en.IsDir() && strings.HasSuffix(en.Name(), ".json") {
					_ = os.RemoveAll(filepath.Join(d, en.Name()))
				}
			}
		}
	}
	_ = writeEffectiveConfig()
	_ = m.Restart()
	return `{"action":"channel_logout","channel":"weixin","removed":true}`, nil
}

// ChannelNotify 把「给人看的」通知文本经 hermes 微信渠道发到与人的对话会话——hermes 数字人
// 只是信使，消息的收件人是人。`hermes send --to weixin` 读 .env 凭据直发，无需 gateway 存活。
// 未绑微信报错跳过（api 侧站内通知已先行落库，不受影响）。实现 ChannelNotifier。
// link 只拼 http(s) 绝对地址：api 现传的是站内相对路径（如 /agent/instance/list），在微信里
// 不可点也不是用户能懂的语言——拼出去就是一行英文噪音（技能装卸通知 6 连发踩过）。
func (m *HermesManager) ChannelNotify(title, content, link string, pb *progressBuffer) (string, error) {
	writef := func(s string) {
		if pb != nil {
			pb.Write(s)
		}
	}
	if !weixinBound() {
		return "", fmt.Errorf("hermes 未绑定微信（.env 无 WEIXIN_TOKEN），跳过渠道推送")
	}
	text := channelNotifyText(title, content, link)
	writef("hermes send --to weixin 推送通知…")
	out, err := runCmdQuick(90*time.Second, hermesBinary(), "send", "--to", "weixin", "--", text)
	if err != nil && strings.Contains(out, "No home channel set") {
		// home channel 未配置的回落：DM id 要等主人首次给数字人发消息后才存在（登录时拿不到），
		// 平台侧也无法预知，故不在绑定/登录时配。若微信渠道恰好只有一个 dm（数字人只与主人
		// 聊的常态），取它直发；多会话无法判定收件人不猜——保持报错，由人显式指定。
		if dm := weixinSoleDM(); dm != "" {
			writef("home channel 未配置，唯一 dm 会话回落直发…")
			out, err = runCmdQuick(90*time.Second, hermesBinary(), "send", "--to", "weixin:"+dm, "--", text)
		}
	}
	if err != nil {
		return out, fmt.Errorf("hermes send 失败: %w", err)
	}
	res, _ := json.Marshal(map[string]any{
		"action":    "channel_notify",
		"channel":   "weixin",
		"delivered": true,
		"output":    strings.TrimSpace(out),
	})
	return string(res), nil
}

// channelNotifyText 组装渠道通知正文：title + content，link 仅 http(s) 绝对地址才拼
// （相对路径在微信里不可点也读不懂，纯噪音）。
func channelNotifyText(title, content, link string) string {
	text := strings.TrimSpace(title + "\n" + content)
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		text += "\n" + link
	}
	return text
}

// weixinSoleDM 查微信渠道的 dm 会话目标（hermes send --list）：恰好一个才返回其 id，
// 零个或多个（无法判定收件人）返回空。供 ChannelNotify 的 home channel 回落用。
func weixinSoleDM() string {
	out, err := runCmdQuick(30*time.Second, hermesBinary(), "send", "--list", "weixin", "--json")
	if err != nil {
		return ""
	}
	var res struct {
		Platforms struct {
			Weixin []struct {
				Id   string `json:"id"`
				Type string `json:"type"`
			} `json:"weixin"`
		} `json:"platforms"`
	}
	if json.Unmarshal([]byte(out), &res) != nil {
		return ""
	}
	var dms []string
	for _, t := range res.Platforms.Weixin {
		if t.Type == "dm" && t.Id != "" {
			dms = append(dms, t.Id)
		}
	}
	if len(dms) == 1 {
		return dms[0]
	}
	return ""
}

// channelLoginWeixin 阻塞跑 Python shim 完成 QR 登录，抓 stdout 的二维码 URL 写 pb。
func (m *HermesManager) channelLoginWeixin(pb *progressBuffer) (string, error) {
	writef := func(s string) {
		if pb != nil {
			pb.Write(s)
		}
	}
	shim, err := ensureWeixinLoginShim()
	if err != nil {
		return "", fmt.Errorf("写微信登录 shim 失败: %w", err)
	}
	py := venvPython()
	src := hermesSourceDir()
	writef(fmt.Sprintf("发起微信扫码登录：python %s", shim))

	// 阻塞跑 shim，限时 270s（命令总限时 300s，留 30s 余量收尾，防 goroutine 泄漏）。
	ctx, cancel := context.WithTimeout(context.Background(), 270*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, py, shim)
	cmd.Dir = src
	cmd.Env = append(os.Environ(), "PYTHONPATH="+src) // 保 gateway 包可 import（即便 editable install 异常）
	cmd.Stderr = os.Stderr                            // 诊断信息进容器日志
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("打开 shim stdout 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动 shim 失败: %w", err)
	}
	log.Printf("[agent] hermes channel_login shim 已启动 pid=%d", cmd.Process.Pid)

	ok := false
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		switch {
		case line == "AA_WEIXIN_LOGIN_OK":
			ok = true
			log.Printf("[agent] hermes channel_login 成功（凭据已写 .env）")
		case strings.HasPrefix(line, "AA_WEIXIN_LOGIN_FAILED"):
			log.Printf("[agent] hermes channel_login shim 报失败: %s", line)
		case isQrUrl(line):
			// QR URL 写进 pb，且须是当前最后写入行：心跳取 pb.LastLine 上报 progress_json.step，
			// UI 据此渲染二维码。ASCII 二维码艺术行/`.` 等待行不写，避免覆盖 LastLine。
			writef(line)
		default:
			log.Printf("[agent] hermes channel_login stdout: %s", line)
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("扫码超时（270s 内未完成）")
	}
	if err := cmd.Wait(); err != nil && !ok {
		return "", fmt.Errorf("微信登录 shim 异常退出: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("微信登录未完成（未收到成功哨兵）")
	}
	return `{"action":"channel_login","channel":"weixin","status":"connected"}`, nil
}

// isQrUrl 判断一行是否为二维码内容 URL（qr_login 打印的 weixin:// 或 http(s):// liteapp 链接）。
func isQrUrl(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "weixin://")
}

// ensureWeixinLoginShim 把 Python shim 写到 $HERMES_HOME/.aa-weixin-login.py（每次覆盖保持最新），返回路径。
func ensureWeixinLoginShim() (string, error) {
	path := filepath.Join(hermesHome(), ".aa-weixin-login.py")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(weixinLoginShimPy), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// weixinLoginShimPy 调 hermes 内部 qr_login：打印二维码 URL 到 stdout（manager 抓取）+ 自持久化账号；
// 成功后把凭据合并写入 .env，打印 AA_WEIXIN_LOGIN_OK 哨兵；失败打印 AA_WEIXIN_LOGIN_FAILED。
const weixinLoginShimPy = `#!/usr/bin/env python3
# aiarsenal 微信扫码登录 shim（由 aiarsenal-manager 调用）。
import asyncio, os


def main():
    try:
        from gateway.platforms.weixin import qr_login
    except Exception as exc:
        print("AA_WEIXIN_LOGIN_FAILED import: %s" % exc)
        return
    home = os.environ.get("HERMES_HOME", "/aa/.hermes")
    # qr_login 内部：打印二维码 URL（weixin://...）到 stdout + save_weixin_account 落账号 json。
    creds = asyncio.run(qr_login(home))
    if not creds:
        print("AA_WEIXIN_LOGIN_FAILED")
        return
    # 合并写 .env：去旧 WEIXIN_* 行，追加新凭据（gateway 启动经 get_secret 解析）。
    env_path = os.path.join(home, ".env")
    kept = []
    try:
        with open(env_path) as f:
            for ln in f:
                if not ln.startswith("WEIXIN_"):
                    kept.append(ln.rstrip("\n"))
    except FileNotFoundError:
        pass
    rows = {
        "WEIXIN_ACCOUNT_ID": creds.get("account_id", ""),
        "WEIXIN_TOKEN": creds.get("token", ""),
        "WEIXIN_BASE_URL": creds.get("base_url", ""),
    }
    out = list(kept)
    for k, v in rows.items():
        if v:
            out.append("%s=%s" % (k, v))
    with open(env_path, "w") as f:
        f.write("\n".join(out) + "\n")
    try:
        os.chmod(env_path, 0o600)
    except OSError:
        pass
    print("AA_WEIXIN_LOGIN_OK")


main()
`

// readDotEnv 解析 .env 为 map（KEY=VALUE，去引号/空白；忽略空行与 # 注释）。文件缺失返回空 map。
func readDotEnv(path string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		out[k] = v
	}
	return out
}

// ensureEnvKey 确保 .env 含 key=val（已存在且值相同则不动；否则合并写入，保留其它行/注释）。
func ensureEnvKey(path, key, val string) {
	data, _ := os.ReadFile(path)
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		eq := strings.IndexByte(trimmed, '=')
		if eq > 0 && strings.TrimSpace(trimmed[:eq]) == key {
			if strings.TrimSpace(trimmed[eq+1:]) == val {
				return // 已是目标值
			}
			lines[i] = key + "=" + val
			_ = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
			return
		}
	}
	out := strings.TrimRight(string(data), "\n")
	if out != "" {
		out += "\n"
	}
	out += key + "=" + val + "\n"
	_ = os.WriteFile(path, []byte(out), 0600)
}

// removeEnvKeys 重写 .env，丢弃 key 命中 drop 的行（保留空行/注释/其它键）。文件缺失无操作。
func removeEnvKeys(path string, drop func(key string) bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			kept = append(kept, line)
			continue
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq < 0 {
			kept = append(kept, line)
			continue
		}
		if drop(strings.TrimSpace(trimmed[:eq])) {
			continue
		}
		kept = append(kept, line)
	}
	_ = os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o600)
}
