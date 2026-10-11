package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// Agent 模块 v2 运行时/团队层仓储实现（agent 库）。统一走 db.AgentDB()。
// 写入时 idgen.New() 生成业务 id；软删表（instance/team）Where deleted_at is null，删除=置 deleted_at。

// ==================== 实例/运行时 ====================

type AgentInstanceRepository struct{ db *gorm.DB }

func NewAgentInstanceRepository() *AgentInstanceRepository {
	return &AgentInstanceRepository{db: db.AgentDB()}
}

func (r *AgentInstanceRepository) List(ctx context.Context, userId, name string, status int, pageNum, pageSize int) (list []*entity.AgentInstance, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstance{}).Where("deleted_at is null and user_id = ?", userId)
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceRepository) ListAll(ctx context.Context, name string, status int, pageNum, pageSize int) (list []*entity.AgentInstance, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstance{}).Where("deleted_at is null")
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceRepository) Get(ctx context.Context, id string) (*entity.AgentInstance, error) {
	var e entity.AgentInstance
	if err := r.db.WithContext(ctx).Where("instance_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceRepository) Add(ctx context.Context, e *entity.AgentInstance) (string, error) {
	e.InstanceId = idgen.New()
	// jsonb 列空字符串会触发解析错误，统一兜底为 {}。
	if e.EnvironmentOverridesJson == "" {
		e.EnvironmentOverridesJson = "{}"
	}
	// gorm 对自定义指针类型 *types.Time 不会自动回填 created_at/updated_at（实测全为 NULL），
	// 显式写入，否则列表「创建时间」列恒空。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.InstanceId, nil
}

func (r *AgentInstanceRepository) Edit(ctx context.Context, e *entity.AgentInstance) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstance{}).
		Where("instance_id = ?", e.InstanceId).Updates(e).Error
}

func (r *AgentInstanceRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentInstance{}).
		Where("instance_id in ?", ids).Update("deleted_at", time.Now()).Error
}

func (r *AgentInstanceRepository) UpdateStatus(ctx context.Context, instanceId string, status int, containerId, containerName string, hostPort int) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstance{}).
		Where("instance_id = ?", instanceId).
		Updates(map[string]interface{}{"status": status, "container_id": containerId, "container_name": containerName, "host_port": hostPort, "updated_at": time.Now()}).Error
}

// UpdateStateOnly 仅更新 status，保留 container_id/container_name/host_port。
// Start/Stop/Restart 用：容器的 id 与端口映射在 docker 生命周期内不变，
// 不能用 UpdateStatus 的全量覆盖（否则 host_port 被清 0 → 桌面代理判 HostPort<=0 失败）。
func (r *AgentInstanceRepository) UpdateStateOnly(ctx context.Context, instanceId string, status int) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstance{}).
		Where("instance_id = ?", instanceId).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()}).Error
}

// UpdateAccessUrl 落 access_url（桌面实例端口映射后拼访问地址）。
func (r *AgentInstanceRepository) UpdateAccessUrl(ctx context.Context, instanceId, accessUrl string) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstance{}).
		Where("instance_id = ?", instanceId).
		Updates(map[string]interface{}{"access_url": accessUrl, "updated_at": time.Now()}).Error
}

// UpdateInstanceToken 落 instance_token（创建实例后签发，注入容器 env）。
func (r *AgentInstanceRepository) UpdateInstanceToken(ctx context.Context, instanceId, token string) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstance{}).
		Where("instance_id = ?", instanceId).
		Updates(map[string]interface{}{"instance_token": token, "updated_at": time.Now()}).Error
}

// UpdateLLMKeyHash 落实例专属 LLM key 的 tokenHash（创建实例时签发密钥后记录）。
func (r *AgentInstanceRepository) UpdateLLMKeyHash(ctx context.Context, instanceId, llmKeyHash string) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstance{}).
		Where("instance_id = ?", instanceId).
		Updates(map[string]interface{}{"llm_key_hash": llmKeyHash, "updated_at": time.Now()}).Error
}

func (r *AgentInstanceRepository) UpdateMcpKeyPlain(ctx context.Context, instanceId, plain string) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstance{}).
		Where("instance_id = ?", instanceId).
		Updates(map[string]interface{}{"mcp_key_plain": plain, "updated_at": time.Now()}).Error
}

// GetByLLMKeyHash 按 LLM key tokenHash 反查绑定的实例（软删过滤）。虚拟 key 编辑/轮换后定位要重推的实例。
func (r *AgentInstanceRepository) GetByLLMKeyHash(ctx context.Context, llmKeyHash string) (*entity.AgentInstance, error) {
	var e entity.AgentInstance
	if err := r.db.WithContext(ctx).Where("llm_key_hash = ?", llmKeyHash).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ListByUserAgent 某用户某 agent 的全部实例（渠道推送通知定位用）：
// status 1=running 优先排前，其余按 id 倒序兜底（running 实例没有再退 stopped 也发，命令会等心跳拉取）。
func (r *AgentInstanceRepository) ListByUserAgent(ctx context.Context, userId, agent string) ([]*entity.AgentInstance, error) {
	var list []*entity.AgentInstance
	err := r.db.WithContext(ctx).
		Where("deleted_at is null and user_id = ? and agent = ?", userId, agent).
		Order("status = 1 desc, id desc").Find(&list).Error
	return list, err
}

// --- AgentInstanceSkill ---
type AgentInstanceSkillRepository struct{ db *gorm.DB }

func NewAgentInstanceSkillRepository() *AgentInstanceSkillRepository {
	return &AgentInstanceSkillRepository{db: db.AgentDB()}
}

func (r *AgentInstanceSkillRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceSkill, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstanceSkill{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sort_order asc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceSkillRepository) Get(ctx context.Context, id string) (*entity.AgentInstanceSkill, error) {
	var e entity.AgentInstanceSkill
	if err := r.db.WithContext(ctx).Where("instance_skill_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// GetByInstanceSkill 按 (instance_id, skill_id) 反查。命中唯一索引；未命中返回 gorm.ErrRecordNotFound。
func (r *AgentInstanceSkillRepository) GetByInstanceSkill(ctx context.Context, instanceId, skillId string) (*entity.AgentInstanceSkill, error) {
	var e entity.AgentInstanceSkill
	if err := r.db.WithContext(ctx).Where("instance_id = ? AND skill_id = ?", instanceId, skillId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceSkillRepository) Add(ctx context.Context, e *entity.AgentInstanceSkill) (string, error) {
	e.InstanceSkillId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.InstanceSkillId, nil
}

// SetVersion 升级/重装成功后更新版本记录（按 (instance_id, skill_id) 定位；无匹配不报错）。
func (r *AgentInstanceSkillRepository) SetVersion(ctx context.Context, instanceId, skillId, versionId, version string) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceSkill{}).
		Where("instance_id = ? AND skill_id = ?", instanceId, skillId).
		Updates(map[string]interface{}{"version_id": versionId, "version": version, "updated_at": time.Now()}).Error
}

func (r *AgentInstanceSkillRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("instance_skill_id in ?", ids).Delete(&entity.AgentInstanceSkill{}).Error
}

// DeleteByInstanceSkill 按 (instance_id, skill_id) 删（卸载）。无匹配不报错。
func (r *AgentInstanceSkillRepository) DeleteByInstanceSkill(ctx context.Context, instanceId, skillId string) error {
	return r.db.WithContext(ctx).Where("instance_id = ? AND skill_id = ?", instanceId, skillId).Delete(&entity.AgentInstanceSkill{}).Error
}

// --- AgentInstanceMcpGroup ---
type AgentInstanceMcpGroupRepository struct{ db *gorm.DB }

func NewAgentInstanceMcpGroupRepository() *AgentInstanceMcpGroupRepository {
	return &AgentInstanceMcpGroupRepository{db: db.AgentDB()}
}

func (r *AgentInstanceMcpGroupRepository) ListByInstance(ctx context.Context, instanceId string) (list []*entity.AgentInstanceMcpGroup, err error) {
	err = r.db.WithContext(ctx).Where("instance_id = ?", instanceId).Order("id asc").Find(&list).Error
	return
}

// ListByGroup 反查直绑某分组的全部实例（分组 server 名单变化后 fan-out 重算配对手册用）。
func (r *AgentInstanceMcpGroupRepository) ListByGroup(ctx context.Context, groupId string) (list []*entity.AgentInstanceMcpGroup, err error) {
	err = r.db.WithContext(ctx).Where("group_id = ?", groupId).Order("id asc").Find(&list).Error
	return
}

func (r *AgentInstanceMcpGroupRepository) GetByInstanceGroup(ctx context.Context, instanceId, groupId string) (*entity.AgentInstanceMcpGroup, error) {
	var e entity.AgentInstanceMcpGroup
	if err := r.db.WithContext(ctx).Where("instance_id = ? AND group_id = ?", instanceId, groupId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceMcpGroupRepository) Add(ctx context.Context, e *entity.AgentInstanceMcpGroup) (string, error) {
	e.InstanceMcpId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.InstanceMcpId, nil
}

func (r *AgentInstanceMcpGroupRepository) DeleteByInstanceGroup(ctx context.Context, instanceId, groupId string) error {
	return r.db.WithContext(ctx).Where("instance_id = ? AND group_id = ?", instanceId, groupId).Delete(&entity.AgentInstanceMcpGroup{}).Error
}

func (r *AgentInstanceMcpGroupRepository) DeleteByInstance(ctx context.Context, instanceId string) error {
	return r.db.WithContext(ctx).Where("instance_id = ?", instanceId).Delete(&entity.AgentInstanceMcpGroup{}).Error
}

// --- AgentInstanceUsage ---
type AgentInstanceUsageRepository struct{ db *gorm.DB }

func NewAgentInstanceUsageRepository() *AgentInstanceUsageRepository {
	return &AgentInstanceUsageRepository{db: db.AgentDB()}
}

func (r *AgentInstanceUsageRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceUsage, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstanceUsage{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceUsageRepository) Add(ctx context.Context, e *entity.AgentInstanceUsage) (string, error) {
	e.InstanceUsageId = idgen.New()
	now := types.Time(time.Now())
	if e.RecordedAt == nil {
		e.RecordedAt = &now
	}
	if e.LogsJson == "" {
		e.LogsJson = "{}"
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.InstanceUsageId, nil
}

// ListRange 时间窗查询（GET /agent/usage 汇总端点）：instanceId 空=全部实例，
// start/end 零值（IsZero）=不限。零值 time 传给 pgx 会编成 year-1，必须显式跳过。
func (r *AgentInstanceUsageRepository) ListRange(ctx context.Context, instanceId string, start, end time.Time, pageNum, pageSize int) (list []*entity.AgentInstanceUsage, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstanceUsage{})
	if instanceId != "" {
		q = q.Where("instance_id = ?", instanceId)
	}
	if !start.IsZero() {
		q = q.Where("recorded_at >= ?", start)
	}
	if !end.IsZero() {
		q = q.Where("recorded_at <= ?", end)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

// --- AgentInstanceAgent ---
type AgentInstanceAgentRepository struct{ db *gorm.DB }

func NewAgentInstanceAgentRepository() *AgentInstanceAgentRepository {
	return &AgentInstanceAgentRepository{db: db.AgentDB()}
}

func (r *AgentInstanceAgentRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceAgent, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstanceAgent{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceAgentRepository) Get(ctx context.Context, id string) (*entity.AgentInstanceAgent, error) {
	var e entity.AgentInstanceAgent
	if err := r.db.WithContext(ctx).Where("instance_agent_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceAgentRepository) Add(ctx context.Context, e *entity.AgentInstanceAgent) (string, error) {
	e.InstanceAgentId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.InstanceAgentId, nil
}

func (r *AgentInstanceAgentRepository) UpdateHeartbeat(ctx context.Context, instanceAgentId string, status int) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceAgent{}).
		Where("instance_agent_id = ?", instanceAgentId).
		Updates(map[string]interface{}{"status": status, "last_heartbeat_at": time.Now()}).Error
}

// --- AgentInstanceRuntimeStatus ---
type AgentInstanceRuntimeStatusRepository struct{ db *gorm.DB }

func NewAgentInstanceRuntimeStatusRepository() *AgentInstanceRuntimeStatusRepository {
	return &AgentInstanceRuntimeStatusRepository{db: db.AgentDB()}
}

func (r *AgentInstanceRuntimeStatusRepository) GetByInstance(ctx context.Context, instanceId string) (*entity.AgentInstanceRuntimeStatus, error) {
	var e entity.AgentInstanceRuntimeStatus
	if err := r.db.WithContext(ctx).Where("instance_id = ?", instanceId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceRuntimeStatusRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceRuntimeStatus, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstanceRuntimeStatus{})
	if instanceId != "" {
		q = q.Where("instance_id = ?", instanceId)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceRuntimeStatusRepository) Upsert(ctx context.Context, e *entity.AgentInstanceRuntimeStatus) error {
	var exist entity.AgentInstanceRuntimeStatus
	err := r.db.WithContext(ctx).Where("instance_id = ?", e.InstanceId).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		if e.InstanceRuntimeStatusId == "" {
			e.InstanceRuntimeStatusId = idgen.New()
		}
		return r.db.WithContext(ctx).Create(e).Error
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceRuntimeStatus{}).
		Where("instance_id = ?", e.InstanceId).Updates(e).Error
}

// UpsertFromHeartbeat 心跳上报专用 upsert（raw SQL，用 now() 填时间戳）。
// 避开 pgx 编不了 *types.Time 的问题（同 desktop reported_at 处理）。
func (r *AgentInstanceRuntimeStatusRepository) UpsertFromHeartbeat(ctx context.Context, e *entity.AgentInstanceRuntimeStatus) error {
	if e.InstanceRuntimeStatusId == "" {
		e.InstanceRuntimeStatusId = idgen.New()
	}
	sql := `INSERT INTO aa_agent_instance_runtime_status
		(instance_runtime_status_id, instance_id, infra_status, agent_status, agent_proc_status,
		 agent_proc_pid, agent_proc_version, summary_json, health_json, last_reported_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, now(), now(), now())
		ON CONFLICT (instance_id) DO UPDATE SET
		 infra_status = EXCLUDED.infra_status,
		 agent_status = EXCLUDED.agent_status,
		 agent_proc_status = EXCLUDED.agent_proc_status,
		 agent_proc_pid = EXCLUDED.agent_proc_pid,
		 agent_proc_version = EXCLUDED.agent_proc_version,
		 summary_json = EXCLUDED.summary_json,
		 health_json = EXCLUDED.health_json,
		 last_reported_at = now(),
		 updated_at = now()`
	summary := e.SummaryJson
	if summary == "" {
		summary = "{}"
	}
	health := e.HealthJson
	if health == "" {
		health = "{}"
	}
	return r.db.WithContext(ctx).Exec(sql,
		e.InstanceRuntimeStatusId, e.InstanceId, e.InfraStatus, e.AgentStatus, e.AgentProcStatus,
		e.AgentProcPid, e.AgentProcVersion, summary, health).Error
}

// --- AgentInstanceRuntimeStatusHistory（状态时间线，追加 + 签名去重，仿 desktop client_status）---
type AgentInstanceRuntimeStatusHistoryRepository struct{ db *gorm.DB }

func NewAgentInstanceRuntimeStatusHistoryRepository() *AgentInstanceRuntimeStatusHistoryRepository {
	return &AgentInstanceRuntimeStatusHistoryRepository{db: db.AgentDB()}
}

func (r *AgentInstanceRuntimeStatusHistoryRepository) ListByInstance(ctx context.Context, instanceId string, limit int) ([]*entity.AgentInstanceRuntimeStatusHistory, error) {
	var list []*entity.AgentInstanceRuntimeStatusHistory
	q := r.db.WithContext(ctx).Where("instance_id = ?", instanceId).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&list).Error
	return list, err
}

// Append 时间线追加（仿 desktop Upsert 去重，判定结构同 client.go Upsert）：同签名连续 +
// 窗口内 → 更新最新行 reported_at；否则插新行。forceInsert（有 runningCommand）强制插新行，
// 保留命令 start/过程/end 各自留痕。
// 窗口取 60s（desktop 是 20s）：manager 一次 doHeartbeat 含 agent 体检 + HTTP，慢环境下
// 实际心跳周期可达 ~26s > 20s，行距恒超窗 → 永不合并（无自愈）。60s 对齐前端 isFresh
// 在线判定：在线期间同状态持续合并，掉线超 60s 恢复插新行保留断点。
func (r *AgentInstanceRuntimeStatusHistoryRepository) Append(ctx context.Context, e *entity.AgentInstanceRuntimeStatusHistory, forceInsert bool) error {
	var rows []entity.AgentInstanceRuntimeStatusHistory
	if err := r.db.WithContext(ctx).Where("instance_id = ?", e.InstanceId).Order("id desc").Limit(2).Find(&rows).Error; err != nil {
		return err
	}
	sig := runtimeStatusSignature(e)
	const mergeWindow = 60 * time.Second
	canMerge := !forceInsert && len(rows) >= 2 &&
		runtimeStatusSignature(&rows[0]) == sig && runtimeStatusSignature(&rows[1]) == sig &&
		rows[0].CreatedAt != nil && rows[1].CreatedAt != nil &&
		time.Time(*rows[0].CreatedAt).Sub(time.Time(*rows[1].CreatedAt)) < mergeWindow
	if canMerge && e.ReportedAt != nil && rows[0].ReportedAt != nil {
		if dt := time.Time(*e.ReportedAt).Sub(time.Time(*rows[0].ReportedAt)); dt > mergeWindow || dt < -mergeWindow {
			canMerge = false
		}
	}
	if canMerge {
		summary := e.SummaryJson
		if summary == "" {
			summary = "{}"
		}
		return r.db.WithContext(ctx).Model(&entity.AgentInstanceRuntimeStatusHistory{}).
			Where("history_id = ?", rows[0].HistoryId).
			Updates(map[string]interface{}{
				"infra_status":        e.InfraStatus,
				"agent_status":        e.AgentStatus,
				"agent_proc_status":   e.AgentProcStatus,
				"agent_proc_pid":      e.AgentProcPid,
				"agent_proc_version":  e.AgentProcVersion,
				"config_hash":         e.ConfigHash,
				"running_command_sig": e.RunningCommandSig,
				"summary_json":        summary,
				"reported_at":         time.Now(),
			}).Error
	}
	if e.HistoryId == "" {
		e.HistoryId = idgen.New()
	}
	summary := e.SummaryJson
	if summary == "" {
		summary = "{}"
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO aa_agent_instance_runtime_status_history
		 (history_id, instance_id, infra_status, agent_status, agent_proc_status, agent_proc_pid, agent_proc_version,
		  config_hash, running_command_sig, summary_json, reported_at, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,now(),now())`,
		e.HistoryId, e.InstanceId, e.InfraStatus, e.AgentStatus, e.AgentProcStatus, e.AgentProcPid, e.AgentProcVersion,
		e.ConfigHash, e.RunningCommandSig, summary,
	).Error
}

// runtimeStatusSignature 状态签名（决定是否同一上报）。不含单调递增项。
// agent_proc_pid 仅 running(1) 时参与（重启变 pid 是有意义事件）；命令签名参与（命令过程各阶段不合并）。
func runtimeStatusSignature(e *entity.AgentInstanceRuntimeStatusHistory) string {
	base := fmt.Sprintf("%d\x00%d\x00%d\x00%s", e.InfraStatus, e.AgentStatus, e.AgentProcStatus, e.AgentProcVersion)
	if e.AgentProcStatus == 1 {
		base += "\x00" + e.AgentProcPid
	}
	if e.RunningCommandSig != "" {
		base += "\x00cmd:" + e.RunningCommandSig
	}
	return base
}

// --- AgentInstanceConfigChange（配置变更历史）---
type AgentInstanceConfigChangeRepository struct{ db *gorm.DB }

func NewAgentInstanceConfigChangeRepository() *AgentInstanceConfigChangeRepository {
	return &AgentInstanceConfigChangeRepository{db: db.AgentDB()}
}

func (r *AgentInstanceConfigChangeRepository) ListByInstance(ctx context.Context, instanceId string, limit int) ([]*entity.AgentInstanceConfigChange, error) {
	var list []*entity.AgentInstanceConfigChange
	q := r.db.WithContext(ctx).Where("instance_id = ?", instanceId).Order("id desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&list).Error
	return list, err
}

func (r *AgentInstanceConfigChangeRepository) GetLatest(ctx context.Context, instanceId string) (*entity.AgentInstanceConfigChange, error) {
	var e entity.AgentInstanceConfigChange
	err := r.db.WithContext(ctx).Where("instance_id = ?", instanceId).Order("id desc").First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

// GetLatestBySourcePrefix 取最近一条 source 前缀匹配的变更（如 "dispatch" 匹配 dispatch.*/dispatch）。
// runtime 上报时据此判断 openclaw 是否接受了最近一次下发（hash 一致=applied，不符=reverted）。
func (r *AgentInstanceConfigChangeRepository) GetLatestBySourcePrefix(ctx context.Context, instanceId, sourcePrefix string) (*entity.AgentInstanceConfigChange, error) {
	var e entity.AgentInstanceConfigChange
	err := r.db.WithContext(ctx).Where("instance_id = ? AND source LIKE ?", instanceId, sourcePrefix+"%").Order("id desc").First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceConfigChangeRepository) Insert(ctx context.Context, e *entity.AgentInstanceConfigChange) error {
	if e.ChangeId == "" {
		e.ChangeId = idgen.New()
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO aa_agent_instance_config_change
		 (change_id, instance_id, agent_id, source, dispatcher, config_hash, config_size, config_mtime, content, changed_at)
		 VALUES (?,?,?,?,?,?,?,?,?,now())`,
		e.ChangeId, e.InstanceId, e.AgentId, e.Source, e.Dispatcher, e.ConfigHash, e.ConfigSize, e.ConfigMtime, e.Content,
	).Error
}

// --- AgentInstanceDesiredState ---
type AgentInstanceDesiredStateRepository struct{ db *gorm.DB }

func NewAgentInstanceDesiredStateRepository() *AgentInstanceDesiredStateRepository {
	return &AgentInstanceDesiredStateRepository{db: db.AgentDB()}
}

func (r *AgentInstanceDesiredStateRepository) GetByInstance(ctx context.Context, instanceId string) (*entity.AgentInstanceDesiredState, error) {
	var e entity.AgentInstanceDesiredState
	if err := r.db.WithContext(ctx).Where("instance_id = ?", instanceId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceDesiredStateRepository) Upsert(ctx context.Context, e *entity.AgentInstanceDesiredState) error {
	var exist entity.AgentInstanceDesiredState
	err := r.db.WithContext(ctx).Where("instance_id = ?", e.InstanceId).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		if e.InstanceDesiredStateId == "" {
			e.InstanceDesiredStateId = idgen.New()
		}
		return r.db.WithContext(ctx).Create(e).Error
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceDesiredState{}).
		Where("instance_id = ?", e.InstanceId).Updates(e).Error
}

// --- AgentInstanceCommand ---
type AgentInstanceCommandRepository struct{ db *gorm.DB }

func NewAgentInstanceCommandRepository() *AgentInstanceCommandRepository {
	return &AgentInstanceCommandRepository{db: db.AgentDB()}
}

func (r *AgentInstanceCommandRepository) List(ctx context.Context, instanceId string, status int, pageNum, pageSize int) (list []*entity.AgentInstanceCommand, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstanceCommand{}).Where("instance_id = ?", instanceId)
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceCommandRepository) Get(ctx context.Context, id string) (*entity.AgentInstanceCommand, error) {
	var e entity.AgentInstanceCommand
	if err := r.db.WithContext(ctx).Where("instance_command_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceCommandRepository) Add(ctx context.Context, e *entity.AgentInstanceCommand) (string, error) {
	e.InstanceCommandId = idgen.New()
	// jsonb 列空字符串会触发解析错误（invalid input syntax for type json），统一兜底为 {}。
	if e.PayloadJson == "" {
		e.PayloadJson = "{}"
	}
	if e.ResultJson == "" {
		e.ResultJson = "{}"
	}
	if e.ProgressJson == "" {
		e.ProgressJson = "{}"
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	// issued_at 用原生 time（避开 pgx 编不了 *types.Time）；created_at 走 DB 默认 NULL。
	_ = r.db.WithContext(ctx).Model(&entity.AgentInstanceCommand{}).
		Where("instance_command_id = ?", e.InstanceCommandId).
		Update("issued_at", time.Now()).Error
	return e.InstanceCommandId, nil
}

func (r *AgentInstanceCommandRepository) UpdateStatus(ctx context.Context, instanceCommandId string, status int, resultJson, errMsg string) error {
	if resultJson == "" {
		resultJson = "{}"
	}
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceCommand{}).
		Where("instance_command_id = ?", instanceCommandId).
		Updates(map[string]interface{}{"status": status, "result_json": resultJson, "error_message": errMsg, "updated_at": time.Now()}).Error
}

// GetLastConfigPushPayload 取某实例最近一条已完成的 config_push（status=3）命令的 payload_json。
// payload 形如 {"configContent":"<openclaw.json 字符串>"}，含 ak。无历史则返回 ""。
func (r *AgentInstanceCommandRepository) GetLastConfigPushPayload(ctx context.Context, instanceId string) (string, error) {
	var e entity.AgentInstanceCommand
	err := r.db.WithContext(ctx).
		Where("instance_id = ? and command_type = ? and status = ?", instanceId, "config_push", 3).
		Order("id desc").Limit(1).First(&e).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return e.PayloadJson, nil
}

// ListPending 取某实例最早 N 条 pending（心跳下行用，不锁不标 running）。
func (r *AgentInstanceCommandRepository) ListPending(ctx context.Context, instanceId string, limit int) (list []*entity.AgentInstanceCommand, err error) {
	if limit <= 0 {
		limit = 8
	}
	err = r.db.WithContext(ctx).Where("instance_id = ? AND status = 0", instanceId).
		Order("id asc").Limit(limit).Find(&list).Error
	return
}

// ListRunningByInstance 某实例 status=2(running) 的命令（认领前置闸：manager 串行执行，
// 上一条未回收不下发新命令；对齐 client 侧 ListRunningByDevice）。
func (r *AgentInstanceCommandRepository) ListRunningByInstance(ctx context.Context, instanceId string) ([]*entity.AgentInstanceCommand, error) {
	var list []*entity.AgentInstanceCommand
	err := r.db.WithContext(ctx).
		Where("instance_id = ? and status = ?", instanceId, 2).
		Order("started_at asc").
		Find(&list).Error
	return list, err
}

// ClaimNext 原子认领某实例最早一条 pending（标 running）。事务 + FOR UPDATE 防并发双认领。
func (r *AgentInstanceCommandRepository) ClaimNext(ctx context.Context, instanceId string) (*entity.AgentInstanceCommand, error) {
	var cmd entity.AgentInstanceCommand
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if rec := recover(); rec != nil {
			tx.Rollback()
		}
	}()
	err := tx.Raw(`SELECT * FROM aa_agent_instance_command WHERE instance_id = ? AND status = 0 ORDER BY id ASC LIMIT 1 FOR UPDATE`, instanceId).
		Scan(&cmd).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if cmd.Id == 0 {
		tx.Rollback()
		return nil, nil // 无待执行
	}
	now := time.Now()
	nowTypes := types.Time(now)
	if err := tx.Model(&entity.AgentInstanceCommand{}).Where("instance_command_id = ?", cmd.InstanceCommandId).
		Updates(map[string]interface{}{"status": 2, "started_at": now, "last_progress_at": now, "updated_at": now}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	cmd.Status = 2
	cmd.StartedAt = &nowTypes
	return &cmd, nil
}

// UpdateResult 回传执行结果 + finished_at。status: 3=done 4=failed 5=timeout。
func (r *AgentInstanceCommandRepository) UpdateResult(ctx context.Context, instanceCommandId string, status int, resultJson, errMsg string) error {
	if resultJson == "" {
		resultJson = "{}"
	}
	now := time.Now()
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceCommand{}).
		Where("instance_command_id = ?", instanceCommandId).
		Updates(map[string]interface{}{"status": status, "result_json": resultJson, "error_message": errMsg, "finished_at": now, "updated_at": now}).Error
}

// UpdateProgress 更新执行进度（interim，命令执行中）。progressJson 形如 {"step":"...","percent":50}。
// idle 超时基准：仅当 progress 内容变化时刷新 last_progress_at（心跳一直上报新进度→基准持续后延→不超时；
// 进度停滞才累计 idle）。单条 SQL 用 CASE 比较 jsonb 文本，避免读-写竞态。
func (r *AgentInstanceCommandRepository) UpdateProgress(ctx context.Context, instanceCommandId, progressJson string) error {
	if progressJson == "" {
		progressJson = "{}"
	}
	return r.db.WithContext(ctx).Exec(
		`UPDATE aa_agent_instance_command
		    SET progress_json = $1::jsonb,
		        updated_at = NOW(),
		        last_progress_at = CASE WHEN COALESCE(progress_json::text,'') <> $1::jsonb::text THEN NOW() ELSE last_progress_at END
		  WHERE instance_command_id = $2`,
		progressJson, instanceCommandId,
	).Error
}

// --- AgentInstanceConfigRevision ---
type AgentInstanceConfigRevisionRepository struct{ db *gorm.DB }

func NewAgentInstanceConfigRevisionRepository() *AgentInstanceConfigRevisionRepository {
	return &AgentInstanceConfigRevisionRepository{db: db.AgentDB()}
}

func (r *AgentInstanceConfigRevisionRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceConfigRevision, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstanceConfigRevision{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("revision_no desc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceConfigRevisionRepository) Get(ctx context.Context, id string) (*entity.AgentInstanceConfigRevision, error) {
	var e entity.AgentInstanceConfigRevision
	if err := r.db.WithContext(ctx).Where("config_revision_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceConfigRevisionRepository) Add(ctx context.Context, e *entity.AgentInstanceConfigRevision) (string, error) {
	e.ConfigRevisionId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ConfigRevisionId, nil
}

func (r *AgentInstanceConfigRevisionRepository) UpdateStatus(ctx context.Context, configRevisionId string, status int) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceConfigRevision{}).
		Where("config_revision_id = ?", configRevisionId).Update("status", status).Error
}

// --- AgentInstanceRuntimeBinding ---
type AgentInstanceRuntimeBindingRepository struct{ db *gorm.DB }

func NewAgentInstanceRuntimeBindingRepository() *AgentInstanceRuntimeBindingRepository {
	return &AgentInstanceRuntimeBindingRepository{db: db.AgentDB()}
}

func (r *AgentInstanceRuntimeBindingRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentInstanceRuntimeBinding, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentInstanceRuntimeBinding{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentInstanceRuntimeBindingRepository) Get(ctx context.Context, id string) (*entity.AgentInstanceRuntimeBinding, error) {
	var e entity.AgentInstanceRuntimeBinding
	if err := r.db.WithContext(ctx).Where("runtime_binding_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceRuntimeBindingRepository) Add(ctx context.Context, e *entity.AgentInstanceRuntimeBinding) (string, error) {
	e.RuntimeBindingId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.RuntimeBindingId, nil
}

func (r *AgentInstanceRuntimeBindingRepository) Update(ctx context.Context, e *entity.AgentInstanceRuntimeBinding) error {
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceRuntimeBinding{}).
		Where("runtime_binding_id = ?", e.RuntimeBindingId).Updates(e).Error
}

// --- AgentInstanceExternalAccess ---
type AgentInstanceExternalAccessRepository struct{ db *gorm.DB }

func NewAgentInstanceExternalAccessRepository() *AgentInstanceExternalAccessRepository {
	return &AgentInstanceExternalAccessRepository{db: db.AgentDB()}
}

func (r *AgentInstanceExternalAccessRepository) GetByInstance(ctx context.Context, instanceId string) (*entity.AgentInstanceExternalAccess, error) {
	var e entity.AgentInstanceExternalAccess
	if err := r.db.WithContext(ctx).Where("instance_id = ?", instanceId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceExternalAccessRepository) GetBySlug(ctx context.Context, slug string) (*entity.AgentInstanceExternalAccess, error) {
	var e entity.AgentInstanceExternalAccess
	if err := r.db.WithContext(ctx).Where("public_slug = ? and enabled = 1", slug).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentInstanceExternalAccessRepository) Upsert(ctx context.Context, e *entity.AgentInstanceExternalAccess) error {
	var exist entity.AgentInstanceExternalAccess
	err := r.db.WithContext(ctx).Where("instance_id = ?", e.InstanceId).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		if e.ExternalAccessId == "" {
			e.ExternalAccessId = idgen.New()
		}
		return r.db.WithContext(ctx).Create(e).Error
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&entity.AgentInstanceExternalAccess{}).
		Where("instance_id = ?", e.InstanceId).Updates(e).Error
}

func (r *AgentInstanceExternalAccessRepository) Delete(ctx context.Context, instanceId string) error {
	return r.db.WithContext(ctx).Where("instance_id = ?", instanceId).Delete(&entity.AgentInstanceExternalAccess{}).Error
}

// --- AgentPersistentVolume ---
type AgentPersistentVolumeRepository struct{ db *gorm.DB }

func NewAgentPersistentVolumeRepository() *AgentPersistentVolumeRepository {
	return &AgentPersistentVolumeRepository{db: db.AgentDB()}
}

func (r *AgentPersistentVolumeRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentPersistentVolume, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentPersistentVolume{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentPersistentVolumeRepository) Get(ctx context.Context, id string) (*entity.AgentPersistentVolume, error) {
	var e entity.AgentPersistentVolume
	if err := r.db.WithContext(ctx).Where("persistent_volume_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ListAll 全量卷（卷列表页对齐 docker 实时元数据；无分页——卷量级小）。
func (r *AgentPersistentVolumeRepository) ListAll(ctx context.Context) ([]*entity.AgentPersistentVolume, error) {
	var list []*entity.AgentPersistentVolume
	err := r.db.WithContext(ctx).Order("id desc").Find(&list).Error
	return list, err
}

// GetByName docker 卷名反查（volume_name 有唯一索引；inspect 实时数据回填用）。
func (r *AgentPersistentVolumeRepository) GetByName(ctx context.Context, volumeName string) (*entity.AgentPersistentVolume, error) {
	var e entity.AgentPersistentVolume
	if err := r.db.WithContext(ctx).Where("volume_name = ?", volumeName).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentPersistentVolumeRepository) Add(ctx context.Context, e *entity.AgentPersistentVolume) (string, error) {
	// 卷服务 Create 需要先拿业务 id 推 docker 卷名（aiarsenal-vol-<bizId>）再落库，
	// 故调用方可能已预生成；此处仅在为空时补（其余 repo 均无条件覆盖，勿照抄）。
	if e.PersistentVolumeId == "" {
		e.PersistentVolumeId = idgen.New()
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.PersistentVolumeId, nil
}

func (r *AgentPersistentVolumeRepository) Update(ctx context.Context, e *entity.AgentPersistentVolume) error {
	return r.db.WithContext(ctx).Model(&entity.AgentPersistentVolume{}).
		Where("persistent_volume_id = ?", e.PersistentVolumeId).Updates(e).Error
}

// UpdateFields 显式字段更新（绕开 gorm 结构体零值跳列）。
func (r *AgentPersistentVolumeRepository) UpdateFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&entity.AgentPersistentVolume{}).
		Where("persistent_volume_id = ?", id).Updates(fields).Error
}

func (r *AgentPersistentVolumeRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("persistent_volume_id in ?", ids).Delete(&entity.AgentPersistentVolume{}).Error
}

// --- AgentBackup ---
type AgentBackupRepository struct{ db *gorm.DB }

func NewAgentBackupRepository() *AgentBackupRepository {
	return &AgentBackupRepository{db: db.AgentDB()}
}

func (r *AgentBackupRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentBackup, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentBackup{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

// ListAll 全量备份（管理端不按实例过滤；备份量级小，不分页）。
func (r *AgentBackupRepository) ListAll(ctx context.Context) ([]*entity.AgentBackup, error) {
	var list []*entity.AgentBackup
	err := r.db.WithContext(ctx).Order("id desc").Find(&list).Error
	return list, err
}

func (r *AgentBackupRepository) Get(ctx context.Context, id string) (*entity.AgentBackup, error) {
	var e entity.AgentBackup
	if err := r.db.WithContext(ctx).Where("backup_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentBackupRepository) Add(ctx context.Context, e *entity.AgentBackup) (string, error) {
	// 备份落盘路径含 backup_id（<dir>/<backupId>.tar.gz），服务层先预生成 id 再落库，
	// 故仅在为空时补（同 AgentPersistentVolumeRepository.Add）。
	if e.BackupId == "" {
		e.BackupId = idgen.New()
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.BackupId, nil
}

func (r *AgentBackupRepository) Update(ctx context.Context, e *entity.AgentBackup) error {
	return r.db.WithContext(ctx).Model(&entity.AgentBackup{}).Where("backup_id = ?", e.BackupId).Updates(e).Error
}

func (r *AgentBackupRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("backup_id in ?", ids).Delete(&entity.AgentBackup{}).Error
}

// ListExpired 过期备份（expires_at < now）：定时保留期清理先查这里，
// 逐条删物理文件后再 Delete 落库。
func (r *AgentBackupRepository) ListExpired(ctx context.Context, now time.Time, limit int) ([]*entity.AgentBackup, error) {
	var list []*entity.AgentBackup
	q := r.db.WithContext(ctx).Where("expires_at is not null and expires_at < ?", now)
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Order("id asc").Find(&list).Error
	return list, err
}

// --- AgentBackupSchedule ---
type AgentBackupScheduleRepository struct{ db *gorm.DB }

func NewAgentBackupScheduleRepository() *AgentBackupScheduleRepository {
	return &AgentBackupScheduleRepository{db: db.AgentDB()}
}

func (r *AgentBackupScheduleRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentBackupSchedule, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentBackupSchedule{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentBackupScheduleRepository) Get(ctx context.Context, id string) (*entity.AgentBackupSchedule, error) {
	var e entity.AgentBackupSchedule
	if err := r.db.WithContext(ctx).Where("backup_schedule_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ListAll 全量备份计划（服务启动时逐条注册定时任务用；计划量级小，不分页）。
func (r *AgentBackupScheduleRepository) ListAll(ctx context.Context) ([]*entity.AgentBackupSchedule, error) {
	var list []*entity.AgentBackupSchedule
	err := r.db.WithContext(ctx).Order("id asc").Find(&list).Error
	return list, err
}

func (r *AgentBackupScheduleRepository) Add(ctx context.Context, e *entity.AgentBackupSchedule) (string, error) {
	e.BackupScheduleId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.BackupScheduleId, nil
}

func (r *AgentBackupScheduleRepository) Edit(ctx context.Context, e *entity.AgentBackupSchedule) error {
	return r.db.WithContext(ctx).Model(&entity.AgentBackupSchedule{}).
		Where("backup_schedule_id = ?", e.BackupScheduleId).Updates(e).Error
}

// EditScheduleFields 显式字段更新（绕开 gorm 结构体零值跳列：isActive=0/retentionDays=0）。
func (r *AgentBackupScheduleRepository) EditScheduleFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&entity.AgentBackupSchedule{}).
		Where("backup_schedule_id = ?", id).Updates(fields).Error
}

func (r *AgentBackupScheduleRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("backup_schedule_id in ?", ids).Delete(&entity.AgentBackupSchedule{}).Error
}

// --- AgentRuntimePod ---
type AgentRuntimePodRepository struct{ db *gorm.DB }

func NewAgentRuntimePodRepository() *AgentRuntimePodRepository {
	return &AgentRuntimePodRepository{db: db.AgentDB()}
}

func (r *AgentRuntimePodRepository) List(ctx context.Context, runtimeType string, state int, pageNum, pageSize int) (list []*entity.AgentRuntimePod, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentRuntimePod{})
	if runtimeType != "" {
		q = q.Where("runtime_type = ?", runtimeType)
	}
	if state >= 0 {
		q = q.Where("state = ?", state)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentRuntimePodRepository) Get(ctx context.Context, id string) (*entity.AgentRuntimePod, error) {
	var e entity.AgentRuntimePod
	if err := r.db.WithContext(ctx).Where("runtime_pod_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ListByRollout 发布单的 pod 批次（滚动发布推进/进度查询）。
func (r *AgentRuntimePodRepository) ListByRollout(ctx context.Context, rolloutId string, pageNum, pageSize int) (list []*entity.AgentRuntimePod, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentRuntimePod{}).Where("rollout_id = ?", rolloutId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id asc").Offset(off).Limit(lim).Find(&list).Error
	return
}

// GetByPodName pod_name 反查（localdocker 约定 pod_name = aiarsenal-<instanceId>，
// 滚动发布按它把 pod 行映射回实例）。
func (r *AgentRuntimePodRepository) GetByPodName(ctx context.Context, podName string) (*entity.AgentRuntimePod, error) {
	var e entity.AgentRuntimePod
	if err := r.db.WithContext(ctx).Where("pod_name = ?", podName).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentRuntimePodRepository) Add(ctx context.Context, e *entity.AgentRuntimePod) (string, error) {
	e.RuntimePodId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.RuntimePodId, nil
}

func (r *AgentRuntimePodRepository) Update(ctx context.Context, e *entity.AgentRuntimePod) error {
	return r.db.WithContext(ctx).Model(&entity.AgentRuntimePod{}).
		Where("runtime_pod_id = ?", e.RuntimePodId).Updates(e).Error
}

func (r *AgentRuntimePodRepository) Drain(ctx context.Context, runtimePodId string) error {
	return r.db.WithContext(ctx).Model(&entity.AgentRuntimePod{}).
		Where("runtime_pod_id = ?", runtimePodId).Update("draining", 1).Error
}

// --- AgentRuntimeRollout ---
type AgentRuntimeRolloutRepository struct{ db *gorm.DB }

func NewAgentRuntimeRolloutRepository() *AgentRuntimeRolloutRepository {
	return &AgentRuntimeRolloutRepository{db: db.AgentDB()}
}

func (r *AgentRuntimeRolloutRepository) List(ctx context.Context, status int, pageNum, pageSize int) (list []*entity.AgentRuntimeRollout, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentRuntimeRollout{})
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentRuntimeRolloutRepository) Get(ctx context.Context, id string) (*entity.AgentRuntimeRollout, error) {
	var e entity.AgentRuntimeRollout
	if err := r.db.WithContext(ctx).Where("runtime_rollout_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentRuntimeRolloutRepository) Add(ctx context.Context, e *entity.AgentRuntimeRollout) (string, error) {
	e.RuntimeRolloutId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.RuntimeRolloutId, nil
}

func (r *AgentRuntimeRolloutRepository) Update(ctx context.Context, e *entity.AgentRuntimeRollout) error {
	return r.db.WithContext(ctx).Model(&entity.AgentRuntimeRollout{}).
		Where("runtime_rollout_id = ?", e.RuntimeRolloutId).Updates(e).Error
}

// --- AgentWorkspaceFileAudit ---
type AgentWorkspaceFileAuditRepository struct{ db *gorm.DB }

func NewAgentWorkspaceFileAuditRepository() *AgentWorkspaceFileAuditRepository {
	return &AgentWorkspaceFileAuditRepository{db: db.AgentDB()}
}

func (r *AgentWorkspaceFileAuditRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentWorkspaceFileAudit, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentWorkspaceFileAudit{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentWorkspaceFileAuditRepository) Add(ctx context.Context, e *entity.AgentWorkspaceFileAudit) (string, error) {
	e.WorkspaceAuditId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.WorkspaceAuditId, nil
}

// --- AgentOpenclawInjectionSnapshot ---
type AgentOpenclawInjectionSnapshotRepository struct{ db *gorm.DB }

func NewAgentOpenclawInjectionSnapshotRepository() *AgentOpenclawInjectionSnapshotRepository {
	return &AgentOpenclawInjectionSnapshotRepository{db: db.AgentDB()}
}

func (r *AgentOpenclawInjectionSnapshotRepository) List(ctx context.Context, instanceId string, pageNum, pageSize int) (list []*entity.AgentOpenclawInjectionSnapshot, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentOpenclawInjectionSnapshot{}).Where("instance_id = ?", instanceId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentOpenclawInjectionSnapshotRepository) Get(ctx context.Context, id string) (*entity.AgentOpenclawInjectionSnapshot, error) {
	var e entity.AgentOpenclawInjectionSnapshot
	if err := r.db.WithContext(ctx).Where("injection_snapshot_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentOpenclawInjectionSnapshotRepository) Add(ctx context.Context, e *entity.AgentOpenclawInjectionSnapshot) (string, error) {
	e.InjectionSnapshotId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.InjectionSnapshotId, nil
}

// --- AgentSystemImageSetting ---
type AgentSystemImageSettingRepository struct{ db *gorm.DB }

func NewAgentSystemImageSettingRepository() *AgentSystemImageSettingRepository {
	return &AgentSystemImageSettingRepository{db: db.AgentDB()}
}

func (r *AgentSystemImageSettingRepository) List(ctx context.Context, agent, version string) (list []*entity.AgentSystemImageSetting, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentSystemImageSetting{})
	if agent != "" {
		q = q.Where("agent = ?", agent)
	}
	if version != "" {
		q = q.Where("version = ?", version)
	}
	err = q.Order("agent, version").Find(&list).Error
	return
}

func (r *AgentSystemImageSettingRepository) Upsert(ctx context.Context, e *entity.AgentSystemImageSetting) error {
	var exist entity.AgentSystemImageSetting
	err := r.db.WithContext(ctx).Where("agent = ? and version = ?", e.Agent, e.Version).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		if e.SystemImageId == "" {
			e.SystemImageId = idgen.New()
		}
		return r.db.WithContext(ctx).Create(e).Error
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&entity.AgentSystemImageSetting{}).
		Where("agent = ? and version = ?", e.Agent, e.Version).
		Updates(map[string]interface{}{"display_name": e.DisplayName, "image": e.Image, "is_enabled": e.IsEnabled, "updated_by": e.UpdatedBy, "updated_at": time.Now()}).Error
}

func (r *AgentSystemImageSettingRepository) Delete(ctx context.Context, agent, version string) error {
	return r.db.WithContext(ctx).Where("agent = ? and version = ?", agent, version).
		Delete(&entity.AgentSystemImageSetting{}).Error
}

// ==================== Team ====================

// --- AgentTeam ---
type AgentTeamRepository struct{ db *gorm.DB }

func NewAgentTeamRepository() *AgentTeamRepository { return &AgentTeamRepository{db: db.AgentDB()} }

func (r *AgentTeamRepository) List(ctx context.Context, userId, name string, pageNum, pageSize int) (list []*entity.AgentTeam, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentTeam{}).Where("deleted_at is null and user_id = ?", userId)
	if name != "" {
		q = q.Where("name like ?", "%"+name+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentTeamRepository) Get(ctx context.Context, id string) (*entity.AgentTeam, error) {
	var e entity.AgentTeam
	if err := r.db.WithContext(ctx).Where("team_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentTeamRepository) Add(ctx context.Context, e *entity.AgentTeam) (string, error) {
	e.TeamId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.TeamId, nil
}

func (r *AgentTeamRepository) Edit(ctx context.Context, e *entity.AgentTeam) error {
	return r.db.WithContext(ctx).Model(&entity.AgentTeam{}).
		Where("team_id = ?", e.TeamId).Updates(e).Error
}

func (r *AgentTeamRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.AgentTeam{}).
		Where("team_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// --- AgentTeamMember ---
type AgentTeamMemberRepository struct{ db *gorm.DB }

func NewAgentTeamMemberRepository() *AgentTeamMemberRepository {
	return &AgentTeamMemberRepository{db: db.AgentDB()}
}

func (r *AgentTeamMemberRepository) List(ctx context.Context, teamId string, pageNum, pageSize int) (list []*entity.AgentTeamMember, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentTeamMember{}).Where("team_id = ?", teamId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentTeamMemberRepository) Get(ctx context.Context, id string) (*entity.AgentTeamMember, error) {
	var e entity.AgentTeamMember
	if err := r.db.WithContext(ctx).Where("team_member_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentTeamMemberRepository) Add(ctx context.Context, e *entity.AgentTeamMember) (string, error) {
	e.TeamMemberId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.TeamMemberId, nil
}

func (r *AgentTeamMemberRepository) Edit(ctx context.Context, e *entity.AgentTeamMember) error {
	return r.db.WithContext(ctx).Model(&entity.AgentTeamMember{}).
		Where("team_member_id = ?", e.TeamMemberId).Updates(e).Error
}

func (r *AgentTeamMemberRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("team_member_id in ?", ids).Delete(&entity.AgentTeamMember{}).Error
}

// --- AgentTeamTask ---
type AgentTeamTaskRepository struct{ db *gorm.DB }

func NewAgentTeamTaskRepository() *AgentTeamTaskRepository {
	return &AgentTeamTaskRepository{db: db.AgentDB()}
}

func (r *AgentTeamTaskRepository) List(ctx context.Context, teamId string, status int, pageNum, pageSize int) (list []*entity.AgentTeamTask, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentTeamTask{}).Where("team_id = ?", teamId)
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentTeamTaskRepository) Get(ctx context.Context, id string) (*entity.AgentTeamTask, error) {
	var e entity.AgentTeamTask
	if err := r.db.WithContext(ctx).Where("team_task_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentTeamTaskRepository) Add(ctx context.Context, e *entity.AgentTeamTask) (string, error) {
	e.TeamTaskId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.TeamTaskId, nil
}

func (r *AgentTeamTaskRepository) Update(ctx context.Context, e *entity.AgentTeamTask) error {
	return r.db.WithContext(ctx).Model(&entity.AgentTeamTask{}).
		Where("team_task_id = ?", e.TeamTaskId).Updates(e).Error
}

// --- AgentTeamEvent ---
type AgentTeamEventRepository struct{ db *gorm.DB }

func NewAgentTeamEventRepository() *AgentTeamEventRepository {
	return &AgentTeamEventRepository{db: db.AgentDB()}
}

func (r *AgentTeamEventRepository) List(ctx context.Context, teamId string, pageNum, pageSize int) (list []*entity.AgentTeamEvent, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentTeamEvent{}).Where("team_id = ?", teamId)
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("sequence_no desc, id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentTeamEventRepository) Add(ctx context.Context, e *entity.AgentTeamEvent) (string, error) {
	e.TeamEventId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.TeamEventId, nil
}

// --- AgentTeamWorkItem ---
type AgentTeamWorkItemRepository struct{ db *gorm.DB }

func NewAgentTeamWorkItemRepository() *AgentTeamWorkItemRepository {
	return &AgentTeamWorkItemRepository{db: db.AgentDB()}
}

func (r *AgentTeamWorkItemRepository) List(ctx context.Context, rootTaskId string, status int, pageNum, pageSize int) (list []*entity.AgentTeamWorkItem, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentTeamWorkItem{}).Where("root_task_id = ?", rootTaskId)
	if status >= 0 {
		q = q.Where("status = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentTeamWorkItemRepository) Get(ctx context.Context, id string) (*entity.AgentTeamWorkItem, error) {
	var e entity.AgentTeamWorkItem
	if err := r.db.WithContext(ctx).Where("work_item_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentTeamWorkItemRepository) Add(ctx context.Context, e *entity.AgentTeamWorkItem) (string, error) {
	e.WorkItemId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.WorkItemId, nil
}

func (r *AgentTeamWorkItemRepository) Edit(ctx context.Context, e *entity.AgentTeamWorkItem) error {
	return r.db.WithContext(ctx).Model(&entity.AgentTeamWorkItem{}).
		Where("work_item_id = ?", e.WorkItemId).Updates(e).Error
}

func (r *AgentTeamWorkItemRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("work_item_id in ?", ids).Delete(&entity.AgentTeamWorkItem{}).Error
}

// --- AgentUser ---
type AgentUserRepository struct{ db *gorm.DB }

func NewAgentUserRepository() *AgentUserRepository {
	return &AgentUserRepository{db: db.AgentDB()}
}

// List keyword 模糊匹配 username/display_name，status -1=全部 0/1=按 is_enabled
// （Agent 用户管理列表）。
func (r *AgentUserRepository) List(ctx context.Context, keyword string, status int, pageNum, pageSize int) (list []*entity.AgentUser, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.AgentUser{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("username like ? or display_name like ?", kw, kw)
	}
	if status >= 0 {
		q = q.Where("is_enabled = ?", status)
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *AgentUserRepository) Get(ctx context.Context, id string) (*entity.AgentUser, error) {
	var e entity.AgentUser
	if err := r.db.WithContext(ctx).Where("agent_user_id = ?", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentUserRepository) GetByUsername(ctx context.Context, username string) (*entity.AgentUser, error) {
	var e entity.AgentUser
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *AgentUserRepository) Add(ctx context.Context, e *entity.AgentUser) (string, error) {
	e.AgentUserId = idgen.New()
	// instance_ids jsonb 空串兜底（空数组语义）。
	if e.InstanceIds == "" {
		e.InstanceIds = "[]"
	}
	// gorm 对 *types.Time 不自动回填时间戳，显式补（同 AgentInstanceRepository.Add）。
	now := types.Time(time.Now())
	if e.CreatedAt == nil {
		e.CreatedAt = &now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = &now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.AgentUserId, nil
}

func (r *AgentUserRepository) Edit(ctx context.Context, e *entity.AgentUser) error {
	return r.db.WithContext(ctx).Model(&entity.AgentUser{}).
		Where("agent_user_id = ?", e.AgentUserId).Updates(e).Error
}

// EditFields 显式字段更新（绕开 gorm 结构体零值跳列：is_enabled=0）。
func (r *AgentUserRepository) EditFields(ctx context.Context, id string, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&entity.AgentUser{}).
		Where("agent_user_id = ?", id).Updates(fields).Error
}

func (r *AgentUserRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("agent_user_id in ?", ids).Delete(&entity.AgentUser{}).Error
}
