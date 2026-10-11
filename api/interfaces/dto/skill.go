package dto

import "aiarsenal/domain/entity"

// ===== 命名空间 =====
type SkillNamespaceSearchReq struct {
	Slug   string `json:"slug" form:"slug"`
	Name   string `json:"name" form:"name"`
	Type   string `json:"type" form:"type"`
	Status string `json:"status" form:"status"`
	PageReq
}
type SkillNamespaceSearchRes struct {
	ListRes
	List []*entity.SkillNamespace `json:"list"`
}
type SkillNamespaceReq struct {
	Slug        string `json:"slug" form:"slug" binding:"required"`
	DisplayName string `json:"displayName" form:"displayName" binding:"required"`
	Type        string `json:"type" form:"type"`
	Description string `json:"description" form:"description"`
	Avatar      string `json:"avatar" form:"avatar"`
	Status      uint   `json:"status" form:"status"`
}
type SkillNamespaceAddReq struct{ *SkillNamespaceReq }
type SkillNamespaceEditReq struct {
	NamespaceId string `json:"namespaceId" form:"namespaceId" binding:"required"`
	*SkillNamespaceReq
}
type SkillNamespaceDeleteReq struct {
	Id string `json:"id" form:"id" binding:"required"`
}
type SkillNamespaceMemberAddReq struct {
	NamespaceId string `json:"namespaceId" form:"namespaceId" binding:"required"`
	UserId      string `json:"userId" form:"userId" binding:"required"`
	Role        string `json:"role" form:"role"`
}
type SkillNamespaceMemberDelReq struct {
	MemberId string `json:"memberId" form:"memberId" binding:"required"`
}

// ===== 技能 =====
type SkillSearchReq struct {
	NamespaceId string `json:"namespaceId" form:"namespaceId"`
	Slug        string `json:"slug" form:"slug"`
	Name        string `json:"name" form:"name"`
	Status      string `json:"status" form:"status"`
	KeyWords    string `json:"keyWords" form:"keyWords"`
	PageReq
}
type SkillSearchRes struct {
	ListRes
	List []*entity.Skill `json:"list"`
}
type SkillReq struct {
	NamespaceId string `json:"namespaceId" form:"namespaceId" binding:"required"`
	Slug        string `json:"slug" form:"slug" binding:"required"`
	DisplayName string `json:"displayName" form:"displayName" binding:"required"`
	Summary     string `json:"summary" form:"summary"`
	Visibility  uint   `json:"visibility" form:"visibility"`
	Status      uint   `json:"status" form:"status"`
}
type SkillAddReq struct{ *SkillReq }
type SkillEditReq struct {
	SkillId string `json:"skillId" form:"skillId" binding:"required"`
	*SkillReq
}
type SkillDeleteReq struct {
	Ids []string `json:"ids" form:"ids"`
}
type SkillHiddenReq struct {
	SkillId string `json:"skillId" form:"skillId" binding:"required"`
	Hidden  uint   `json:"hidden" form:"hidden"`
}
type SkillVersionAddReq struct {
	SkillId   string `json:"skillId" form:"skillId" binding:"required"`
	Version   string `json:"version" form:"version" binding:"required"`
	Changelog string `json:"changelog" form:"changelog"`
}
type SkillVersionYankReq struct {
	VersionId string `json:"versionId" form:"versionId" binding:"required"`
	Reason    string `json:"reason" form:"reason"`
}
type SkillTagAddReq struct {
	SkillId string `json:"skillId" form:"skillId" binding:"required"`
	TagName string `json:"tagName" form:"tagName" binding:"required"`
}
type SkillTagDelReq struct {
	TagId string `json:"tagId" form:"tagId" binding:"required"`
}

// ===== 标签 =====
type SkillLabelSearchReq struct {
	Slug string `json:"slug" form:"slug"`
	Type string `json:"type" form:"type"`
	PageReq
}
type SkillLabelSearchRes struct {
	ListRes
	List []*entity.SkillLabelDefinition `json:"list"`
}
type SkillLabelReq struct {
	Slug            string `json:"slug" form:"slug" binding:"required"`
	Type            string `json:"type" form:"type"`
	VisibleInFilter uint   `json:"visibleInFilter" form:"visibleInFilter"`
	SortOrder       int    `json:"sortOrder" form:"sortOrder"`
}
type SkillLabelAddReq struct{ *SkillLabelReq }
type SkillLabelEditReq struct {
	LabelId string `json:"labelId" form:"labelId" binding:"required"`
	*SkillLabelReq
}
type SkillLabelDeleteReq struct {
	Id string `json:"id" form:"id" binding:"required"`
}
type SkillLabelTransReq struct {
	LabelId     string `json:"labelId" form:"labelId" binding:"required"`
	Locale      string `json:"locale" form:"locale" binding:"required"`
	DisplayName string `json:"displayName" form:"displayName" binding:"required"`
}

// ===== 安全审计 =====
type SkillAuditSearchReq struct {
	VersionId string `json:"versionId" form:"versionId"`
	Status    string `json:"status" form:"status"`
	PageReq
}
type SkillAuditSearchRes struct {
	ListRes
	List []*entity.SkillSecurityAudit `json:"list"`
}
type SkillAuditRescanReq struct {
	VersionId string `json:"versionId" form:"versionId" binding:"required"`
}
