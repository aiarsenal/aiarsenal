package dto

import (
	"encoding/json"

	"aiarsenal/domain/entity"
)

// Agent 模块 v2 运行时/团队层请求/响应 DTO（镜像 agent.go 写法）。

// ===== Team 协作 =====
type AgentTeamSearchReq struct {
	Name string `json:"name" form:"name"`
	PageReq
}
type AgentTeamSearchRes struct {
	ListRes
	List []*entity.AgentTeam `json:"list"`
}
type AgentTeamReq struct {
	Name              string `json:"name" form:"name" binding:"required"`
	Description       string `json:"description" form:"description"`
	CommunicationMode string `json:"communicationMode" form:"communicationMode"`
	SharedVolumeName  string `json:"sharedVolumeName" form:"sharedVolumeName"`
	SharedMountPath   string `json:"sharedMountPath" form:"sharedMountPath"`
	RedisUrl          string `json:"redisUrl" form:"redisUrl"`
	TeamToken         string `json:"teamToken" form:"teamToken"`
}
type AgentTeamAddReq struct{ *AgentTeamReq }
type AgentTeamEditReq struct {
	TeamId string `json:"teamId" form:"teamId" binding:"required"`
	*AgentTeamReq
}

type AgentTeamMemberSearchReq struct {
	TeamId string `json:"teamId" form:"teamId"`
	PageReq
}
type AgentTeamMemberSearchRes struct {
	ListRes
	List []*entity.AgentTeamMember `json:"list"`
}
type AgentTeamMemberReq struct {
	TeamId      string `json:"teamId" form:"teamId" binding:"required"`
	InstanceId  string `json:"instanceId" form:"instanceId"`
	UserId      string `json:"userId" form:"userId"`
	MemberKey   string `json:"memberKey" form:"memberKey" binding:"required"`
	DisplayName string `json:"displayName" form:"displayName" binding:"required"`
	Role        string `json:"role" form:"role"`
	Description string `json:"description" form:"description"`
}
type AgentTeamMemberAddReq struct{ *AgentTeamMemberReq }
type AgentTeamMemberEditReq struct {
	TeamMemberId string `json:"teamMemberId" form:"teamMemberId" binding:"required"`
	*AgentTeamMemberReq
}

type AgentTeamTaskSearchReq struct {
	TeamId string `json:"teamId" form:"teamId"`
	Status int    `json:"status" form:"status"`
	PageReq
}
type AgentTeamTaskSearchRes struct {
	ListRes
	List []*entity.AgentTeamTask `json:"list"`
}
type AgentTeamTaskDispatchReq struct {
	TeamId         string `json:"teamId" form:"teamId" binding:"required"`
	TargetMemberId string `json:"targetMemberId" form:"targetMemberId"`
	PayloadJson    string `json:"payloadJson" form:"payloadJson"`
}

type AgentTeamEventSearchReq struct {
	TeamId string `json:"teamId" form:"teamId"`
	PageReq
}
type AgentTeamEventSearchRes struct {
	ListRes
	List []*entity.AgentTeamEvent `json:"list"`
}

type AgentTeamWorkItemSearchReq struct {
	RootTaskId string `json:"rootTaskId" form:"rootTaskId"`
	Status     int    `json:"status" form:"status"`
	PageReq
}
type AgentTeamWorkItemSearchRes struct {
	ListRes
	List []*entity.AgentTeamWorkItem `json:"list"`
}
type AgentTeamWorkItemReq struct {
	TeamId        string `json:"teamId" form:"teamId" binding:"required"`
	RootTaskId    string `json:"rootTaskId" form:"rootTaskId" binding:"required"`
	WorkId        string `json:"workId" form:"workId" binding:"required"`
	OwnerMemberId string `json:"ownerMemberId" form:"ownerMemberId"`
	Title         string `json:"title" form:"title" binding:"required"`
	DependsOnJson string `json:"dependsOnJson" form:"dependsOnJson"`
}
type AgentTeamWorkItemAddReq struct{ *AgentTeamWorkItemReq }
type AgentTeamWorkItemEditReq struct {
	WorkItemId string `json:"workItemId" form:"workItemId" binding:"required"`
	Status     uint   `json:"status" form:"status"`
	ResultJson string `json:"resultJson" form:"resultJson"`
}

// ===== 系统设置（镜像卡片） =====
type AgentSystemImageSearchReq struct {
	Agent   string `json:"agent" form:"agent"`
	Version string `json:"version" form:"version"`
}
type AgentSystemImageSearchRes struct {
	List []*entity.AgentSystemImageSetting `json:"list"`
}
type AgentSystemImageReq struct {
	Agent       string `json:"agent" form:"agent" binding:"required"`
	Version     string `json:"version" form:"version" binding:"required"`
	DisplayName string `json:"displayName" form:"displayName" binding:"required"`
	Image       string `json:"image" form:"image" binding:"required"`
	IsEnabled   uint   `json:"isEnabled" form:"isEnabled"`
}

// ===== 概览（dashboard 聚合） =====
type AgentDashboardRes struct {
	InstanceTotal   int64   `json:"instanceTotal"`
	InstanceRunning int64   `json:"instanceRunning"`
	TeamTotal       int64   `json:"teamTotal"`
	ModelTotal      int64   `json:"modelTotal"`
	ScanJobActive   int64   `json:"scanJobActive"`
	Invocation24h   int64   `json:"invocation24h"`
	Cost24h         float64 `json:"cost24h"`
}

// ===== 实例（localdocker 真跑容器）=====
type AgentInstanceSearchReq struct {
	Name   string `json:"name" form:"name"`
	Status int    `json:"status" form:"status"`
	PageReq
}
type AgentInstanceSearchRes struct {
	ListRes
	List []*entity.AgentInstance `json:"list"`
}
type AgentInstanceReq struct {
	Name                     string       `json:"name" form:"name" binding:"required"`
	Description              string       `json:"description" form:"description"`
	Agent                    string       `json:"agent" form:"agent" binding:"required"`
	Version                  string       `json:"version" form:"version" binding:"required"`
	CpuCores                 int          `json:"cpuCores" form:"cpuCores"`
	MemoryGb                 int          `json:"memoryGb" form:"memoryGb"`
	DiskGb                   int          `json:"diskGb" form:"diskGb"`
	ImageTag                 string       `json:"imageTag" form:"imageTag"` // 留空则按 type/runtime/mode 查 system_image_setting
	EnvironmentOverridesJson string       `json:"environmentOverridesJson" form:"environmentOverridesJson"`
	MountPath                string       `json:"mountPath" form:"mountPath"`
	Ports                    []PortMapReq `json:"ports" form:"ports"`
}
type PortMapReq struct {
	HostPort      int `json:"hostPort" form:"hostPort"`
	ContainerPort int `json:"containerPort" form:"containerPort" binding:"required"`
}
type AgentInstanceExecReq struct {
	Cmd []string `json:"cmd" form:"cmd" binding:"required"`
}

// ===== Phase3 控制面（desktop 心跳+认领模式，instance 维度）=====

// AgentRuntimeHeartbeatReq 容器内 agent 上行心跳：模块状态 + aa.json 声明的 support。
type AgentRuntimeHeartbeatReq struct {
	Modules map[string]AgentModuleReport `json:"modules"`
	Support [][2]string                  `json:"support,omitempty"` // aa.json support（[agent,version]）；空=common pod，待服务端下发
}

// AgentModuleReport 单模块上报（对齐 desktop ModuleReport）。
type AgentModuleReport struct {
	Status           string                     `json:"status"` // running/stopped/error
	Version          string                     `json:"version"`
	PidOrContainerId string                     `json:"pidOrContainerId"`
	UptimeSeconds    int64                      `json:"uptimeSeconds"`
	LastError        string                     `json:"lastError"`
	ConfigHash       string                     `json:"configHash"`
	ConfigSize       int64                      `json:"configSize"`
	ConfigMtime      string                     `json:"configMtime"`
	ConfigContent    string                     `json:"configContent,omitempty"`
	Summary          map[string]interface{}     `json:"summary,omitempty"`
	RunningCommand   *AgentRunningCommandReport `json:"runningCommand,omitempty"`
}

// AgentRunningCommandReport 当前执行中命令的进度/结果（对齐 desktop_service.RunningCommandReport）。
// manager 随心跳上行：服务端据此合并命令结果（status=done/failed → UpdateResult）。
type AgentRunningCommandReport struct {
	CommandId string `json:"commandId"`
	Action    string `json:"action"`
	Status    string `json:"status"` // running/done/failed
	Progress  string `json:"progress"`
	Percent   int    `json:"percent"`
	LastLine  string `json:"lastLine"`
	Result    string `json:"result"`
	Error     string `json:"error"`
}

// AgentRuntimeHeartbeatRes 心跳响应：下行 pending 命令 + 该 pod 该支持的 agent 名。
type AgentRuntimeHeartbeatRes struct {
	Stored        bool                           `json:"stored"`
	Commands      []*entity.AgentInstanceCommand `json:"commands"`
	SupportAgents [][2]string                    `json:"supportAgents"` // 非空原样回 support 的 [agent,version]；空=common pod 待服务端下发
}

// AgentEnqueueCommandReq 管理员下发命令。
type AgentEnqueueCommandReq struct {
	InstanceId     string          `json:"instanceId" form:"instanceId"`                      // 可选：路由 :id 优先，仅作无路径时兜底
	CommandType    string          `json:"commandType" form:"commandType" binding:"required"` // start/stop/restart/config_push/channel_login/...
	AgentId        string          `json:"agentId" form:"agentId"`
	Payload        json.RawMessage `json:"payload" form:"payload"`
	TimeoutSeconds int             `json:"timeoutSeconds" form:"timeoutSeconds"`
}

// AgentEnqueueCommandRes 下发结果。
type AgentEnqueueCommandRes struct {
	CommandId string `json:"commandId"`
}

// AgentCommandListRes 命令列表响应。
type AgentCommandListRes struct {
	ListRes
	List []*entity.AgentInstanceCommand `json:"list"`
}

// AgentInstanceSkillReq 实例技能安装/卸载请求。instanceId 可选：路由 :id 优先。
// versionId 可选：空=装库最新；显式指定=升级/回滚到该版本（仅安装用）。
type AgentInstanceSkillReq struct {
	InstanceId string `json:"instanceId" form:"instanceId"`
	SkillId    string `json:"skillId" form:"skillId" binding:"required"`
	VersionId  string `json:"versionId" form:"versionId"`
}

// AgentInstanceMcpReq 实例 MCP 分组绑定/解绑请求。instanceId 可选：路由 :id 优先。
type AgentInstanceMcpReq struct {
	InstanceId string `json:"instanceId" form:"instanceId"`
	GroupId    string `json:"groupId" form:"groupId" binding:"required"`
}

// AgentInstanceSkillOpRes 安装/卸载结果（返回下发的 commandId，便于前端跟进命令历史）。
type AgentInstanceSkillOpRes struct {
	CommandId string `json:"commandId"`
}
