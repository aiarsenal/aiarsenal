package service

// 项目空间/个人空间同步器（aa_doc 库托管目录）。
//
// 结构：两个固定根（保留字面量 dir_id，visibility=2 全员可见空壳）+
//   每项目一目录（kind=project，ref_id=project_id，visibility=1 按条件，
//     授权行随项目成员表：read=全体成员，write=owner/maintainer/member，observer 只读）+
//   每员工一目录（kind=personal，ref_id=user_id，visibility=0 私有=本人+超管，零授权行）。
// 同步时机：api 启动 Bootstrap 全量对账（幂等，仅 api 进程跑，防双进程竞争）；
//   项目/成员/用户变更钩子增量同步（api+ops 两进程接线，失败 slog.Warn 不阻断业务，
//   唯一索引兜底竞态，下次 api 重启自愈）。
// 生命周期：项目软删→目录保留追加（已归档）；用户硬删→目录保留追加（已注销）；禁用不动。

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/idgen"
)

// aa_doc_dir.kind 取值与固定根保留 dir_id（迁移 aa_doc/000005 与 doc.sql Part A 同源）
const (
	DocDirKindProjectRoot  = "project_space" // 固定根：项目空间
	DocDirKindPersonalRoot = "personal_space" // 固定根：个人空间
	DocDirKindProject      = "project"        // 托管：每项目一目录（ref_id=project_id）
	DocDirKindPersonal     = "personal"       // 托管：每员工一目录（ref_id=user_id）

	DocSpaceProjectRootDirId  = "docspaceprojectroot000000000a"
	DocSpacePersonalRootDirId = "docspacepersonalroot000000000a"

	docSpaceArchivedSuffix  = "（已归档）" // 项目软删后追加
	docSpaceCancelledSuffix = "（已注销）" // 用户硬删后追加
	docSpaceManualSuffix    = "（手动创建）" // 固定根让位时给存量同名普通目录追加
)

type DocSpaceService struct {
	dirRepo        repository.IDocDirRepository
	grantRepo      repository.IDocGrantRepository
	projRepo       repository.IProjectRepository
	projMemberRepo repository.IProjectMemberRepository
	userRepo       repository.IUserRepository
}

func NewDocSpaceService(
	dr repository.IDocDirRepository,
	gr repository.IDocGrantRepository,
	pr repository.IProjectRepository,
	mr repository.IProjectMemberRepository,
	ur repository.IUserRepository,
) *DocSpaceService {
	return &DocSpaceService{dirRepo: dr, grantRepo: gr, projRepo: pr, projMemberRepo: mr, userRepo: ur}
}

// Bootstrap api 启动全量对账：固定根 + 全部未删项目 + 全部员工。幂等 best-effort。
func (s *DocSpaceService) Bootstrap(ctx context.Context) {
	s.ensureRoot(ctx, DocSpaceProjectRootDirId, DocDirKindProjectRoot, "项目空间")
	s.ensureRoot(ctx, DocSpacePersonalRootDirId, DocDirKindPersonalRoot, "个人空间")
	s.reconcileProjects(ctx)
	s.reconcileUsers(ctx)
}

// SyncProject 项目创建/编辑后：取-或-建目录 + 名字对齐 + 成员授权刷新。
func (s *DocSpaceService) SyncProject(ctx context.Context, projectID string) {
	if projectID == "" {
		return
	}
	s.ensureRoot(ctx, DocSpaceProjectRootDirId, DocDirKindProjectRoot, "项目空间")
	p, err := s.projRepo.Get(ctx, projectID)
	if err != nil {
		slog.Warn("doc-space 项目同步取项目失败", "projectId", projectID, "err", err)
		return
	}
	s.ensureProjectDir(ctx, p, "")
	s.SyncProjectMembers(ctx, projectID)
}

// SyncProjectDeleted 项目软删后：目录与文档保留，名字追加（已归档）；成员授权保留（文档仍可访问）。
func (s *DocSpaceService) SyncProjectDeleted(ctx context.Context, projectID string) {
	if projectID == "" {
		return
	}
	d, err := s.dirRepo.GetByKindRef(ctx, DocDirKindProject, projectID)
	if err != nil {
		return // 目录本就不存在（项目早于本功能）——无需处理
	}
	if strings.HasSuffix(d.Name, docSpaceArchivedSuffix) {
		return
	}
	s.renameManaged(ctx, d, truncateRunes(d.Name, 100)+docSpaceArchivedSuffix)
}

// SyncProjectMembers 成员增删改后：整体重写该目录 read/write 授权行 + 回正 visibility=1。
// 空成员表 → Replace(nil)=清光行（visibility=1 + 零行 = 仅超管可见）；绝不写全空维度行（语义是"不限"）。
func (s *DocSpaceService) SyncProjectMembers(ctx context.Context, projectID string) {
	if projectID == "" {
		return
	}
	d, err := s.dirRepo.GetByKindRef(ctx, DocDirKindProject, projectID)
	if err != nil {
		slog.Warn("doc-space 成员同步未找到项目目录", "projectId", projectID)
		return
	}
	members, err := s.projMemberRepo.ListByProject(ctx, projectID)
	if err != nil {
		slog.Warn("doc-space 成员同步取成员失败", "projectId", projectID, "err", err)
		return
	}
	var readIDs, writeIDs []string
	for _, m := range members {
		readIDs = append(readIDs, m.UserId)
		if m.Role != "observer" { // owner/maintainer/member 可写，observer 只读
			writeIDs = append(writeIDs, m.UserId)
		}
	}
	if err := s.dirRepo.SetVisibility(ctx, d.DirId, 1); err != nil { // 回正档位（防手工改动锁死成员）
		slog.Warn("doc-space 回正目录可见性失败", "dirId", d.DirId, "err", err)
	}
	if err := s.grantRepo.Replace(ctx, "dir", d.DirId, "read", grantRowsByUsers(readIDs), ""); err != nil {
		slog.Warn("doc-space 写读授权失败", "dirId", d.DirId, "err", err)
	}
	if err := s.grantRepo.Replace(ctx, "dir", d.DirId, "write", grantRowsByUsers(writeIDs), ""); err != nil {
		slog.Warn("doc-space 写写授权失败", "dirId", d.DirId, "err", err)
	}
}

// SyncUser 员工创建/编辑后：取-或-建个人目录 + 名字对齐（姓名→账号名回落）。
func (s *DocSpaceService) SyncUser(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	s.ensureRoot(ctx, DocSpacePersonalRootDirId, DocDirKindPersonalRoot, "个人空间")
	u, err := s.userRepo.GetEntityById(ctx, userID)
	if err != nil {
		slog.Warn("doc-space 用户同步取用户失败", "userId", userID, "err", err)
		return
	}
	s.ensurePersonalDir(ctx, u)
}

// SyncUserDeleted 员工硬删后：目录与文档保留，名字追加（已注销）。不得回查用户（行已删）。
func (s *DocSpaceService) SyncUserDeleted(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	d, err := s.dirRepo.GetByKindRef(ctx, DocDirKindPersonal, userID)
	if err != nil {
		return
	}
	if strings.HasSuffix(d.Name, docSpaceCancelledSuffix) {
		return
	}
	s.renameManaged(ctx, d, truncateRunes(d.Name, 100)+docSpaceCancelledSuffix)
}

// GetByRef 跳转反查（kind=project|personal）；非-nil 身份须对该目录可读。
// refId=me（仅 kind=personal）=当前登录人自己的个人目录——「我的空间」入口用。
func (s *DocSpaceService) GetByRef(ctx context.Context, kind, refId string, t *model.Identity) (*entity.DocDir, error) {
	if kind != DocDirKindProject && kind != DocDirKindPersonal {
		return nil, errors.New("kind 仅支持 project/personal")
	}
	if refId == "" {
		return nil, errors.New("refId 不能为空")
	}
	if refId == "me" {
		if kind != DocDirKindPersonal {
			return nil, errors.New("refId=me 仅支持 kind=personal")
		}
		if t == nil {
			return nil, errors.New("refId=me 需要登录身份")
		}
		refId = t.UserId
	}
	d, err := s.dirRepo.GetByKindRef(ctx, kind, refId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("目录尚未生成")
		}
		return nil, err
	}
	if t != nil {
		view, err := buildDocPermView(ctx, s.dirRepo, s.grantRepo, *t)
		if err != nil {
			return nil, err
		}
		if !view.readableDirs[d.DirId] {
			return nil, errors.New("没有访问权限")
		}
	}
	return d, nil
}

// ---- 内部：对账与取-或-建 ----

// reconcileProjects 全量对账项目（List 只滤 deleted_at，含已归档项目——归档目录名字由删除钩子管）。
func (s *DocSpaceService) reconcileProjects(ctx context.Context) {
	const size = 500
	for pn := 1; ; pn++ {
		list, total, err := s.projRepo.List(ctx, "", "", pn, size)
		if err != nil {
			slog.Warn("doc-space 项目对账翻页失败", "pageNum", pn, "err", err)
			return
		}
		for _, p := range list {
			s.ensureProjectDir(ctx, p, "")
			s.SyncProjectMembers(ctx, p.ProjectId)
		}
		if int64(pn*size) >= total || len(list) == 0 {
			return
		}
	}
}

// reconcileUsers 全量对账员工（含禁用——禁用不动目录，但缺目录要补建）。
func (s *DocSpaceService) reconcileUsers(ctx context.Context) {
	const size = 500
	for pn := 1; ; pn++ {
		list, total, err := s.userRepo.List(ctx, "", "", "", "", pn, size)
		if err != nil {
			slog.Warn("doc-space 用户对账翻页失败", "pageNum", pn, "err", err)
			return
		}
		for _, u := range list {
			s.ensurePersonalDir(ctx, u)
		}
		if int64(pn*size) >= total || len(list) == 0 {
			return
		}
	}
}

// ensureRoot 固定根取-或-建；根级同名普通目录先改名（手动创建）让位（数据不丢）。
func (s *DocSpaceService) ensureRoot(ctx context.Context, dirId, kind, name string) {
	if _, err := s.dirRepo.Get(ctx, dirId); err == nil {
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Warn("doc-space 固定根查询失败", "dirId", dirId, "err", err)
		return
	}
	if got, err := s.dirRepo.GetByName(ctx, "", name); err == nil && got.DirId != dirId && got.Kind == "" {
		s.renameManaged(ctx, got, s.uniqueDirName(ctx, "", truncateRunes(name, 100)+docSpaceManualSuffix, "", got.DirId))
	}
	if err := s.dirRepo.Add(ctx, &entity.DocDir{
		DirId: dirId, Name: name, ParentId: "", Visibility: 2, Kind: kind,
	}); err != nil {
		slog.Warn("doc-space 建固定根失败", "dirId", dirId, "err", err)
	}
}

// ensureProjectDir 项目目录取-或-建；名字漂移（项目改名/重名消解变化）→ 对齐。
func (s *DocSpaceService) ensureProjectDir(ctx context.Context, p *entity.Project, suffix string) {
	desired := sanitizeName(p.Name, 100) + suffix
	disambig := p.Code
	if disambig == "" {
		disambig = shortID(p.ProjectId)
	}
	d, err := s.dirRepo.GetByKindRef(ctx, DocDirKindProject, p.ProjectId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.dirRepo.Add(ctx, &entity.DocDir{
			Name: s.uniqueDirName(ctx, DocSpaceProjectRootDirId, desired, disambig, ""),
			ParentId: DocSpaceProjectRootDirId, Visibility: 1,
			Kind: DocDirKindProject, RefId: p.ProjectId,
		}); err != nil {
			slog.Warn("doc-space 建项目目录失败", "projectId", p.ProjectId, "err", err)
		}
		return
	}
	if err != nil {
		slog.Warn("doc-space 项目目录反查失败", "projectId", p.ProjectId, "err", err)
		return
	}
	if d.Name != desired {
		s.renameManaged(ctx, d, s.uniqueDirName(ctx, DocSpaceProjectRootDirId, desired, disambig, d.DirId))
	}
}

// ensurePersonalDir 个人目录取-或-建；名字漂移（改名）→ 对齐。姓名→账号名→userId 前 8 位回落。
func (s *DocSpaceService) ensurePersonalDir(ctx context.Context, u *entity.User) {
	base := u.Name
	if base == "" {
		base = u.UserName
	}
	if base == "" {
		base = shortID(u.UserId)
	}
	desired := sanitizeName(base, 100)
	disambig := u.EmployeeNo
	if disambig == "" {
		disambig = u.UserName
	}
	if disambig == "" {
		disambig = shortID(u.UserId)
	}
	d, err := s.dirRepo.GetByKindRef(ctx, DocDirKindPersonal, u.UserId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := s.dirRepo.Add(ctx, &entity.DocDir{
			Name: s.uniqueDirName(ctx, DocSpacePersonalRootDirId, desired, disambig, ""),
			ParentId: DocSpacePersonalRootDirId, Visibility: 0, CreatedBy: u.UserId,
			Kind: DocDirKindPersonal, RefId: u.UserId,
		}); err != nil {
			slog.Warn("doc-space 建个人目录失败", "userId", u.UserId, "err", err)
		}
		return
	}
	if err != nil {
		slog.Warn("doc-space 个人目录反查失败", "userId", u.UserId, "err", err)
		return
	}
	if d.Name != desired {
		s.renameManaged(ctx, d, s.uniqueDirName(ctx, DocSpacePersonalRootDirId, desired, disambig, d.DirId))
	}
}

// renameManaged 托管目录改名（best-effort；唯一索引兜底，失败下次对账重试）。
func (s *DocSpaceService) renameManaged(ctx context.Context, d *entity.DocDir, name string) {
	if name == "" || name == d.Name {
		return
	}
	if err := s.dirRepo.Rename(ctx, d.DirId, name); err != nil {
		slog.Warn("doc-space 托管目录改名失败", "dirId", d.DirId, "name", name, "err", err)
	}
}

// uniqueDirName 同级不重名消解：base → base（消解符）→ base·2…·10。
// selfDirId 非空 = 改名场景（重名校验跳过自身）。
func (s *DocSpaceService) uniqueDirName(ctx context.Context, parentId, base, disambig, selfDirId string) string {
	if s.dirNameFree(ctx, parentId, base, selfDirId) {
		return base
	}
	if disambig != "" {
		c := fmt.Sprintf("%s（%s）", base, disambig)
		if s.dirNameFree(ctx, parentId, c, selfDirId) {
			return c
		}
	}
	for i := 2; i <= 10; i++ {
		c := base + "·" + strconv.Itoa(i)
		if s.dirNameFree(ctx, parentId, c, selfDirId) {
			return c
		}
	}
	slog.Warn("doc-space 目录名消解 10 轮未果，交由唯一索引兜底", "parentId", parentId, "base", base)
	return base + "·" + shortID(idgen.New())
}

func (s *DocSpaceService) dirNameFree(ctx context.Context, parentId, name, selfDirId string) bool {
	got, err := s.dirRepo.GetByName(ctx, parentId, name)
	if err != nil {
		return true // 查无 = 空位（其余错误按空位处理，唯一索引最终兜底）
	}
	return got.DirId == selfDirId
}

// grantRowsByUsers 用户 id 列表 → 授权条件行（单行 IN 语义）。空列表 → nil（=清光该动作行，绝不"不限"）。
func grantRowsByUsers(userIds []string) []model.GrantRow {
	if len(userIds) == 0 {
		return nil
	}
	return []model.GrantRow{{UserIds: userIds}}
}

// shortID 业务 id 前 8 位（消解后缀用；idgen 产物分布均匀，前缀足够区分）。
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
