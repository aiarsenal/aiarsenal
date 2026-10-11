package entity

import "aiarsenal/domain/types"

// Agent 模块 v2 运行时/实例/团队层实体（agent 库）。
// 约定与 v1 一致：Id uint bigserial PK（内部）+ <实体>_Id string biz_id（对外）；
// jsonb 列用 string + gorm type:jsonb；时间 *types.Time；smallint→uint/int；bigint→int64。
// 软删表（instance/team）含 DeletedAt；其余硬删或仅追加。
// 来源：instances/runtime_pool/team 等表翻译。

// ==================== 实例/运行时 ====================

// AgentInstance Agent 运行实例（Phase 2 由 DockerAdapter 真起容器）。
type AgentInstance struct {
	Id                       uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InstanceId               string      `gorm:"column:instance_id;uniqueIndex" json:"instanceId"`
	UserId                   string      `gorm:"column:user_id" json:"userId"`
	Name                     string      `gorm:"column:name" json:"name"`
	Description              string      `gorm:"column:description" json:"description"`
	Agent                    string      `gorm:"column:agent" json:"agent"`
	Version                  string      `gorm:"column:version" json:"version"`
	Status                   uint        `gorm:"column:status" json:"status"`
	CpuCores                 int         `gorm:"column:cpu_cores" json:"cpuCores"`
	MemoryGb                 int         `gorm:"column:memory_gb" json:"memoryGb"`
	DiskGb                   int         `gorm:"column:disk_gb" json:"diskGb"`
	GpuEnabled               uint        `gorm:"column:gpu_enabled" json:"gpuEnabled"`
	GpuType                  string      `gorm:"column:gpu_type" json:"gpuType"`
	GpuCount                 int         `gorm:"column:gpu_count" json:"gpuCount"`
	OsType                   string      `gorm:"column:os_type" json:"osType"`
	OsVersion                string      `gorm:"column:os_version" json:"osVersion"`
	ImageRegistry            string      `gorm:"column:image_registry" json:"imageRegistry"`
	ImageTag                 string      `gorm:"column:image_tag" json:"imageTag"`
	EnvironmentOverridesJson string      `gorm:"column:environment_overrides_json;type:jsonb" json:"environmentOverridesJson"`
	StorageClass             string      `gorm:"column:storage_class" json:"storageClass"`
	MountPath                string      `gorm:"column:mount_path" json:"mountPath"`
	ContainerId              string      `gorm:"column:container_id" json:"containerId"`
	ContainerName            string      `gorm:"column:container_name" json:"containerName"`
	HostPort                 int         `gorm:"column:host_port" json:"hostPort"`
	AccessUrl                string      `gorm:"column:access_url" json:"accessUrl"`
	AccessToken              string      `gorm:"column:access_token" json:"accessToken"`
	InstanceToken            string      `gorm:"column:instance_token" json:"-"`        // Phase3: 容器内 agent 控制面 JWT，不回显
	LLMKeyHash               string      `gorm:"column:llm_key_hash" json:"llmKeyHash"` // 实例专属 LLM 虚拟密钥 tokenHash（agent 接 LLM 网关用）
	McpKeyPlain              string      `gorm:"column:mcp_key_plain" json:"-"`         // 实例专属 MCP Bearer key 明文（绑分组时签发；配置重推随时要用，密钥库只存哈希故落实例行，永不回显）
	StartedAt                *types.Time `gorm:"column:started_at" json:"startedAt"`
	StoppedAt                *types.Time `gorm:"column:stopped_at" json:"stoppedAt"`
	CreatedBy                string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy                string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt                *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt                *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt                *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentInstance) TableName() string { return "aa_agent_instance" }

// AgentInstanceSkill 实例↔Skill 关联。
type AgentInstanceSkill struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InstanceSkillId string      `gorm:"column:instance_skill_id;uniqueIndex" json:"instanceSkillId"`
	InstanceId      string      `gorm:"column:instance_id" json:"instanceId"`
	SkillId         string      `gorm:"column:skill_id" json:"skillId"`
	VersionId       string      `gorm:"column:version_id" json:"versionId"` // 实际装上的版本（install_skill done 时从 payload 落）；历史行为空=未知
	Version         string      `gorm:"column:version" json:"version"`      // 版本号字符串（展示用；版本是自由文本，仅做 ≠ 比较不猜大小）
	Enabled         uint        `gorm:"column:enabled" json:"enabled"`
	SortOrder       int         `gorm:"column:sort_order" json:"sortOrder"`
	CreatedAt       *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceSkill) TableName() string { return "aa_agent_instance_skill" }

// AgentInstanceMcpGroup 实例↔MCP 分组关联（绑分组→自动签收紧密钥+注入 openclaw mcp.servers）。
type AgentInstanceMcpGroup struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InstanceMcpId string      `gorm:"column:instance_mcp_id;uniqueIndex" json:"instanceMcpId"`
	InstanceId    string      `gorm:"column:instance_id" json:"instanceId"`
	GroupId       string      `gorm:"column:group_id" json:"groupId"`     // → aa_mcp_group.group_id
	GroupName     string      `gorm:"column:group_name" json:"groupName"` // 缓存展示名（网关 scope 即组名）
	CreatedBy     string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt     *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceMcpGroup) TableName() string { return "aa_agent_instance_mcp_group" }

// AgentInstanceUsage 实例资源用量（定时上报，仅追加）。
// v2 扩展：runtime /report 端点补充模型用量四列（model/prompt_tokens/
// completion_tokens/cost）与 logs_json（本批上报的运行日志聚合，挂在
// 批量首行，避免逐行复制）；资源类列照旧。
type AgentInstanceUsage struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InstanceUsageId  string      `gorm:"column:instance_usage_id;uniqueIndex" json:"instanceUsageId"`
	InstanceId       string      `gorm:"column:instance_id" json:"instanceId"`
	CpuUsagePercent  float64     `gorm:"column:cpu_usage_percent" json:"cpuUsagePercent"`
	MemoryUsageGb    float64     `gorm:"column:memory_usage_gb" json:"memoryUsageGb"`
	DiskUsageGb      float64     `gorm:"column:disk_usage_gb" json:"diskUsageGb"`
	GpuUsagePercent  float64     `gorm:"column:gpu_usage_percent" json:"gpuUsagePercent"`
	UptimeSeconds    int         `gorm:"column:uptime_seconds" json:"uptimeSeconds"`
	Model            string      `gorm:"column:model" json:"model"`
	PromptTokens     int64       `gorm:"column:prompt_tokens" json:"promptTokens"`
	CompletionTokens int64       `gorm:"column:completion_tokens" json:"completionTokens"`
	Cost             float64     `gorm:"column:cost" json:"cost"`
	LogsJson         string      `gorm:"column:logs_json;type:jsonb" json:"logsJson"`
	RecordedAt       *types.Time `gorm:"column:recorded_at" json:"recordedAt"`
}

func (AgentInstanceUsage) TableName() string { return "aa_agent_instance_usage" }

// AgentInstanceAgent 实例内 Agent 控制面注册。
type AgentInstanceAgent struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InstanceAgentId  string      `gorm:"column:instance_agent_id;uniqueIndex" json:"instanceAgentId"`
	InstanceId       string      `gorm:"column:instance_id" json:"instanceId"`
	AgentId          string      `gorm:"column:agent_id" json:"agentId"`
	AgentVersion     string      `gorm:"column:agent_version" json:"agentVersion"`
	ProtocolVersion  string      `gorm:"column:protocol_version" json:"protocolVersion"`
	Status           uint        `gorm:"column:status" json:"status"`
	CapabilitiesJson string      `gorm:"column:capabilities_json;type:jsonb" json:"capabilitiesJson"`
	HostInfoJson     string      `gorm:"column:host_info_json;type:jsonb" json:"hostInfoJson"`
	SessionToken     string      `gorm:"column:session_token" json:"sessionToken"`
	SessionExpiresAt *types.Time `gorm:"column:session_expires_at" json:"sessionExpiresAt"`
	LastHeartbeatAt  *types.Time `gorm:"column:last_heartbeat_at" json:"lastHeartbeatAt"`
	LastReportedAt   *types.Time `gorm:"column:last_reported_at" json:"lastReportedAt"`
	LastSeenIp       string      `gorm:"column:last_seen_ip" json:"lastSeenIp"`
	RegisteredAt     *types.Time `gorm:"column:registered_at" json:"registeredAt"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceAgent) TableName() string { return "aa_agent_instance_agent" }

// AgentInstanceRuntimeStatus 实例运行时状态聚合。
type AgentInstanceRuntimeStatus struct {
	Id                      uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InstanceRuntimeStatusId string      `gorm:"column:instance_runtime_status_id;uniqueIndex" json:"instanceRuntimeStatusId"`
	InstanceId              string      `gorm:"column:instance_id;uniqueIndex" json:"instanceId"`
	InfraStatus             uint        `gorm:"column:infra_status" json:"infraStatus"`
	AgentStatus             uint        `gorm:"column:agent_status" json:"agentStatus"`
	AgentProcStatus         uint        `gorm:"column:agent_proc_status" json:"agentProcStatus"`
	AgentProcPid            string      `gorm:"column:agent_proc_pid" json:"agentProcPid"`
	AgentProcVersion        string      `gorm:"column:agent_proc_version" json:"agentProcVersion"`
	CurrentConfigRevisionId string      `gorm:"column:current_config_revision_id" json:"currentConfigRevisionId"`
	DesiredConfigRevisionId string      `gorm:"column:desired_config_revision_id" json:"desiredConfigRevisionId"`
	SummaryJson             string      `gorm:"column:summary_json;type:jsonb" json:"summaryJson"`
	SystemInfoJson          string      `gorm:"column:system_info_json;type:jsonb" json:"systemInfoJson"`
	HealthJson              string      `gorm:"column:health_json;type:jsonb" json:"healthJson"`
	LastReportedAt          *types.Time `gorm:"column:last_reported_at" json:"lastReportedAt"`
	CreatedAt               *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt               *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceRuntimeStatus) TableName() string { return "aa_agent_instance_runtime_status" }

// AgentInstanceDesiredState 实例期望状态。
type AgentInstanceDesiredState struct {
	Id                      uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InstanceDesiredStateId  string      `gorm:"column:instance_desired_state_id;uniqueIndex" json:"instanceDesiredStateId"`
	InstanceId              string      `gorm:"column:instance_id;uniqueIndex" json:"instanceId"`
	DesiredPowerState       string      `gorm:"column:desired_power_state" json:"desiredPowerState"`
	DesiredConfigRevisionId string      `gorm:"column:desired_config_revision_id" json:"desiredConfigRevisionId"`
	DesiredRuntimeAction    string      `gorm:"column:desired_runtime_action" json:"desiredRuntimeAction"`
	UpdatedBy               string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt               *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt               *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceDesiredState) TableName() string { return "aa_agent_instance_desired_state" }

// AgentInstanceCommand 实例命令（控制面下发）。
type AgentInstanceCommand struct {
	Id                uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InstanceCommandId string      `gorm:"column:instance_command_id;uniqueIndex" json:"instanceCommandId"`
	InstanceId        string      `gorm:"column:instance_id" json:"instanceId"`
	AgentId           string      `gorm:"column:agent_id" json:"agentId"`
	CommandType       string      `gorm:"column:command_type" json:"commandType"`
	PayloadJson       string      `gorm:"column:payload_json;type:jsonb" json:"payloadJson"`
	Status            uint        `gorm:"column:status" json:"status"`
	IdempotencyKey    string      `gorm:"column:idempotency_key" json:"idempotencyKey"`
	IssuedBy          string      `gorm:"column:issued_by" json:"issuedBy"`
	IssuedByName      string      `gorm:"-" json:"issuedByName"`  // 仅响应回填（ListCommands 内存富化 aa_user.name），非持久列
	IssuedByEmpNo     string      `gorm:"-" json:"issuedByEmpNo"` // 仅响应回填（同上 aa_user.employee_no）
	TimeoutSeconds    int         `gorm:"column:timeout_seconds" json:"timeoutSeconds"`
	ResultJson        string      `gorm:"column:result_json;type:jsonb" json:"resultJson"`
	ProgressJson      string      `gorm:"column:progress_json;type:jsonb" json:"progressJson"` // 执行进度（interim：step/percent/lastLine）
	ErrorMessage      string      `gorm:"column:error_message" json:"errorMessage"`
	DispatchedAt      *types.Time `gorm:"column:dispatched_at" json:"dispatchedAt"`
	StartedAt         *types.Time `gorm:"column:started_at" json:"startedAt"`
	LastProgressAt    *types.Time `gorm:"column:last_progress_at" json:"lastProgressAt"` // idle 超时基准：进度最近变化时间（认领时=started_at，UpdateProgress 检测到变化才刷新）
	FinishedAt        *types.Time `gorm:"column:finished_at" json:"finishedAt"`
	IssuedAt          *types.Time `gorm:"column:issued_at" json:"issuedAt"`
	CreatedAt         *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceCommand) TableName() string { return "aa_agent_instance_command" }

// AgentInstanceRuntimeStatusHistory 实例运行时状态历史（时间线追加，仿 desktop client_status）。
// 每次心跳一行，按签名去重（同状态连续→合并刷新 reported_at；状态变化/命令过程→插新行）。
type AgentInstanceRuntimeStatusHistory struct {
	Id                uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	HistoryId         string      `gorm:"column:history_id;uniqueIndex" json:"historyId"`
	InstanceId        string      `gorm:"column:instance_id" json:"instanceId"`
	InfraStatus       uint        `gorm:"column:infra_status" json:"infraStatus"`
	AgentStatus       uint        `gorm:"column:agent_status" json:"agentStatus"`
	AgentProcStatus   uint        `gorm:"column:agent_proc_status" json:"agentProcStatus"`
	AgentProcPid      string      `gorm:"column:agent_proc_pid" json:"agentProcPid"`
	AgentProcVersion  string      `gorm:"column:agent_proc_version" json:"agentProcVersion"`
	ConfigHash        string      `gorm:"column:config_hash" json:"configHash"`
	RunningCommandSig string      `gorm:"column:running_command_sig" json:"runningCommandSig"` // 命令签名 action:status，区分命令过程
	SummaryJson       string      `gorm:"column:summary_json;type:jsonb" json:"summaryJson"`
	ReportedAt        *types.Time `gorm:"column:reported_at" json:"reportedAt"`
	CreatedAt         *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AgentInstanceRuntimeStatusHistory) TableName() string {
	return "aa_agent_instance_runtime_status_history"
}

// AgentInstanceConfigChange 实例配置变更历史（下发 + 运行时上报，按 agent 区分）。
type AgentInstanceConfigChange struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ChangeId        string      `gorm:"column:change_id;uniqueIndex" json:"changeId"`
	InstanceId      string      `gorm:"column:instance_id" json:"instanceId"`
	AgentId         string      `gorm:"column:agent_id" json:"agentId"`
	Source          string      `gorm:"column:source" json:"source"`         // dispatch（下发）/ runtime（运行时上报）
	Dispatcher      string      `gorm:"column:dispatcher" json:"dispatcher"` // user_id（下发人）
	DispatcherName  string      `gorm:"-" json:"dispatcherName"`             // 仅响应回填（ListConfigChanges 内存富化 aa_user.name），非持久列
	DispatcherEmpNo string      `gorm:"-" json:"dispatcherEmpNo"`            // 仅响应回填（同上 aa_user.employee_no）
	ConfigHash      string      `gorm:"column:config_hash" json:"configHash"`
	ConfigSize      int64       `gorm:"column:config_size" json:"configSize"`
	ConfigMtime     *types.Time `gorm:"column:config_mtime" json:"configMtime"`
	Content         string      `gorm:"column:content" json:"content"`
	ChangedAt       *types.Time `gorm:"column:changed_at" json:"changedAt"`
}

func (AgentInstanceConfigChange) TableName() string { return "aa_agent_instance_config_change" }

// AgentInstanceConfigRevision 实例配置版本。
type AgentInstanceConfigRevision struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ConfigRevisionId string      `gorm:"column:config_revision_id;uniqueIndex" json:"configRevisionId"`
	InstanceId       string      `gorm:"column:instance_id" json:"instanceId"`
	SourceSnapshotId string      `gorm:"column:source_snapshot_id" json:"sourceSnapshotId"`
	SourceBundleId   string      `gorm:"column:source_bundle_id" json:"sourceBundleId"`
	RevisionNo       int         `gorm:"column:revision_no" json:"revisionNo"`
	ContentJson      string      `gorm:"column:content_json;type:jsonb" json:"contentJson"`
	Checksum         string      `gorm:"column:checksum" json:"checksum"`
	Status           uint        `gorm:"column:status" json:"status"`
	PublishedBy      string      `gorm:"column:published_by" json:"publishedBy"`
	PublishedAt      *types.Time `gorm:"column:published_at" json:"publishedAt"`
	ActivatedAt      *types.Time `gorm:"column:activated_at" json:"activatedAt"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceConfigRevision) TableName() string { return "aa_agent_instance_config_revision" }

// AgentInstanceRuntimeBinding 实例运行时绑定。
type AgentInstanceRuntimeBinding struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RuntimeBindingId string      `gorm:"column:runtime_binding_id;uniqueIndex" json:"runtimeBindingId"`
	InstanceId       string      `gorm:"column:instance_id" json:"instanceId"`
	RuntimePodId     string      `gorm:"column:runtime_pod_id" json:"runtimePodId"`
	RuntimeType      string      `gorm:"column:runtime_type" json:"runtimeType"`
	GatewayId        string      `gorm:"column:gateway_id" json:"gatewayId"`
	GatewayPort      int         `gorm:"column:gateway_port" json:"gatewayPort"`
	GatewayPid       string      `gorm:"column:gateway_pid" json:"gatewayPid"`
	WorkspacePath    string      `gorm:"column:workspace_path" json:"workspacePath"`
	State            uint        `gorm:"column:state" json:"state"`
	Generation       int         `gorm:"column:generation" json:"generation"`
	LastHealthAt     *types.Time `gorm:"column:last_health_at" json:"lastHealthAt"`
	ErrorMessage     string      `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceRuntimeBinding) TableName() string { return "aa_agent_instance_runtime_binding" }

// AgentInstanceExternalAccess 实例外部访问。
type AgentInstanceExternalAccess struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ExternalAccessId string      `gorm:"column:external_access_id;uniqueIndex" json:"externalAccessId"`
	InstanceId       string      `gorm:"column:instance_id;uniqueIndex" json:"instanceId"`
	Enabled          uint        `gorm:"column:enabled" json:"enabled"`
	AuthMode         string      `gorm:"column:auth_mode" json:"authMode"`
	PublicSlug       string      `gorm:"column:public_slug" json:"publicSlug"`
	PublicTokenHash  string      `gorm:"column:public_token_hash" json:"publicTokenHash"`
	ShortCodeHash    string      `gorm:"column:short_code_hash" json:"shortCodeHash"`
	ApiKeyHash       string      `gorm:"column:api_key_hash" json:"apiKeyHash"`
	PasswordValue    string      `gorm:"column:password_value" json:"passwordValue"`
	ApiKeyPrefix     string      `gorm:"column:api_key_prefix" json:"apiKeyPrefix"`
	ExpiresAt        *types.Time `gorm:"column:expires_at" json:"expiresAt"`
	CreatedBy        string      `gorm:"column:created_by" json:"createdBy"`
	LastUsedAt       *types.Time `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentInstanceExternalAccess) TableName() string { return "aa_agent_instance_external_access" }

// AgentPersistentVolume 持久卷（docker named volume 等价）。
type AgentPersistentVolume struct {
	Id                 uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PersistentVolumeId string      `gorm:"column:persistent_volume_id;uniqueIndex" json:"persistentVolumeId"`
	InstanceId         string      `gorm:"column:instance_id" json:"instanceId"`
	VolumeName         string      `gorm:"column:volume_name" json:"volumeName"`
	StorageSizeGb      int         `gorm:"column:storage_size_gb" json:"storageSizeGb"`
	StorageClass       string      `gorm:"column:storage_class" json:"storageClass"`
	MountPath          string      `gorm:"column:mount_path" json:"mountPath"`
	Status             uint        `gorm:"column:status" json:"status"`
	CreatedAt          *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentPersistentVolume) TableName() string { return "aa_agent_persistent_volume" }

// AgentBackup 备份。
type AgentBackup struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BackupId     string      `gorm:"column:backup_id;uniqueIndex" json:"backupId"`
	InstanceId   string      `gorm:"column:instance_id" json:"instanceId"`
	BackupName   string      `gorm:"column:backup_name" json:"backupName"`
	BackupSizeGb int         `gorm:"column:backup_size_gb" json:"backupSizeGb"`
	BackupPath   string      `gorm:"column:backup_path" json:"backupPath"`
	Status       uint        `gorm:"column:status" json:"status"`
	BackupType   uint        `gorm:"column:backup_type" json:"backupType"`
	CompletedAt  *types.Time `gorm:"column:completed_at" json:"completedAt"`
	ExpiresAt    *types.Time `gorm:"column:expires_at" json:"expiresAt"`
	CreatedBy    string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentBackup) TableName() string { return "aa_agent_backup" }

// AgentBackupSchedule 备份计划。
type AgentBackupSchedule struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	BackupScheduleId string      `gorm:"column:backup_schedule_id;uniqueIndex" json:"backupScheduleId"`
	InstanceId       string      `gorm:"column:instance_id" json:"instanceId"`
	ScheduleName     string      `gorm:"column:schedule_name" json:"scheduleName"`
	CronExpression   string      `gorm:"column:cron_expression" json:"cronExpression"`
	RetentionDays    int         `gorm:"column:retention_days" json:"retentionDays"`
	IsActive         uint        `gorm:"column:is_active" json:"isActive"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentBackupSchedule) TableName() string { return "aa_agent_backup_schedule" }

// AgentRuntimePod 运行时容器池（docker container 等价）。
// v2 扩展：rollout_id 关联所属滚动发布单（localdocker 场景 pod 行按
// pod_name = aiarsenal-<instanceId> 与实例一一对应）。
type AgentRuntimePod struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RuntimePodId    string      `gorm:"column:runtime_pod_id;uniqueIndex" json:"runtimePodId"`
	RolloutId       string      `gorm:"column:rollout_id" json:"rolloutId"` // → aa_agent_runtime_rollout.runtime_rollout_id
	RuntimeType     string      `gorm:"column:runtime_type" json:"runtimeType"`
	Namespace       string      `gorm:"column:namespace" json:"namespace"`
	PodName         string      `gorm:"column:pod_name" json:"podName"`
	PodUid          string      `gorm:"column:pod_uid" json:"podUid"`
	PodIp           string      `gorm:"column:pod_ip" json:"podIp"`
	NodeName        string      `gorm:"column:node_name" json:"nodeName"`
	DeploymentName  string      `gorm:"column:deployment_name" json:"deploymentName"`
	ImageRef        string      `gorm:"column:image_ref" json:"imageRef"`
	AgentEndpoint   string      `gorm:"column:agent_endpoint" json:"agentEndpoint"`
	State           uint        `gorm:"column:state" json:"state"`
	Capacity        int         `gorm:"column:capacity" json:"capacity"`
	UsedSlots       int         `gorm:"column:used_slots" json:"usedSlots"`
	Draining        uint        `gorm:"column:draining" json:"draining"`
	CpuMillisUsed   int64       `gorm:"column:cpu_millis_used" json:"cpuMillisUsed"`
	MemoryBytesUsed int64       `gorm:"column:memory_bytes_used" json:"memoryBytesUsed"`
	DiskBytesUsed   int64       `gorm:"column:disk_bytes_used" json:"diskBytesUsed"`
	NetworkRxBytes  int64       `gorm:"column:network_rx_bytes" json:"networkRxBytes"`
	NetworkTxBytes  int64       `gorm:"column:network_tx_bytes" json:"networkTxBytes"`
	MetricsJson     string      `gorm:"column:metrics_json;type:jsonb" json:"metricsJson"`
	LastSeenAt      *types.Time `gorm:"column:last_seen_at" json:"lastSeenAt"`
	CreatedAt       *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentRuntimePod) TableName() string { return "aa_agent_runtime_pod" }

// AgentRuntimeRollout 运行时滚动发布。
type AgentRuntimeRollout struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RuntimeRolloutId string      `gorm:"column:runtime_rollout_id;uniqueIndex" json:"runtimeRolloutId"`
	RuntimeType      string      `gorm:"column:runtime_type" json:"runtimeType"`
	TargetImageRef   string      `gorm:"column:target_image_ref" json:"targetImageRef"`
	Status           uint        `gorm:"column:status" json:"status"`
	BatchSize        int         `gorm:"column:batch_size" json:"batchSize"`
	MaxUnavailable   int         `gorm:"column:max_unavailable" json:"maxUnavailable"`
	StartedBy        string      `gorm:"column:started_by" json:"startedBy"`
	ErrorMessage     string      `gorm:"column:error_message" json:"errorMessage"`
	StartedAt        *types.Time `gorm:"column:started_at" json:"startedAt"`
	FinishedAt       *types.Time `gorm:"column:finished_at" json:"finishedAt"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentRuntimeRollout) TableName() string { return "aa_agent_runtime_rollout" }

// AgentWorkspaceFileAudit 工作区文件操作审计（仅追加）。
type AgentWorkspaceFileAudit struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	WorkspaceAuditId string      `gorm:"column:workspace_audit_id;uniqueIndex" json:"workspaceAuditId"`
	InstanceId       string      `gorm:"column:instance_id" json:"instanceId"`
	UserId           string      `gorm:"column:user_id" json:"userId"`
	Action           string      `gorm:"column:action" json:"action"`
	RelativePath     string      `gorm:"column:relative_path" json:"relativePath"`
	Bytes            int64       `gorm:"column:bytes" json:"bytes"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AgentWorkspaceFileAudit) TableName() string { return "aa_agent_workspace_file_audit" }

// AgentOpenclawInjectionSnapshot OpenClaw 注入快照。
type AgentOpenclawInjectionSnapshot struct {
	Id                  uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InjectionSnapshotId string      `gorm:"column:injection_snapshot_id;uniqueIndex" json:"injectionSnapshotId"`
	InstanceId          string      `gorm:"column:instance_id" json:"instanceId"`
	ConfigRevisionId    string      `gorm:"column:config_revision_id" json:"configRevisionId"`
	BundleId            string      `gorm:"column:bundle_id" json:"bundleId"`
	CompiledJson        string      `gorm:"column:compiled_json;type:jsonb" json:"compiledJson"`
	Checksum            string      `gorm:"column:checksum" json:"checksum"`
	InjectedAt          *types.Time `gorm:"column:injected_at" json:"injectedAt"`
	CreatedAt           *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AgentOpenclawInjectionSnapshot) TableName() string {
	return "aa_agent_openclaw_injection_snapshot"
}

// AgentSystemImageSetting 系统镜像设置（admin 镜像卡片）。
type AgentSystemImageSetting struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	SystemImageId string      `gorm:"column:system_image_id;uniqueIndex" json:"systemImageId"`
	Agent         string      `gorm:"column:agent" json:"agent"`
	Version       string      `gorm:"column:version" json:"version"`
	DisplayName   string      `gorm:"column:display_name" json:"displayName"`
	Image         string      `gorm:"column:image" json:"image"`
	IsEnabled     uint        `gorm:"column:is_enabled" json:"isEnabled"`
	UpdatedBy     string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt     *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentSystemImageSetting) TableName() string { return "aa_agent_system_image_setting" }

// ==================== Team（多 agent 协作） ====================

// AgentTeam 多 agent 协作组。
type AgentTeam struct {
	Id                uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TeamId            string      `gorm:"column:team_id;uniqueIndex" json:"teamId"`
	UserId            string      `gorm:"column:user_id" json:"userId"`
	Name              string      `gorm:"column:name" json:"name"`
	Description       string      `gorm:"column:description" json:"description"`
	Status            uint        `gorm:"column:status" json:"status"`
	CommunicationMode string      `gorm:"column:communication_mode" json:"communicationMode"`
	RedisUrl          string      `gorm:"column:redis_url" json:"redisUrl"`
	TeamToken         string      `gorm:"column:team_token" json:"teamToken"`
	RedisEventsLastId string      `gorm:"column:redis_events_last_id" json:"redisEventsLastId"`
	SharedVolumeName  string      `gorm:"column:shared_volume_name" json:"sharedVolumeName"`
	SharedMountPath   string      `gorm:"column:shared_mount_path" json:"sharedMountPath"`
	StorageClass      string      `gorm:"column:storage_class" json:"storageClass"`
	CreatedBy         string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt         *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (AgentTeam) TableName() string { return "aa_agent_team" }

// AgentTeamMember Team 成员。
type AgentTeamMember struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TeamMemberId  string      `gorm:"column:team_member_id;uniqueIndex" json:"teamMemberId"`
	TeamId        string      `gorm:"column:team_id" json:"teamId"`
	InstanceId    string      `gorm:"column:instance_id" json:"instanceId"`
	UserId        string      `gorm:"column:user_id" json:"userId"`
	MemberKey     string      `gorm:"column:member_key" json:"memberKey"`
	DisplayName   string      `gorm:"column:display_name" json:"displayName"`
	Role          string      `gorm:"column:role" json:"role"`
	Description   string      `gorm:"column:description" json:"description"`
	Status        uint        `gorm:"column:status" json:"status"`
	CurrentTaskId string      `gorm:"column:current_task_id" json:"currentTaskId"`
	Progress      int         `gorm:"column:progress" json:"progress"`
	LastSeenAt    *types.Time `gorm:"column:last_seen_at" json:"lastSeenAt"`
	CreatedAt     *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentTeamMember) TableName() string { return "aa_agent_team_member" }

// AgentTeamTask Team 任务。
type AgentTeamTask struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TeamTaskId     string      `gorm:"column:team_task_id;uniqueIndex" json:"teamTaskId"`
	TeamId         string      `gorm:"column:team_id" json:"teamId"`
	TargetMemberId string      `gorm:"column:target_member_id" json:"targetMemberId"`
	CreatedBy      string      `gorm:"column:created_by" json:"createdBy"`
	MessageId      string      `gorm:"column:message_id" json:"messageId"`
	Status         uint        `gorm:"column:status" json:"status"`
	PayloadJson    string      `gorm:"column:payload_json;type:jsonb" json:"payloadJson"`
	ResultJson     string      `gorm:"column:result_json;type:jsonb" json:"resultJson"`
	ErrorMessage   string      `gorm:"column:error_message" json:"errorMessage"`
	RedisStreamId  string      `gorm:"column:redis_stream_id" json:"redisStreamId"`
	DispatchedAt   *types.Time `gorm:"column:dispatched_at" json:"dispatchedAt"`
	StartedAt      *types.Time `gorm:"column:started_at" json:"startedAt"`
	FinishedAt     *types.Time `gorm:"column:finished_at" json:"finishedAt"`
	CreatedAt      *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentTeamTask) TableName() string { return "aa_agent_team_task" }

// AgentTeamEvent Team 事件。
type AgentTeamEvent struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TeamEventId   string      `gorm:"column:team_event_id;uniqueIndex" json:"teamEventId"`
	EventId       string      `gorm:"column:event_id" json:"eventId"`
	CompletionId  string      `gorm:"column:completion_id" json:"completionId"`
	SequenceNo    int64       `gorm:"column:sequence_no" json:"sequenceNo"`
	TeamId        string      `gorm:"column:team_id" json:"teamId"`
	MemberId      string      `gorm:"column:member_id" json:"memberId"`
	TaskId        string      `gorm:"column:task_id" json:"taskId"`
	MessageId     string      `gorm:"column:message_id" json:"messageId"`
	EventType     string      `gorm:"column:event_type" json:"eventType"`
	PayloadJson   string      `gorm:"column:payload_json;type:jsonb" json:"payloadJson"`
	RedisStreamId string      `gorm:"column:redis_stream_id" json:"redisStreamId"`
	OccurredAt    *types.Time `gorm:"column:occurred_at" json:"occurredAt"`
	CreatedAt     *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (AgentTeamEvent) TableName() string { return "aa_agent_team_event" }

// AgentTeamWorkItem Team 工作项。
type AgentTeamWorkItem struct {
	Id               uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	WorkItemId       string      `gorm:"column:work_item_id;uniqueIndex" json:"workItemId"`
	TeamId           string      `gorm:"column:team_id" json:"teamId"`
	RootTaskId       string      `gorm:"column:root_task_id" json:"rootTaskId"`
	WorkId           string      `gorm:"column:work_id" json:"workId"`
	OwnerMemberId    string      `gorm:"column:owner_member_id" json:"ownerMemberId"`
	Title            string      `gorm:"column:title" json:"title"`
	Status           uint        `gorm:"column:status" json:"status"`
	DependsOnJson    string      `gorm:"column:depends_on_json;type:jsonb" json:"dependsOnJson"`
	ResultJson       string      `gorm:"column:result_json;type:jsonb" json:"resultJson"`
	ArtifactRefsJson string      `gorm:"column:artifact_refs_json;type:jsonb" json:"artifactRefsJson"`
	StartedAt        *types.Time `gorm:"column:started_at" json:"startedAt"`
	FinishedAt       *types.Time `gorm:"column:finished_at" json:"finishedAt"`
	CreatedAt        *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt        *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentTeamWorkItem) TableName() string { return "aa_agent_team_work_item" }
