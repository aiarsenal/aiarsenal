package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// AgentRuntimeService Phase3 控制面：容器内 agent 用 desktop 的「心跳上报 + 认领命令」模式。
// agent 跑在容器里，定时 POST /agent/runtime/heartbeat 上报状态、GET claim 认领命令、POST result 回传。
// 管理员经 /agent/instance/:id/command/enqueue 下发，落 aa_agent_instance_command（status=0 pending）。
type AgentRuntimeService struct {
	statusRepo        repository.IAgentInstanceRuntimeStatusRepository
	historyRepo       repository.IAgentInstanceRuntimeStatusHistoryRepository
	configChangeRepo  repository.IAgentInstanceConfigChangeRepository
	commandRepo       repository.IAgentInstanceCommandRepository
	instanceRepo      repository.IAgentInstanceRepository
	agentRepo         repository.IAgentInstanceAgentRepository
	userRepo          repository.IUserRepository
	instanceSkillRepo repository.IAgentInstanceSkillRepository // 实例↔skill 安装关联（aa_agent_instance_skill）
	skillRepo         repository.ISkillRepository              // aa_skill 库详情富化 + 安装时取 slug/versionId（跨库软引用）
	skillSecRepo      repository.ISkillSecurityRepository      // 技能安装门禁：版本最新审计须为「安全」
	channelNotice     *ChannelNoticeService                    // 技能漂移巡检双通道通知（构造依赖成环，装配后 SetChannelNotice 注入）
	instanceSvc       *AgentInstanceService                    // hermes mcp_servers 段漂移自愈（ops 进程不装配，nil 安全）
}

func NewAgentRuntimeService(
	sr repository.IAgentInstanceRuntimeStatusRepository,
	hr repository.IAgentInstanceRuntimeStatusHistoryRepository,
	ccr repository.IAgentInstanceConfigChangeRepository,
	cr repository.IAgentInstanceCommandRepository,
	ir repository.IAgentInstanceRepository,
	ar repository.IAgentInstanceAgentRepository,
	ur repository.IUserRepository,
	isr repository.IAgentInstanceSkillRepository,
	skr repository.ISkillRepository,
	ssr repository.ISkillSecurityRepository,
) *AgentRuntimeService {
	return &AgentRuntimeService{statusRepo: sr, historyRepo: hr, configChangeRepo: ccr, commandRepo: cr, instanceRepo: ir, agentRepo: ar, userRepo: ur, instanceSkillRepo: isr, skillRepo: skr, skillSecRepo: ssr}
}

// SetChannelNotice 注入技能漂移巡检的通知出口。ChannelNoticeService 构造依赖本服务
// （notice→runtime 下发 channel_notify），不能进构造参数，装配处事后注入；nil 安全（只记日志）。
func (s *AgentRuntimeService) SetChannelNotice(c *ChannelNoticeService) { s.channelNotice = c }

// SetInstanceService 注入实例服务供 hermes mcp_servers 段漂移自愈复用（SyncHermesMcpSection）。
// 装配顺序本可进构造参数，但 ops 进程不构造实例服务——沿用 SetChannelNotice 的事后注入惯例，nil 安全。
func (s *AgentRuntimeService) SetInstanceService(i *AgentInstanceService) { s.instanceSvc = i }

// Heartbeat 容器内 agent 上行：聚合 modules → upsert runtime_status（最新单行）+ 追加历史时间线 +
// 配置变更 + 命令结果合并 + 超时清理；下行返回认领命令 + supportAgents。合并对齐 desktop_service.Heartbeat。
func (s *AgentRuntimeService) Heartbeat(ctx context.Context, instanceId string, req map[string]interface{}) (stored bool, commands []*entity.AgentInstanceCommand, supportAgents [][2]string, err error) {
	// 聚合 modules → runtime_status。modules 形如 {"infra":…, "agent":…, "<agent>": {status,version,pid,…}}：
	// 智能体进程模块的 key 就是实例的 agent 名（openclaw/hermes…），按实例记录匹配、不写死具体智能体。
	modules, _ := req["modules"].(map[string]interface{})
	agentKey := "" // 实例的 agent 名（智能体进程模块 key）；查不到实例则不采集智能体模块
	if ins, gerr := s.instanceRepo.Get(ctx, instanceId); gerr == nil && ins != nil {
		agentKey = agentAlias(ins.Agent)
	}
	var (
		infraStatus   uint = 1 // running
		agentStatus   uint = 1
		procStatus    uint = 1
		procPid       string
		procVer       string
		configHash    string
		configSize    int64
		configMtime   *types.Time
		configContent string // 智能体运行配置全文（manager 上报，落 config_change.content）
		runningCmdSig string // 命令签名 action:status（历史时间线区分命令过程）
	)
	summary, _ := json.Marshal(modules)
	if len(summary) == 0 || string(summary) == "null" {
		summary = []byte("{}")
	}
	for key, raw := range modules {
		m, _ := raw.(map[string]interface{})
		st, _ := m["status"].(string)
		stat := mapStatus(st)
		switch key {
		case "infra":
			infraStatus = stat
		case "agent":
			agentStatus = stat
		case agentKey: // 智能体进程模块（key=实例 agent 名）：状态/pid/版本/配置指纹都在这
			procStatus = stat
			procPid, _ = m["pidOrContainerId"].(string)
			procVer = truncateRunes(m["version"], 100) // agent_proc_version varchar(100)，多行版本串在此兜底截断
			configHash, _ = m["configHash"].(string)
			configContent, _ = m["configContent"].(string)
			if sz, ok := m["configSize"].(float64); ok {
				configSize = int64(sz)
			}
			if ms, ok := m["configMtime"].(string); ok && ms != "" {
				if t, e := time.ParseInLocation("2006-01-02 15:04:05", ms, time.Local); e == nil {
					tt := types.Time(t)
					configMtime = &tt
				}
			}
			// 命令签名（智能体模块挂 runningCommand）。
			if rc, ok := m["runningCommand"].(map[string]interface{}); ok {
				action, _ := rc["action"].(string)
				rcs, _ := rc["status"].(string)
				if action != "" {
					runningCmdSig = action + ":" + rcs
				}
			}
		}
		// 合并执行中命令的结果（任何模块的 runningCommand：done/failed → UpdateResult）。
		if rc, ok := m["runningCommand"].(map[string]interface{}); ok {
			s.mergeCommandResult(ctx, rc)
		}
	}
	// 技能漂移巡检：manager 随心跳上行技能目录清单（hermes 起），对照平台注册集与上一次清单，
	// 出现「未注册且新增」的技能（CLI 直装 / curator 自建）→ 双通道通知实例归属人。
	// 放在命令结果合并之后：同一心跳里 install_skill 的 done 回传已写注册行，不会误报刚装的技能。
	if agentKey != "" {
		if cur, ok := skillsFromModules(modules, agentKey); ok {
			s.checkSkillDrift(ctx, instanceId, agentKey, cur)
		}
	}
	// hermes mcp_servers 段漂移自愈：最后下发的 config 里没有 mcp 段而实例已绑 MCP 分组
	// （model-edit/model-offline 等重推路径漏带会整份重建配置冲掉 mcp-sync 推的段——9/6 踩过：
	// gateway restart 后 65 个平台工具无声消失，agent 拿不到技能市场/项目/通知工具只能满容器乱探）。
	// 心跳发现即收敛重推（复用最近 config_push 的 LLM 键，restart 生效）。
	if agentKey == "hermes" {
		s.healHermesMcpDrift(ctx, instanceId)
	}
	slog.Debug("心跳聚合", "instance", instanceId, "infra", infraStatus, "manager", agentStatus, "agentProc", procStatus, "version", procVer, "configHash", configHash, "cmdSig", runningCmdSig)
	// health_json 存 configHash（jsonb 要求合法 JSON，空时用 {}）。
	health, _ := json.Marshal(map[string]string{"configHash": configHash})
	now := time.Now()
	nowT := types.Time(now)
	e := &entity.AgentInstanceRuntimeStatus{
		InstanceId:       instanceId,
		InfraStatus:      infraStatus,
		AgentStatus:      agentStatus,
		AgentProcStatus:  procStatus,
		AgentProcPid:     procPid,
		AgentProcVersion: procVer,
		SummaryJson:      string(summary),
		HealthJson:       string(health),
		LastReportedAt:   &nowT,
		UpdatedAt:        &nowT,
	}
	// 状态落库失败不 Fail 整个心跳：supportAgents 回显与命令认领是心跳协议主体，
	// 存储抖动（如版本串超列宽）若冒泡成 Fail 响应，manager 会按空 supportAgents
	// 误移除被管 agent → 移除/拉起死循环（hermes 多行 --version 超 varchar(100) 踩过）。
	if err = s.statusRepo.UpsertFromHeartbeat(ctx, e); err != nil {
		slog.Error("心跳状态落库失败（降级继续，不阻断回显/命令下发）", "instance", instanceId, "err", err)
	}
	stored = true
	// 状态历史追加（时间线去重；有命令时强制插新行，保留命令 start/过程/end 各自留痕）。
	hist := &entity.AgentInstanceRuntimeStatusHistory{
		InstanceId:        instanceId,
		InfraStatus:       infraStatus,
		AgentStatus:       agentStatus,
		AgentProcStatus:   procStatus,
		AgentProcPid:      procPid,
		AgentProcVersion:  procVer,
		ConfigHash:        configHash,
		RunningCommandSig: runningCmdSig,
		SummaryJson:       string(summary),
		ReportedAt:        &nowT,
	}
	slog.Debug("状态历史追加", "instance", instanceId, "forceInsert", runningCmdSig != "", "cmdSig", runningCmdSig)
	_ = s.historyRepo.Append(ctx, hist, runningCmdSig != "")
	// 配置变更：据智能体上报 configHash 判定是否落一条 runtime 记录（source=runtime，含 content 全文）。
	// dispatch 记录与 applied 记录 hash 相同（都是下发配置的 hash），不能用「hash 是否变了」做唯一去重，
	// 否则 dispatch 落库后 latest 已是该 hash，agent 应用后上报同 hash 会被当成「无变化」漏记 applied。
	// 真正的判定交给 resolveRuntimeConfigSource：reported==下发 hash 且尚未确认 → applied（首次确认生效）；
	// 已确认 applied 后偏离 → reverted；未确认生效前的非下发 hash → 跳过（避 restart 过渡期旧 hash 假 reverted）。
	if configHash != "" {
		if latest, ce := s.configChangeRepo.GetLatest(ctx, instanceId); ce == nil {
			// 稳态快路径：latest 已 applied 且上报 hash 一致 → 无变化，跳过（省一次 dispatch 查询）。
			if !(latest != nil && latest.Source == "runtime:applied" && latest.ConfigHash == configHash) {
				oldHash := ""
				latestSource := ""
				if latest != nil {
					oldHash = latest.ConfigHash
					latestSource = latest.Source
				}
				d, derr := s.configChangeRepo.GetLatestBySourcePrefix(ctx, instanceId, "dispatch")
				rtSource := resolveRuntimeConfigSource(latest, configHash, d, derr)
				if rtSource == "" {
					slog.Debug("配置变更跳过（下发未确认生效，restart 过渡期旧 hash）", "instance", instanceId, "reportedHash", configHash, "latestHash", oldHash, "latestSource", latestSource)
				} else {
					slog.Info("配置变更", "instance", instanceId, "agent", agentKey, "oldHash", oldHash, "newHash", configHash, "size", configSize, "source", rtSource)
					_ = s.configChangeRepo.Insert(ctx, &entity.AgentInstanceConfigChange{
						InstanceId:  instanceId,
						AgentId:     agentKey,
						Source:      rtSource,
						ConfigHash:  configHash,
						ConfigSize:  configSize,
						ConfigMtime: configMtime,
						Content:     configContent,
					})
				}
			}
		}
	}
	// 超时清理：实例有 running 命令超过 timeout_seconds → 标 failed（孤儿兜底，释放队列）。
	s.cleanupStaleRunning(ctx, instanceId)
	// 下行 supportAgents：aa.json support 非空 → 原样提取（openclaw/hermes pod 自声明）；空=common 待下发。
	supportAgents = supportAgentsFromReq(req["support"])
	// 下行：原子认领下一条 pending（pending→running），随心跳响应下发——单接口合并掉 claim。
	// 前置闸（对齐 desktop_service.Heartbeat 步骤8）：实例仍有 running 命令时不认领——ClaimNext
	// 返回即标 running，而 manager 串行执行、忙时收到下发只静默丢弃，新命令会变「已认领未执行」
	// 的孤儿，只能等超时清理判 failed（2026-09-10 解绑后 channel_status 与 restart 并发下发，被
	// restart 的忙窗口吃掉、30s 误判超时实锤）。上方 mergeCommandResult 已先落上一条终态，
	// done/failed 上报的同一拍这里即可认领下一条，流水线不卡。
	commands = []*entity.AgentInstanceCommand{}
	if running, e := s.commandRepo.ListRunningByInstance(ctx, instanceId); e == nil && len(running) == 0 {
		if cmd, ce := s.commandRepo.ClaimNext(ctx, instanceId); ce == nil && cmd != nil {
			slog.Info("命令认领下发", "instance", instanceId, "cmd", cmd.InstanceCommandId, "type", cmd.CommandType, "agent", cmd.AgentId)
			commands = append(commands, cmd)
		}
	}
	return stored, commands, supportAgents, nil
}

// skillsFromModules 从心跳 modules 里取指定 agent 模块上报的技能清单（manager 端 hermes 起
// 逐跳上报 skills 字段）。第二个返回值=false 表示本次心跳没有技能清单（旧 manager / 非 hermes），
// 调用方跳过巡检。
func skillsFromModules(modules map[string]interface{}, agentKey string) ([]string, bool) {
	m, ok := modules[agentKey].(map[string]interface{})
	if !ok {
		return nil, false
	}
	raw, ok := m["skills"].([]interface{})
	if !ok {
		return nil, false
	}
	names := make([]string, 0, len(raw))
	for _, v := range raw {
		if n, ok := v.(string); ok && n != "" {
			names = append(names, n)
		}
	}
	return names, true
}

// checkSkillDrift 技能漂移巡检：当前清单 vs（上次清单 ∪ 平台注册集），差集非空即有新落地的
// 未注册技能 → Warn 日志 + 双通道通知实例归属人。语义要点：
//   - 首见清单（上次无记录）作基线不告警：镜像自带的技能目录随版本变化，不能在平台侧写死名单；
//   - 注册集含本心跳刚回传 done 的 install_skill（调用点已保证在合并之后），刚装的不会误报；
//   - 只对「新增」告警一次，持续存在不重复刷（上次清单在 runtime_status.summary_json 里天然持久化）；
//   - 删除漂移技能后再装回来会再次告警（重新经历「新增」）——符合预期。
func (s *AgentRuntimeService) checkSkillDrift(ctx context.Context, instanceId, agentKey string, cur []string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("技能漂移巡检 panic（不影响心跳）", "instance", instanceId, "panic", r)
		}
	}()
	curSet := make(map[string]struct{}, len(cur))
	for _, n := range cur {
		curSet[n] = struct{}{}
	}
	// 上次清单：runtime_status.summary_json（心跳原文）里同一 agent 模块的 skills。
	var prev map[string]struct{}
	if st, err := s.statusRepo.GetByInstance(ctx, instanceId); err == nil && st != nil && st.SummaryJson != "" {
		var lastModules map[string]interface{}
		if json.Unmarshal([]byte(st.SummaryJson), &lastModules) == nil {
			if names, ok := skillsFromModules(lastModules, agentKey); ok {
				prev = make(map[string]struct{}, len(names))
				for _, n := range names {
					prev[n] = struct{}{}
				}
			}
		}
	}
	// 稳态短路：清单与上次完全一致 → 无新增，不查注册集（心跳 2s 一跳，省稳态查询）。
	// 注册集只影响「同一心跳里刚装完」的场景（prev 无它但注册行已落），清单有变时才可能命中。
	if prev != nil && len(prev) == len(curSet) {
		same := true
		for n := range curSet {
			if _, ok := prev[n]; !ok {
				same = false
				break
			}
		}
		if same {
			return
		}
	}
	// 平台注册集：aa_agent_instance_skill 关联的技能 slug（装到 skills/<slug>/ 目录）。
	known := make(map[string]struct{}, len(curSet)+8)
	if prev != nil {
		for n := range prev {
			known[n] = struct{}{}
		}
	}
	if rows, _, err := s.instanceSkillRepo.List(ctx, instanceId, 1, 500); err == nil {
		for _, row := range rows {
			if sk, err := s.skillRepo.Get(ctx, row.SkillId); err == nil && sk != nil && sk.Slug != "" {
				known[sk.Slug] = struct{}{}
			}
		}
	}
	var drift []string
	for n := range curSet {
		if _, ok := known[n]; !ok {
			drift = append(drift, n)
		}
	}
	if prev == nil {
		// 首见：基线入库（本次心跳 summary 随后落库即为基线），不告警。
		slog.Info("技能清单基线入库", "instance", instanceId, "agent", agentKey, "skills", len(curSet), "unregistered", len(drift))
		return
	}
	if len(drift) == 0 {
		return
	}
	sort.Strings(drift)
	instName := instanceId
	owner := ""
	if ins, err := s.instanceRepo.Get(ctx, instanceId); err == nil && ins != nil {
		owner = ins.UserId
		if ins.Name != "" {
			instName = ins.Name
		}
	}
	slog.Warn("发现未走平台审核的技能", "instance", instanceId, "agent", agentKey, "skills", drift)
	title := "发现未审核技能"
	content := fmt.Sprintf("实例「%s」的技能目录出现未走平台审核的新技能：%s。若非预期（外部 CLI 直装/agent 自建），请到实例页核对处理；正规安装请走平台技能库（导入→安全扫描→审核→安装）。", instName, strings.Join(drift, "、"))
	s.channelNotice.Push(ctx, owner, title, content, "/agent/monitor/list", "warning", "drift-audit")
}

// mcpHealCooldown 自愈重推冷却窗：愈合推失败（网关不可达/无历史 payload）时按窗重试，
// 避免每心跳重推 config_push+restart 打断会话；推成功则下次心跳配置已带 mcp 段，不再触发。
const mcpHealCooldown = 10 * time.Minute

// mcpHealLast 实例 → 上次自愈时刻（进程内存态；api 单进程写，重启即清零重算，无持久化必要）。
var mcpHealLast sync.Map

// healHermesMcpDrift hermes mcp_servers 段漂移自愈：检测源=最近一次 config_push 的 payload
// （manager 心跳只报 configHash 不报全文，用「最后下发了什么」判断，与 SyncHermesMcpSection
// 的重推源头同源——检测看到的和愈合重建的天然一致）。实例已绑 MCP 分组而 payload 无 mcp 段
// → 调 SyncHermesMcpSection 收敛重推（带 restart 生效）。未绑分组/无段是正常态，不动。
func (s *AgentRuntimeService) healHermesMcpDrift(ctx context.Context, instanceId string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("hermes mcp 段漂移自愈 panic（不影响心跳）", "instance", instanceId, "panic", r)
		}
	}()
	if s.instanceSvc == nil || s.instanceSvc.mcpSvc == nil {
		return
	}
	payloadJSON, err := s.commandRepo.GetLastConfigPushPayload(ctx, instanceId)
	if err != nil || payloadJSON == "" {
		return // 无历史下发（provision 未完成）：下次全量下发自动带上
	}
	var payload struct {
		ConfigContent string `json:"configContent"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return
	}
	if strings.Contains(payload.ConfigContent, "\nmcp_servers:") {
		return
	}
	servers := s.instanceSvc.mcpSvc.InstanceMcpServersFor(ctx, instanceId, "hermes")
	if len(servers) == 0 {
		return
	}
	if last, ok := mcpHealLast.Load(instanceId); ok {
		if t, ok := last.(time.Time); ok && time.Since(t) < mcpHealCooldown {
			return
		}
	}
	mcpHealLast.Store(instanceId, time.Now())
	slog.Warn("hermes 配置缺 mcp_servers 段但已绑 MCP 分组，自愈重推", "instance", instanceId, "groups", len(servers))
	_ = s.instanceSvc.SyncHermesMcpSection(ctx, instanceId, "system:mcp-drift-heal", servers)
}

// cleanupStaleRunning idle 超时兜底：基准为 last_progress_at（进度最近变化时间，认领时=started_at）。
// 进度连续「无新进度上报/无变化」超过 timeout 才标 failed——是「无进度时长」非「命令总时长」。
// manager 一直上报新进度（UpdateProgress 检测变化刷新 last_progress_at）就不会触发；仅兜底卡死/无输出。
// timeout：timeout_seconds>0 用它，否则默认 300s（5min）。
func (s *AgentRuntimeService) cleanupStaleRunning(ctx context.Context, instanceId string) {
	running, _, err := s.commandRepo.List(ctx, instanceId, 2, 1, 100) // status=2 running
	if err != nil {
		return
	}
	now := time.Now()
	for _, c := range running {
		base := c.LastProgressAt
		if base == nil {
			base = c.StartedAt
		}
		if base == nil {
			continue
		}
		timeoutSec := c.TimeoutSeconds
		if timeoutSec <= 0 {
			timeoutSec = 300 // 默认 5 分钟
		}
		if now.Sub(time.Time(*base)) > time.Duration(timeoutSec)*time.Second {
			msg := fmt.Sprintf("执行超时（连续%ds无进度变化）", timeoutSec)
			slog.Info("命令超时清理(idle)", "cmd", c.InstanceCommandId, "lastProgress", base, "timeout", timeoutSec)
			_ = s.commandRepo.UpdateResult(ctx, c.InstanceCommandId, 4, "{}", msg)
		}
	}
}

// supportAgentsFromReq 从心跳上行的 support（aa.json 声明）提取 [agent,version] 列表。
// 非空 → 原样返回（openclaw/hermes pod 自声明）；空（common pod）→ nil，留待服务端下发。
func supportAgentsFromReq(raw interface{}) [][2]string {
	arr, ok := raw.([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	out := make([][2]string, 0, len(arr))
	for _, item := range arr {
		pair, ok := item.([]interface{})
		if !ok || len(pair) < 1 {
			continue
		}
		agent, _ := pair[0].(string)
		if agent == "" {
			continue
		}
		ver := ""
		if len(pair) > 1 {
			ver, _ = pair[1].(string)
		}
		out = append(out, [2]string{agent, ver})
	}
	return out
}

// mergeCommandResult 合并心跳上报的命令进度/结果（对齐 desktop_service.Heartbeat 步骤4）。
// status=done/failed → UpdateResult（单接口合并掉 PostCommandResult）；running 的 interim 进度
// 暂不落（aa_agent_instance_command 无 progress 列，manager 命令秒级无长任务）。
func (s *AgentRuntimeService) mergeCommandResult(ctx context.Context, rc map[string]interface{}) {
	cmdId, _ := rc["commandId"].(string)
	if cmdId == "" {
		return
	}
	status, _ := rc["status"].(string)
	if status == "done" || status == "failed" {
		result, _ := rc["result"].(string)
		errMsg, _ := rc["error"].(string)
		code := mapCmdStatusCode(status)
		slog.Info("命令结果合并", "cmd", cmdId, "status", status, "code", code, "err", errMsg)
		_ = s.commandRepo.UpdateResult(ctx, cmdId, code, result, errMsg)
		// 仅 done 才按命令类型落业务副作用（install/uninstall 的实例↔skill 关联）；
		// failed 不落——关联随真实安装结果走（避免 stale 行，方案 B）。
		if status == "done" {
			s.applyCommandSideEffect(ctx, cmdId)
		}
		return
	}
	// running（interim）：进度写 progress_json（step 优先 progress，回退 lastLine + percent）。
	step, _ := rc["progress"].(string)
	if step == "" {
		step, _ = rc["lastLine"].(string)
	}
	if step == "" {
		return
	}
	prog := map[string]any{"step": step}
	if pct, ok := rc["percent"].(float64); ok && pct > 0 {
		prog["percent"] = int(pct)
	}
	b, _ := json.Marshal(prog)
	slog.Debug("命令进度合并", "cmd", cmdId, "step", step)
	_ = s.commandRepo.UpdateProgress(ctx, cmdId, string(b))
}

// mapCmdStatusCode 字符串状态 → command 表数字状态码（对齐 executor：3=done 4=failed）。
func mapCmdStatusCode(s string) int {
	if s == "failed" {
		return 4
	}
	return 3
}

// applyCommandSideEffect 命令成功（done）后的业务副作用。当前仅 install_skill/uninstall_skill：
// 按真实安装结果维护 aa_agent_instance_skill（方案 B——失败不落关联，避免 stale 行）。
// 多次合并同一 done 命令安全：install 幂等（已存在则更新版本记录=升级）、uninstall 幂等（无则 no-op）。
func (s *AgentRuntimeService) applyCommandSideEffect(ctx context.Context, cmdId string) {
	cmd, err := s.commandRepo.Get(ctx, cmdId)
	if err != nil || cmd == nil {
		return
	}
	skillId := payloadSkillId(cmd.PayloadJson)
	if skillId == "" {
		return
	}
	switch cmd.CommandType {
	case "install_skill":
		// 版本记录随真实安装结果落（payload.versionId 由下发方塞入；查不到版本串只落 id）
		versionId := payloadVersionId(cmd.PayloadJson)
		version := ""
		if versionId != "" && s.skillRepo != nil {
			if v, e := s.skillRepo.GetVersion(ctx, versionId); e == nil && v != nil {
				version = v.Version
			}
		}
		if existing, e := s.instanceSkillRepo.GetByInstanceSkill(ctx, cmd.InstanceId, skillId); e == nil && existing != nil {
			if e := s.instanceSkillRepo.SetVersion(ctx, cmd.InstanceId, skillId, versionId, version); e != nil {
				slog.Warn("升级成功但更新版本记录失败", "instance", cmd.InstanceId, "skill", skillId, "err", e)
			} else {
				slog.Info("技能安装成功，版本记录已更新", "instance", cmd.InstanceId, "skill", skillId, "version", version)
			}
			return
		}
		if _, e := s.instanceSkillRepo.Add(ctx, &entity.AgentInstanceSkill{
			InstanceId: cmd.InstanceId,
			SkillId:    skillId,
			VersionId:  versionId,
			Version:    version,
			Enabled:    1,
		}); e != nil {
			slog.Warn("安装成功但写关联失败", "instance", cmd.InstanceId, "skill", skillId, "err", e)
		} else {
			slog.Info("技能安装成功，关联已落", "instance", cmd.InstanceId, "skill", skillId, "version", version)
		}
		s.notifySkillChange(ctx, cmd.InstanceId, skillId, true)
	case "uninstall_skill":
		if e := s.instanceSkillRepo.DeleteByInstanceSkill(ctx, cmd.InstanceId, skillId); e != nil {
			slog.Warn("卸载成功但删关联失败", "instance", cmd.InstanceId, "skill", skillId, "err", e)
		} else {
			slog.Info("技能卸载成功，关联已删", "instance", cmd.InstanceId, "skill", skillId)
		}
		s.notifySkillChange(ctx, cmd.InstanceId, skillId, false)
	}
}

// notifySkillChange 实例技能装/卸结果双通道通知（收件人=实例归属人）。挂在命令 done 的
// 副作用点——含绑定自动配对装的手册，让「悄悄多出来的技能」对归属人可见。
// 走 PushSkillChange：站内逐条，渠道推送短窗聚合（自动配套装一批技能时免 N 连发）。
func (s *AgentRuntimeService) notifySkillChange(ctx context.Context, instanceId, skillId string, installed bool) {
	if s.channelNotice == nil {
		return
	}
	ins, err := s.instanceRepo.Get(ctx, instanceId)
	if err != nil || ins == nil {
		return
	}
	name := skillId
	if s.skillRepo != nil {
		if sk, e := s.skillRepo.Get(ctx, skillId); e == nil && sk != nil {
			name = sk.DisplayName
			if name == "" {
				name = sk.Slug
			}
		}
	}
	s.channelNotice.PushSkillChange(ctx, ins.UserId, instanceId, ins.Name, name, installed)
}

// payloadSkillId 从命令 payload_json 取 skillId（install/uninstall 下发时塞入，供成功副作用定位关联）。
func payloadSkillId(payloadJson string) string {
	var p struct {
		SkillId string `json:"skillId"`
	}
	if json.Unmarshal([]byte(payloadJson), &p) != nil {
		return ""
	}
	return p.SkillId
}

// payloadVersionId 从命令 payload_json 取 versionId（install_skill 下发时塞入，供成功副作用记录已装版本）。
func payloadVersionId(payloadJson string) string {
	var p struct {
		VersionId string `json:"versionId"`
	}
	if json.Unmarshal([]byte(payloadJson), &p) != nil {
		return ""
	}
	return p.VersionId
}

// EnqueueCommand 管理员下发命令（pending）。
func (s *AgentRuntimeService) EnqueueCommand(ctx context.Context, instanceId, commandType, agentId, issuedBy string, payload json.RawMessage, timeout int) (string, error) {
	// 校验实例存在
	inst, err := s.instanceRepo.Get(ctx, instanceId)
	if err != nil || inst == nil {
		return "", fmt.Errorf("实例不存在: %s", instanceId)
	}
	// agentId 留空时回填实例自身的 agent（openclaw/hermes…），命令历史「agent」列才不致空。
	if agentId == "" {
		agentId = inst.Agent
	}
	if timeout <= 0 {
		timeout = 300
	}
	// payload_json 是 jsonb，空时用 {}（空字符串会触发 jsonb 解析错误）。
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}
	e := &entity.AgentInstanceCommand{
		InstanceId:     instanceId,
		AgentId:        agentId,
		CommandType:    commandType,
		PayloadJson:    string(payload),
		Status:         0, // pending
		IssuedBy:       issuedBy,
		TimeoutSeconds: timeout,
	}
	cmdId, err := s.commandRepo.Add(ctx, e)
	if err != nil {
		return cmdId, err
	}
	// config_push 下发时记录配置内容到 config_change（source=dispatch，便于查看每次下发了什么）。
	if commandType == "config_push" {
		var pm map[string]interface{}
		if json.Unmarshal(payload, &pm) == nil {
			if content, ok := pm["configContent"].(string); ok && content != "" {
				_ = s.configChangeRepo.Insert(ctx, &entity.AgentInstanceConfigChange{
					InstanceId: instanceId,
					AgentId:    agentId,
					Source:     "dispatch",
					Dispatcher: issuedBy,
					ConfigHash: sha256Hex(content),
					ConfigSize: int64(len(content)),
					Content:    content,
				})
				slog.Info("配置下发记录", "instance", instanceId, "agent", agentId, "dispatcher", issuedBy, "size", len(content))
			}
		}
	}
	return cmdId, nil
}

// sha256Hex 计算字符串 sha256（配置内容指纹，去重 + 对比用）。
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// skillPackagePath manager 用 instance token 拉取 skill 整包 ZIP 的 InstanceAuth 端点（相对路径，
// manager 自身持有 api baseURL + Bearer token，拼接即得完整 https 链接）。即用户所说「把 skill 的
// https 链接当成参考传过去让 manager 自己解压安装」。
const skillPackagePath = "/api/v1/agent/runtime/skill/package"

// InstanceSkillItem 实例已安装 skill 的富化响应项（关联表 + aa_skill 详情）。
// 已装/库最新双版本供前端判升级：versionId 空的历史行=版本未知，视为可升级（装最新即对齐）。
type InstanceSkillItem struct {
	InstanceSkillId string      `json:"instanceSkillId"`
	InstanceId      string      `json:"instanceId"`
	SkillId         string      `json:"skillId"`
	Slug            string      `json:"slug"`
	DisplayName     string      `json:"displayName"`
	Summary         string      `json:"summary"`
	VersionId       string      `json:"versionId"`       // 实际装上的版本 id（install done 时落）
	Version         string      `json:"version"`         // 实际装上的版本号
	LatestVersionId string      `json:"latestVersionId"` // 库最新版本 id（解析口径同 InstallSkill）
	LatestVersion   string      `json:"latestVersion"`   // 库最新版本号（自由文本，仅做 ≠ 比较不猜大小）
	Enabled         uint        `json:"enabled"`
	SortOrder       int         `json:"sortOrder"`
	InstalledAt     *types.Time `json:"installedAt"`
}

// ListInstanceSkills 列出实例已安装的 skill，富化 aa_skill（displayName/slug/summary/库最新版本）。
// 跨库软引用：关联表在 agent 库，aa_skill 在 skill 库，逐条 Get 富化（量小，不走 join）。
func (s *AgentRuntimeService) ListInstanceSkills(ctx context.Context, instanceId string) ([]*InstanceSkillItem, error) {
	list, _, err := s.instanceSkillRepo.List(ctx, instanceId, 1, 1000)
	if err != nil {
		return nil, err
	}
	out := make([]*InstanceSkillItem, 0, len(list))
	for _, rel := range list {
		item := &InstanceSkillItem{
			InstanceSkillId: rel.InstanceSkillId,
			InstanceId:      rel.InstanceId,
			SkillId:         rel.SkillId,
			VersionId:       rel.VersionId,
			Version:         rel.Version,
			Enabled:         rel.Enabled,
			SortOrder:       rel.SortOrder,
			InstalledAt:     rel.CreatedAt,
		}
		if s.skillRepo != nil {
			if sk, e := s.skillRepo.Get(ctx, rel.SkillId); e == nil && sk != nil {
				item.Slug = sk.Slug
				item.DisplayName = sk.DisplayName
				item.Summary = sk.Summary
				// 库最新解析口径同 InstallSkill（latest_version_id → 最新非 yanked 回退），
				// 保证「升级」装的正是列表展示的版本
				if vid, rerr := s.resolveLatestVersionId(ctx, sk); rerr == nil && vid != "" {
					item.LatestVersionId = vid
					if v, gerr := s.skillRepo.GetVersion(ctx, vid); gerr == nil && v != nil {
						item.LatestVersion = v.Version
					}
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

// resolveLatestVersionId 解析技能「最新版本」id：优先 skill.latest_version_id（仅回填于
// PublishVersion）；未发布则回退最新非 yanked 版本（草稿也可装，DownloadVersion 不卡 status）。
// InstallSkill 与 ListInstanceSkills 的「库最新」列共用，保证两处口径一致。
// 安全口径：候选版本还须最新安全审计为「安全」（不安全/扫描中/失败/无记录跳过）。
func (s *AgentRuntimeService) resolveLatestVersionId(ctx context.Context, sk *entity.Skill) (string, error) {
	if sk.LatestVersionId != "" {
		if v, err := s.skillRepo.GetVersion(ctx, sk.LatestVersionId); err == nil && v != nil &&
			v.Status != 2 && ensureSkillVersionSafe(ctx, s.skillSecRepo, v.VersionId) == nil {
			return v.VersionId, nil
		}
	}
	versions, err := s.skillRepo.ListVersions(ctx, sk.SkillId)
	if err != nil {
		return "", err
	}
	for _, v := range versions { // ListVersions 按 id desc，最新优先
		if v.Status == 2 { // 跳过已 yanked
			continue
		}
		if ensureSkillVersionSafe(ctx, s.skillSecRepo, v.VersionId) != nil { // 跳过非「安全」
			continue
		}
		return v.VersionId, nil
	}
	return "", nil
}

// InstallSkill 安装/升级 skill 到实例：下发 install_skill 命令，payload 携带取包 https 链接
// + slug + versionId。versionId 空则装库最新（解析口径同上）；显式指定版本=升级/回滚。
// 关联与版本记录随真实安装结果落（方案 B，见 applyCommandSideEffect）。
func (s *AgentRuntimeService) InstallSkill(ctx context.Context, instanceId, skillId, versionId, issuedBy string) (string, error) {
	sk, err := s.skillRepo.Get(ctx, skillId)
	if err != nil || sk == nil {
		return "", fmt.Errorf("技能不存在: %s", skillId)
	}
	if versionId == "" {
		versionId, err = s.resolveLatestVersionId(ctx, sk)
		if err != nil {
			return "", fmt.Errorf("查询版本失败: %w", err)
		}
	}
	if versionId == "" {
		return "", fmt.Errorf("技能无可用版本: %s", skillId)
	}
	// 安全门禁：显式指定/解析出的版本最新审计须为「安全」，否则拒绝下发（取包端点同拦）。
	if err := ensureSkillVersionSafe(ctx, s.skillSecRepo, versionId); err != nil {
		return "", err
	}
	// 关联随真实安装结果落（方案 B）：这里不预写 aa_agent_instance_skill，
	// 等 manager 回传 install_skill done 时由 applyCommandSideEffect 写入；失败则不落（无 stale 行）。
	payload, _ := json.Marshal(map[string]any{
		"url":       fmt.Sprintf("%s?versionId=%s", skillPackagePath, versionId),
		"versionId": versionId,
		"slug":      sk.Slug,
		"name":      sk.DisplayName,
		"skillId":   skillId, // 供成功副作用定位关联
	})
	// agentId 留空，EnqueueCommand 内回填实例自身 agent（openclaw/hermes…）。
	cmdId, err := s.EnqueueCommand(ctx, instanceId, "install_skill", "", issuedBy, payload, 600)
	if err != nil {
		return "", err
	}
	slog.Info("下发 install_skill", "instance", instanceId, "skill", skillId, "slug", sk.Slug, "version", versionId, "cmd", cmdId)
	return cmdId, nil
}

// InstallPairedSkills 绑定即配手册：MCP 分组里的 platform-<域> 端点按约定配对内置技能
// （skills-builtin/<slug>/，slug=aiarsenal-<域>，读写端点同域去重），未安装的自动补装。
// 只加不减——解绑不卸技能，手册去留由用户主导（显式卸载后下次重绑会再补，属可接受的
// 收敛语义）。库中无该域配对技能（如 budget）是约定内正常，跳过。
// 已安装（aa_agent_instance_skill）或在途（pending/running 的 install_skill 命令）跳过，
// 不重复下发；单个失败仅记日志——调用点在 SyncInstance 尾部，绑定主流程已成功，不该被
// 手册补装失败拖挂。issuedBy 用绑定操作人，命令时间线可归因。
func (s *AgentRuntimeService) InstallPairedSkills(ctx context.Context, instanceId string, domains []string, issuedBy string) {
	if s.skillRepo == nil || len(domains) == 0 {
		return
	}
	installed := map[string]struct{}{}
	if rel, _, err := s.instanceSkillRepo.List(ctx, instanceId, 1, 500); err == nil {
		for _, r := range rel {
			installed[r.SkillId] = struct{}{}
		}
	}
	inflight := map[string]struct{}{}
	for _, st := range []int{0, 1} { // 命令状态：0=pending 1=running（3=done 4=failed）
		cmds, _, err := s.commandRepo.List(ctx, instanceId, st, 1, 100)
		if err != nil {
			continue
		}
		for _, c := range cmds {
			if c.CommandType != "install_skill" {
				continue
			}
			var p struct {
				SkillId string `json:"skillId"`
			}
			if json.Unmarshal([]byte(c.PayloadJson), &p) == nil && p.SkillId != "" {
				inflight[p.SkillId] = struct{}{}
			}
		}
	}
	for _, d := range domains {
		list, err := s.skillRepo.ListBySlug(ctx, "aiarsenal-"+d)
		if err != nil || len(list) == 0 {
			continue
		}
		sk := list[0]
		for _, cand := range list { // 跨命名空间同 slug 时优先内置源（Source=builtin）
			if cand.Source == "builtin" {
				sk = cand
				break
			}
		}
		if _, ok := installed[sk.SkillId]; ok {
			continue
		}
		if _, ok := inflight[sk.SkillId]; ok {
			continue
		}
		if _, err := s.InstallSkill(ctx, instanceId, sk.SkillId, "", issuedBy); err != nil {
			slog.Warn("绑定配对技能自动安装失败", "instance", instanceId, "slug", sk.Slug, "err", err)
			continue
		}
		slog.Info("绑定配对技能已下发安装", "instance", instanceId, "domain", d, "slug", sk.Slug)
	}
}

// UninstallSkill 卸载实例上的 skill：删 aa_agent_instance_skill 关联 + 下发 uninstall_skill 命令。
// payload 携带 slug，manager 据此定位 openclaw 托管目录并删除（openclaw path-watch 自动卸载）。
func (s *AgentRuntimeService) UninstallSkill(ctx context.Context, instanceId, skillId, issuedBy string) (string, error) {
	sk, err := s.skillRepo.Get(ctx, skillId)
	if err != nil || sk == nil {
		return "", fmt.Errorf("技能不存在或已删除，无法卸载: %s", skillId)
	}
	slug := sk.Slug
	// 关联随真实卸载结果删（方案 B）：不预删，等 manager 回传 uninstall_skill done 时由
	// applyCommandSideEffect 删除。失败则关联保留（技能仍在实例上）。
	payload, _ := json.Marshal(map[string]any{"slug": slug, "skillId": skillId})
	cmdId, err := s.EnqueueCommand(ctx, instanceId, "uninstall_skill", "", issuedBy, payload, 300)
	if err != nil {
		return "", err
	}
	slog.Info("下发 uninstall_skill", "instance", instanceId, "skill", skillId, "slug", slug, "cmd", cmdId)
	return cmdId, nil
}

// ListCommands 管理端查命令列表。
func (s *AgentRuntimeService) ListCommands(ctx context.Context, instanceId string, status int, pageNum, pageSize int) ([]*entity.AgentInstanceCommand, int64, error) {
	list, total, err := s.commandRepo.List(ctx, instanceId, status, pageNum, pageSize)
	if err != nil {
		return nil, 0, err
	}
	// 不跨库 join：先取命令数据，再用 issued_by（user_id）批量查员工信息回填姓名/工号。
	if len(list) > 0 && s.userRepo != nil {
		uidSet := make(map[string]struct{}, len(list))
		var uids []string
		for _, c := range list {
			if c.IssuedBy == "" {
				continue
			}
			if _, ok := uidSet[c.IssuedBy]; !ok {
				uidSet[c.IssuedBy] = struct{}{}
				uids = append(uids, c.IssuedBy)
			}
		}
		if len(uids) > 0 {
			users, _ := s.userRepo.GetUsersByIds(ctx, uids)
			umap := make(map[string]*model.UserSimpleRes, len(users))
			for _, u := range users {
				umap[u.Id] = u
			}
			for _, c := range list {
				if u, ok := umap[c.IssuedBy]; ok {
					c.IssuedByName = u.Name
					c.IssuedByEmpNo = u.EmployeeNo
				}
			}
		}
	}
	return list, total, nil
}

// GetRuntimeStatus 管理端查实例运行状态（最新一行）。
func (s *AgentRuntimeService) GetRuntimeStatus(ctx context.Context, instanceId string) (*entity.AgentInstanceRuntimeStatus, error) {
	return s.statusRepo.GetByInstance(ctx, instanceId)
}

// ListStatusHistory 管理端查实例状态历史时间线（仿 desktop status-history）。
func (s *AgentRuntimeService) ListStatusHistory(ctx context.Context, instanceId string, limit int) ([]*entity.AgentInstanceRuntimeStatusHistory, error) {
	return s.historyRepo.ListByInstance(ctx, instanceId, limit)
}

// ListConfigChanges 管理端查实例配置变更历史（仿 desktop config-changes）。
func (s *AgentRuntimeService) ListConfigChanges(ctx context.Context, instanceId string, limit int) ([]*entity.AgentInstanceConfigChange, error) {
	list, err := s.configChangeRepo.ListByInstance(ctx, instanceId, limit)
	if err != nil {
		return nil, err
	}
	// 不跨库 join：先取配置变更数据，再拿 dispatcher（user_id）批量查员工信息回填姓名/工号。
	if len(list) > 0 && s.userRepo != nil {
		uidSet := make(map[string]struct{}, len(list))
		var uids []string
		for _, c := range list {
			if c.Dispatcher == "" {
				continue
			}
			if _, ok := uidSet[c.Dispatcher]; !ok {
				uidSet[c.Dispatcher] = struct{}{}
				uids = append(uids, c.Dispatcher)
			}
		}
		if len(uids) > 0 {
			users, _ := s.userRepo.GetUsersByIds(ctx, uids)
			umap := make(map[string]*model.UserSimpleRes, len(users))
			for _, u := range users {
				umap[u.Id] = u
			}
			for _, c := range list {
				if u, ok := umap[c.Dispatcher]; ok {
					c.DispatcherName = u.Name
					c.DispatcherEmpNo = u.EmployeeNo
				}
			}
		}
	}
	return list, nil
}

// mapStatus 字符串状态 → uint 码（0=creating 1=running 2=stopped 3=error）。
func mapStatus(s string) uint {
	switch s {
	case "running":
		return 1
	case "stopped":
		return 2
	case "error":
		return 3
	case "creating":
		return 0
	default:
		return 1
	}
}

// resolveRuntimeConfigSource 心跳上报的 configHash 与历史对比，决定落哪种 runtime 配置变更（""=不落）。
//
//	hasDispatch && reported==下发hash && 未确认过  → runtime:applied（首次确认下发生效）
//	hasDispatch && reported==下发hash && 已确认过  → ""（已生效，同 hash 无变化，稳态）
//	hasDispatch && reported≠下发hash && 已确认过  → runtime:reverted（生效过又偏离：守卫恢复/被改回，真·回滚）
//	hasDispatch && reported≠下发hash && 未确认过  → ""（下发尚未生效，restart 过渡期旧 hash，跳过避免假 reverted）
//	无下发记录（openclaw 自改）                    → hash 变了记 runtime:applied，未变跳过
//
// 关键：dispatch 与 applied 共用同一 hash，所以「已确认生效」以 latest 是否为 applied 且 hash 与下发一致判定；
// reverted 只在该下发先 applied 过、之后偏离才记，未确认前的非下发 hash 是重启过渡，不当回滚。
func resolveRuntimeConfigSource(latest *entity.AgentInstanceConfigChange, reported string, d *entity.AgentInstanceConfigChange, dispatchErr error) string {
	hasDispatch := dispatchErr == nil && d != nil
	if hasDispatch {
		alreadyApplied := latest != nil && latest.Source == "runtime:applied" && latest.ConfigHash == d.ConfigHash
		switch {
		case reported == d.ConfigHash && !alreadyApplied:
			return "runtime:applied"
		case reported == d.ConfigHash:
			return "" // 已确认生效，同 hash：稳态，不重复记
		case alreadyApplied:
			return "runtime:reverted" // 生效过又偏离下发 hash
		default:
			return "" // 未确认生效、reported≠下发 hash：过渡期，跳过
		}
	}
	// 无下发记录：openclaw 自改——hash 变了记 applied，未变跳过。
	if latest != nil && latest.ConfigHash == reported {
		return ""
	}
	return "runtime:applied"
}

// truncateRunes 截断字符串到 n 个 rune（按字符不按字节，中文版本串不会被拦腰截出乱码）。
// 上报字段落 varchar 有限列（如 agent_proc_version varchar(100)）前的兜底。
func truncateRunes(v interface{}, n int) string {
	s, _ := v.(string)
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
