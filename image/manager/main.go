package main

import (
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aiarsenal/agent-image/logx"
)

// 容器内 agent 入口：读环境变量 + aa.json，起单心跳循环。
// 单接口模式（对齐 desktop）：上行 modules（每个 supportAgent 状态 + 执行中命令）+ aa.json support，
// 下行认领命令 + supportAgents。被管 agent 集合由 supportAgents 决定：
// openclaw/hermes pod 单个；common pod（未来）多个/服务端下发。
func main() {
	logx.Init("/aa/.aa/logs", "manager", slog.LevelInfo)

	baseURL := os.Getenv("AIARSENAL_BASE_URL")
	token := os.Getenv("AIARSENAL_INSTANCE_TOKEN")
	instanceID := os.Getenv("AIARSENAL_INSTANCE_ID")
	if baseURL == "" || token == "" || instanceID == "" {
		log.Fatalf("缺少环境变量: AIARSENAL_BASE_URL/AIARSENAL_INSTANCE_TOKEN/AIARSENAL_INSTANCE_ID")
	}
	log.Printf("[agent] 启动: instance=%s base=%s", instanceID, baseURL)

	api := NewAPI(baseURL, token)
	// 读 aa.json：初始被管 agent = support 里的 agent 名（openclaw/hermes pod 非空；common pod 空，等心跳服务端下发）。
	cfg := loadAAConfigSafe(aaConfigPath)
	ms := newManagerSet(initialAgents(cfg))
	// 先配置后启动：仅拉起配置已就绪的 agent（容器重启/存量卷）。
	// 首次创建（卷空、配置待 config_push 下发）的不启动——等心跳认领 config_push 写盘后由 executor 拉起。
	ms.startAllIfReady()
	log.Printf("[agent] 初始 agents=%v", ms.names())

	exe := NewExecutor(ms, api)
	state := &cmdState{notify: make(chan struct{}, 1)}

	stop := make(chan struct{})
	go heartbeatLoop(api, ms, exe, state, cfg, stop)

	// 信号优雅退出：停止所有被管 agent 后退出。
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Printf("[agent] 收到退出信号，停止 %d 个 agent...", len(ms.names()))
	close(stop)
	ms.stopAll()
	time.Sleep(500 * time.Millisecond)
}

// initialAgents 从 aa.json support 取 agent 名列表。support 非空→取全部 agent 名；空（common pod）→空。
func initialAgents(cfg *AAConfig) []string {
	if cfg == nil {
		return nil
	}
	out := make([]string, 0, len(cfg.Support))
	for _, p := range cfg.Support {
		if p[0] != "" {
			out = append(out, p[0])
		}
	}
	return out
}

// loadAAConfigSafe 读 aa.json，失败返回 nil。成功打印 support 便于排查。
func loadAAConfigSafe(path string) *AAConfig {
	c, err := loadAAConfig(path)
	if err != nil {
		log.Printf("[agent] aa.json 读取失败（%v）", err)
		return nil
	}
	log.Printf("[agent] aa.json loaded: support=%v", c.Support)
	return c
}

// heartbeatLoop 单接口心跳循环，间隔自适应（执行中 2s / 空闲 8s）。启动立即报一次。
func heartbeatLoop(api *API, ms *managerSet, exe *Executor, state *cmdState, cfg *AAConfig, stop <-chan struct{}) {
	busy := envDuration("AIARSENAL_HEARTBEAT_BUSY_INTERVAL", 1*time.Second) // 命令执行中 1s（对齐 desktop）
	doHeartbeat(api, ms, exe, state, cfg)
	for {
		interval := idleInterval()
		if state.isBusy() {
			interval = busy
		}
		select {
		case <-stop:
			return
		case <-time.After(interval):
		case <-state.notify: // 命令 start/done 立即触发（不等间隔，对齐 desktop）
		}
		doHeartbeat(api, ms, exe, state, cfg)
	}
}

// idleInterval 空闲心跳间隔（对齐 desktop heartbeat_interval_secs）：
// 当地时区 8–20 点 2s，其它时段 10s。env AIARSENAL_HEARTBEAT_INTERVAL 可覆盖（调试用）。
func idleInterval() time.Duration {
	if v := os.Getenv("AIARSENAL_HEARTBEAT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	h := time.Now().Hour()
	if h >= 8 && h < 20 {
		return 2 * time.Second
	}
	return 10 * time.Second
}

// doHeartbeat 一次心跳：上行 infra/agent + 每个 supportAgent 状态（含执行中命令）→ 认领命令异步执行。
func doHeartbeat(api *API, ms *managerSet, exe *Executor, state *cmdState, cfg *AAConfig) {
	state.clearReported()
	// modules：infra/agent 固定（manager 自身）+ 每个 supportAgent 的体检。
	modules := map[string]ModuleReport{
		"infra": {Status: "running", Version: agentVersion},
		"agent": {Status: "running", Version: agentVersion, PidOrContainerId: pidStr()},
	}
	for name, m := range ms.all() {
		modules[name] = m.Status()
	}
	// 挂当前执行中命令到对应 agent module。
	if rc := state.snapshot(); rc != nil {
		if aid := state.currentAgentId(); aid != "" {
			if mod, ok := modules[aid]; ok {
				mod.RunningCommand = rc
				modules[aid] = mod
			}
		}
	}
	req := &HeartbeatReq{Modules: modules}
	if cfg != nil {
		req.Support = cfg.Support
	}
	res, err := api.Heartbeat(req)
	if err != nil {
		log.Printf("[agent] 心跳失败: %v", err)
		return
	}
	// 业务 Fail（code!=0，如服务端存储抖动）时 data 无语义：跳过 sync/认领保持现状。
	// 否则空 supportAgents 会误移除被管 agent → 移除/拉起死循环（hermes 心跳落库失败踩过）。
	if res.Code != 0 {
		log.Printf("[agent] 心跳业务失败(code=%d msg=%s)，跳过 sync/认领", res.Code, res.Message)
		return
	}
	// 按 supportAgents 增删被管 agent（common pod 服务端下发时生效；单 agent pod 稳定不变）。
	ms.sync(res.Data.SupportAgents)
	// 认领命令：空闲且响应有命令 → 异步执行（命令须能定位到某个 supportAgent）。
	if !state.isBusy() && len(res.Data.Commands) > 0 {
		cmd := res.Data.Commands[0]
		agentId := cmd.AgentId
		if agentId == "" { // 命令未指定 agent：仅单 agent pod 可兜底用唯一那个
			if names := ms.names(); len(names) == 1 {
				agentId = names[0]
			}
		}
		if agentId != "" && ms.get(agentId) != nil {
			log.Printf("[agent] 认领命令: %s agent=%s type=%s", cmd.InstanceCommandId, agentId, cmd.CommandType)
			pb := state.begin(cmd.InstanceCommandId, agentId, cmd.CommandType)
			go func() {
				status, result, errMsg := exe.Exec(agentId, cmd.CommandType, cmd.PayloadJson, pb)
				state.finish(cmd.InstanceCommandId, status, result, errMsg)
				log.Printf("[agent] 命令完成: %s status=%d", cmd.InstanceCommandId, status)
			}()
		} else if agentId != "" {
			log.Printf("[agent] 命令 %s 指定 agent=%s 不在 supportAgents，跳过", cmd.InstanceCommandId, agentId)
		}
	} else if len(res.Data.Commands) > 0 {
		// 服务端认领前置闸（running 未回收不下发）生效后不应到达；兜底记日志，不再静默吞命令。
		log.Printf("[agent] 忙时收到下发命令 %s，无法执行（服务端已标 running，将等看门狗超时回收）", res.Data.Commands[0].InstanceCommandId)
	}
}

// statusStr executor 数字状态码（3=done 4=failed 5=timeout）→ 字符串（服务端合并认 done/failed）。
func statusStr(s int) string {
	switch s {
	case 3:
		return "done"
	case 4, 5:
		return "failed"
	default:
		return "done"
	}
}

// ===== 当前执行中命令（串行）=====

// runningCmd 当前正在执行（或刚执行完待上报）的命令。agentId 标记它作用于哪个被管 agent。
type runningCmd struct {
	commandId string
	agentId   string
	action    string
	status    string // running/done/failed
	result    string
	errMsg    string
	reported  bool // 终态是否已随心跳上行一次（上行后下一轮 clearReported 清空）
	pb        *progressBuffer // 执行过程输出（executor 边写、心跳边读最新行上报）
}

// progressBuffer 线程安全的执行过程输出收集器。
// executor 执行命令时写入关键步骤 + 外部命令输出；心跳循环并发读 LastLine 上报（running 进度可见），
// 命令完成时 String() 取全文并入 result（done/failed 记录完整执行过程）。
type progressBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (p *progressBuffer) Write(line string) {
	if p == nil || line == "" {
		return
	}
	p.mu.Lock()
	p.lines = append(p.lines, line)
	p.mu.Unlock()
}

// LastLine 最近一行（心跳 running 上报 LastLine/进度用）。空返回 ""。
func (p *progressBuffer) LastLine() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.lines) == 0 {
		return ""
	}
	return p.lines[len(p.lines)-1]
}

// String 全部过程行（done 时并入 result）。
func (p *progressBuffer) String() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.lines) == 0 {
		return ""
	}
	return strings.Join(p.lines, "\n")
}

// attachProcessLog 把执行过程日志并入 result JSON。result_json 是 jsonb 列，必须合法 JSON：
// 解析 executor 返回的 JSON 摘要（如 {"action":"config_push","applied":true}），追加 _processLog 字段；
// result 非 JSON 时兜底包一层 {"result":...,"_processLog":...}。
func attachProcessLog(result, log string) string {
	m := map[string]interface{}{}
	if json.Unmarshal([]byte(result), &m) != nil {
		m = map[string]interface{}{"result": result}
	}
	m["_processLog"] = log
	b, _ := json.Marshal(m)
	return string(b)
}

// cmdState 共享当前命令，mutex 保护。current==nil 表示空闲，可接新命令。
type cmdState struct {
	mu      sync.Mutex
	current *runningCmd
	notify  chan struct{} // 命令 start/done 立即触发心跳（不等间隔，对齐 desktop）
}

// isBusy 是否有命令正在执行。只认 running 态：已 done/failed 但还未 clearReported 清掉
// 的上一条不算忙——否则服务端在「上报 done 的那次心跳」里顺带下发的下一条命令会被这里挡掉、
// 静默丢弃（ClaimNext 不重投 running 命令→命令孤儿化→60s 超时标 failed）。
func (s *cmdState) isBusy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current != nil && s.current.status == "running"
}

// begin 占位一条 running 命令（调用方已确认空闲）。返回该命令的过程 buffer，供 executor 边写边报。
func (s *cmdState) begin(commandId, agentId, action string) *progressBuffer {
	pb := &progressBuffer{}
	s.mu.Lock()
	s.current = &runningCmd{commandId: commandId, agentId: agentId, action: action, status: "running", pb: pb}
	s.mu.Unlock()
	s.triggerNotify() // 命令开始：立即上报 running
	return pb
}

// currentAgentId 当前命令作用的 agent（挂 runningCommand 到对应 module 用）。
func (s *cmdState) currentAgentId() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return ""
	}
	return s.current.agentId
}

// finish 执行协程回写结果（数字状态→字符串）。只回写仍属本命令的 current（防错位）。
// pb 的执行过程全文并入 result：done/failed 时随心跳上报（→ result_json），UI 结果区可见完整过程。
func (s *cmdState) finish(commandId string, status int, result, errMsg string) {
	s.mu.Lock()
	if s.current != nil && s.current.commandId == commandId {
		s.current.status = statusStr(status)
		if s.current.pb != nil {
			if out := s.current.pb.String(); out != "" {
				// result_json 是 jsonb 列：必须合法 JSON。把执行过程作为 _processLog 字段并入，
				// 而非裸文本拼接（裸文本会让 UpdateResult 写 jsonb 失败，done 落不了库 → 命令卡 running 被超时清理）。
				result = attachProcessLog(result, out)
			}
		}
		s.current.result = result
		s.current.errMsg = errMsg
	}
	s.mu.Unlock()
	s.triggerNotify() // 命令完成：立即上报结果
}

// triggerNotify 非阻塞触发一次立即心跳（已有待触发则不重复，对齐 desktop notify）。
func (s *cmdState) triggerNotify() {
	if s.notify == nil {
		return
	}
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// snapshot 返回 current 的上报副本；终态标记 reported（下一轮 clearReported 清空）。
// running 中附带 pb 最新一行（→ LastLine）：心跳上报执行进度，服务端落 progress_json，UI 进度列可见。
func (s *cmdState) snapshot() *RunningCommandReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return nil
	}
	if s.current.status == "done" || s.current.status == "failed" {
		s.current.reported = true
	}
	rep := &RunningCommandReport{
		CommandId: s.current.commandId,
		Action:    s.current.action,
		Status:    s.current.status,
		Result:    s.current.result,
		Error:     s.current.errMsg,
	}
	// 仅 running 态上报过程行（done/failed 的完整过程已并入 Result，不再重复占 LastLine）。
	if s.current.status == "running" && s.current.pb != nil {
		if line := s.current.pb.LastLine(); line != "" {
			rep.LastLine = line
		}
	}
	return rep
}

// clearReported 清掉已上报的终态命令，释放队列接下一条。
func (s *cmdState) clearReported() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil && s.current.reported {
		s.current = nil
	}
}

const agentVersion = "aiarsenal-manager/0.1.0"

func pidStr() string { return strconv.Itoa(os.Getpid()) }

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
