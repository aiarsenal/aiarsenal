package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Executor 按 commandType 分派到对应 agent 的 runtime 管理。返回 (status, result, errMsg)。
// status: 3=done 4=failed 5=timeout
type Executor struct {
	ms  *managerSet
	api *API // install_skill 用 instance token 拉取 skill 整包 zip
}

func NewExecutor(ms *managerSet, api *API) *Executor { return &Executor{ms: ms, api: api} }

// Exec 执行单条命令（作用于 agentId 指定的 agent）。payload 是 PayloadJson（JSON 字符串）。
// pb 记录执行过程（关键步骤 + 外部命令输出）：执行中由心跳上报最新一行（进度可见），完成时全文并入 result。
func (e *Executor) Exec(agentId, cmdType, payload string, pb *progressBuffer) (int, string, string) {
	m := e.ms.get(agentId)
	if m == nil {
		return 4, "", fmt.Sprintf("agent %s 不在 supportAgents", agentId)
	}
	p := parsePayload(payload)
	// writef 往 pb 追加一行过程（pb 可能为 nil，Write 内部兜底；格式化便于带实体信息）。
	writef := func(format string, args ...interface{}) { pb.Write(fmt.Sprintf(format, args...)) }
	switch strings.ToLower(cmdType) {
	case "start":
		writef("拉起 %s", m.Name())
		if err := m.Start(); err != nil {
			writef("启动失败: %v", err)
			return 4, "", err.Error()
		}
		writef("%s 已启动", m.Name())
		return 3, `{"action":"start","status":"running"}`, ""
	case "stop":
		writef("停止 %s", m.Name())
		if err := m.Stop(); err != nil {
			writef("停止失败: %v", err)
			return 4, "", err.Error()
		}
		writef("%s 已停止", m.Name())
		return 3, `{"action":"stop","status":"stopped"}`, ""
	case "restart":
		writef("重启 %s", m.Name())
		if err := m.Restart(); err != nil {
			writef("重启失败: %v", err)
			return 4, "", err.Error()
		}
		writef("%s 已重启", m.Name())
		return 3, `{"action":"restart","status":"running"}`, ""
	case "config_push":
		content, _ := p["configContent"].(string)
		writef("写入配置 %d bytes → %s", len(content), m.Name())
		if err := m.PushConfig(content); err != nil {
			writef("写配置失败: %v", err)
			return 4, "", err.Error()
		}
		writef("配置已写盘")
		// 先配置后启动：写盘后确保 agent 在跑。Start 内部按 gatewayCmd 状态决定 spawn/跳过——
		// 首次（未拉起）spawn 带新配置启动；已运行则跳过（由后续 restart 命令重读新配置）。
		// 不再用 IsRunning() 前置判定：它的 pgrep 在首次 config_push 存在假阳性，会漏掉拉起（bug1）。
		if err := m.Start(); err != nil {
			writef("拉起失败: %v", err)
			return 4, "", err.Error()
		}
		writef("%s 已确保运行（首次拉起或已在跑）", m.Name())
		return 3, `{"action":"config_push","applied":true}`, ""
	case "soul_push":
		// 平台 SOUL 铁律写入（仅实现 SoulWriter 的 runtime，如 hermes）。幂等：已含铁律标记则跳过。
		sw, ok := m.(SoulWriter)
		if !ok {
			return 4, "", fmt.Sprintf("soul_push not supported on runtime: %s", m.Name())
		}
		content, _ := p["soulContent"].(string)
		writef("写入 SOUL 铁律 %d bytes → %s", len(content), m.Name())
		if err := sw.PushSoul(content); err != nil {
			writef("写 SOUL 失败: %v", err)
			return 4, "", err.Error()
		}
		writef("SOUL 铁律已写入（幂等追加，不覆盖既有内容）")
		return 3, `{"action":"soul_push","applied":true}`, ""
	case "channel_login":
		// 仅支持实现了 ChannelLoginer 的 runtime（如 openclaw）；hermes 等不支持。
		cl, ok := m.(ChannelLoginer)
		if !ok {
			return 4, "", fmt.Sprintf("channel_login not supported on runtime: %s", m.Name())
		}
		channel, _ := p["channel"].(string)
		writef("发起渠道登录 channel=%s", channel)
		// pb 透传：微信 CLI 路径把抓到的 QR URL 边写边报（→ progress_json），RPC 路径写若干步骤。
		out, err := cl.ChannelLogin(channel, pb)
		if err != nil {
			// out 含 openclaw CLI 的合并输出（stderr 诊断信息），一并落进执行过程便于排查。
			writef("渠道登录失败: %s", strings.TrimSpace(out))
			return 4, "", err.Error()
		}
		writef("渠道登录完成")
		return 3, out, ""
	case "channel_status":
		// 查渠道登录态（仅支持实现了 ChannelStatuser 的 runtime，如 openclaw）。
		cs, ok := m.(ChannelStatuser)
		if !ok {
			return 4, "", fmt.Sprintf("channel_status not supported on runtime: %s", m.Name())
		}
		channel, _ := p["channel"].(string)
		writef("查询渠道状态 channel=%s", channel)
		out, err := cs.ChannelStatus(channel)
		if err != nil {
			writef("查询渠道状态失败: %s", strings.TrimSpace(out))
			return 4, "", err.Error()
		}
		return 3, out, ""
	case "channel_logout":
		// 解绑渠道（清账号 token；仅支持实现了 ChannelLogouter 的 runtime）。
		cl, ok := m.(ChannelLogouter)
		if !ok {
			return 4, "", fmt.Sprintf("channel_logout not supported on runtime: %s", m.Name())
		}
		channel, _ := p["channel"].(string)
		writef("解绑渠道 channel=%s", channel)
		out, err := cl.ChannelLogout(channel)
		if err != nil {
			writef("解绑渠道失败: %s", strings.TrimSpace(out))
			return 4, "", err.Error()
		}
		return 3, out, ""
	case "channel_notify":
		// 渠道推送通知：把人读的文本经数字人渠道（微信）发给人（仅实现了 ChannelNotifier 的 runtime，如 hermes）。
		// 平台侧站内通知已先行落库，这里失败只影响渠道触达。
		cn, ok := m.(ChannelNotifier)
		if !ok {
			return 4, "", fmt.Sprintf("channel_notify not supported on runtime: %s", m.Name())
		}
		title, _ := p["title"].(string)
		content, _ := p["content"].(string)
		link, _ := p["link"].(string)
		writef("渠道推送通知 title=%s", title)
		out, err := cn.ChannelNotify(title, content, link, pb)
		if err != nil {
			writef("渠道推送失败: %s", strings.TrimSpace(out))
			return 4, "", err.Error()
		}
		writef("渠道推送完成")
		return 3, out, ""
	case "collect_data":
		// 上报当前状态快照
		writef("采集 %s 状态快照", m.Name())
		st := m.Status()
		b, _ := json.Marshal(st)
		return 3, string(b), ""
	case "install_skill":
		// 仅支持实现了 SkillInstaller 的 runtime（openclaw）。
		si, ok := m.(SkillInstaller)
		if !ok {
			return 4, "", fmt.Sprintf("install_skill not supported on runtime: %s", m.Name())
		}
		url, _ := p["url"].(string)
		slug, _ := p["slug"].(string)
		if url == "" || slug == "" {
			return 4, "", "install_skill 缺少 url/slug"
		}
		// 下载 + 解压（manager 自行解压，因 openclaw skills install 只收本地目录）。
		skillDir, cleanup, err := downloadAndExtract(e.api, url, pb)
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			return 4, "", err.Error()
		}
		writef("安装 skill slug=%s", slug)
		out, err := si.InstallSkill(skillDir, slug, pb)
		if err != nil {
			writef("安装失败: %s", strings.TrimSpace(out))
			return 4, "", err.Error()
		}
		writef("安装完成 slug=%s", slug)
		return 3, out, ""
	case "uninstall_skill":
		si, ok := m.(SkillInstaller)
		if !ok {
			return 4, "", fmt.Sprintf("uninstall_skill not supported on runtime: %s", m.Name())
		}
		slug, _ := p["slug"].(string)
		if slug == "" {
			return 4, "", "uninstall_skill 缺少 slug"
		}
		writef("卸载 skill slug=%s", slug)
		out, err := si.UninstallSkill(slug, pb)
		if err != nil {
			writef("卸载失败: %s", strings.TrimSpace(out))
			return 4, "", err.Error()
		}
		writef("卸载完成 slug=%s", slug)
		return 3, out, ""
	default:
		return 4, "", fmt.Sprintf("未知命令类型: %s", cmdType)
	}
}

func parsePayload(s string) map[string]interface{} {
	m := map[string]interface{}{}
	if s == "" {
		return m
	}
	_ = json.Unmarshal([]byte(s), &m)
	return m
}
