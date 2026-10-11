package repository

import (
	"context"
	"time"

	"aiarsenal/domain/entity"
)

// Agent 模块 v2 运行时/团队层仓储接口（agent 库）。
// 软删表（instance/team）Where deleted_at is null，删除=置 deleted_at；
// 追加/只读表（usage/workspace_audit/team_event/injection_snapshot 等）仅 List/Get/Add。
// Phase 1 落数据层 CRUD；容器生命周期方法（SetContainer/Drain/UpsertStatus 等）
// 声明在此供 Phase 2 DockerAdapter 使用。

// ==================== 实例/运行时 ====================

type IAgentInstanceRepository interface {
	List(ctx context.Context, userId, name string, status int, pageNum, pageSize int) (list []*entity.AgentInstance, total int64, err error)
	ListAll(ctx context.Context, name string, status int, pageNum, pageSize int) (list []*entity.AgentInstance, total int64, err error) // 管理员
	Get(ctx context.Context, id string) (*entity.AgentInstance, error)
	Add(ctx context.Context, e *entity.AgentInstance) (string, error)
	Edit(ctx context.Context, e *entity.AgentInstance) error
	Delete(ctx context.Context, ids []string) error
	UpdateStatus(ctx context.Context, instanceId string, status int, containerId, containerName string, hostPort int) error // Phase2
	UpdateStateOnly(ctx context.Context, instanceId string, status int) error                                               // 仅更状态，保留 container_id/name/host_port（Start/Stop/Restart 用）
	UpdateInstanceToken(ctx context.Context, instanceId, token string) error                                                // Phase3: 落 instance_token
	UpdateAccessUrl(ctx context.Context, instanceId, accessUrl string) error                                                // Phase3b: 桌面实例访问地址
	UpdateLLMKeyHash(ctx context.Context, instanceId, llmKeyHash string) error                                              // 实例专属 LLM key tokenHash
	UpdateMcpKeyPlain(ctx context.Context, instanceId, plain string) error                                                  // 实例专属 MCP key 明文（绑分组签发/清空；密钥库只存哈希）
	GetByLLMKeyHash(ctx context.Context, llmKeyHash string) (*entity.AgentInstance, error)                                  // 按 LLM key tokenHash 反查绑定的实例（虚拟 key 编辑/轮换后重推用）
	ListByUserAgent(ctx context.Context, userId, agent string) ([]*entity.AgentInstance, error)                             // 某用户某 agent 的全部实例（渠道推送通知定位用，优先 running）
}

type IAgentInstanceSkillRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceSkill, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentInstanceSkill, error)
	GetByInstanceSkill(ctx context.Context, instanceId, skillId string) (*entity.AgentInstanceSkill, error) // 按 (instance_id, skill_id) 反查（安装幂等 + 卸载定位）
	Add(ctx context.Context, e *entity.AgentInstanceSkill) (string, error)
	SetVersion(ctx context.Context, instanceId, skillId, versionId, version string) error // 升级/重装成功后更新版本记录
	Delete(ctx context.Context, ids []string) error
	DeleteByInstanceSkill(ctx context.Context, instanceId, skillId string) error // 卸载：按 (instance_id, skill_id) 删
}

// IAgentInstanceMcpGroupRepository 实例↔MCP 分组直绑（项目继承走 aa_project_binding，不入此表）。
type IAgentInstanceMcpGroupRepository interface {
	ListByInstance(ctx context.Context, instanceId string) (list []*entity.AgentInstanceMcpGroup, err error)
	ListByGroup(ctx context.Context, groupId string) (list []*entity.AgentInstanceMcpGroup, err error) // 反查直绑某分组的实例（分组 server 名单变化 fan-out 用）
	GetByInstanceGroup(ctx context.Context, instanceId, groupId string) (*entity.AgentInstanceMcpGroup, error) // 命中唯一索引；未命中 gorm.ErrRecordNotFound
	Add(ctx context.Context, e *entity.AgentInstanceMcpGroup) (string, error)
	DeleteByInstanceGroup(ctx context.Context, instanceId, groupId string) error // 解绑：按 (instance_id, group_id) 删
	DeleteByInstance(ctx context.Context, instanceId string) error               // 删实例清理
}

type IAgentInstanceUsageRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceUsage, total int64, err error)
	// ListRange 时间窗查询（GET /agent/usage 汇总端点用）：instanceId 为空=全部实例；
	// start/end 零值=不限定。按 recorded_at 倒序。
	ListRange(ctx context.Context, instanceId string, start, end time.Time, pageNum, pageSize int) (list []*entity.AgentInstanceUsage, total int64, err error)
	Add(ctx context.Context, e *entity.AgentInstanceUsage) (string, error)
}

type IAgentInstanceAgentRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceAgent, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentInstanceAgent, error)
	Add(ctx context.Context, e *entity.AgentInstanceAgent) (string, error)
	UpdateHeartbeat(ctx context.Context, instanceAgentId string, status int) error // Phase3
}

type IAgentInstanceRuntimeStatusRepository interface {
	GetByInstance(ctx context.Context, instanceId string) (*entity.AgentInstanceRuntimeStatus, error)
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceRuntimeStatus, total int64, err error)
	Upsert(ctx context.Context, e *entity.AgentInstanceRuntimeStatus) error              // Phase2
	UpsertFromHeartbeat(ctx context.Context, e *entity.AgentInstanceRuntimeStatus) error // Phase3: 心跳 raw SQL upsert（避开 *types.Time pgx 编码）
}

type IAgentInstanceDesiredStateRepository interface {
	GetByInstance(ctx context.Context, instanceId string) (*entity.AgentInstanceDesiredState, error)
	Upsert(ctx context.Context, e *entity.AgentInstanceDesiredState) error
}

type IAgentInstanceCommandRepository interface {
	List(ctx context.Context, instanceId string, status int, pageNum, pageSize int) (list []*entity.AgentInstanceCommand, total int64, err error)
	ListPending(ctx context.Context, instanceId string, limit int) (list []*entity.AgentInstanceCommand, err error)
	ListRunningByInstance(ctx context.Context, instanceId string) ([]*entity.AgentInstanceCommand, error) // 认领前置闸：上一条 running 未回收不下发新命令
	Get(ctx context.Context, id string) (*entity.AgentInstanceCommand, error)
	Add(ctx context.Context, e *entity.AgentInstanceCommand) (string, error)
	UpdateStatus(ctx context.Context, instanceCommandId string, status int, resultJson, errMsg string) error
	// GetLastConfigPushPayload 取某实例最近一条已完成的 config_push 命令的 payload_json（含完整 agent 运行配置+ak）。
	// 模型下线后读旧配置改 models 重推用。无则返回 ""。
	GetLastConfigPushPayload(ctx context.Context, instanceId string) (string, error)
	ClaimNext(ctx context.Context, instanceId string) (*entity.AgentInstanceCommand, error)                  // Phase3: 原子 pending→running
	UpdateResult(ctx context.Context, instanceCommandId string, status int, resultJson, errMsg string) error // Phase3: 回传结果 + finished_at
	UpdateProgress(ctx context.Context, instanceCommandId string, progressJson string) error                 // Phase3: 命令执行中进度（interim）
}

// IAgentInstanceRuntimeStatusHistoryRepository 实例运行时状态历史（时间线追加，签名去重）。
type IAgentInstanceRuntimeStatusHistoryRepository interface {
	ListByInstance(ctx context.Context, instanceId string, limit int) (list []*entity.AgentInstanceRuntimeStatusHistory, err error)
	Append(ctx context.Context, e *entity.AgentInstanceRuntimeStatusHistory, forceInsert bool) error // 仿 desktop Upsert：同签名+20s 内合并，否则插新行
}

// IAgentInstanceConfigChangeRepository 实例配置变更历史（configHash 变化才落）。
type IAgentInstanceConfigChangeRepository interface {
	ListByInstance(ctx context.Context, instanceId string, limit int) (list []*entity.AgentInstanceConfigChange, err error)
	GetLatest(ctx context.Context, instanceId string) (*entity.AgentInstanceConfigChange, error)
	GetLatestBySourcePrefix(ctx context.Context, instanceId, sourcePrefix string) (*entity.AgentInstanceConfigChange, error)
	Insert(ctx context.Context, e *entity.AgentInstanceConfigChange) error
}

type IAgentInstanceConfigRevisionRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceConfigRevision, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentInstanceConfigRevision, error)
	Add(ctx context.Context, e *entity.AgentInstanceConfigRevision) (string, error)
	UpdateStatus(ctx context.Context, configRevisionId string, status int) error
}

type IAgentInstanceRuntimeBindingRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceRuntimeBinding, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentInstanceRuntimeBinding, error)
	Add(ctx context.Context, e *entity.AgentInstanceRuntimeBinding) (string, error)
	Update(ctx context.Context, e *entity.AgentInstanceRuntimeBinding) error
}

type IAgentInstanceExternalAccessRepository interface {
	GetByInstance(ctx context.Context, instanceId string) (*entity.AgentInstanceExternalAccess, error)
	GetBySlug(ctx context.Context, slug string) (*entity.AgentInstanceExternalAccess, error)
	Upsert(ctx context.Context, e *entity.AgentInstanceExternalAccess) error
	Delete(ctx context.Context, instanceId string) error
}

type IAgentPersistentVolumeRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentPersistentVolume, total int64, err error)
	ListAll(ctx context.Context) ([]*entity.AgentPersistentVolume, error)                    // 全量（卷列表页对齐 docker 实时元数据）
	GetByName(ctx context.Context, volumeName string) (*entity.AgentPersistentVolume, error) // docker 卷名反查（卷名唯一索引）
	Get(ctx context.Context, id string) (*entity.AgentPersistentVolume, error)
	Add(ctx context.Context, e *entity.AgentPersistentVolume) (string, error)
	Update(ctx context.Context, e *entity.AgentPersistentVolume) error
	// UpdateFields 显式字段更新（gorm 结构体 Updates 跳零值，解绑 instance_id=''/status=0 要能生效）。
	UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentBackupRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentBackup, total int64, err error)
	ListAll(ctx context.Context) ([]*entity.AgentBackup, error) // 全量（管理端不按实例过滤；备份量级小，不分页）
	Get(ctx context.Context, id string) (*entity.AgentBackup, error)
	Add(ctx context.Context, e *entity.AgentBackup) (string, error)
	Update(ctx context.Context, e *entity.AgentBackup) error
	Delete(ctx context.Context, ids []string) error
	// ListExpired 过期备份（expires_at < now，保留期清理用），供定时清理物理文件。
	ListExpired(ctx context.Context, now time.Time, limit int) ([]*entity.AgentBackup, error)
}

type IAgentBackupScheduleRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentBackupSchedule, total int64, err error)
	ListAll(ctx context.Context) ([]*entity.AgentBackupSchedule, error) // 全量（启动时逐条注册定时任务）
	Get(ctx context.Context, id string) (*entity.AgentBackupSchedule, error)
	Add(ctx context.Context, e *entity.AgentBackupSchedule) (string, error)
	Edit(ctx context.Context, e *entity.AgentBackupSchedule) error
	// EditScheduleFields 显式字段更新（gorm 结构体 Updates 跳零值，isActive=0 要能生效）。
	EditScheduleFields(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentRuntimePodRepository interface {
	List(ctx context.Context, runtimeType string, state int, pageNum, pageSize int) (list []*entity.AgentRuntimePod, total int64, err error)
	ListByRollout(ctx context.Context, rolloutId string, pageNum, pageSize int) (list []*entity.AgentRuntimePod, total int64, err error) // 发布单的 pod 批次
	GetByPodName(ctx context.Context, podName string) (*entity.AgentRuntimePod, error)                                                   // pod_name 反查（localdocker: aiarsenal-<instanceId>）
	Get(ctx context.Context, id string) (*entity.AgentRuntimePod, error)
	Add(ctx context.Context, e *entity.AgentRuntimePod) (string, error)
	Update(ctx context.Context, e *entity.AgentRuntimePod) error
	Drain(ctx context.Context, runtimePodId string) error // Phase2
}

type IAgentRuntimeRolloutRepository interface {
	List(ctx context.Context, status int, pageNum, pageSize int) (list []*entity.AgentRuntimeRollout, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentRuntimeRollout, error)
	Add(ctx context.Context, e *entity.AgentRuntimeRollout) (string, error)
	Update(ctx context.Context, e *entity.AgentRuntimeRollout) error
}

type IAgentWorkspaceFileAuditRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentWorkspaceFileAudit, total int64, err error)
	Add(ctx context.Context, e *entity.AgentWorkspaceFileAudit) (string, error)
}

type IAgentOpenclawInjectionSnapshotRepository interface {
	List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentOpenclawInjectionSnapshot, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentOpenclawInjectionSnapshot, error)
	Add(ctx context.Context, e *entity.AgentOpenclawInjectionSnapshot) (string, error)
}

type IAgentSystemImageSettingRepository interface {
	List(ctx context.Context, agent, version string) (list []*entity.AgentSystemImageSetting, err error)
	Upsert(ctx context.Context, e *entity.AgentSystemImageSetting) error
	Delete(ctx context.Context, agent, version string) error
}

// ==================== Team ====================

type IAgentTeamRepository interface {
	List(ctx context.Context, userId, name string, pageNum, pageSize int) (list []*entity.AgentTeam, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentTeam, error)
	Add(ctx context.Context, e *entity.AgentTeam) (string, error)
	Edit(ctx context.Context, e *entity.AgentTeam) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentTeamMemberRepository interface {
	List(ctx context.Context, teamId string, pageNum, pageSize int) (list []*entity.AgentTeamMember, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentTeamMember, error)
	Add(ctx context.Context, e *entity.AgentTeamMember) (string, error)
	Edit(ctx context.Context, e *entity.AgentTeamMember) error
	Delete(ctx context.Context, ids []string) error
}

type IAgentTeamTaskRepository interface {
	List(ctx context.Context, teamId string, status int, pageNum, pageSize int) (list []*entity.AgentTeamTask, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentTeamTask, error)
	Add(ctx context.Context, e *entity.AgentTeamTask) (string, error) // dispatch
	Update(ctx context.Context, e *entity.AgentTeamTask) error
}

type IAgentTeamEventRepository interface {
	List(ctx context.Context, teamId string, pageNum, pageSize int) (list []*entity.AgentTeamEvent, total int64, err error)
	Add(ctx context.Context, e *entity.AgentTeamEvent) (string, error)
}

type IAgentTeamWorkItemRepository interface {
	List(ctx context.Context, rootTaskId string, status int, pageNum, pageSize int) (list []*entity.AgentTeamWorkItem, total int64, err error)
	Get(ctx context.Context, id string) (*entity.AgentTeamWorkItem, error)
	Add(ctx context.Context, e *entity.AgentTeamWorkItem) (string, error)
	Edit(ctx context.Context, e *entity.AgentTeamWorkItem) error
	Delete(ctx context.Context, ids []string) error
}
