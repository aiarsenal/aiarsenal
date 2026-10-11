// Package main: 容器内 agent，desktop 的「单心跳接口」模式。
// 跑在 openclaw 镜像里，定时向 aiarsenal api 上报状态（含执行中命令的进度/结果），
// 认领响应里下发的命令并本地执行（管理 openclaw）。命令结果走 modules[x].runningCommand
// 上行→服务端合并；下一条命令走 heartbeat 响应 commands 下行。不再单独 claim / post-result。
package main

// ModuleReport 单模块上报（对齐服务端 AgentModuleReport）。
type ModuleReport struct {
	Status           string                 `json:"status"`
	Version          string                 `json:"version"`
	PidOrContainerId string                 `json:"pidOrContainerId"`
	UptimeSeconds    int64                  `json:"uptimeSeconds"`
	LastError        string                 `json:"lastError"`
	ConfigHash       string                 `json:"configHash"`
	ConfigSize       int64                  `json:"configSize"`
	ConfigMtime      string                 `json:"configMtime"`
	ConfigContent    string                 `json:"configContent,omitempty"` // 配置全文（hash 变化时落 config_change.content）
	Summary          map[string]interface{} `json:"summary,omitempty"`
	RunningCommand   *RunningCommandReport  `json:"runningCommand,omitempty"`
	Skills           []string               `json:"skills,omitempty"` // 技能目录清单（hermes：$HERMES_HOME/skills 下含 SKILL.md 的相对路径；服务端据此做漂移巡检）
}

// RunningCommandReport 当前执行中命令的进度/结果（对齐 desktop RunningCommandReport）。
// 随心跳上行：服务端据此合并命令结果（status=done/failed → UpdateResult）。
type RunningCommandReport struct {
	CommandId string `json:"commandId"`
	Action    string `json:"action"`
	Status    string `json:"status"` // running/done/failed
	Progress  string `json:"progress,omitempty"`
	Percent   int    `json:"percent,omitempty"`
	LastLine  string `json:"lastLine,omitempty"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

// HeartbeatReq 心跳请求。Support 取自 aa.json（镜像声明的支持列表 [agent,version]）；空=common pod，待服务端下发。
type HeartbeatReq struct {
	Modules map[string]ModuleReport `json:"modules"`
	Support [][2]string             `json:"support,omitempty"`
}

// HeartbeatRes 心跳响应（下行 pending 命令 + 该 pod 该支持的 agent 名）。
type HeartbeatRes struct {
	Code int32 `json:"code"`
	Data struct {
		Stored        bool        `json:"stored"`
		Commands      []Command   `json:"commands"`
		SupportAgents [][2]string `json:"supportAgents"` // 服务端下发：[agent,version]（非空原样回，空=common 待下发）
	} `json:"data"`
	Message string `json:"message"`
}

// Command 远程指令（对齐 AgentInstanceCommand）。
type Command struct {
	InstanceCommandId string `json:"instanceCommandId"`
	InstanceId        string `json:"instanceId"`
	AgentId           string `json:"agentId"`
	CommandType       string `json:"commandType"`
	PayloadJson       string `json:"payloadJson"`
	Status            uint   `json:"status"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
}
