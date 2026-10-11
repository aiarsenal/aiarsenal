package persistence

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// Client 模块仓储实现（client 库，db.ClientDB()）。

// ==================== agent（类型+介绍） ====================

type ClientAgentRepository struct{ db *gorm.DB }

func NewClientAgentRepository() *ClientAgentRepository {
	return &ClientAgentRepository{db: db.ClientDB()}
}

func (r *ClientAgentRepository) ListAll(ctx context.Context) ([]*entity.ClientAgent, error) {
	var list []*entity.ClientAgent
	err := r.db.WithContext(ctx).Order("weigh desc, agent asc").Find(&list).Error
	return list, err
}

func (r *ClientAgentRepository) GetByAgent(ctx context.Context, agent string) (*entity.ClientAgent, error) {
	var e entity.ClientAgent
	if err := r.db.WithContext(ctx).Where("agent = ?", agent).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// Upsert 按 agent_id 更新或按 agent 冲突插入。新行用 idgen 生成 agent_id。
func (r *ClientAgentRepository) Upsert(ctx context.Context, e *entity.ClientAgent) error {
	now := time.Now()
	if e.AgentId != "" {
		res := r.db.WithContext(ctx).Model(&entity.ClientAgent{}).
			Where("agent_id = ?", e.AgentId).
			Updates(map[string]interface{}{
				"agent":      e.Agent,
				"name":       e.Name,
				"intro":      e.Intro,
				"icon":       e.Icon,
				"enabled":    e.Enabled,
				"weigh":      e.Weigh,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}
	if e.AgentId == "" {
		e.AgentId = idgen.New()
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "agent"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"name":       e.Name,
			"intro":      e.Intro,
			"icon":       e.Icon,
			"enabled":    e.Enabled,
			"weigh":      e.Weigh,
			"updated_at": now,
		}),
	}).Create(e).Error
}

func (r *ClientAgentRepository) Delete(ctx context.Context, agentId string) error {
	return r.db.WithContext(ctx).Where("agent_id = ?", agentId).Delete(&entity.ClientAgent{}).Error
}

// ==================== agent 版本 ====================

type ClientAgentVersionRepository struct{ db *gorm.DB }

func NewClientAgentVersionRepository() *ClientAgentVersionRepository {
	return &ClientAgentVersionRepository{db: db.ClientDB()}
}

func (r *ClientAgentVersionRepository) ListByAgent(ctx context.Context, agentId string) ([]*entity.ClientAgentVersion, error) {
	var list []*entity.ClientAgentVersion
	err := r.db.WithContext(ctx).
		Where("agent_id = ?", agentId).
		Order("is_default desc, version asc").
		Find(&list).Error
	return list, err
}

func (r *ClientAgentVersionRepository) ListAllByAgents(ctx context.Context, agentIds []string) (map[string][]*entity.ClientAgentVersion, error) {
	out := map[string][]*entity.ClientAgentVersion{}
	if len(agentIds) == 0 {
		return out, nil
	}
	var list []*entity.ClientAgentVersion
	err := r.db.WithContext(ctx).
		Where("agent_id in ?", agentIds).
		Order("agent_id asc, is_default desc, version asc").
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	for _, v := range list {
		out[v.AgentId] = append(out[v.AgentId], v)
	}
	return out, nil
}

// Upsert 按 version_id 更新或按 (agent_id, version) 冲突插入。新行用 idgen 生成 version_id。
func (r *ClientAgentVersionRepository) Upsert(ctx context.Context, e *entity.ClientAgentVersion) error {
	now := time.Now()
	// jsonb 列不接受空串
	if e.ConfigTemplate == "" {
		e.ConfigTemplate = "{}"
	}
	if e.EnvVars == "" {
		e.EnvVars = "{}"
	}
	if e.Extra == "" {
		e.Extra = "{}"
	}
	if e.VersionId != "" {
		res := r.db.WithContext(ctx).Model(&entity.ClientAgentVersion{}).
			Where("version_id = ?", e.VersionId).
			Updates(map[string]interface{}{
				"version":         e.Version,
				"is_default":      e.IsDefault,
				"enabled":         e.Enabled,
				"download_url":    e.DownloadUrl,
				"container_image": e.ContainerImage,
				"channel":         e.Channel,
				"config_template": e.ConfigTemplate,
				"env_vars":        e.EnvVars,
				"extra":           e.Extra,
				"updated_at":      now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}
	if e.VersionId == "" {
		e.VersionId = idgen.New()
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "agent_id"}, {Name: "version"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"is_default":      e.IsDefault,
			"enabled":         e.Enabled,
			"download_url":    e.DownloadUrl,
			"container_image": e.ContainerImage,
			"channel":         e.Channel,
			"config_template": e.ConfigTemplate,
			"env_vars":        e.EnvVars,
			"extra":           e.Extra,
			"updated_at":      now,
		}),
	}).Create(e).Error
}

func (r *ClientAgentVersionRepository) Delete(ctx context.Context, versionId string) error {
	return r.db.WithContext(ctx).Where("version_id = ?", versionId).Delete(&entity.ClientAgentVersion{}).Error
}

func (r *ClientAgentVersionRepository) DeleteByAgent(ctx context.Context, agentId string) error {
	return r.db.WithContext(ctx).Where("agent_id = ?", agentId).Delete(&entity.ClientAgentVersion{}).Error
}

// ClearDefault 把某 agent 内除 exceptVersionId 外的版本 is_default 置 0。
func (r *ClientAgentVersionRepository) ClearDefault(ctx context.Context, agentId, exceptVersionId string) error {
	q := r.db.WithContext(ctx).Model(&entity.ClientAgentVersion{}).
		Where("agent_id = ? and is_default = 1", agentId)
	if exceptVersionId != "" {
		q = q.Where("version_id <> ?", exceptVersionId)
	}
	return q.Update("is_default", 0).Error
}

// ==================== 客户端运行状态 ====================

type ClientAgentStatusRepository struct{ db *gorm.DB }

func NewClientAgentStatusRepository() *ClientAgentStatusRepository {
	return &ClientAgentStatusRepository{db: db.ClientDB()}
}

// Upsert 时间线去重：按 (device_id, agent) 维护状态时间线（多条）。
// 「内容签名」= status + version + last_error（pid_or_container_id/uptime_seconds 列已
// drop；客户端从不真实上报二者）。连续 3 条相同 → 第 3 条起更新第 2 条（刷新
// reported_at），状态变化才插新行；同一状态最多保留 2 条（首条=首次出现，
// 第 2 条=最近上报，持续刷新）。device_id 为空回退 user_id 兼容旧客户端。
func (r *ClientAgentStatusRepository) Upsert(ctx context.Context, e *entity.ClientAgentStatus, forceInsert bool) error {
	now := time.Now()
	nowTypes := types.Time(now)
	// reported_at 列 NOT NULL：GORM 把 nil *types.Time 显式插 NULL（绕过 DEFAULT now()），
	// 触发 23502。Upsert 前必填。
	if e.ReportedAt == nil {
		e.ReportedAt = &nowTypes
	}
	// version varchar(64)：App 端 openclaw --version 异常时会把 node 报错全文塞进来，
	// 截断防 22001（last_error 是 text 不限长，无需截）。
	if len(e.Version) > 64 {
		e.Version = e.Version[:64]
	}
	// jsonb 列不接受空串（SQLSTATE 22P02），空值统一落 "{}"
	if e.Extra == "" {
		e.Extra = "{}"
	}
	// 取最近 2 条（rows[0]=最新）。device_id 为空回退 user_id。
	q := r.db.WithContext(ctx).Model(&entity.ClientAgentStatus{}).Order("id desc").Limit(2)
	if e.DeviceId != "" {
		q = q.Where("device_id = ? and agent = ?", e.DeviceId, e.Agent)
	} else {
		q = q.Where("user_id = ? and agent = ?", e.UserId, e.Agent)
	}
	var rows []entity.ClientAgentStatus
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	sig := statusSignature(e)
	// 合并条件（全满足才合并，否则插新行）：
	// 1. 最近 2 条都与本次同签名
	// 2. rows[0] 与 rows[1] 的【插入时间】(created_at，不随合并刷新) 间隔 < 20s（同一快速突发）
	// 3. 本次上报时间(e.ReportedAt) 与被合并行 rows[0].reported_at 间隔 ≤ 20s——
	//    离上次上报超过 20s 视为不同突发，插新行保留时间线断点。
	//    （DSN 已设 TimeZone=Asia/Shanghai，reported_at 读写同为 CST，无错位。）
	// 0 条 / 1 条：必插新行（无 2 条可判合并）。
	canMerge := !forceInsert && len(rows) >= 2 && statusSignature(&rows[0]) == sig && statusSignature(&rows[1]) == sig &&
		rows[0].CreatedAt != nil && rows[1].CreatedAt != nil &&
		time.Time(*rows[0].CreatedAt).Sub(time.Time(*rows[1].CreatedAt)) < 20*time.Second
	if canMerge && e.ReportedAt != nil && rows[0].ReportedAt != nil {
		if dt := time.Time(*e.ReportedAt).Sub(time.Time(*rows[0].ReportedAt)); dt > 20*time.Second || dt < -20*time.Second {
			canMerge = false
		}
	}
	if canMerge {
		return r.db.WithContext(ctx).Model(&entity.ClientAgentStatus{}).
			Where("status_id = ?", rows[0].StatusId).
			Updates(map[string]interface{}{
				"device_id":   e.DeviceId,
				"user_id":     e.UserId,
				"status":      e.Status,
				"version":     e.Version,
				"last_error":  e.LastError,
				"extra":       e.Extra,
				"reported_at": now,
			}).Error
	}
	// 否则插新行（时间线新增一条）。raw SQL：pgx 编不了 *types.Time（reported_at），用 now()。
	// unique 索引已去，无需 OnConflict。
	e.StatusId = idgen.New()
	kind := e.Kind
	if kind == "" {
		kind = "agent"
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO aa_client_agent_status
		 (status_id, device_id, user_id, agent, kind, status, version, last_error, extra, reported_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now())`,
		e.StatusId, e.DeviceId, e.UserId, e.Agent, kind, e.Status, e.Version,
		e.LastError, e.Extra,
	).Error
}

// statusSignature 状态内容签名：决定「上报内容是否一样」。
// pid_or_container_id/uptime_seconds 列已 drop（客户端从不真实上报二者，恒空/0，
// 无事件意义），不参与签名。含命令标记（extra.action+status）：有命令的行
// （开始/执行中/结束）与 idle 行签名不同，idle 心跳不会合并覆盖命令结束行——
// 保证每条命令的开始/过程/结束在时间线各自留痕。
func statusSignature(e *entity.ClientAgentStatus) string {
	base := e.Status + "\x00" + e.Version + "\x00" + e.LastError
	if cmd := cmdSignature(e.Extra); cmd != "" {
		base += "\x00cmd:" + cmd
	}
	return base
}

// cmdSignature 从 extra jsonb 取命令标记（action:status），无命令返回空。
func cmdSignature(extra string) string {
	if extra == "" || extra == "{}" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(extra), &m) != nil {
		return ""
	}
	action, _ := m["action"].(string)
	if action == "" {
		return ""
	}
	status, _ := m["status"].(string)
	return action + ":" + status
}

func (r *ClientAgentStatusRepository) ListByDevice(ctx context.Context, deviceId string) ([]*entity.ClientAgentStatus, error) {
	var list []*entity.ClientAgentStatus
	err := r.db.WithContext(ctx).
		Where("device_id = ?", deviceId).
		Order("agent asc").
		Find(&list).Error
	return list, err
}

// ListLatestByDevice 每个客户端取最新一条（DISTINCT ON agent，id 最大）。
// 供管理后台设备详情展示「当前状态」用（时间线下只看最新）。
func (r *ClientAgentStatusRepository) ListLatestByDevice(ctx context.Context, deviceId string) ([]*entity.ClientAgentStatus, error) {
	var list []*entity.ClientAgentStatus
	err := r.db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (agent) * FROM aa_client_agent_status WHERE device_id = ? ORDER BY agent, id DESC`, deviceId).
		Scan(&list).Error
	return list, err
}

func (r *ClientAgentStatusRepository) ListByUser(ctx context.Context, userId string) ([]*entity.ClientAgentStatus, error) {
	var list []*entity.ClientAgentStatus
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userId).
		Order("agent asc").
		Find(&list).Error
	return list, err
}

func (r *ClientAgentStatusRepository) ListAll(ctx context.Context) ([]*entity.ClientAgentStatus, error) {
	var list []*entity.ClientAgentStatus
	err := r.db.WithContext(ctx).
		Order("device_id asc, agent asc").
		Find(&list).Error
	return list, err
}

// ==================== 远程指令队列 ====================

type ClientCommandRepository struct{ db *gorm.DB }

func NewClientCommandRepository() *ClientCommandRepository {
	return &ClientCommandRepository{db: db.ClientDB()}
}

// Enqueue 入队一条 pending 指令。raw SQL：pgx 编不了 *types.Time（created_at），用 now()。
func (r *ClientCommandRepository) Enqueue(ctx context.Context, e *entity.ClientCommand) error {
	if e.CommandId == "" {
		e.CommandId = idgen.New()
	}
	if len(e.Payload) == 0 {
		e.Payload = json.RawMessage("{}")
	}
	e.Status = "pending"
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO aa_client_command
		 (command_id, device_id, action, agent, version, payload, status, chain_id, created_by, created_at, timeout_seconds)
		 VALUES ($1,$2,$3,$4,$5,$6,'pending',$7,$8,now(),$9)`,
		e.CommandId, e.DeviceId, e.Action, e.Agent, e.Version, e.Payload, e.ChainId, e.CreatedBy, e.TimeoutSeconds,
	).Error
}

// CancelChainPending 取消同一链下所有 pending（前置步骤失败，后续不再执行）。
func (r *ClientCommandRepository) CancelChainPending(ctx context.Context, chainId string) error {
	if chainId == "" {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ClientCommand{}).
		Where("chain_id = ? AND status = 'pending'", chainId).
		Updates(map[string]interface{}{"status": "skipped", "error": "前置步骤失败，已自动取消", "finished_at": time.Now()}).Error
}

// ClaimNext 原子认领某设备最早一条 pending（标 running）。
// 用事务 + 行级锁（FOR UPDATE）防并发双认领；无 pending 返回 nil。
func (r *ClientCommandRepository) ClaimNext(ctx context.Context, deviceId string) (*entity.ClientCommand, error) {
	var cmd entity.ClientCommand
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	// 取最早一条 pending（FOR UPDATE 锁行）
	err := tx.Raw(`SELECT * FROM aa_client_command WHERE device_id = ? AND status = 'pending' ORDER BY created_at ASC LIMIT 1 FOR UPDATE`, deviceId).
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
	if err := tx.Model(&entity.ClientCommand{}).Where("command_id = ?", cmd.CommandId).
		Updates(map[string]interface{}{"status": "running", "started_at": now, "last_progress_at": now}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	cmd.Status = "running"
	cmd.StartedAt = &nowTypes
	return &cmd, nil
}

// UpdateResult 回传执行结果。
func (r *ClientCommandRepository) UpdateResult(ctx context.Context, commandId, status, result, errMsg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&entity.ClientCommand{}).
		Where("command_id = ?", commandId).
		Updates(map[string]interface{}{
			"status":      status,
			"result":      result,
			"error":       errMsg,
			"finished_at": now,
		}).Error
}

// UpdateProgress 更新执行进度；仅当 progress 内容变化时刷新 last_progress_at（idle 超时基准：
// 心跳一直上报新进度→last_progress_at 持续后延→不超时；进度停滞才累计 idle 时间）。
// 单条 SQL 用 CASE 比较 jsonb 文本，避免读-写竞态。
func (r *ClientCommandRepository) UpdateProgress(ctx context.Context, commandId, progress string) error {
	if progress == "" {
		progress = "{}"
	}
	return r.db.WithContext(ctx).Exec(
		`UPDATE aa_client_command
		    SET progress = $1,
		        last_progress_at = CASE WHEN COALESCE(progress::text,'') <> $1::jsonb::text THEN NOW() ELSE last_progress_at END
		  WHERE command_id = $2`,
		progress, commandId,
	).Error
}

func (r *ClientCommandRepository) ListByDevice(ctx context.Context, deviceId string, limit int) ([]*entity.ClientCommand, error) {
	var list []*entity.ClientCommand
	if limit <= 0 {
		limit = 50
	}
	err := r.db.WithContext(ctx).
		Where("device_id = ?", deviceId).
		Order("created_at DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// ListRunningByDevice 某设备 status=running 的命令（串行校验 + 超时清理用）。
func (r *ClientCommandRepository) ListRunningByDevice(ctx context.Context, deviceId string) ([]*entity.ClientCommand, error) {
	var list []*entity.ClientCommand
	err := r.db.WithContext(ctx).
		Where("device_id = ? and status = ?", deviceId, "running").
		Order("started_at asc").
		Find(&list).Error
	return list, err
}

// ListPendingByDevice 某设备 status=pending 的命令（已入队、客户端尚未认领执行）。
// 串行去重用：防快速双击在第一条还没被认领（pending）时又入队一条相同动作。
func (r *ClientCommandRepository) ListPendingByDevice(ctx context.Context, deviceId string) ([]*entity.ClientCommand, error) {
	var list []*entity.ClientCommand
	err := r.db.WithContext(ctx).
		Where("device_id = ? and status = ?", deviceId, "pending").
		Order("created_at asc").
		Find(&list).Error
	return list, err
}

// CommandedAgents 该设备下「曾经下过命令」的 agent key 集合。
// 用于心跳过滤：只有装过/下过命令的 agent 才落状态，避免把本机自装的
// Obsidian、从未触碰的 hermes 当 agent 上报污染状态列表。
func (r *ClientCommandRepository) CommandedAgents(ctx context.Context, deviceId string) (map[string]bool, error) {
	var agents []string
	err := r.db.WithContext(ctx).
		Model(&entity.ClientCommand{}).
		Where("device_id = ?", deviceId).
		Distinct("agent").
		Pluck("agent", &agents).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(agents))
	for _, c := range agents {
		m[c] = true
	}
	return m, nil
}

func (r *ClientCommandRepository) GetById(ctx context.Context, commandId string) (*entity.ClientCommand, error) {
	var e entity.ClientCommand
	if err := r.db.WithContext(ctx).Where("command_id = ?", commandId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// ==================== 设备登录记录 ====================

type ClientDeviceRepository struct{ db *gorm.DB }

func NewClientDeviceRepository() *ClientDeviceRepository {
	return &ClientDeviceRepository{db: db.ClientDB()}
}

// Upsert 按 device_id 冲突更新：刷新机器信息/last_login/user_id，status 重置为 active，
// 但保留 blocked（封禁不因重新登录自动解除）。
func (r *ClientDeviceRepository) Upsert(ctx context.Context, e *entity.ClientDevice) error {
	now := time.Now()
	// jsonb 列不接受空串
	if e.Extra == "" {
		e.Extra = "{}"
	}
	if e.MacExtra == "" {
		e.MacExtra = "[]"
	}
	res := r.db.WithContext(ctx).Model(&entity.ClientDevice{}).
		Where("device_id = ?", e.DeviceId).
		Updates(map[string]interface{}{
			"user_id":       e.UserId,
			"user_name":     e.UserName,
			"name":          e.Name,
			"platform":      e.Platform,
			"os":            e.Os,
			"arch":          e.Arch,
			"app_version":   e.AppVersion,
			"mac_address":   e.MacAddress,
			"mac_extra":     e.MacExtra,
			"ip":            e.Ip,
			"status":        "active", // 重新登录=新会话生效
			"last_login_at": now,
			"last_login_ip": e.LastLoginIp,
			"updated_at":    now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	// 新设备：插入（raw SQL，pgx 编不了 *types.Time，用 now() + time.Time）
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO aa_client_device
		 (device_id, user_id, user_name, name, platform, os, arch, app_version, mac_address, mac_extra, ip, status, last_login_at, last_login_ip, created_at, updated_at, extra)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'active',now(),$12,now(),now(),$13)
		 ON CONFLICT (device_id) DO UPDATE SET
		   user_id=EXCLUDED.user_id, user_name=EXCLUDED.user_name, name=EXCLUDED.name,
		   platform=EXCLUDED.platform, os=EXCLUDED.os, arch=EXCLUDED.arch, app_version=EXCLUDED.app_version,
		   mac_address=EXCLUDED.mac_address, mac_extra=EXCLUDED.mac_extra, ip=EXCLUDED.ip,
		   status='active', last_login_at=now(), last_login_ip=EXCLUDED.last_login_ip,
		   updated_at=now()`,
		e.DeviceId, e.UserId, e.UserName, e.Name, e.Platform, e.Os, e.Arch, e.AppVersion,
		e.MacAddress, e.MacExtra, e.Ip, e.LastLoginIp, e.Extra,
	).Error
}

func (r *ClientDeviceRepository) GetById(ctx context.Context, deviceId string) (*entity.ClientDevice, error) {
	var e entity.ClientDevice
	if err := r.db.WithContext(ctx).Where("device_id = ?", deviceId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// InsertPending 落一条 status=pending 的设备记录（App 打开登录页时即落库，
// 用户尚未在浏览器完成授权，故无 user）。已存在（含 active/revoked）则不动，
// 避免把已激活/已撤销的设备降级回 pending。
// 用 FirstOrCreate 而非 OnConflict DoNothing（后者在 GORM 下偶发 nil 但不插行）。
// InsertPending 落一条 status=pending 的设备记录（App 打开登录页时即落库，
// 用户尚未在浏览器完成授权，故无 user）。已存在（含 active/revoked）则不动，
// 避免把已激活/已撤销的设备降级回 pending。
// 用 raw SQL：pgx v5 二进制协议编不了命名类型 *types.Time（entity 字段），
// 改用 now() + time.Time 参数绕开。
func (r *ClientDeviceRepository) InsertPending(ctx context.Context, e *entity.ClientDevice) error {
	if e.Extra == "" {
		e.Extra = "{}"
	}
	if e.MacExtra == "" {
		e.MacExtra = "[]"
	}
	// 已存在则不动（保留 active/revoked + user/last_login）
	var existing entity.ClientDevice
	err := r.db.WithContext(ctx).Where("device_id = ?", e.DeviceId).First(&existing).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO aa_client_device
		 (device_id, name, platform, os, arch, app_version, mac_address, mac_extra, ip, status, created_at, updated_at, extra)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'pending',now(),now(),$10)
		 ON CONFLICT (device_id) DO NOTHING`,
		e.DeviceId, e.Name, e.Platform, e.Os, e.Arch, e.AppVersion, e.MacAddress, e.MacExtra, e.Ip, e.Extra,
	).Error
}

func (r *ClientDeviceRepository) ListAll(ctx context.Context) ([]*entity.ClientDevice, error) {
	var list []*entity.ClientDevice
	err := r.db.WithContext(ctx).Order("last_login_at desc nulls last, created_at desc").Find(&list).Error
	return list, err
}

func (r *ClientDeviceRepository) UpdateStatus(ctx context.Context, deviceId, status string) error {
	return r.db.WithContext(ctx).Model(&entity.ClientDevice{}).
		Where("device_id = ?", deviceId).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()}).Error
}

func (r *ClientDeviceRepository) UpdateBlocked(ctx context.Context, deviceId string, blocked uint) error {
	return r.db.WithContext(ctx).Model(&entity.ClientDevice{}).
		Where("device_id = ?", deviceId).
		Updates(map[string]interface{}{"blocked": blocked, "updated_at": time.Now()}).Error
}

// UpdateExtra 覆盖 extra jsonb。空串落 "{}"（jsonb 不接受空串）。
func (r *ClientDeviceRepository) UpdateExtra(ctx context.Context, deviceId, extra string) error {
	if extra == "" {
		extra = "{}"
	}
	return r.db.WithContext(ctx).Model(&entity.ClientDevice{}).
		Where("device_id = ?", deviceId).
		Updates(map[string]interface{}{"extra": extra, "updated_at": time.Now()}).Error
}

// UpdateChannels 覆盖设备 channels jsonb（渠道绑定状态数组）。
func (r *ClientDeviceRepository) UpdateChannels(ctx context.Context, deviceId, channels string) error {
	if channels == "" {
		channels = "[]"
	}
	return r.db.WithContext(ctx).Model(&entity.ClientDevice{}).
		Where("device_id = ?", deviceId).
		Updates(map[string]interface{}{"channels": channels, "updated_at": time.Now()}).Error
}

// UpdateSkills 覆盖设备 skills jsonb（已装技能快照数组，心跳上报合并）。
func (r *ClientDeviceRepository) UpdateSkills(ctx context.Context, deviceId, skills string) error {
	if skills == "" {
		skills = "[]"
	}
	return r.db.WithContext(ctx).Model(&entity.ClientDevice{}).
		Where("device_id = ?", deviceId).
		Updates(map[string]interface{}{"skills": skills, "updated_at": time.Now()}).Error
}

func (r *ClientDeviceRepository) Delete(ctx context.Context, deviceId string) error {
	return r.db.WithContext(ctx).Where("device_id = ?", deviceId).Delete(&entity.ClientDevice{}).Error
}

func (r *ClientDeviceRepository) UpdateMcpKeyPlain(ctx context.Context, deviceId, plain string) error {
	return r.db.WithContext(ctx).Model(&entity.ClientDevice{}).
		Where("device_id = ?", deviceId).
		Updates(map[string]interface{}{"mcp_key_plain": plain, "updated_at": time.Now()}).Error
}

// ==================== 设备↔MCP 分组绑定 ====================

type ClientDeviceMcpGroupRepository struct{ db *gorm.DB }

func NewClientDeviceMcpGroupRepository() *ClientDeviceMcpGroupRepository {
	return &ClientDeviceMcpGroupRepository{db: db.ClientDB()}
}

func (r *ClientDeviceMcpGroupRepository) ListByDevice(ctx context.Context, deviceId string) (list []*entity.ClientDeviceMcpGroup, err error) {
	err = r.db.WithContext(ctx).Where("device_id = ?", deviceId).Order("id asc").Find(&list).Error
	return
}

func (r *ClientDeviceMcpGroupRepository) GetByDeviceGroup(ctx context.Context, deviceId, groupId string) (*entity.ClientDeviceMcpGroup, error) {
	var e entity.ClientDeviceMcpGroup
	if err := r.db.WithContext(ctx).Where("device_id = ? AND group_id = ?", deviceId, groupId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ClientDeviceMcpGroupRepository) Add(ctx context.Context, e *entity.ClientDeviceMcpGroup) (string, error) {
	e.DeviceMcpId = idgen.New()
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.DeviceMcpId, nil
}

func (r *ClientDeviceMcpGroupRepository) DeleteByDeviceGroup(ctx context.Context, deviceId, groupId string) error {
	return r.db.WithContext(ctx).Where("device_id = ? AND group_id = ?", deviceId, groupId).Delete(&entity.ClientDeviceMcpGroup{}).Error
}

func (r *ClientDeviceMcpGroupRepository) DeleteByDevice(ctx context.Context, deviceId string) error {
	return r.db.WithContext(ctx).Where("device_id = ?", deviceId).Delete(&entity.ClientDeviceMcpGroup{}).Error
}

// ==================== 安装包缓存 ====================

type ClientPackageRepository struct{ db *gorm.DB }

func NewClientPackageRepository() *ClientPackageRepository {
	return &ClientPackageRepository{db: db.ClientDB()}
}

// Upsert 按 package_id 更新或按 (agent,version,platform) 冲突插入。
func (r *ClientPackageRepository) Upsert(ctx context.Context, e *entity.ClientPackage) error {
	now := time.Now()
	if e.PackageId != "" {
		res := r.db.WithContext(ctx).Model(&entity.ClientPackage{}).
			Where("package_id = ?", e.PackageId).
			Updates(map[string]interface{}{
				"agent":        e.Agent,
				"version":      e.Version,
				"platform":     e.Platform,
				"filename":     e.Filename,
				"file_size":    e.FileSize,
				"sha256":       e.Sha256,
				"official_url": e.OfficialUrl,
				"local_path":   e.LocalPath,
				"status":       e.Status,
				"fetched_by":   e.FetchedBy,
				"fetched_at":   now,
				"updated_at":   now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}
	if e.PackageId == "" {
		e.PackageId = idgen.New()
	}
	// raw SQL：pgx 编不了 *types.Time，用 now() 绕开。
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO aa_client_package
		 (package_id, agent, version, platform, filename, file_size, sha256, official_url, local_path, status, fetched_by, created_at, updated_at, fetched_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now(),now())
		 ON CONFLICT (agent, version, platform) DO UPDATE SET
		   filename=EXCLUDED.filename, file_size=EXCLUDED.file_size, sha256=EXCLUDED.sha256,
		   official_url=EXCLUDED.official_url, local_path=EXCLUDED.local_path, status=EXCLUDED.status,
		   fetched_by=EXCLUDED.fetched_by, fetched_at=now(), updated_at=now()`,
		e.PackageId, e.Agent, e.Version, e.Platform, e.Filename, e.FileSize, e.Sha256,
		e.OfficialUrl, e.LocalPath, e.Status, e.FetchedBy,
	).Error
}

func (r *ClientPackageRepository) ListAll(ctx context.Context) ([]*entity.ClientPackage, error) {
	var list []*entity.ClientPackage
	err := r.db.WithContext(ctx).Order("agent asc, version desc, platform asc").Find(&list).Error
	return list, err
}

func (r *ClientPackageRepository) GetByCVP(ctx context.Context, agent, version, platform string) (*entity.ClientPackage, error) {
	var e entity.ClientPackage
	if err := r.db.WithContext(ctx).
		Where("agent = ? and version = ? and platform = ?", agent, version, platform).
		First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ClientPackageRepository) ListByAgent(ctx context.Context, agent string) ([]*entity.ClientPackage, error) {
	var list []*entity.ClientPackage
	err := r.db.WithContext(ctx).
		Where("agent = ?", agent).
		Order("version desc, platform asc").
		Find(&list).Error
	return list, err
}

func (r *ClientPackageRepository) Delete(ctx context.Context, packageId string) error {
	return r.db.WithContext(ctx).Where("package_id = ?", packageId).Delete(&entity.ClientPackage{}).Error
}

// ==================== 运行日志 ====================

type ClientLogRepository struct{ db *gorm.DB }

func NewClientLogRepository() *ClientLogRepository {
	return &ClientLogRepository{db: db.ClientDB()}
}

// AppendLogs 批量追加日志（每条自动生成 log_id；ts 用 now()）。
// raw SQL：pgx 编不了 *types.Time（ts），用 now() 绕开。
func (r *ClientLogRepository) AppendLogs(ctx context.Context, logs []*entity.ClientLog) error {
	if len(logs) == 0 {
		return nil
	}
	for _, l := range logs {
		if l.LogId == "" {
			l.LogId = idgen.New()
		}
		if l.Level == "" {
			l.Level = "info"
		}
	}
	// 单条单条 Exec（日志量不大，避免拼巨型 SQL）；ts 列 DEFAULT NULL，这里统一 now()
	for _, l := range logs {
		if err := r.db.WithContext(ctx).Exec(
			`INSERT INTO aa_client_log (log_id, device_id, agent, command_id, level, line, ts)
			 VALUES ($1,$2,$3,$4,$5,$6,now())`,
			l.LogId, l.DeviceId, l.Agent, l.CommandId, l.Level, l.Line,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *ClientLogRepository) ListByDevice(ctx context.Context, deviceId, agent string, limit int) ([]*entity.ClientLog, error) {
	if limit <= 0 {
		limit = 200
	}
	var list []*entity.ClientLog
	q := r.db.WithContext(ctx).Where("device_id = ?", deviceId)
	if agent != "" {
		q = q.Where("agent = ?", agent)
	}
	err := q.Order("ts DESC, id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// ListByCommand 某条命令的分步日志，按时间正序（UI 命令历史展开用，展示执行每一步）。
func (r *ClientLogRepository) ListByCommand(ctx context.Context, commandId string, limit int) ([]*entity.ClientLog, error) {
	if commandId == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 200
	}
	var list []*entity.ClientLog
	err := r.db.WithContext(ctx).
		Where("command_id = ?", commandId).
		Order("ts ASC, id ASC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// ==================== 配置文件变更 ====================

type ClientConfigChangeRepository struct{ db *gorm.DB }

func NewClientConfigChangeRepository() *ClientConfigChangeRepository {
	return &ClientConfigChangeRepository{db: db.ClientDB()}
}

func (r *ClientConfigChangeRepository) Insert(ctx context.Context, e *entity.ClientConfigChange) error {
	if e.ChangeId == "" {
		e.ChangeId = idgen.New()
	}
	// raw SQL：pgx 编不了 *types.Time（changed_at/config_mtime），用 now()；config_mtime 传 time.Time
	var mtimeArg interface{}
	if e.ConfigMtime != nil {
		mtimeArg = time.Time(*e.ConfigMtime)
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO aa_client_config_change
		 (change_id, device_id, agent, config_hash, config_size, config_mtime, content, source, source_detail, changed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,now())`,
		e.ChangeId, e.DeviceId, e.Agent, e.ConfigHash, e.ConfigSize, mtimeArg, e.Content, e.Source, e.SourceDetail,
	).Error
}

func (r *ClientConfigChangeRepository) GetLatest(ctx context.Context, deviceId, agent string) (*entity.ClientConfigChange, error) {
	var e entity.ClientConfigChange
	err := r.db.WithContext(ctx).
		Where("device_id = ? and agent = ?", deviceId, agent).
		Order("changed_at DESC, id DESC").
		First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ClientConfigChangeRepository) GetLatestReported(ctx context.Context, deviceId, agent string) (*entity.ClientConfigChange, error) {
	var e entity.ClientConfigChange
	err := r.db.WithContext(ctx).
		Where("device_id = ? and agent = ? and source <> 'app_perm'", deviceId, agent).
		Order("changed_at DESC, id DESC").
		First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ClientConfigChangeRepository) GetLatestBySource(ctx context.Context, deviceId, agent, source string) (*entity.ClientConfigChange, error) {
	var e entity.ClientConfigChange
	err := r.db.WithContext(ctx).
		Where("device_id = ? and agent = ? and source = ?", deviceId, agent, source).
		Order("changed_at DESC, id DESC").
		First(&e).Error
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ClientConfigChangeRepository) ListByDevice(ctx context.Context, deviceId string, limit int) ([]*entity.ClientConfigChange, error) {
	if limit <= 0 {
		limit = 50
	}
	var list []*entity.ClientConfigChange
	err := r.db.WithContext(ctx).
		Where("device_id = ?", deviceId).
		Order("changed_at DESC, id DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// ==================== App 目录（App 权限） ====================

type ClientAppRepository struct{ db *gorm.DB }

func NewClientAppRepository() *ClientAppRepository {
	return &ClientAppRepository{db: db.ClientDB()}
}

func (r *ClientAppRepository) ListAll(ctx context.Context) ([]*entity.ClientApp, error) {
	var list []*entity.ClientApp
	err := r.db.WithContext(ctx).Order("weigh desc, app_name asc").Find(&list).Error
	return list, err
}

func (r *ClientAppRepository) GetByBundleId(ctx context.Context, bundleId string) (*entity.ClientApp, error) {
	var e entity.ClientApp
	if err := r.db.WithContext(ctx).Where("bundle_id = ?", bundleId).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

// Upsert 按 app_id 更新或按 bundle_id 冲突插入。新行用 idgen 生成 app_id。
func (r *ClientAppRepository) Upsert(ctx context.Context, e *entity.ClientApp) error {
	now := time.Now()
	if e.AppId != "" {
		res := r.db.WithContext(ctx).Model(&entity.ClientApp{}).
			Where("app_id = ?", e.AppId).
			Updates(map[string]interface{}{
				"app_name":   e.AppName,
				"logo":       e.Logo,
				"category":   e.Category,
				"weigh":      e.Weigh,
				"remarks":    e.Remarks,
				"updated_at": now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			return nil
		}
	}
	if e.AppId == "" {
		e.AppId = idgen.New()
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "bundle_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"app_name":   e.AppName,
			"logo":       e.Logo,
			"category":   e.Category,
			"weigh":      e.Weigh,
			"remarks":    e.Remarks,
			"updated_at": now,
		}),
	}).Create(e).Error
}

func (r *ClientAppRepository) Delete(ctx context.Context, appId string) error {
	return r.db.WithContext(ctx).Where("app_id = ?", appId).Delete(&entity.ClientApp{}).Error
}

func (r *ClientAppRepository) CountAppPermRefs(ctx context.Context, bundleId string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.ClientAppPerm{}).
		Where("bundle_id = ?", bundleId).Count(&n).Error
	return n, err
}

// ==================== 设备 App 权限配置（App 权限） ====================

type ClientAppPermRepository struct{ db *gorm.DB }

func NewClientAppPermRepository() *ClientAppPermRepository {
	return &ClientAppPermRepository{db: db.ClientDB()}
}

func (r *ClientAppPermRepository) ListByDevice(ctx context.Context, deviceId string) ([]*entity.ClientAppPerm, error) {
	var list []*entity.ClientAppPerm
	err := r.db.WithContext(ctx).
		Where("device_id = ?", deviceId).
		Order("bundle_id asc").
		Find(&list).Error
	return list, err
}

// ReplaceDevice 整组覆盖（后台勾选保存语义）：事务内删掉不在新清单的行（空清单=全删），
// 保留行不动（granted/denied 是客户端上报事实，别被保存清零——客户端状态没变不会重发），
// 新行 OnConflict 跳过（(device_id,bundle_id) 唯一索引）。新行用 idgen 生成 perm_id。
func (r *ClientAppPermRepository) ReplaceDevice(ctx context.Context, deviceId string, bundleIds []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Where("device_id = ?", deviceId)
		if len(bundleIds) > 0 {
			q = tx.Where("device_id = ? AND bundle_id NOT IN ?", deviceId, bundleIds)
		}
		if err := q.Delete(&entity.ClientAppPerm{}).Error; err != nil {
			return err
		}
		for _, b := range bundleIds {
			row := &entity.ClientAppPerm{
				PermId:   idgen.New(),
				DeviceId: deviceId,
				BundleId: b,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "device_id"}, {Name: "bundle_id"}},
				DoNothing: true,
			}).Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateStatesByDevice 授权状态快照落库：逐行按成员关系覆盖 granted/denied。
// 客户端只在状态变化时上报（快照=该设备当前全集），不在任一数组即置 false。
func (r *ClientAppPermRepository) UpdateStatesByDevice(ctx context.Context, deviceId string, granted, denied []string) error {
	rows, err := r.ListByDevice(ctx, deviceId)
	if err != nil {
		return err
	}
	gset := map[string]bool{}
	for _, b := range granted {
		gset[b] = true
	}
	dset := map[string]bool{}
	for _, b := range denied {
		dset[b] = true
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			g, d := gset[row.BundleId], dset[row.BundleId]
			if row.Granted == g && row.Denied == d {
				continue // 无变化不写（省 updated_at 抖动）
			}
			if err := tx.Model(&entity.ClientAppPerm{}).
				Where("id = ?", row.Id).
				Updates(map[string]interface{}{"granted": g, "denied": d}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ClientAppPermRepository) DeleteByDevice(ctx context.Context, deviceId string) error {
	return r.db.WithContext(ctx).Where("device_id = ?", deviceId).Delete(&entity.ClientAppPerm{}).Error
}
