package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

// Client 模块仓储接口（client 库）。
// agent（类型+介绍）：ListAll 取全部、GetByAgent 按 agent key 取、Upsert、Delete。
// agent 版本：ListByAgent 按 agent 取、ListAllByAgents 批量取、Upsert、Delete、ClearDefault。
// 运行状态：按 (user_id, agent) upsert；按 user_id 列出。

type IClientAgentRepository interface {
	// ListAll 返回全部 agent（按 weigh desc, agent asc 排序）。
	ListAll(ctx context.Context) ([]*entity.ClientAgent, error)
	// GetByAgent 按 agent key（agent 列）取 agent。
	GetByAgent(ctx context.Context, agent string) (*entity.ClientAgent, error)
	// Upsert 按 agent_id 更新或按 agent 冲突插入（管理员维护）。
	Upsert(ctx context.Context, e *entity.ClientAgent) error
	// Delete 按 agent_id 删除（同时应删其全部 version，由 service 协调）。
	Delete(ctx context.Context, agentId string) error
}

type IClientAgentVersionRepository interface {
	// ListByAgent 返回某 agent 的全部版本（按 is_default desc, version asc）。
	ListByAgent(ctx context.Context, agentId string) ([]*entity.ClientAgentVersion, error)
	// ListAllByAgents 返回多 agent 的全部版本，按 agent_id 分组返回 map。
	ListAllByAgents(ctx context.Context, agentIds []string) (map[string][]*entity.ClientAgentVersion, error)
	// Upsert 按 version_id 更新或按 (agent_id, version) 冲突插入。
	Upsert(ctx context.Context, e *entity.ClientAgentVersion) error
	// Delete 按 version_id 删除。
	Delete(ctx context.Context, versionId string) error
	// DeleteByAgent 删除某 agent 的全部版本（agent 删除时调用）。
	DeleteByAgent(ctx context.Context, agentId string) error
	// ClearDefault 清除某 agent 内其它版本的 is_default 标记（设默认版本时用）。
	ClearDefault(ctx context.Context, agentId string, exceptVersionId string) error
}

type IClientAgentStatusRepository interface {
	// Upsert 按 (device_id, agent) 插入或更新；新行用 idgen 生成 status_id。
	// 时间线去重：连续相同状态（status/version/pid/last_error）最多留 2 条，
	// 第 3 条起更新第 2 条（刷新 reported_at/uptime），状态变化才插新行。
	// forceInsert=true 时跳过合并，强制插新行（命令执行期间保留每一步状态变化）。
	Upsert(ctx context.Context, e *entity.ClientAgentStatus, forceInsert bool) error
	// ListByDevice 返回某设备全部客户端状态（时间线，多条/客户端）。
	ListByDevice(ctx context.Context, deviceId string) ([]*entity.ClientAgentStatus, error)
	// ListLatestByDevice 返回某设备每个客户端的最新一条状态（管理后台操作 agent 用）。
	ListLatestByDevice(ctx context.Context, deviceId string) ([]*entity.ClientAgentStatus, error)
	// ListByUser 返回某用户全部客户端状态（兼容旧入口）。
	ListByUser(ctx context.Context, userId string) ([]*entity.ClientAgentStatus, error)
	// ListAll 返回全部客户端状态（管理后台用）。
	ListAll(ctx context.Context) ([]*entity.ClientAgentStatus, error)
}

type IClientCommandRepository interface {
	// Enqueue 入队一条 pending 指令。
	Enqueue(ctx context.Context, e *entity.ClientCommand) error
	// CancelChainPending 取消同一链下所有 pending（前置步骤失败）。
	CancelChainPending(ctx context.Context, chainId string) error
	// ClaimNext 原子认领某设备最早一条 pending（标 running），返回该条。无则返回 nil。
	ClaimNext(ctx context.Context, deviceId string) (*entity.ClientCommand, error)
	// UpdateResult 回传执行结果。
	UpdateResult(ctx context.Context, commandId, status, result, errMsg string) error
	// UpdateProgress 更新执行进度。
	UpdateProgress(ctx context.Context, commandId, progress string) error
	// ListByDevice 某设备指令历史（最近优先）。
	ListByDevice(ctx context.Context, deviceId string, limit int) ([]*entity.ClientCommand, error)
	// ListRunningByDevice 某设备 status=running 的命令（串行校验 + 超时清理用）。
	ListRunningByDevice(ctx context.Context, deviceId string) ([]*entity.ClientCommand, error)
	// ListPendingByDevice 某设备 status=pending 的命令（已入队、客户端尚未认领）。
	// 串行去重用：防快速双击在第一条还在 pending 时又入队一条相同动作。
	ListPendingByDevice(ctx context.Context, deviceId string) ([]*entity.ClientCommand, error)
	// CommandedAgents 该设备下过命令的 agent key 集合（心跳过滤用）。
	CommandedAgents(ctx context.Context, deviceId string) (map[string]bool, error)
	// GetById 取单条。
	GetById(ctx context.Context, commandId string) (*entity.ClientCommand, error)
}

type IClientLogRepository interface {
	// AppendLogs 批量追加日志。
	AppendLogs(ctx context.Context, logs []*entity.ClientLog) error
	// ListByDevice 某设备某客户端最近 limit 行日志。
	ListByDevice(ctx context.Context, deviceId, agent string, limit int) ([]*entity.ClientLog, error)
	// ListByCommand 某条命令的分步日志（按时间正序，用于 UI 命令历史展开看每一步）。
	ListByCommand(ctx context.Context, commandId string, limit int) ([]*entity.ClientLog, error)
}

type IClientConfigChangeRepository interface {
	// Insert 落一条配置变更。
	Insert(ctx context.Context, e *entity.ClientConfigChange) error
	// GetLatest 取某设备某客户端最近一条变更（比 hash 用）。
	GetLatest(ctx context.Context, deviceId, agent string) (*entity.ClientConfigChange, error)
	// GetLatestReported 取客户端最近一次「真实上报的文件快照」（比 hash 用）——跳过
	// source=app_perm 的后台保存审计行：它不是客户端文件状态，当基线会把保存后的
	// 首条上报（内容仍是旧文件）误记成一条重复的 local_change。
	GetLatestReported(ctx context.Context, deviceId, agent string) (*entity.ClientConfigChange, error)
	// GetLatestBySource 取某来源最近一条（如 app_perm 归因回溯用）。
	GetLatestBySource(ctx context.Context, deviceId, agent, source string) (*entity.ClientConfigChange, error)
	// ListByDevice 某设备全部客户端的变更历史。
	ListByDevice(ctx context.Context, deviceId string, limit int) ([]*entity.ClientConfigChange, error)
}

type IClientDeviceRepository interface {
	// Upsert 按 device_id 冲突更新：刷新机器信息/last_login/user_id，status 重置为 active，
	// 但保留 blocked（封禁不因重新登录自动解除）。
	Upsert(ctx context.Context, e *entity.ClientDevice) error
	// InsertPending 落一条 status=pending 的设备记录（App 打开登录页时即落库）。
	// 已存在（active/revoked）则不动。
	InsertPending(ctx context.Context, e *entity.ClientDevice) error
	// GetById 按 device_id 取设备。
	GetById(ctx context.Context, deviceId string) (*entity.ClientDevice, error)
	// ListAll 全部设备（按 last_login_at desc）。
	ListAll(ctx context.Context) ([]*entity.ClientDevice, error)
	// UpdateStatus 设置 status（active/revoked）。
	UpdateStatus(ctx context.Context, deviceId, status string) error
	// UpdateBlocked 设置 blocked 0/1。
	UpdateBlocked(ctx context.Context, deviceId string, blocked uint) error
	// UpdateExtra 覆盖 extra jsonb（存设备级配置，如 selectedAgents）。
	UpdateExtra(ctx context.Context, deviceId, extra string) error
	// UpdateChannels 覆盖 channels jsonb（渠道绑定状态数组：微信等）。
	UpdateChannels(ctx context.Context, deviceId, channels string) error
	// UpdateSkills 覆盖 skills jsonb（设备已装技能快照数组，心跳上报合并）。
	UpdateSkills(ctx context.Context, deviceId, skills string) error
	// UpdateMcpKeyPlain 设备专属 MCP key 明文（绑分组签发/清空；密钥库只存哈希）。
	UpdateMcpKeyPlain(ctx context.Context, deviceId, plain string) error
	// Delete 按 device_id 删除。
	Delete(ctx context.Context, deviceId string) error
}

// IClientDeviceMcpGroupRepository 设备↔MCP 分组直绑（设备侧仅直绑，无项目继承）。
type IClientDeviceMcpGroupRepository interface {
	ListByDevice(ctx context.Context, deviceId string) (list []*entity.ClientDeviceMcpGroup, err error)
	GetByDeviceGroup(ctx context.Context, deviceId, groupId string) (*entity.ClientDeviceMcpGroup, error)
	Add(ctx context.Context, e *entity.ClientDeviceMcpGroup) (string, error)
	DeleteByDeviceGroup(ctx context.Context, deviceId, groupId string) error
	DeleteByDevice(ctx context.Context, deviceId string) error
}

type IClientPackageRepository interface {
	// Upsert 按 package_id 更新或按 (agent,version,platform) 冲突插入。
	Upsert(ctx context.Context, e *entity.ClientPackage) error
	// ListAll 全部安装包（按 agent, version desc）。
	ListAll(ctx context.Context) ([]*entity.ClientPackage, error)
	// GetByCVP 按 agent+version+platform 取包。
	GetByCVP(ctx context.Context, agent, version, platform string) (*entity.ClientPackage, error)
	// ListByAgent 按 agent 取全部包。
	ListByAgent(ctx context.Context, agent string) ([]*entity.ClientPackage, error)
	// Delete 按 package_id 删除记录。
	Delete(ctx context.Context, packageId string) error
}

type IClientAppRepository interface {
	// ListAll 全部 App（按 weigh desc, app_name asc）。
	ListAll(ctx context.Context) ([]*entity.ClientApp, error)
	// GetByBundleId 按 bundle_id 取。
	GetByBundleId(ctx context.Context, bundleId string) (*entity.ClientApp, error)
	// Upsert 按 app_id 更新或按 bundle_id 冲突插入（管理员维护目录）。
	Upsert(ctx context.Context, e *entity.ClientApp) error
	// Delete 按 app_id 删除。
	Delete(ctx context.Context, appId string) error
	// CountAppPermRefs 统计某 bundle_id 被设备 App 权限配置引用的次数（删除目录时拦截用）。
	CountAppPermRefs(ctx context.Context, bundleId string) (int64, error)
}

type IClientAppPermRepository interface {
	// ListByDevice 某设备配置的全部 App 权限（按 bundle_id asc）。
	ListByDevice(ctx context.Context, deviceId string) ([]*entity.ClientAppPerm, error)
	// ReplaceDevice 整组覆盖某设备的权限清单（事务：删不在清单的行 + 新行冲突跳过；
	// 保留行的 granted/denied 是客户端上报事实，不因后台保存清零）。
	ReplaceDevice(ctx context.Context, deviceId string, bundleIds []string) error
	// UpdateStatesByDevice 客户端上报的授权状态快照落库：granted/denied 数组内的置 true，
	// 不在任一数组的双双置 false（快照语义，全量覆盖）。
	UpdateStatesByDevice(ctx context.Context, deviceId string, granted, denied []string) error
	// DeleteByDevice 清空某设备权限清单（设备删除时联动）。
	DeleteByDevice(ctx context.Context, deviceId string) error
}
