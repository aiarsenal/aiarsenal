package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

type SkillService struct {
	repo     repository.ISkillRepository
	userRepo repository.IUserRepository
	secRepo  repository.ISkillSecurityRepository // 发布门禁：版本最新审计须为「安全」
}

func NewSkillService(r repository.ISkillRepository, ur repository.IUserRepository, sr repository.ISkillSecurityRepository) *SkillService {
	return &SkillService{repo: r, userRepo: ur, secRepo: sr}
}

func (s *SkillService) List(ctx context.Context, namespaceId, slug, name, status, keyWords string, pageNum, pageSize int) ([]*entity.Skill, int64, error) {
	list, total, err := s.repo.List(ctx, namespaceId, slug, name, status, keyWords, pageNum, pageSize)
	if err != nil {
		return nil, 0, err
	}
	s.enrichOwners(ctx, list)
	return list, total, nil
}

// enrichOwners 回填所有者姓名/工号（跨库读 aiarsenal.aa_user，不 join）。
func (s *SkillService) enrichOwners(ctx context.Context, list []*entity.Skill) {
	ids := make([]string, 0, len(list))
	seen := map[string]struct{}{}
	for _, sk := range list {
		if sk.OwnerId == "" {
			continue
		}
		if _, ok := seen[sk.OwnerId]; !ok {
			seen[sk.OwnerId] = struct{}{}
			ids = append(ids, sk.OwnerId)
		}
	}
	if len(ids) == 0 {
		return
	}
	users, err := s.userRepo.GetUsersByIds(ctx, ids)
	if err != nil {
		return
	}
	m := make(map[string]*model.UserSimpleRes, len(users))
	for _, u := range users {
		m[u.Id] = u
	}
	for _, sk := range list {
		if u, ok := m[sk.OwnerId]; ok {
			sk.OwnerName = u.Name
			sk.OwnerEmpNo = u.EmployeeNo
		}
	}
}

// Search 语义检索：query → embedding → pgvector 余弦相似度排序。
func (s *SkillService) Search(ctx context.Context, query string, limit int, embed EmbeddingProvider) ([]map[string]interface{}, error) {
	if embed == nil || query == "" {
		return nil, nil
	}
	vec, err := embed.Embed(query)
	if err != nil {
		return nil, err
	}
	return s.repo.SearchByVector(ctx, VectorToPGText(vec), limit)
}

func (s *SkillService) Get(ctx context.Context, skillId string) (*entity.Skill, error) {
	sk, err := s.repo.Get(ctx, skillId)
	if err != nil {
		return nil, err
	}
	// 回填所有者姓名/工号（与 List 一致；详情抽屉/接口要展示「姓名（工号）」）。
	s.enrichOwners(ctx, []*entity.Skill{sk})
	return sk, nil
}

func (s *SkillService) Add(ctx context.Context, sk *entity.Skill) (string, error) {
	if sk.NamespaceId == "" || sk.Slug == "" {
		return "", errors.New("命名空间与标识不能为空")
	}
	sk.Slug = normalizeSlug(sk.Slug)
	if sk.Status == 0 {
		sk.Status = 1
	}
	return s.repo.Add(ctx, sk)
}

// Edit 编辑技能：显式列更新（Updates(struct) 会跳过零值字段——visibility=0 公开、
// status=0 下架、清空简介都会静默失效，故用列 map 全量落）。调用方须传整实体
// （如先 Get 再改字段），部分字段实体会把空值写入库。
func (s *SkillService) Edit(ctx context.Context, sk *entity.Skill) error {
	now := types.Time(time.Now())
	return s.repo.EditFields(ctx, sk.SkillId, map[string]any{
		"namespace_id": sk.NamespaceId, "slug": sk.Slug, "display_name": sk.DisplayName,
		"summary": sk.Summary, "visibility": sk.Visibility, "status": sk.Status,
		"updated_by": sk.UpdatedBy, "updated_at": &now,
	})
}

func (s *SkillService) Delete(ctx context.Context, skillIds []string) error {
	return s.repo.Delete(ctx, skillIds)
}

func (s *SkillService) SetHidden(ctx context.Context, skillId string, hidden uint, by string) error {
	return s.repo.SetHidden(ctx, skillId, hidden, by)
}

// ===== 版本 =====

// ListVersions 版本列表：回填各版本最新安全审计状态（auditStatus，见 entity.AuditStatus*），
// 供 UI 展示「安全/不安全/扫描中…」并预禁发布/安装不安全版本。
func (s *SkillService) ListVersions(ctx context.Context, skillId string) ([]*entity.SkillVersion, error) {
	list, err := s.repo.ListVersions(ctx, skillId)
	if err != nil {
		return nil, err
	}
	if s.secRepo != nil && len(list) > 0 {
		ids := make([]string, 0, len(list))
		for _, v := range list {
			ids = append(ids, v.VersionId)
		}
		if audits, aerr := s.secRepo.LatestAuditsByVersions(ctx, ids); aerr == nil {
			for _, v := range list {
				if a := audits[v.VersionId]; a != nil {
					st := a.Status
					v.AuditStatus = &st
				}
			}
		}
	}
	return list, nil
}

func (s *SkillService) GetVersion(ctx context.Context, versionId string) (*entity.SkillVersion, error) {
	return s.repo.GetVersion(ctx, versionId)
}

// AddVersion 新建草稿版本（版本默认 DRAFT 状态）
func (s *SkillService) AddVersion(ctx context.Context, v *entity.SkillVersion, by string) (string, error) {
	if v.SkillId == "" || v.Version == "" {
		return "", errors.New("技能与版本号不能为空")
	}
	v.Status = 0 // DRAFT
	v.CreatedBy = by
	verId, err := s.repo.AddVersion(ctx, v)
	if err != nil {
		return "", err
	}
	return verId, nil
}

// PublishVersion 发布版本：置为 PUBLISHED 并更新技能 latest_version_id。
// 门禁：该版本最新安全审计须为「安全」（PASSED）——不安全/扫描中/扫描失败/无记录均拒绝，
// 报错带不安全原因摘要。内置技能导入（skill_builtin_service）走 repo 层直发不受此门禁
// （平台自带内容可信，且异步扫描未完成会永久卡死导入）。
func (s *SkillService) PublishVersion(ctx context.Context, versionId string) error {
	v, err := s.repo.GetVersion(ctx, versionId)
	if err != nil {
		return err
	}
	if v.Status == 2 {
		return errors.New("已撤回的版本不可发布")
	}
	if err := ensureSkillVersionSafe(ctx, s.secRepo, versionId); err != nil {
		return err
	}
	if err := s.repo.PublishVersion(ctx, versionId); err != nil {
		return err
	}
	return s.repo.SetLatestVersion(ctx, v.SkillId, versionId)
}

// YankVersion 撤回版本（软删，保留记录）
func (s *SkillService) YankVersion(ctx context.Context, versionId, by, reason string) error {
	return s.repo.YankVersion(ctx, versionId, by, reason)
}

// ===== 标签 =====

func (s *SkillService) ListTags(ctx context.Context, skillId string) ([]*entity.SkillTag, error) {
	return s.repo.ListTags(ctx, skillId)
}

func (s *SkillService) AddTag(ctx context.Context, t *entity.SkillTag) error {
	return s.repo.AddTag(ctx, t)
}

func (s *SkillService) RemoveTag(ctx context.Context, tagId string) error {
	return s.repo.RemoveTag(ctx, tagId)
}

// normalizeSlug：小写、空格/标点 → -、去首尾 -、折叠连续 -、长度 2~64（按 rune 计）。
// 保留中文等 Unicode 字母数字（英文 slug 行为不变）。
func normalizeSlug(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "_", "-")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r) // 中文等 Unicode 字母/数字保留
		default:
			b.WriteByte('-') // 空格/标点 → -
		}
	}
	s = strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if utf8.RuneCountInString(s) < 2 {
		s = s + "0"
	}
	if utf8.RuneCountInString(s) > 64 {
		s = string([]rune(s)[:64])
	}
	return s
}
