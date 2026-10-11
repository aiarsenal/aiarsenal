package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/grantmeta"
)

// PermGroupService 权限分组/员工集合两字典的用例编排。
// 写路径（增删改）在写锁内完成「环校验 + 写库 + grantmeta.Reload」，
// 防并发编辑各自过 DFS 后合成 A→B→A 环；改完即时生效（引用行鉴权时实时解析）。
type PermGroupService struct {
	groupRepo repository.IPermGroupRepository
	setRepo   repository.IEmpSetRepository
	menuRepo  repository.IMenuRepository
	userRepo  repository.IUserRepository
	mu        sync.Mutex
}

func NewPermGroupService(g repository.IPermGroupRepository, s repository.IEmpSetRepository, m repository.IMenuRepository, u repository.IUserRepository) *PermGroupService {
	return &PermGroupService{groupRepo: g, setRepo: s, menuRepo: m, userRepo: u}
}

// creatorDescMap 批量解析创建人「姓名（工号）」（工号空=纯姓名）；
// 回填失败不挡列表，返回空 map 留空显示。
func (s *PermGroupService) creatorDescMap(ctx context.Context, ids []string) map[string]string {
	m := make(map[string]string, len(ids))
	dedup := make([]string, 0, len(ids))
	seen := make(map[string]bool)
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			dedup = append(dedup, id)
		}
	}
	if len(dedup) == 0 {
		return m
	}
	users, err := s.userRepo.GetUsersByIds(ctx, dedup)
	if err != nil {
		return m
	}
	for _, u := range users {
		if u.EmployeeNo != "" {
			m[u.Id] = u.Name + "（" + u.EmployeeNo + "）"
		} else {
			m[u.Id] = u.Name
		}
	}
	return m
}

// ===== 权限分组 =====

func (s *PermGroupService) ListGroups(ctx context.Context, name string) ([]*entity.PermGroup, error) {
	list, err := s.groupRepo.List(ctx, name)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(list))
	for i, g := range list {
		ids[i] = g.CreatedBy
	}
	m := s.creatorDescMap(ctx, ids)
	for _, g := range list {
		g.CreatedByDesc = m[g.CreatedBy]
	}
	return list, nil
}

// SaveGroup 新增/编辑分组（groupId 空 = 新增）。名称唯一（锁内查重，与写入串行防并发同名）；
// 词条行内六维任意组合 AND（ParseGrantRows），引用须存在，分组引用不得成环（DFS）；写后刷新快照。
func (s *PermGroupService) SaveGroup(ctx context.Context, g *entity.PermGroup, termsJSON string) error {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return errors.New("请填写分组名称")
	}
	rows, err := model.ParseGrantRows(termsJSON)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if dup, err := s.groupRepo.NameExists(ctx, g.Name, g.GroupId); err != nil {
		return err
	} else if dup {
		return errors.New("分组名称已存在")
	}

	groups, sets, err := grantmeta.Snapshots()
	if err != nil {
		return err
	}
	// 引用存在性（分组可自引其他分组但不得引用自身——自身即环）
	for _, r := range rows {
		if r.GroupId == g.GroupId && g.GroupId != "" {
			return errors.New("分组词条不能引用自身")
		}
		if r.GroupId != "" && !groupExists(groups, r.GroupId, g.GroupId) {
			return errors.New("引用的权限分组不存在，请刷新后重试")
		}
		if r.SetId != "" {
			if _, ok := sets[r.SetId]; !ok {
				return errors.New("引用的员工集合不存在，请刷新后重试")
			}
		}
	}
	// 环校验：以「保存后」的词条集做 DFS（本分组以新词条代入）
	if g.GroupId != "" {
		groups[g.GroupId] = rows
	} else {
		groups["__new__"] = rows
	}
	if err := detectGroupCycle(groups); err != nil {
		return err
	}

	g.Terms = model.GrantRowsJSON(rows)
	if g.GroupId == "" {
		if _, err := s.groupRepo.Add(ctx, g); err != nil {
			return err
		}
	} else {
		if old, err := s.groupRepo.GetById(ctx, g.GroupId); err != nil {
			return err
		} else if old == nil {
			return errors.New("分组不存在")
		}
		if err := s.groupRepo.Update(ctx, g); err != nil {
			return err
		}
	}
	if err := grantmeta.Reload(); err != nil {
		return err
	}
	return nil
}

func groupExists(groups map[string][]model.GrantRow, id, exclude string) bool {
	if id == exclude {
		return false
	}
	_, ok := groups[id]
	return ok
}

// detectGroupCycle 全图 DFS 找环（白/灰/黑三色；灰=在途，回边即环）。
func detectGroupCycle(groups map[string][]model.GrantRow) error {
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := make(map[string]int, len(groups))
	var visit func(id, path string) error
	visit = func(id, path string) error {
		color[id] = grey
		next := path + " → " + id
		for _, r := range groups[id] {
			if r.GroupId == "" {
				continue
			}
			if _, ok := groups[r.GroupId]; !ok {
				continue // 悬挂引用：存在性检查另有报错，此处跳过
			}
			switch color[r.GroupId] {
			case grey:
				return fmt.Errorf("分组引用成环：%s → %s", strings.TrimPrefix(next, " → "), r.GroupId)
			case white:
				if err := visit(r.GroupId, next); err != nil {
					return err
				}
			}
		}
		color[id] = black
		return nil
	}
	for id := range groups {
		if color[id] == white {
			if err := visit(id, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteGroups 批量删除分组。被引用（菜单授权行 / 文档目录授权行 / 其他分组词条）即拒删。
func (s *PermGroupService) DeleteGroups(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	inBatch := make(map[string]bool, len(ids))
	for _, id := range ids {
		inBatch[id] = true
	}
	for _, id := range ids {
		if n, err := s.menuRepo.CountGrantRef(ctx, id); err == nil && n > 0 {
			return errors.New("有菜单的授权配置引用了该分组，请先解除引用")
		}
		var n int64
		if err := db.DocDB().WithContext(ctx).Model(&entity.DocGrant{}).
			Where("group_id = ?", id).Count(&n).Error; err == nil && n > 0 {
			return errors.New("有文档/目录的授权配置引用了该分组，请先解除引用")
		}
		if err := db.KbDB().WithContext(ctx).Model(&entity.KbGrant{}).
			Where("group_id = ?", id).Count(&n).Error; err == nil && n > 0 {
			return errors.New("有知识库文档/目录的授权配置引用了该分组，请先解除引用")
		}
		refs, err := s.groupRepo.TermsLike(ctx, id)
		if err != nil {
			return err
		}
		for _, g := range refs {
			// 同批一起删的分组互引随批消失，不算悬挂引用
			if !inBatch[g.GroupId] {
				return errors.New("有其他权限分组的词条引用了该分组，请先解除引用")
			}
		}
	}
	if err := s.groupRepo.Delete(ctx, ids); err != nil {
		return err
	}
	return grantmeta.Reload()
}

// ===== 员工集合 =====

func (s *PermGroupService) ListSets(ctx context.Context, name string) ([]*entity.EmpSet, error) {
	list, err := s.setRepo.List(ctx, name)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(list))
	for i, st := range list {
		ids[i] = st.CreatedBy
	}
	m := s.creatorDescMap(ctx, ids)
	for _, st := range list {
		st.CreatedByDesc = m[st.CreatedBy]
	}
	return list, nil
}

// ResolveMembers 工号批量解析姓名+部门（集合弹窗成员明细确认用）。
// 上限 500 防滥用；不在册工号不出现在结果（前端标注「未匹配」）。
func (s *PermGroupService) ResolveMembers(ctx context.Context, empNos []string) ([]*model.MemberInfoRes, error) {
	seen := make(map[string]bool, len(empNos))
	nos := make([]string, 0, len(empNos))
	for _, no := range empNos {
		no = strings.TrimSpace(no)
		if no != "" && !seen[no] {
			seen[no] = true
			nos = append(nos, no)
		}
	}
	if len(nos) > 500 {
		return nil, errors.New("单次最多解析 500 个工号")
	}
	return s.userRepo.GetUsersByEmpNos(ctx, nos)
}

// SaveSet 新增/编辑集合（setId 空 = 新增）。名称唯一（锁内查重）；成员 trim/去空/去重
// （ParseEmpNos），不校验工号在册。
func (s *PermGroupService) SaveSet(ctx context.Context, st *entity.EmpSet, empNosJSON string) error {
	st.Name = strings.TrimSpace(st.Name)
	if st.Name == "" {
		return errors.New("请填写集合名称")
	}
	nos, err := model.ParseEmpNos(empNosJSON)
	if err != nil {
		return err
	}
	st.EmpNos = model.EmpNosJSON(nos)
	s.mu.Lock()
	defer s.mu.Unlock()

	if dup, err := s.setRepo.NameExists(ctx, st.Name, st.SetId); err != nil {
		return err
	} else if dup {
		return errors.New("集合名称已存在")
	}

	if st.SetId == "" {
		if _, err := s.setRepo.Add(ctx, st); err != nil {
			return err
		}
	} else {
		if old, err := s.setRepo.GetById(ctx, st.SetId); err != nil {
			return err
		} else if old == nil {
			return errors.New("集合不存在")
		}
		if err := s.setRepo.Update(ctx, st); err != nil {
			return err
		}
	}
	return grantmeta.Reload()
}

// DeleteSets 批量删除集合。被引用（文档目录授权行 / 分组词条）即拒删。
func (s *PermGroupService) DeleteSets(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		if n, err := s.menuRepo.CountGrantRef(ctx, id); err == nil && n > 0 {
			return errors.New("有菜单的授权配置引用了该集合，请先解除引用")
		}
		var n int64
		if err := db.DocDB().WithContext(ctx).Model(&entity.DocGrant{}).
			Where("set_id = ?", id).Count(&n).Error; err == nil && n > 0 {
			return errors.New("有文档/目录的授权配置引用了该集合，请先解除引用")
		}
		if err := db.KbDB().WithContext(ctx).Model(&entity.KbGrant{}).
			Where("set_id = ?", id).Count(&n).Error; err == nil && n > 0 {
			return errors.New("有知识库文档/目录的授权配置引用了该集合，请先解除引用")
		}
		refs, err := s.setRepo.TermsLike(ctx, id)
		if err != nil {
			return err
		}
		if len(refs) > 0 {
			return errors.New("有权限分组的词条引用了该集合，请先解除引用")
		}
	}
	if err := s.setRepo.Delete(ctx, ids); err != nil {
		return err
	}
	return grantmeta.Reload()
}

// Options 瘦投影（GrantEditor 选择器拉选项；登录即可见，不吐词条/工号明细）。
func (s *PermGroupService) Options(ctx context.Context) (groups, sets []map[string]string, err error) {
	gs, err := s.groupRepo.List(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	ss, err := s.setRepo.List(ctx, "")
	if err != nil {
		return nil, nil, err
	}
	for _, g := range gs {
		groups = append(groups, map[string]string{"id": g.GroupId, "name": g.Name})
	}
	for _, st := range ss {
		sets = append(sets, map[string]string{"id": st.SetId, "name": st.Name})
	}
	return groups, sets, nil
}
