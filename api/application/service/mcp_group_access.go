package service

// MCP 分组访问判定 + 组合授权管理（单动作「可使用」）。
//
// 判定语义（与 KB/菜单同一套条件行模型 model.GrantRow；kb_access.go 的单动作版）：
//   canUse    = 超管 ∨ owner ∨ visibility=2 ∨ (visibility=1 ∧ ∃条件行命中)   —— 可见 + 可绑定
//   canManage = 超管 ∨ owner                                                —— 编辑/删除/配授权
//   行命中 = 非空维度全 AND（维度内多值=其一），Exclude 行取反（黑名单）。
//
// 身份参数 t *model.Identity：nil = 系统/维护路径（网关 SSE、内部调用），跳过判定全放行；
// 非-nil 一律收紧。List 谓词（mcp_repo.go mcpGroupReadPredicate）与本文件 Go 判定语义逐条对齐。
// 注意：All/Get/GetByName/List 保持不过滤——effective()/网关进程消费，语义绝不能改；
// 过滤一律走本文件 Visible*（List/All 的带身份版）。

import (
	"context"
	"encoding/json"
	"errors"

	"aiarsenal/consts"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/grantmeta"
)

// mcpGrantRowFromEntity 实体条件行 → 模型条件行（jsonb 文本 → []string；解析失败按空=不限）
func mcpGrantRowFromEntity(g *entity.McpGroupGrant) model.GrantRow {
	return model.GrantRow{
		UserIds: mcpJSONToIDs(g.UserIds),
		DeptId:  g.DeptId,
		RoleIds: mcpJSONToIDs(g.RoleIds),
		PostIds: mcpJSONToIDs(g.PostIds),
		Exclude: g.Exclude,
		GroupId: g.GroupId,
		SetId:   g.SetId,
	}
}

func mcpJSONToIDs(s string) []string {
	if s == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(s), &ids); err != nil {
		return nil
	}
	return ids
}

// ===== 判定核心 =====

// canUseGroup 分组「可使用」（可见 + 可绑定）。nil 身份 = 系统路径全放行。
func (s *McpGroupService) canUseGroup(ctx context.Context, t *model.Identity, g *entity.McpGroup) bool {
	if t == nil {
		return true
	}
	if s.canManageGroup(t, g) {
		return true
	}
	switch g.Visibility {
	case 2:
		return true
	case 1:
		rows, err := s.grantRepo.ListByGroup(ctx, g.GroupId)
		if err != nil {
			return false
		}
		grants := make([]model.GrantRow, 0, len(rows))
		for _, r := range rows {
			grants = append(grants, mcpGrantRowFromEntity(r))
		}
		return model.MatchAny(grants, *t)
	default:
		return false // 0私有：仅 owner/超管（上面已判）
	}
}

// canManageGroup 分组可管理（编辑/删除/配授权）。nil 身份 = 系统路径全放行。
func (s *McpGroupService) canManageGroup(t *model.Identity, g *entity.McpGroup) bool {
	if t == nil {
		return true
	}
	return t.IsAdmin || (t.UserId != "" && (g.Owner == t.UserId || g.CreatedBy == t.UserId))
}

// assertGroupUsable 绑定入口校验：分组存在且当前身份可使用。
func (s *McpGroupService) assertGroupUsable(ctx context.Context, groupId string, t *model.Identity) (*entity.McpGroup, error) {
	g, err := s.repo.Get(ctx, groupId)
	if err != nil {
		return nil, errors.New("分组不存在")
	}
	if !s.canUseGroup(ctx, t, g) {
		return nil, errors.New("没有分组的访问权限")
	}
	return g, nil
}

// ===== 列表（带身份过滤；控制器 List/All 切到这两个） =====

// VisibleList 带身份过滤的分页列表（nil/超管 → 不加谓词）。
func (s *McpGroupService) VisibleList(ctx context.Context, name string, pn, ps int, t *model.Identity) ([]*entity.McpGroup, int64, error) {
	var (
		list  []*entity.McpGroup
		total int64
		err   error
	)
	if t == nil || t.IsAdmin {
		list, total, err = s.repo.List(ctx, name, pn, ps)
	} else {
		list, total, err = s.repo.ListVisible(ctx, name, pn, ps, t)
	}
	if err == nil {
		s.fillCanManage(t, list)
		s.fillOwnerNames(ctx, list)
	}
	return list, total, err
}

// VisibleAll 带身份过滤的全量（绑定抽屉「添加绑定」选项、设备自助 options 共用）。
func (s *McpGroupService) VisibleAll(ctx context.Context, t *model.Identity) ([]*entity.McpGroup, error) {
	var (
		list []*entity.McpGroup
		err  error
	)
	if t == nil || t.IsAdmin {
		list, err = s.repo.All(ctx)
	} else {
		list, err = s.repo.AllFor(ctx, t)
	}
	if err == nil {
		s.fillCanManage(t, list)
		s.fillOwnerNames(ctx, list)
	}
	return list, err
}

// fillCanManage 回填「可管理」标记（前端据此隐藏编辑/删除/权限按钮）。
func (s *McpGroupService) fillCanManage(t *model.Identity, list []*entity.McpGroup) {
	for _, g := range list {
		g.CanManage = s.canManageGroup(t, g)
	}
}

// PermittedGroups 自动绑定源：归属人「可使用」的全部分组（与 VisibleAll 同判定、不回填展示字段）。
func (s *McpGroupService) PermittedGroups(ctx context.Context, t *model.Identity) ([]*entity.McpGroup, error) {
	if t == nil || t.IsAdmin {
		return s.repo.All(ctx)
	}
	return s.repo.AllFor(ctx, t)
}

// ===== 组合授权存取（配置入口：超管或 owner） =====

// McpGroupGrantInfo 授权配置回显
type McpGroupGrantInfo struct {
	Visibility uint             `json:"visibility"` // 0私有 1按条件 2全员
	Rules      []model.GrantRow `json:"rules"`
}

// GetGrants 查询分组授权配置。nil 身份（系统）不限；否则与保存同门槛 canManage（防探查他人配置）。
func (s *McpGroupService) GetGrants(ctx context.Context, groupId string, t *model.Identity) (*McpGroupGrantInfo, error) {
	g, err := s.repo.Get(ctx, groupId)
	if err != nil {
		return nil, errors.New("分组不存在")
	}
	if !s.canManageGroup(t, g) {
		return nil, errors.New("没有操作权限")
	}
	info := &McpGroupGrantInfo{Visibility: g.Visibility, Rules: []model.GrantRow{}}
	rows, err := s.grantRepo.ListByGroup(ctx, groupId)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		info.Rules = append(info.Rules, mcpGrantRowFromEntity(r))
	}
	return info, nil
}

// SaveGrants 保存分组可见性档位 + 条件行（整体重写）。绝不走 Edit——
// Edit 是 struct Updates（visibility 零值会被跳过）且带 servers 名单 fan-out，授权配置天然隔离。
func (s *McpGroupService) SaveGrants(ctx context.Context, groupId string, visibility uint, rulesJSON string, t *model.Identity) error {
	if visibility > 2 {
		return errors.New("visibility 取值 0|1|2")
	}
	rules, err := model.ParseGrantRows(rulesJSON)
	if err != nil {
		return errors.New("条件行：" + err.Error())
	}
	g, err := s.repo.Get(ctx, groupId)
	if err != nil {
		return errors.New("分组不存在")
	}
	actor := ""
	if t != nil {
		if !s.canManageGroup(t, g) {
			return errors.New("没有操作权限")
		}
		actor = t.UserId
	}
	if err := s.repo.SetVisibility(ctx, groupId, visibility); err != nil {
		return err
	}
	return s.grantRepo.Replace(ctx, groupId, rules, actor)
}

// ===== 身份组装（自动绑定路径：只有归属人 userId 可用） =====

// IdentityOf 裸 userId → 六维权限身份（实例 provision / 设备首登自动绑定用；
// 与 ops 层 identityByUserId 同源组装）：userRep 取部门/岗位/工号/超管位、casbin g 行取角色、
// grantmeta 实时解析权限分组/员工集合引用。userRep nil（网关进程）→ 仅 UserId 裸身份（保守放行 owner 臂）。
func (s *McpGroupService) IdentityOf(ctx context.Context, userId string) *model.Identity {
	if userId == "" {
		return nil
	}
	t := &model.Identity{UserId: userId, RoleIds: []string{}, PostIds: []string{}}
	if s.userRep == nil {
		return t
	}
	if deptId, postIds, employeeNo, err := s.userRep.GetDeptAndPosts(ctx, userId); err == nil {
		if deptId != "" {
			t.DeptId = deptId
		}
		if len(postIds) > 0 {
			t.PostIds = postIds
		}
		if employeeNo != "" {
			t.EmployeeNo = employeeNo
		}
	}
	if u, err := s.userRep.GetEntityById(ctx, userId); err == nil && u != nil {
		t.DeptId = u.DeptId
		t.EmployeeNo = u.EmployeeNo
		t.IsAdmin = u.IsAdmin == 1
	}
	if e, err := auth.Enforcer(); err == nil {
		if gp, gerr := e.GetFilteredGroupingPolicy(0, consts.CasBinUserPrefix+userId); gerr == nil {
			roleIds := make([]string, 0, len(gp))
			for _, v := range gp {
				if len(v) >= 2 {
					roleIds = append(roleIds, v[1])
				}
			}
			t.RoleIds = roleIds
		}
	}
	grantmeta.ResolveIdentity(t)
	return t
}
