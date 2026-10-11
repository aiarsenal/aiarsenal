package repository

import (
	"context"
	"encoding/json"

	"aiarsenal/domain/entity"
)

// ===== 命名空间 =====
type ISkillNamespaceRepository interface {
	List(ctx context.Context, slug, name, nsType, status string, pageNum, pageSize int) (list []*entity.SkillNamespace, total int64, err error)
	All(ctx context.Context) (list []*entity.SkillNamespace, err error)
	Get(ctx context.Context, namespaceId string) (*entity.SkillNamespace, error)
	GetBySlug(ctx context.Context, slug string) (*entity.SkillNamespace, error)
	Add(ctx context.Context, n *entity.SkillNamespace) (string, error)
	Edit(ctx context.Context, n *entity.SkillNamespace) error
	Delete(ctx context.Context, namespaceId string) error
	// 成员
	ListMembers(ctx context.Context, namespaceId string) ([]*entity.SkillNamespaceMember, error)
	AddMember(ctx context.Context, m *entity.SkillNamespaceMember) error
	RemoveMember(ctx context.Context, memberId string) error
}

// ===== 第三方市场目录（aa_skill_market_site：只存链接，引导用户去原站搜索） =====
type ISkillMarketSiteRepository interface {
	List(ctx context.Context, keyword, status string, pageNum, pageSize int) (list []*entity.SkillMarketSite, total int64, err error)
	All(ctx context.Context) (list []*entity.SkillMarketSite, err error) // 仅 status=1，目录展示用
	Get(ctx context.Context, marketId string) (*entity.SkillMarketSite, error)
	Add(ctx context.Context, m *entity.SkillMarketSite) (string, error)
	Edit(ctx context.Context, m *entity.SkillMarketSite) error
	Delete(ctx context.Context, marketId string) error
}

// ===== 技能 =====
type ISkillRepository interface {
	List(ctx context.Context, namespaceId, slug, name, status, keyWords string, pageNum, pageSize int) (list []*entity.Skill, total int64, err error)
	Get(ctx context.Context, skillId string) (*entity.Skill, error)
	// ListBySlug 按 slug 精确匹配（List 的 slug 参数是 like 模糊，不适合回查）。
	ListBySlug(ctx context.Context, slug string) (list []*entity.Skill, err error)
	Add(ctx context.Context, s *entity.Skill) (string, error)
	Edit(ctx context.Context, s *entity.Skill) error
	// EditFields 显式列更新（Updates(struct) 跳零值，visibility=0/status=0/清空文本需走这里）。
	EditFields(ctx context.Context, skillId string, fields map[string]any) error
	Delete(ctx context.Context, skillIds []string) error
	SetHidden(ctx context.Context, skillId string, hidden uint, by string) error
	// 版本
	ListVersions(ctx context.Context, skillId string) ([]*entity.SkillVersion, error)
	GetVersion(ctx context.Context, versionId string) (*entity.SkillVersion, error)
	AddVersion(ctx context.Context, v *entity.SkillVersion) (string, error)
	PublishVersion(ctx context.Context, versionId string) error
	YankVersion(ctx context.Context, versionId, by, reason string) error
	SetLatestVersion(ctx context.Context, skillId, versionId string) error
	// 标签
	ListTags(ctx context.Context, skillId string) ([]*entity.SkillTag, error)
	AddTag(ctx context.Context, t *entity.SkillTag) error
	RemoveTag(ctx context.Context, tagId string) error
	// 文件
	ListFiles(ctx context.Context, versionId string) ([]*entity.SkillFile, error)
	GetFile(ctx context.Context, versionId, filePath string) (*entity.SkillFile, error)
	AddFile(ctx context.Context, f *entity.SkillFile) error
	// 搜索文档（含向量）
	UpsertSearchDocument(ctx context.Context, skillId, title, summary, keywords, searchText, namespaceId, namespaceSlug, ownerId string, visibility, status uint, vectorText string) error
	SearchByVector(ctx context.Context, queryVectorText string, limit int) ([]map[string]interface{}, error)
}

// ===== 标签 =====
type ISkillLabelRepository interface {
	List(ctx context.Context, slug, ltype string, pageNum, pageSize int) (list []*entity.SkillLabelDefinition, total int64, err error)
	All(ctx context.Context) (list []*entity.SkillLabelDefinition, err error)
	Get(ctx context.Context, labelId string) (*entity.SkillLabelDefinition, error)
	Add(ctx context.Context, l *entity.SkillLabelDefinition) (string, error)
	Edit(ctx context.Context, l *entity.SkillLabelDefinition) error
	Delete(ctx context.Context, labelId string) error
	// CreatorMapByIds 批量取 user_id→name（跨库读 aiarsenal.aa_user），供「创建人」列回填。
	CreatorMapByIds(ctx context.Context, userIds []string) (map[string]string, error)
	// 翻译
	ListTranslations(ctx context.Context, labelId string) ([]*entity.SkillLabelTranslation, error)
	UpsertTranslation(ctx context.Context, t *entity.SkillLabelTranslation) error
}

// ===== 安全审计 =====
type ISkillSecurityRepository interface {
	ListAudits(ctx context.Context, versionId string, status string, pageNum, pageSize int) (list []*entity.SkillSecurityAudit, total int64, err error)
	GetAudit(ctx context.Context, auditId string) (*entity.SkillSecurityAudit, error)
	AddAudit(ctx context.Context, a *entity.SkillSecurityAudit) (string, error)
	Rescan(ctx context.Context, versionId string) (string, error) // 创建新扫描任务，返回 auditId
	UpdateAudit(ctx context.Context, auditId string, status, severity uint, findings json.RawMessage, scannerRaw string) error
	// LatestAuditBySkillVersion 取版本最新一条审计（安装/发布门禁用；无记录返回 err）。
	LatestAuditBySkillVersion(ctx context.Context, versionId string) (*entity.SkillSecurityAudit, error)
	// LatestAuditsByVersions 批量取各版本最新审计（版本列表回填 auditStatus 用）。
	LatestAuditsByVersions(ctx context.Context, versionIds []string) (map[string]*entity.SkillSecurityAudit, error)
}
