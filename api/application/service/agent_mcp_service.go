package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/crypto"
)

// agent_mcp_service.go — docker 实例（openclaw）接入 MCP 网关。
//
// 绑定模型：实例有效分组 = 直绑（aa_agent_instance_mcp_group）∪ 所属项目的 mcp_group 绑定
// （aa_project_binding kind=agent 指到本实例的项目，其 kind=mcp_group 且 enabled=1 的绑定）。
// 有效分组变化（直绑/解绑、项目绑定增删改）→ SyncInstance：
//   - 非空 → 无 key 则签实例专属 Bearer key（AccessType=groups，AllowedGroups=组名集），明文落
//     实例行（密钥库只存哈希，配置重推随时要用）；已有 key 只改 AllowedGroups（token 不换 →
//     key_id 稳定 → 网关调用审计按 key 归因稳定）。
//   - 空 → 删 key + 清明文。不走 Edit 置 enabled=0：Bearer key 的 Edit 是 struct Updates，
//     零值字段会被 gorm 跳过，停用落不了库；删除即失效，重绑再签也自洽。
//   - 读最近一次 config_push 的现有配置，仅替换/删除顶层 mcp 段后重推（reload）——
//     ak/models/channels/plugins/meta 原样保留，避开 openclaw 配置守卫回滚。
//
// 全量重建路径（provision/RefreshLLMKey 走 deliverAgentConfig）由 InstanceMcpServersFor
// 补上 mcp 段。两种 agent 落点不同：
//   - openclaw：mcp.servers JSON 段（带 transport 键），增量重推走 repushMcpSection 读改写
//     （保 ak/models/channels/plugins/meta 原样，避开配置守卫回滚）；
//   - hermes：config.yaml 顶层 mcp_servers 段（hermes 原生 streamable-http，不带 transport 键），
//     增量重推走 AgentInstanceService.SyncHermesMcpSection 全量重建（整文件平台生成、无守卫）。
// Bind/SyncInstance 对 openclaw/hermes 均开放，按 ins.Agent 分派。

// InstanceMcpGroupView 实例 MCP 抽屉列表行：分组 + 来源 + 组内 server 概览。
type InstanceMcpGroupView struct {
	GroupId      string              `json:"groupId"`
	GroupName    string              `json:"groupName"`
	Description  string              `json:"description"`
	Source       string              `json:"source"`       // direct=直绑 / project=来自项目
	ProjectNames []string            `json:"projectNames"` // source=project 时的来源项目名
	Servers      []GroupServerConfig `json:"servers"`
	CanUnbind    bool                `json:"canUnbind"` // 直绑行：当前身份可解（可使用该组 ∨ 本人绑的）；项目行恒 false
}

type AgentMcpService struct {
	instanceRepo repository.IAgentInstanceRepository
	groupRepo    repository.IAgentInstanceMcpGroupRepository
	keySvc       *McpKeyService
	mcpGroupSvc  *McpGroupService
	bindingRepo  repository.IProjectBindingRepository
	projectRepo  repository.IProjectRepository
	cmdRepo      repository.IAgentInstanceCommandRepository
	instanceSvc  *AgentInstanceService   // 复用 pushOpenclawConfigRaw（main.go 装配后回指，防构造循环）
	runtimeSvc   *AgentRuntimeService    // 绑定即配手册：SyncInstance 尾部自动装配对技能（main.go 回指，防构造循环）
	channelNotice *ChannelNoticeService  // 分组 server 名单变化：受影响实例归属人双通道通知（main.go 回指）
	cfg          *config.Config
}

func NewAgentMcpService(
	ir repository.IAgentInstanceRepository,
	gr repository.IAgentInstanceMcpGroupRepository,
	ks *McpKeyService,
	mg *McpGroupService,
	br repository.IProjectBindingRepository,
	pr repository.IProjectRepository,
	cr repository.IAgentInstanceCommandRepository,
	ins *AgentInstanceService,
	cfg *config.Config,
) *AgentMcpService {
	return &AgentMcpService{
		instanceRepo: ir, groupRepo: gr, keySvc: ks, mcpGroupSvc: mg,
		bindingRepo: br, projectRepo: pr, cmdRepo: cr, instanceSvc: ins, cfg: cfg,
	}
}

// SetRuntimeService main.go 装配后回指（防构造循环）：绑定成功后自动补装配对内置技能用。
func (s *AgentMcpService) SetRuntimeService(rs *AgentRuntimeService) { s.runtimeSvc = rs }

// SetChannelNotice main.go 装配后回指：分组 server 名单变化的通知出口（收件人=实例归属人）。
func (s *AgentMcpService) SetChannelNotice(c *ChannelNoticeService) { s.channelNotice = c }

// instanceMcpBinding 有效分组条目（分组实体 + 来源）。
type instanceMcpBinding struct {
	group      *entity.McpGroup
	source     string // direct / project
	projectIds []string
}

// effective 计算实例有效分组：直绑 ∪ 项目继承（按 group_id 去重，直绑优先，剔除已删组）。
func (s *AgentMcpService) effective(ctx context.Context, instanceId string) ([]instanceMcpBinding, error) {
	live, err := s.mcpGroupSvc.All(ctx) // 已滤 deleted_at
	if err != nil {
		return nil, err
	}
	byId := make(map[string]*entity.McpGroup, len(live))
	for _, g := range live {
		byId[g.GroupId] = g
	}

	var out []instanceMcpBinding
	seen := map[string]int{} // group_id → out 下标
	add := func(g *entity.McpGroup, source, projectId string) {
		if g == nil {
			return
		}
		if i, ok := seen[g.GroupId]; ok {
			// 已有（直绑优先）：项目继承不覆盖来源，但补记来源项目
			if source == "project" && projectId != "" {
				out[i].projectIds = append(out[i].projectIds, projectId)
			}
			return
		}
		seen[g.GroupId] = len(out)
		b := instanceMcpBinding{group: g, source: source}
		if source == "project" {
			b.projectIds = []string{projectId}
		}
		out = append(out, b)
	}

	direct, err := s.groupRepo.ListByInstance(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	for _, row := range direct {
		add(byId[row.GroupId], "direct", "")
	}

	agentBindings, err := s.bindingRepo.ListByTarget(ctx, "agent", instanceId)
	if err != nil {
		return nil, err
	}
	for _, ab := range agentBindings {
		if ab.Enabled == 0 {
			continue
		}
		pbs, err := s.bindingRepo.ListByProject(ctx, ab.ProjectId)
		if err != nil {
			return nil, err
		}
		for _, pb := range pbs {
			if pb.Kind != "mcp_group" || pb.Enabled == 0 {
				continue
			}
			add(byId[pb.TargetId], "project", ab.ProjectId)
		}
	}
	return out, nil
}

// List 实例 MCP 绑定列表（抽屉）：有效分组 + 来源标注 + 组内 server 概览。
// t 非-nil 时回填 canUnbind：直绑行 = 可使用该组 ∨ 本人创建的绑定行；项目继承行恒 false。
func (s *AgentMcpService) List(ctx context.Context, instanceId string, t *model.Identity) ([]InstanceMcpGroupView, error) {
	bindings, err := s.effective(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	// 直绑行 CreatedBy 索引（canUnbind 判定用）
	directRows := map[string]string{} // group_id → created_by
	if t != nil {
		if rows, err := s.groupRepo.ListByInstance(ctx, instanceId); err == nil {
			for _, row := range rows {
				directRows[row.GroupId] = row.CreatedBy
			}
		}
	}
	// 项目名按需解析（来源展示用）
	projNames := map[string]string{}
	views := make([]InstanceMcpGroupView, 0, len(bindings))
	for _, b := range bindings {
		v := InstanceMcpGroupView{
			GroupId:     b.group.GroupId,
			GroupName:   b.group.Name,
			Description: b.group.Description,
			Source:      b.source,
		}
		if b.source == "direct" {
			if t == nil {
				v.CanUnbind = true
			} else {
				v.CanUnbind = s.mcpGroupSvc.canUseGroup(ctx, t, b.group) || directRows[b.group.GroupId] == t.UserId
			}
		}
		for _, pid := range b.projectIds {
			name, ok := projNames[pid]
			if !ok {
				if p, err := s.projectRepo.Get(ctx, pid); err == nil && p != nil {
					name = p.Name
				} else {
					name = pid
				}
				projNames[pid] = name
			}
			v.ProjectNames = append(v.ProjectNames, name)
		}
		servers, err := s.mcpGroupSvc.GetServerConfigs(b.group)
		if err != nil {
			log.Printf("[instance:%s] 解析分组 %s servers 失败: %v", instanceId, b.group.Name, err)
		} else {
			v.Servers = servers
		}
		views = append(views, v)
	}
	return views, nil
}

// Bind 直绑分组：可使用校验（t 非-nil）→ 写关系表（幂等）→ SyncInstance 签 key + 重推配置。
func (s *AgentMcpService) Bind(ctx context.Context, instanceId, groupId, userId string, t *model.Identity) error {
	ins, err := s.instanceRepo.Get(ctx, instanceId)
	if err != nil {
		return err
	}
	if a := agentAlias(ins.Agent); a != "openclaw" && a != "hermes" {
		return errors.New("该类型实例暂不支持 MCP 网关")
	}
	g, err := s.mcpGroupSvc.assertGroupUsable(ctx, groupId, t)
	if err != nil {
		return err
	}
	if existing, err := s.groupRepo.GetByInstanceGroup(ctx, instanceId, groupId); err != nil || existing == nil {
		// 未绑过才插入（幂等；真 DB 错误会在 Add 处撞唯一索引/原样报错）
		if _, err := s.groupRepo.Add(ctx, &entity.AgentInstanceMcpGroup{
			InstanceId: instanceId, GroupId: groupId, GroupName: g.Name, CreatedBy: userId,
		}); err != nil {
			return err
		}
	}
	return s.SyncInstance(ctx, instanceId, userId)
}

// Unbind 解绑直绑分组（项目继承的分组不可在此解，返回提示）→ SyncInstance。
// 权限：可使用该组 ∨ 该绑定行是本人创建的（t nil=系统路径不限）。
func (s *AgentMcpService) Unbind(ctx context.Context, instanceId, groupId, userId string, t *model.Identity) error {
	bindings, err := s.effective(ctx, instanceId)
	if err != nil {
		return err
	}
	var target instanceMcpBinding // direct 行才允许解
	var found bool
	for _, b := range bindings {
		if b.group.GroupId != groupId {
			continue
		}
		if b.source != "direct" {
			return errors.New("该分组来自项目绑定，请在项目里解绑")
		}
		target = b
		found = true
	}
	if t != nil && found {
		row, rerr := s.groupRepo.GetByInstanceGroup(ctx, instanceId, groupId)
		if rerr == nil && row != nil {
			if !s.mcpGroupSvc.canUseGroup(ctx, t, target.group) && row.CreatedBy != t.UserId {
				return errors.New("没有分组的访问权限")
			}
		}
	}
	if err := s.groupRepo.DeleteByInstanceGroup(ctx, instanceId, groupId); err != nil {
		return err
	}
	return s.SyncInstance(ctx, instanceId, userId)
}

// AutoBindPermitted 创建实例时自动绑定归属人「可使用」的全部分组（provision 挂钩，先于
// deliverAgentConfig，使初始配置带上 mcp 段）。best-effort：失败只记日志，不阻断建实例主链路。
// 绑定行 CreatedBy=归属人 → 此后即使权限被回收，归属人仍可按「本人创建的绑定行」解绑。
func (s *AgentMcpService) AutoBindPermitted(ctx context.Context, instanceId, ownerId string) {
	if ownerId == "" {
		return
	}
	ins, err := s.instanceRepo.Get(ctx, instanceId)
	if err != nil || ins == nil {
		return
	}
	if a := agentAlias(ins.Agent); a != "openclaw" && a != "hermes" {
		return
	}
	t := s.mcpGroupSvc.IdentityOf(ctx, ownerId)
	groups, err := s.mcpGroupSvc.PermittedGroups(ctx, t)
	if err != nil {
		log.Printf("[instance:%s] 自动绑定 MCP 分组失败（读可见分组）: %v", instanceId, err)
		return
	}
	added := 0
	for _, g := range groups {
		if existing, err := s.groupRepo.GetByInstanceGroup(ctx, instanceId, g.GroupId); err != nil || existing == nil {
			if _, err := s.groupRepo.Add(ctx, &entity.AgentInstanceMcpGroup{
				InstanceId: instanceId, GroupId: g.GroupId, GroupName: g.Name, CreatedBy: ownerId,
			}); err != nil {
				log.Printf("[instance:%s] 自动绑定分组 %s 失败: %v", instanceId, g.Name, err)
				continue
			}
			added++
		}
	}
	if added == 0 {
		return
	}
	log.Printf("[instance:%s] 已自动绑定 %d 个 MCP 分组（按归属人 %s 权限）", instanceId, added, ownerId)
	if err := s.SyncInstance(ctx, instanceId, ownerId); err != nil {
		log.Printf("[instance:%s] 自动绑定后收敛 MCP 失败: %v", instanceId, err)
	}
}

// SyncInstance 把实例的 key/配置收敛到当前有效分组（绑/解绑/项目绑定变化的统一出口）。
func (s *AgentMcpService) SyncInstance(ctx context.Context, instanceId, issuedBy string) error {
	ins, err := s.instanceRepo.Get(ctx, instanceId)
	if err != nil {
		return err
	}
	bindings, err := s.effective(ctx, instanceId)
	if err != nil {
		return err
	}

	if len(bindings) == 0 {
		if ins.McpKeyPlain != "" {
			if k, err := s.keySvc.GetByHash(ctx, crypto.HashToken(ins.McpKeyPlain)); err == nil && k != nil {
				if err := s.keySvc.Delete(ctx, []string{k.KeyId}); err != nil {
					return fmt.Errorf("清理实例 MCP key 失败: %w", err)
				}
			}
			if err := s.instanceRepo.UpdateMcpKeyPlain(ctx, instanceId, ""); err != nil {
				return err
			}
		}
		return s.repushFor(ctx, ins.Agent, instanceId, issuedBy, nil, "")
	}

	names := make([]string, 0, len(bindings))
	for _, b := range bindings {
		names = append(names, b.group.Name)
	}
	allowed, _ := json.Marshal(names)

	plain := ins.McpKeyPlain
	if plain != "" {
		// key 被密钥页手动删过则重签（哈希反查不到）；在则只对齐 AllowedGroups，token 不换。
		if k, err := s.keySvc.GetByHash(ctx, crypto.HashToken(plain)); err != nil || k == nil {
			plain = ""
		} else if k.AllowedGroups != string(allowed) {
			k.AllowedGroups = string(allowed)
			if err := s.keySvc.Edit(ctx, k); err != nil {
				return fmt.Errorf("更新 key 分组范围失败: %w", err)
			}
		}
	}
	if plain == "" {
		_, p, err := s.keySvc.Create(ctx, &entity.McpBearerKey{
			Name: "instance:" + instanceId, Kind: "agent", Owner: ins.UserId,
			AccessType: "groups", AllowedGroups: string(allowed), Enabled: 1, CreatedBy: issuedBy,
		})
		if err != nil {
			return fmt.Errorf("签发实例 MCP key 失败: %w", err)
		}
		plain = p
		if err := s.instanceRepo.UpdateMcpKeyPlain(ctx, instanceId, plain); err != nil {
			return err
		}
	}

	if err := s.repushFor(ctx, ins.Agent, instanceId, issuedBy, names, plain); err != nil {
		return err
	}
	s.pairDomainSkills(ctx, bindings, instanceId, issuedBy)
	return nil
}

// pairDomainSkills 绑定成功后自动补装配对内置技能：有效分组里每个 platform-<域> server
// （读写端点同域）映射 skills-builtin 的 aiarsenal-<域> 技能，交给 runtimeSvc 装缺失的。
// best-effort——不返回错误，绑定本身已成功，手册补装失败只记日志。
func (s *AgentMcpService) pairDomainSkills(ctx context.Context, bindings []instanceMcpBinding, instanceId, issuedBy string) {
	if s.runtimeSvc == nil {
		return
	}
	domains := map[string]struct{}{}
	for _, b := range bindings {
		servers, err := s.mcpGroupSvc.GetServerConfigs(b.group)
		if err != nil {
			continue
		}
		for _, sv := range servers {
			if d := strings.TrimPrefix(sv.Name, "platform-"); d != sv.Name {
				domains[strings.TrimSuffix(d, "-write")] = struct{}{} // 读写端点同域配同一份手册
			}
		}
	}
	if len(domains) == 0 {
		return
	}
	uniq := make([]string, 0, len(domains))
	for d := range domains {
		uniq = append(uniq, d)
	}
	sort.Strings(uniq)
	s.runtimeSvc.InstallPairedSkills(ctx, instanceId, uniq, issuedBy)
}

// repushFor 按 agent 分派 mcp 段重推：openclaw→repushMcpSection（JSON 读改写），
// hermes→instanceSvc.SyncHermesMcpSection（config.yaml 全量重建，复用历史 key/default）。
// groupNames=nil 表示绑定已清空（重推为无 mcp 段的配置）。
func (s *AgentMcpService) repushFor(ctx context.Context, agent, instanceId, issuedBy string, groupNames []string, plainKey string) error {
	if agentAlias(agent) == "hermes" {
		if s.instanceSvc == nil {
			return nil
		}
		var servers map[string]interface{}
		if groupNames != nil {
			servers = HermesMcpServerEntries(s.cfg.McpBaseURL(), groupNames, plainKey)
		}
		return s.instanceSvc.SyncHermesMcpSection(ctx, instanceId, issuedBy, servers)
	}
	var servers map[string]interface{}
	if groupNames != nil {
		servers = McpServerEntries(s.cfg.McpBaseURL(), groupNames, plainKey)
	}
	return s.repushMcpSection(ctx, instanceId, issuedBy, servers)
}

// repushMcpSection 读最近一次 config_push 的现有配置，替换/删除顶层 mcp 段后重推（reload=true）。
// 无历史 payload（provision 未完成）跳过——绑定入口在详情抽屉，能绑即已下发过；下次全量下发自动带上。
func (s *AgentMcpService) repushMcpSection(ctx context.Context, instanceId, issuedBy string, servers map[string]interface{}) error {
	payloadJSON, err := s.cmdRepo.GetLastConfigPushPayload(ctx, instanceId)
	if err != nil {
		log.Printf("[instance:%s] 读历史 config_push 失败，跳过 mcp 段同步: %v", instanceId, err)
		return nil
	}
	if payloadJSON == "" {
		log.Printf("[instance:%s] 无历史 config_push，跳过 mcp 段同步（下次全量下发自动带上）", instanceId)
		return nil
	}
	var payload struct {
		ConfigContent string `json:"configContent"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		log.Printf("[instance:%s] 解析 config_push payload 失败，跳过 mcp 段同步: %v", instanceId, err)
		return nil
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(payload.ConfigContent), &cfg); err != nil {
		log.Printf("[instance:%s] 解析历史配置失败，跳过 mcp 段同步: %v", instanceId, err)
		return nil
	}
	if servers == nil {
		if _, ok := cfg["mcp"]; !ok {
			return nil // 本来就没有 mcp 段且已无绑定，无需重推
		}
		delete(cfg, "mcp")
	} else {
		cfg["mcp"] = map[string]interface{}{"servers": servers}
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// 缩进两空格，与 BuildOpenclawConfig 一致（openclaw 配置守卫要求，否则下发被回滚）。
	s.instanceSvc.pushOpenclawConfigRaw(ctx, instanceId, issuedBy, "dispatch:mcp-sync", string(out), true)
	log.Printf("[instance:%s] 已同步 mcp.servers 并重推 openclaw 配置（%d 个分组）", instanceId, len(servers))
	return nil
}

// InstanceMcpServersFor 全量重建配置（provision/RefreshLLMKey）时按 agent 补 mcp 段：
// openclaw→mcp.servers（带 transport），hermes→顶层 mcp_servers（不带）。
// best-effort：agent 不符/无绑定/未签 key/出错一律 nil，不阻断主链路。
func (s *AgentMcpService) InstanceMcpServersFor(ctx context.Context, instanceId, agent string) map[string]interface{} {
	ins, err := s.instanceRepo.Get(ctx, instanceId)
	if err != nil || ins == nil || agentAlias(ins.Agent) != agentAlias(agent) || ins.McpKeyPlain == "" {
		return nil
	}
	bindings, err := s.effective(ctx, instanceId)
	if err != nil || len(bindings) == 0 {
		return nil
	}
	names := make([]string, 0, len(bindings))
	for _, b := range bindings {
		names = append(names, b.group.Name)
	}
	return mcpServerEntries(s.cfg.McpBaseURL(), names, ins.McpKeyPlain, agentAlias(agent) != "hermes")
}

// OnInstancesRemoved 删实例清理：MCP key 删掉 + 明文清空 + 直绑关系清空（Remove 挂钩，best-effort）。
func (s *AgentMcpService) OnInstancesRemoved(ctx context.Context, ids []string) {
	for _, id := range ids {
		ins, err := s.instanceRepo.Get(ctx, id)
		if err != nil || ins == nil {
			continue
		}
		if ins.McpKeyPlain != "" {
			if k, err := s.keySvc.GetByHash(ctx, crypto.HashToken(ins.McpKeyPlain)); err == nil && k != nil {
				_ = s.keySvc.Delete(ctx, []string{k.KeyId})
			}
			_ = s.instanceRepo.UpdateMcpKeyPlain(ctx, id, "")
		}
		_ = s.groupRepo.DeleteByInstance(ctx, id)
	}
}

// OnBindingsChanged 项目绑定增删改后的联动：找受影响实例逐个 SyncInstance（best-effort，只记日志）。
// kind=agent → target 实例本身；kind=mcp_group → 该项目全部 agent 绑定的实例。
func (s *AgentMcpService) OnBindingsChanged(ctx context.Context, affected []*entity.ProjectBinding, by string) {
	if s.instanceSvc == nil {
		return
	}
	instances := map[string]bool{}
	for _, b := range affected {
		if b == nil {
			continue
		}
		switch b.Kind {
		case "agent":
			if b.Enabled == 1 {
				instances[b.TargetId] = true
			} else {
				// 禁用的绑定不再继承 → 也要重算（target 仍受影响）
				instances[b.TargetId] = true
			}
		case "mcp_group":
			pbs, err := s.bindingRepo.ListByProject(ctx, b.ProjectId)
			if err != nil {
				log.Printf("[mcp-sync] 读项目 %s 绑定失败: %v", b.ProjectId, err)
				continue
			}
			for _, pb := range pbs {
				if pb.Kind == "agent" {
					instances[pb.TargetId] = true
				}
			}
		}
	}
	for id := range instances {
		if err := s.SyncInstance(ctx, id, by); err != nil {
			log.Printf("[mcp-sync] 实例 %s 同步 MCP 失败: %v", id, err)
		}
	}
}

// OnGroupServersChanged 分组内 server 名单变化后的联动。分组是共享资源，事件方向与绑定相反：
// 绑定动作单实例就地重算（SyncInstance 尾部），分组编辑要反查全部消费者——直绑该组的实例 ∪
// 项目继承该组的实例——逐个按「全部有效分组」重算配对手册（幂等补装缺失，只加不减；
// 不能只看被编辑的组：某域从本组移除但别的组仍有，手册该留），并向实例归属人双通道通知
// 工具变化（站内铃铛 + 名下 hermes 数字人转微信）。
// 只补手册不签 key/不重推配置：下发配置里只有聚合端点，server 清单由网关按组名现算，
// 工具已即时生效；重推反而要 restart gateway 打断会话。best-effort，失败只记日志。
func (s *AgentMcpService) OnGroupServersChanged(ctx context.Context, groupId, by string, added, removed []string) {
	instances := map[string]bool{}
	if rows, err := s.groupRepo.ListByGroup(ctx, groupId); err == nil {
		for _, row := range rows {
			instances[row.InstanceId] = true
		}
	} else {
		log.Printf("[mcp-sync] 反查分组 %s 直绑实例失败: %v", groupId, err)
	}
	// 项目继承：绑了该分组的项目（kind=mcp_group 且 enabled）→ 项目内全部 agent 绑定的实例。
	pbs, err := s.bindingRepo.ListByTarget(ctx, "mcp_group", groupId)
	if err != nil {
		log.Printf("[mcp-sync] 读分组 %s 的项目绑定失败: %v", groupId, err)
	}
	for _, pb := range pbs {
		if pb.Enabled == 0 {
			continue
		}
		abs, err := s.bindingRepo.ListByProject(ctx, pb.ProjectId)
		if err != nil {
			continue
		}
		for _, ab := range abs {
			if ab.Kind == "agent" && ab.Enabled == 1 {
				instances[ab.TargetId] = true
			}
		}
	}
	for id := range instances {
		bindings, err := s.effective(ctx, id)
		if err != nil {
			log.Printf("[mcp-sync] 实例 %s 重算有效分组失败: %v", id, err)
		} else {
			s.pairDomainSkills(ctx, bindings, id, by)
		}
		s.notifyGroupServerChange(ctx, id, added, removed)
	}
}

// notifyGroupServerChange 实例 MCP 工具变化双通道通知（收件人=实例归属人）。
func (s *AgentMcpService) notifyGroupServerChange(ctx context.Context, instanceId string, added, removed []string) {
	if s.channelNotice == nil {
		return
	}
	ins, err := s.instanceRepo.Get(ctx, instanceId)
	if err != nil || ins == nil {
		return
	}
	var parts []string
	if len(added) > 0 {
		parts = append(parts, "新增 "+strings.Join(added, "、"))
	}
	if len(removed) > 0 {
		parts = append(parts, "移除 "+strings.Join(removed, "、"))
	}
	content := fmt.Sprintf("实例「%s」绑定的 MCP 分组 server 名单调整：%s。工具即时生效，缺失的配套手册将自动补装。",
		ins.Name, strings.Join(parts, "；"))
	s.channelNotice.Push(ctx, ins.UserId, "实例 MCP 工具已变化", content, "/agent/instance/list", "info", "mcp-group")
}

// mcpServerEntries 构造聚合端点 server 表：key=组名（网关 scope），url 走 /mcp/{组名}，
// Bearer 用各自专属 key 明文。withTransport：openclaw mcp.servers 需 transport: streamable-http；
// hermes 顶层 mcp_servers 原生 streamable-http 不认该键，须省略。
// key 遇 "all"/"*" 加 aa- 前缀：hermes 把 server key 注册为 toolset 别名，而这俩是它
// 内置的「全部工具」特殊别名——撞名后解析工具集无限递归，实例一收消息就崩（2026-09-12 踩过）。
// 只改下发 key 不动 url：/mcp/{组名} 是网关聚合 scope，与 agent 侧 server key 无关。
func mcpServerEntries(baseURL string, groupNames []string, plainKey string, withTransport bool) map[string]interface{} {
	servers := make(map[string]interface{}, len(groupNames))
	for _, name := range groupNames {
		key := name
		if name == "all" || name == "*" {
			key = "aa-" + name
		}
		entry := map[string]interface{}{
			"url":     baseURL + "/mcp/" + name,
			"headers": map[string]interface{}{"Authorization": "Bearer " + plainKey},
		}
		if withTransport {
			entry["transport"] = "streamable-http"
		}
		servers[key] = entry
	}
	return servers
}

// McpServerEntries 构造 openclaw mcp.servers 段（实例/设备两条链路共用）：
// key=组名（网关 scope），url 走 /mcp/{组名} 聚合端点，Bearer 用各自专属 key 明文。
func McpServerEntries(baseURL string, groupNames []string, plainKey string) map[string]interface{} {
	return mcpServerEntries(baseURL, groupNames, plainKey, true)
}

// HermesMcpServerEntries 构造 hermes config.yaml 顶层 mcp_servers 段：
// 与 openclaw 版同构，仅不带 transport 键（hermes 原生 streamable-http）。
func HermesMcpServerEntries(baseURL string, groupNames []string, plainKey string) map[string]interface{} {
	return mcpServerEntries(baseURL, groupNames, plainKey, false)
}
