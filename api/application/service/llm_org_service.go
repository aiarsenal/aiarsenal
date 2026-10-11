package service

import (
	"context"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// LLM 组织服务：部门 LLM 配置 / 项目 / 终端用户 / 标签。
// 团队 = 部门（复用 aa_dept）：部门 LLM 配置存 aa_dept 扩展列；成员关系走 aa_user.dept_id；
// 部门累计花费从 aa_llm_daily_spend（scope_type='team', scope_id=dept_id）聚合回填。

type LLMOrgService struct {
	deptRepo    repository.ILLMDeptProfileRepository
	dailyRepo   repository.ILLMDailySpendRepository
	projectRepo repository.ILLMProjectRepository
	endUserRepo repository.ILLMEndUserRepository
	tagRepo     repository.ILLMTagRepository
}

func NewLLMOrgService(d repository.ILLMDeptProfileRepository, ds repository.ILLMDailySpendRepository, p repository.ILLMProjectRepository, e repository.ILLMEndUserRepository, tg repository.ILLMTagRepository) *LLMOrgService {
	return &LLMOrgService{deptRepo: d, dailyRepo: ds, projectRepo: p, endUserRepo: e, tagRepo: tg}
}

// ===== Dept LLM 配置 =====
func (s *LLMOrgService) ListDepts(ctx context.Context, name string, pn, ps int) ([]*entity.LLMDeptProfile, int64, error) {
	list, total, err := s.deptRepo.List(ctx, name, pn, ps)
	if err != nil {
		return nil, 0, err
	}
	// 回填累计花费 + 负责人姓名/工号 + 层级路径（均不 join，分别批量读）
	leaderMap, _ := s.deptRepo.LeaderMapByIds(ctx, collectLeaderIds(list))
	nameMap, _ := s.deptRepo.NameMapByIds(ctx, collectAncestorIds(list))
	for _, d := range list {
		d.Spend = s.sumDeptSpend(ctx, d.DeptId)
		if info, ok := leaderMap[d.LeaderId]; ok {
			d.LeaderName = info.Name
			d.LeaderEmpNo = info.EmployeeNo
		}
		d.LevelPath = joinAncestorNames(d.Ancestors, nameMap)
	}
	return list, total, nil
}

// collectLeaderIds 收集本页部门负责人 user_id（去重，剔除 "0"/空）。
func collectLeaderIds(list []*entity.LLMDeptProfile) []string {
	set := map[string]struct{}{}
	for _, d := range list {
		if d.LeaderId != "" && d.LeaderId != "0" {
			set[d.LeaderId] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

// collectAncestorIds 收集本页所有 dept 的 ancestors 中的 dept_id（去重，剔除 "0" 根哨兵）。
func collectAncestorIds(list []*entity.LLMDeptProfile) []string {
	set := map[string]struct{}{}
	for _, d := range list {
		for _, id := range splitAncestorIds(d.Ancestors) {
			if id != "" && id != "0" {
				set[id] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

// splitAncestorIds 拆分 ancestors 串（逗号分隔，首位通常为 "0" 根哨兵）。
func splitAncestorIds(ancestors string) []string {
	if ancestors == "" {
		return nil
	}
	parts := strings.Split(ancestors, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}

// joinAncestorNames 按 ancestors 顺序拼出 "祖先1/祖先2/..."；跳过 "0" 哨兵；无祖先返回 ""。
func joinAncestorNames(ancestors string, nameMap map[string]string) string {
	ids := splitAncestorIds(ancestors)
	var names []string
	for _, id := range ids {
		if id == "0" {
			continue
		}
		if n, ok := nameMap[id]; ok && n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, "/")
}

func (s *LLMOrgService) GetDept(ctx context.Context, deptId string) (*entity.LLMDeptProfile, error) {
	d, err := s.deptRepo.Get(ctx, deptId)
	if err != nil {
		return nil, err
	}
	d.Spend = s.sumDeptSpend(ctx, deptId)
	return d, nil
}

// sumDeptSpend 聚合某部门累计 LLM 花费（scope_type='team', scope_id=dept_id）。
// dailyRepo.List 走分页，这里用空 scope + 后过滤不够高效；简化用直接 SQL 计数——通过 repo 暴露的 List 兜底。
func (s *LLMOrgService) sumDeptSpend(ctx context.Context, deptId string) types.Milli {
	// 取该 dept 的全部 daily 行求和（分页取大页）
	list, _, _ := s.dailyRepo.List(ctx, []string{"team"}, []string{deptId}, nil, 1, 10000)
	var sum types.Milli
	for _, r := range list {
		sum += r.Spend
	}
	return sum
}

func (s *LLMOrgService) EditDeptConfig(ctx context.Context, e *entity.LLMDeptProfile) error {
	return s.deptRepo.UpdateLLMConfig(ctx, e)
}

// ===== Project =====
func (s *LLMOrgService) ListProjects(ctx context.Context, alias, teamId string, pn, ps int) ([]*entity.LLMProject, int64, error) {
	return s.projectRepo.List(ctx, alias, teamId, pn, ps)
}
func (s *LLMOrgService) GetProject(ctx context.Context, id string) (*entity.LLMProject, error) {
	return s.projectRepo.Get(ctx, id)
}
func (s *LLMOrgService) AddProject(ctx context.Context, e *entity.LLMProject) (string, error) {
	return s.projectRepo.Add(ctx, e)
}
func (s *LLMOrgService) EditProject(ctx context.Context, e *entity.LLMProject) error {
	return s.projectRepo.Edit(ctx, e)
}
func (s *LLMOrgService) DeleteProjects(ctx context.Context, ids []string) error {
	return s.projectRepo.Delete(ctx, ids)
}

// ===== EndUser =====
func (s *LLMOrgService) ListEndUsers(ctx context.Context, alias string, pn, ps int) ([]*entity.LLMEndUser, int64, error) {
	return s.endUserRepo.List(ctx, alias, pn, ps)
}
func (s *LLMOrgService) GetEndUser(ctx context.Context, id string) (*entity.LLMEndUser, error) {
	return s.endUserRepo.Get(ctx, id)
}
func (s *LLMOrgService) AddEndUser(ctx context.Context, e *entity.LLMEndUser) (string, error) {
	return s.endUserRepo.Add(ctx, e)
}
func (s *LLMOrgService) EditEndUser(ctx context.Context, e *entity.LLMEndUser) error {
	return s.endUserRepo.Edit(ctx, e)
}
func (s *LLMOrgService) DeleteEndUsers(ctx context.Context, ids []string) error {
	return s.endUserRepo.Delete(ctx, ids)
}

// ===== Tag =====
func (s *LLMOrgService) ListTags(ctx context.Context, name string, pn, ps int) ([]*entity.LLMTag, int64, error) {
	return s.tagRepo.List(ctx, name, pn, ps)
}
func (s *LLMOrgService) GetTag(ctx context.Context, name string) (*entity.LLMTag, error) {
	return s.tagRepo.Get(ctx, name)
}
func (s *LLMOrgService) AddTag(ctx context.Context, e *entity.LLMTag) error {
	return s.tagRepo.Add(ctx, e)
}
func (s *LLMOrgService) EditTag(ctx context.Context, e *entity.LLMTag) error {
	return s.tagRepo.Edit(ctx, e)
}
func (s *LLMOrgService) DeleteTags(ctx context.Context, names []string) error {
	return s.tagRepo.Delete(ctx, names)
}
