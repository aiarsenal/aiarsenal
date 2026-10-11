package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// MCP 分组服务。
// group.servers 是 jsonb 数组，元素为 server name 字符串或对象 {name,alias,tools[],prompts[],resources[]|'all'}。
// 分组归一化：normalizeGroupServers / getServerConfigInGroup。

// GroupServerConfig 分组内单个 server 的可见性配置。
type GroupServerConfig struct {
	Name      string      `json:"name"`
	Alias     string      `json:"alias,omitempty"`
	Tools     interface{} `json:"tools,omitempty"`     // []string 或 "all"
	Prompts   interface{} `json:"prompts,omitempty"`   // []string 或 "all"
	Resources interface{} `json:"resources,omitempty"` // []string 或 "all"
}

type McpGroupService struct {
	repo       repository.IMcpGroupRepository
	svcRep     repository.IMcpServerRepository
	userRep    repository.IUserRepository          // 归属人姓名回填（owner 是 aa_user id，列表展示转姓名）
	grantRepo  repository.IMcpGroupGrantRepository // 组合授权条件行（aa_mcp_group_grant；判定见 mcp_group_access.go）
	agentMcpSvc *AgentMcpService                   // 分组 server 名单变化 fan-out（main.go 装配后回指，防构造循环；nil 安全）
}

func NewMcpGroupService(r repository.IMcpGroupRepository, sr repository.IMcpServerRepository, ur repository.IUserRepository, gr repository.IMcpGroupGrantRepository) *McpGroupService {
	return &McpGroupService{repo: r, svcRep: sr, userRep: ur, grantRepo: gr}
}

// SetAgentMcpService main.go 装配后回指（防构造循环）：分组 server 名单变化后反查消费者重算配对手册。
func (s *McpGroupService) SetAgentMcpService(a *AgentMcpService) { s.agentMcpSvc = a }

func (s *McpGroupService) List(ctx context.Context, name string, pn, ps int) ([]*entity.McpGroup, int64, error) {
	list, total, err := s.repo.List(ctx, name, pn, ps)
	if err == nil {
		s.fillOwnerNames(ctx, list)
	}
	return list, total, err
}

func (s *McpGroupService) All(ctx context.Context) ([]*entity.McpGroup, error) {
	list, err := s.repo.All(ctx)
	if err == nil {
		s.fillOwnerNames(ctx, list)
	}
	return list, err
}

// fillOwnerNames 批量回填归属人姓名（owner/created_by → aa_user.name；查不到保持空，前端回落 id）。
func (s *McpGroupService) fillOwnerNames(ctx context.Context, list []*entity.McpGroup) {
	if s.userRep == nil || len(list) == 0 {
		return
	}
	uidSet := make(map[string]struct{}, len(list))
	var uids []string
	for _, g := range list {
		for _, uid := range []string{g.Owner, g.CreatedBy} {
			if uid == "" || uid == "0" {
				continue
			}
			if _, ok := uidSet[uid]; !ok {
				uidSet[uid] = struct{}{}
				uids = append(uids, uid)
			}
		}
	}
	if len(uids) == 0 {
		return
	}
	users, err := s.userRep.GetUsersByIds(ctx, uids)
	if err != nil {
		return
	}
	names := make(map[string]string, len(users))
	for _, u := range users {
		names[u.Id] = u.Name
	}
	for _, g := range list {
		if n, ok := names[g.Owner]; ok && n != "" {
			g.OwnerName = n
		} else if n, ok := names[g.CreatedBy]; ok && n != "" {
			g.OwnerName = n // 归属未记（存量）回落创建人姓名
		}
	}
}

func (s *McpGroupService) Get(ctx context.Context, id string) (*entity.McpGroup, error) {
	return s.repo.Get(ctx, id)
}

func (s *McpGroupService) GetByName(ctx context.Context, name string) (*entity.McpGroup, error) {
	return s.repo.GetByName(ctx, name)
}

func (s *McpGroupService) Add(ctx context.Context, e *entity.McpGroup) (string, error) {
	if err := s.normalizeServers(ctx, e); err != nil {
		return "", err
	}
	// 归属=创建人（与 aa_mcp_bearer_key 的 Owner 语义一致）；时间列显式写——
	// types.Time 不被 GORM autoCreateTime 识别、表 DDL 也无 DEFAULT now()，
	// 不写就落空串/NULL，分组列表「归属/创建时间」列全空（踩过）。
	if e.Owner == "" || e.Owner == "0" {
		e.Owner = e.CreatedBy
	}
	now := types.Time(time.Now())
	e.CreatedAt = &now
	e.UpdatedAt = &now
	return s.repo.Add(ctx, e)
}

func (s *McpGroupService) Edit(ctx context.Context, e *entity.McpGroup, t *model.Identity) error {
	if e.GroupId == "" {
		return errors.New("groupId 不能为空")
	}
	if cur, gerr := s.repo.Get(ctx, e.GroupId); gerr != nil {
		return errors.New("分组不存在")
	} else if !s.canManageGroup(t, cur) {
		return errors.New("没有操作权限")
	}
	if err := s.normalizeServers(ctx, e); err != nil {
		return err
	}
	// server 名单增删检测：旧名单先取，Edit 成功后做差集，有增删才 fan-out（可见性微调不触发）。
	var oldCfgs []GroupServerConfig
	if cur, gerr := s.repo.Get(ctx, e.GroupId); gerr == nil && cur != nil {
		oldCfgs, _ = s.GetServerConfigs(cur)
	}
	now := types.Time(time.Now())
	e.UpdatedAt = &now
	if err := s.repo.Edit(ctx, e); err != nil {
		return err
	}
	if s.agentMcpSvc == nil {
		return nil
	}
	newCfgs, _ := s.GetServerConfigs(e)
	added, removed := diffServerNames(oldCfgs, newCfgs)
	if len(added) > 0 || len(removed) > 0 {
		s.agentMcpSvc.OnGroupServersChanged(ctx, e.GroupId, e.UpdatedBy, added, removed)
	}
	return nil
}

func (s *McpGroupService) Delete(ctx context.Context, ids []string, t *model.Identity) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		if g, gerr := s.repo.Get(ctx, id); gerr != nil {
			return errors.New("分组不存在：" + id)
		} else if !s.canManageGroup(t, g) {
			return errors.New("没有操作权限")
		}
	}
	if err := s.repo.Delete(ctx, ids); err != nil {
		return err
	}
	// 授权条件行随删（best-effort；表小失败罕见，失败不回滚软删）
	_ = s.grantRepo.DeleteByGroups(ctx, ids)
	return nil
}

// normalizeServers 把 servers 字段规整为对象数组，并校验每个 server name 存在。
func (s *McpGroupService) normalizeServers(ctx context.Context, e *entity.McpGroup) error {
	if e.Servers == "" {
		e.Servers = "[]"
		return nil
	}
	// 兼容字符串元素与对象元素混用：统一转成 GroupServerConfig。
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(e.Servers), &raw); err != nil {
		return errors.New("servers 不是合法数组")
	}
	out := make([]GroupServerConfig, 0, len(raw))
	for _, r := range raw {
		var sc GroupServerConfig
		if err := json.Unmarshal(r, &sc); err != nil {
			// 可能是纯字符串
			var nameOnly string
			if err2 := json.Unmarshal(r, &nameOnly); err2 == nil {
				sc = GroupServerConfig{Name: nameOnly}
			} else {
				return errors.New("servers 元素必须是 server name 字符串或对象")
			}
		}
		if sc.Name == "" {
			return errors.New("分组内 server name 不能为空")
		}
		// 校验 server 存在（软校验：server 可能被删，仅告警不阻断）
		if _, err := s.svcRep.GetByName(ctx, sc.Name); err != nil {
			return errors.New("server 不存在：" + sc.Name)
		}
		out = append(out, sc)
	}
	b, _ := json.Marshal(out)
	e.Servers = string(b)
	return nil
}

// diffServerNames 分组 server 名单差集（增/删各自按名排序）：fan-out 判定与通知文案共用。
// 只看名字不看可见性——tools/prompts 过滤微调不改变配对域集合，不该触发补装。
func diffServerNames(old, cur []GroupServerConfig) (added, removed []string) {
	oldSet := make(map[string]struct{}, len(old))
	for _, c := range old {
		oldSet[c.Name] = struct{}{}
	}
	curSet := make(map[string]struct{}, len(cur))
	for _, c := range cur {
		curSet[c.Name] = struct{}{}
	}
	for n := range curSet {
		if _, ok := oldSet[n]; !ok {
			added = append(added, n)
		}
	}
	for n := range oldSet {
		if _, ok := curSet[n]; !ok {
			removed = append(removed, n)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return
}

// GetServerConfigs 解析分组 servers 为对象数组（供 SSE 端点确定某分组的成员可见性）。
func (s *McpGroupService) GetServerConfigs(e *entity.McpGroup) ([]GroupServerConfig, error) {
	if e == nil {
		return nil, nil
	}
	var out []GroupServerConfig
	if e.Servers == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(e.Servers), &out); err != nil {
		// 兼容字符串数组
		var names []string
		if err2 := json.Unmarshal([]byte(e.Servers), &names); err2 == nil {
			for _, n := range names {
				out = append(out, GroupServerConfig{Name: n})
			}
			return out, nil
		}
		return nil, err
	}
	return out, nil
}
