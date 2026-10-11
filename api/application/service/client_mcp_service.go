package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/crypto"
)

// client_mcp_service.go — 桌面设备（client 模块，openclaw）接入 MCP 网关。
//
// 与 docker 实例的差异：设备配置不落服务端、随 install/restart/start 命令全量重建下发
// （issueOpenclawConfig 覆盖写）。因此绑定变化不走读改写重推，而是：
//   签/改/删设备专属 Bearer key（与实例同规则：非空才签、token 不换只对齐 AllowedGroups、
//   清空直接删 key）→ 若设备已装 openclaw/hermes，尝试 enqueue restart——restart 触发全量重建，
//   经 DeviceMcpServers 自动带上/去掉 mcp 段（openclaw→mcp.servers，hermes→mcp_servers）。
//   enqueue 失败（设备忙/离线/未装）不影响绑定：下次 install/start/restart 自然带最新绑定。
//   绑定还触发 pairDomainSkills：platform-<域> server 配对 aiarsenal-<域> 内置技能手册，
//   对设备已装 agent 补装缺失的（镜像实例侧 pairDomainSkills）。
// 设备侧 v1 仅直绑（无项目继承）。hermes 的 mcp_servers 注入 2026-09-19 补齐
//（issueHermesConfig 经 deviceMcpServers 带段；此前设备 hermes 无 MCP 工具，agent 只能空答）。

// DeviceMcpGroupView 设备 MCP 抽屉列表行：分组 + 组内 server 概览。
type DeviceMcpGroupView struct {
	GroupId     string              `json:"groupId"`
	GroupName   string              `json:"groupName"`
	Description string              `json:"description"`
	Servers     []GroupServerConfig `json:"servers"`
	CanUnbind   bool                `json:"canUnbind"` // 当前身份可解（可使用该组 ∨ 本人绑的；设备全直绑）
}

type ClientMcpService struct {
	deviceRepo  repository.IClientDeviceRepository
	statusRepo  repository.IClientAgentStatusRepository
	groupRepo   repository.IClientDeviceMcpGroupRepository
	keySvc      *McpKeyService
	mcpGroupSvc *McpGroupService
	clientSvc   *ClientService // 复用 AdminEnqueueCommand 下发 restart（main.go 装配后回指，防构造循环）
	cfg         *config.Config
}

func NewClientMcpService(
	dr repository.IClientDeviceRepository,
	sr repository.IClientAgentStatusRepository,
	gr repository.IClientDeviceMcpGroupRepository,
	ks *McpKeyService,
	mg *McpGroupService,
	cs *ClientService,
	cfg *config.Config,
) *ClientMcpService {
	return &ClientMcpService{
		deviceRepo: dr, statusRepo: sr, groupRepo: gr, keySvc: ks, mcpGroupSvc: mg, clientSvc: cs, cfg: cfg,
	}
}

// DeviceMcpOption 设备自助「可绑分组」选项（id+名，桌面端下拉用；VisibleAll 已按权限过滤）。
type DeviceMcpOption struct {
	GroupId string `json:"groupId"`
	Name    string `json:"name"`
}

// DeviceMcpSelfView 设备自助一次返回：bound（含 canUnbind）+ options（当前身份可见分组），省一次往返。
type DeviceMcpSelfView struct {
	Bound   []DeviceMcpGroupView `json:"bound"`
	Options []DeviceMcpOption    `json:"options"`
}

// SelfView 桌面客户端 MCP 管理模块视图（列表 + 可绑选项一次取齐）。
func (s *ClientMcpService) SelfView(ctx context.Context, deviceId string, t *model.Identity) (*DeviceMcpSelfView, error) {
	bound, err := s.List(ctx, deviceId, t)
	if err != nil {
		return nil, err
	}
	groups, err := s.mcpGroupSvc.VisibleAll(ctx, t)
	if err != nil {
		return nil, err
	}
	options := make([]DeviceMcpOption, 0, len(groups))
	for _, g := range groups {
		options = append(options, DeviceMcpOption{GroupId: g.GroupId, Name: g.Name})
	}
	return &DeviceMcpSelfView{Bound: bound, Options: options}, nil
}

// effective 设备有效分组 = 直绑（按 group_id 去重，剔除已删组）。
func (s *ClientMcpService) effective(ctx context.Context, deviceId string) ([]*entity.McpGroup, error) {
	live, err := s.mcpGroupSvc.All(ctx)
	if err != nil {
		return nil, err
	}
	byId := make(map[string]*entity.McpGroup, len(live))
	for _, g := range live {
		byId[g.GroupId] = g
	}
	rows, err := s.groupRepo.ListByDevice(ctx, deviceId)
	if err != nil {
		return nil, err
	}
	var out []*entity.McpGroup
	for _, row := range rows {
		if g, ok := byId[row.GroupId]; ok {
			out = append(out, g)
		}
	}
	return out, nil
}

// List 设备 MCP 绑定列表（抽屉）。t 非-nil 时回填 canUnbind（设备全直绑：可使用 ∨ 本人绑的）。
func (s *ClientMcpService) List(ctx context.Context, deviceId string, t *model.Identity) ([]DeviceMcpGroupView, error) {
	groups, err := s.effective(ctx, deviceId)
	if err != nil {
		return nil, err
	}
	// 直绑行 CreatedBy 索引（canUnbind 判定用）
	rowOwners := map[string]string{} // group_id → created_by
	if t != nil {
		if rows, err := s.groupRepo.ListByDevice(ctx, deviceId); err == nil {
			for _, row := range rows {
				rowOwners[row.GroupId] = row.CreatedBy
			}
		}
	}
	views := make([]DeviceMcpGroupView, 0, len(groups))
	for _, g := range groups {
		v := DeviceMcpGroupView{GroupId: g.GroupId, GroupName: g.Name, Description: g.Description}
		if t == nil {
			v.CanUnbind = true
		} else {
			v.CanUnbind = s.mcpGroupSvc.canUseGroup(ctx, t, g) || rowOwners[g.GroupId] == t.UserId
		}
		servers, err := s.mcpGroupSvc.GetServerConfigs(g)
		if err != nil {
			log.Printf("[device:%s] 解析分组 %s servers 失败: %v", deviceId, g.Name, err)
		} else {
			v.Servers = servers
		}
		views = append(views, v)
	}
	return views, nil
}

// Bind 设备直绑分组：可使用校验（t 非-nil）→ 写关系表（幂等）→ SyncDevice。
func (s *ClientMcpService) Bind(ctx context.Context, deviceId, groupId, userId, llmBaseURL string, t *model.Identity) error {
	if _, err := s.deviceRepo.GetById(ctx, deviceId); err != nil {
		return errors.New("设备不存在")
	}
	g, err := s.mcpGroupSvc.assertGroupUsable(ctx, groupId, t)
	if err != nil {
		return err
	}
	if existing, err := s.groupRepo.GetByDeviceGroup(ctx, deviceId, groupId); err != nil || existing == nil {
		// 未绑过才插入（幂等；真 DB 错误会在 Add 处撞唯一索引/原样报错）
		if _, err := s.groupRepo.Add(ctx, &entity.ClientDeviceMcpGroup{
			DeviceId: deviceId, GroupId: groupId, GroupName: g.Name, CreatedBy: userId,
		}); err != nil {
			return err
		}
	}
	return s.SyncDevice(ctx, deviceId, userId, llmBaseURL)
}

// Unbind 解绑直绑分组：权限 = 可使用该组 ∨ 绑定行本人创建（t nil=系统路径不限）→ SyncDevice。
func (s *ClientMcpService) Unbind(ctx context.Context, deviceId, groupId, userId, llmBaseURL string, t *model.Identity) error {
	if t != nil {
		if row, rerr := s.groupRepo.GetByDeviceGroup(ctx, deviceId, groupId); rerr == nil && row != nil {
			g, gerr := s.mcpGroupSvc.Get(ctx, groupId)
			if gerr != nil || g == nil || (!s.mcpGroupSvc.canUseGroup(ctx, t, g) && row.CreatedBy != t.UserId) {
				return errors.New("没有分组的访问权限")
			}
		}
	}
	if err := s.groupRepo.DeleteByDeviceGroup(ctx, deviceId, groupId); err != nil {
		return err
	}
	return s.SyncDevice(ctx, deviceId, userId, llmBaseURL)
}

// AutoBindPermitted 设备首次登录时自动绑定归属人「可使用」的全部分组（Exchange 首登挂钩，
// best-effort：失败只记日志，绝不影响登录主链路）。绑定行 CreatedBy=归属人 → 权限被回收后
// 归属人仍可按「本人创建的绑定行」解绑。设备配置不落服务端：SyncDevice 只签 key + 尝试 restart；
// openclaw/hermes 未装时静默跳过，首次 install 的 issueXxxConfig→DeviceMcpServers 自动带上 mcp 段。
func (s *ClientMcpService) AutoBindPermitted(ctx context.Context, deviceId, ownerId, llmBaseURL string) {
	if ownerId == "" {
		return
	}
	if _, err := s.deviceRepo.GetById(ctx, deviceId); err != nil {
		return
	}
	t := s.mcpGroupSvc.IdentityOf(ctx, ownerId)
	groups, err := s.mcpGroupSvc.PermittedGroups(ctx, t)
	if err != nil {
		log.Printf("[device:%s] 自动绑定 MCP 分组失败（读可见分组）: %v", deviceId, err)
		return
	}
	added := 0
	for _, g := range groups {
		if existing, err := s.groupRepo.GetByDeviceGroup(ctx, deviceId, g.GroupId); err != nil || existing == nil {
			if _, err := s.groupRepo.Add(ctx, &entity.ClientDeviceMcpGroup{
				DeviceId: deviceId, GroupId: g.GroupId, GroupName: g.Name, CreatedBy: ownerId,
			}); err != nil {
				log.Printf("[device:%s] 自动绑定分组 %s 失败: %v", deviceId, g.Name, err)
				continue
			}
			added++
		}
	}
	if added == 0 {
		return
	}
	log.Printf("[device:%s] 已自动绑定 %d 个 MCP 分组（按归属人 %s 权限）", deviceId, added, ownerId)
	if err := s.SyncDevice(ctx, deviceId, ownerId, llmBaseURL); err != nil {
		log.Printf("[device:%s] 自动绑定后收敛 MCP 失败: %v", deviceId, err)
	}
}

// SyncDevice 把设备的 key 收敛到当前直绑分组，并尝试 restart 生效（统一出口）。
func (s *ClientMcpService) SyncDevice(ctx context.Context, deviceId, issuedBy, llmBaseURL string) error {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil {
		return err
	}
	groups, err := s.effective(ctx, deviceId)
	if err != nil {
		return err
	}

	hadKey := dev.McpKeyPlain != ""
	if len(groups) == 0 {
		if hadKey {
			if k, err := s.keySvc.GetByHash(ctx, crypto.HashToken(dev.McpKeyPlain)); err == nil && k != nil {
				if err := s.keySvc.Delete(ctx, []string{k.KeyId}); err != nil {
					return fmt.Errorf("清理设备 MCP key 失败: %w", err)
				}
			}
			if err := s.deviceRepo.UpdateMcpKeyPlain(ctx, deviceId, ""); err != nil {
				return err
			}
		}
		s.tryRestartAgents(ctx, deviceId, issuedBy, llmBaseURL, hadKey)
		return nil
	}

	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, g.Name)
	}
	allowed, _ := json.Marshal(names)

	plain := dev.McpKeyPlain
	if plain != "" {
		if k, err := s.keySvc.GetByHash(ctx, crypto.HashToken(plain)); err != nil || k == nil {
			plain = "" // key 被密钥页手动删过 → 重签
		} else if k.AllowedGroups != string(allowed) {
			k.AllowedGroups = string(allowed)
			if err := s.keySvc.Edit(ctx, k); err != nil {
				return fmt.Errorf("更新 key 分组范围失败: %w", err)
			}
		}
	}
	if plain == "" {
		_, p, err := s.keySvc.Create(ctx, &entity.McpBearerKey{
			Name: "device:" + deviceId, Kind: "client", Owner: dev.UserId,
			AccessType: "groups", AllowedGroups: string(allowed), Enabled: 1, CreatedBy: issuedBy,
		})
		if err != nil {
			return fmt.Errorf("签发设备 MCP key 失败: %w", err)
		}
		plain = p
		if err := s.deviceRepo.UpdateMcpKeyPlain(ctx, deviceId, plain); err != nil {
			return err
		}
	}

	s.pairDomainSkills(ctx, dev, groups, issuedBy)
	s.tryRestartAgents(ctx, deviceId, issuedBy, llmBaseURL, true)
	return nil
}

// installedAgents 设备已装的 agent 列表（最新心跳状态非 notinstalled）。
func (s *ClientMcpService) installedAgents(ctx context.Context, deviceId string) []string {
	installed := map[string]bool{}
	if latest, err := s.statusRepo.ListLatestByDevice(ctx, deviceId); err == nil {
		for _, st := range latest {
			if (st.Agent == "openclaw" || st.Agent == "hermes") && st.Status != "notinstalled" && st.Status != "" {
				installed[st.Agent] = true
			}
		}
	}
	out := make([]string, 0, 2)
	for _, a := range []string{"openclaw", "hermes"} {
		if installed[a] {
			out = append(out, a)
		}
	}
	return out
}

// pairDomainSkills 绑定后自动补装配对内置技能（镜像实例侧 agent_mcp_service.pairDomainSkills）：
// 有效分组里每个 platform-<域> server（读写端点同域）映射技能库的 aiarsenal-<域> 内置手册，
// 对设备已装的 openclaw/hermes 各装缺失的（设备技能快照按 slug 判重；卸载不回收——
// 手册无副作用，留着无害）。best-effort：失败只记日志，不阻断绑定。
func (s *ClientMcpService) pairDomainSkills(ctx context.Context, dev *entity.ClientDevice, groups []*entity.McpGroup, issuedBy string) {
	if s.clientSvc == nil || len(groups) == 0 {
		return
	}
	agents := s.installedAgents(ctx, dev.DeviceId)
	if len(agents) == 0 {
		log.Printf("[device:%s] 无已装 agent，配对内置技能随下次安装生效", dev.DeviceId)
		return
	}
	domains := map[string]struct{}{}
	for _, g := range groups {
		servers, err := s.mcpGroupSvc.GetServerConfigs(g)
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
	snap := map[string]struct{}{} // 设备已装 slug（快照跨 agent 合并，按 slug 判重足够）
	for _, st := range parseDeviceSkills(dev.Skills) {
		snap[st.Slug] = struct{}{}
	}
	for d := range domains {
		slug := "aiarsenal-" + d
		skillId := s.clientSvc.builtinSkillIdBySlug(ctx, slug)
		if skillId == "" {
			continue // 该域无内置手册（如 schedule/budget），正常跳过
		}
		if _, ok := snap[slug]; ok {
			continue
		}
		for _, agent := range agents {
			if _, err := s.clientSvc.AdminInstallDeviceSkill(ctx, dev.DeviceId, skillId, "", agent, issuedBy); err != nil {
				log.Printf("[device:%s] 配对内置技能 %s 下发 %s 安装失败: %v", dev.DeviceId, slug, agent, err)
				continue
			}
			log.Printf("[device:%s] 配对内置技能已下发安装 %s → %s", dev.DeviceId, slug, agent)
		}
	}
}

// tryRestartAgents 设备已装的 openclaw/hermes enqueue restart 让配置重建带上/去掉 mcp 段
//（openclaw→openclaw.json mcp.servers，hermes→config.yaml mcp_servers，均在配置重签时注入）。
// best-effort：设备忙/离线/未装/无 LLM 地址都只记日志——下次 install/start/restart 自然带最新绑定。
// needPush=false（无变化且本来就没 key）时跳过，避免空重启。
func (s *ClientMcpService) tryRestartAgents(ctx context.Context, deviceId, issuedBy, llmBaseURL string, needPush bool) {
	if !needPush || s.clientSvc == nil || llmBaseURL == "" {
		return
	}
	for _, agent := range s.installedAgents(ctx, deviceId) {
		if _, err := s.clientSvc.AdminEnqueueCommand(ctx, deviceId, "restart", agent, "", nil, issuedBy, llmBaseURL); err != nil {
			log.Printf("[device:%s] MCP 绑定后 enqueue %s restart 失败（下次安装/启动自动带上）: %v", deviceId, agent, err)
		}
	}
}

// DeviceMcpServers 全量重建设备 openclaw 配置（issueOpenclawConfig）时补 mcp 段。
// best-effort：无绑定/未签 key/出错一律 nil，不写段。
func (s *ClientMcpService) DeviceMcpServers(ctx context.Context, dev *entity.ClientDevice) map[string]interface{} {
	if dev == nil || dev.McpKeyPlain == "" {
		return nil
	}
	groups, err := s.effective(ctx, dev.DeviceId)
	if err != nil || len(groups) == 0 {
		return nil
	}
	names := make([]string, 0, len(groups))
	for _, g := range groups {
		names = append(names, g.Name)
	}
	return McpServerEntries(s.cfg.McpBaseURL(), names, dev.McpKeyPlain)
}

// OnDeviceRemoved 删设备清理：MCP key 删掉 + 明文清空 + 绑定行清空（AdminDeleteDevice 挂钩，best-effort）。
func (s *ClientMcpService) OnDeviceRemoved(ctx context.Context, deviceId string) {
	dev, err := s.deviceRepo.GetById(ctx, deviceId)
	if err != nil || dev == nil {
		return
	}
	if dev.McpKeyPlain != "" {
		if k, err := s.keySvc.GetByHash(ctx, crypto.HashToken(dev.McpKeyPlain)); err == nil && k != nil {
			_ = s.keySvc.Delete(ctx, []string{k.KeyId})
		}
		_ = s.deviceRepo.UpdateMcpKeyPlain(ctx, deviceId, "")
	}
	_ = s.groupRepo.DeleteByDevice(ctx, deviceId)
}
