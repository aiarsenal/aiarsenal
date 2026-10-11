package entity

import "aiarsenal/domain/types"

// Skill 对应表 aa_skill（技能主表）。
// SkillId 为业务 id；NamespaceId 引用 aa_skill_namespace.namespace_id；
// OwnerId / CreatedBy / UpdatedBy / HiddenBy 引用 aa_user.user_id；
// LatestVersionId 引用 aa_skill_version.version_id；SourceSkillId 引用源 skill。
type Skill struct {
	Id                uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	SkillId           string      `gorm:"column:skill_id;uniqueIndex" json:"skillId"`
	NamespaceId       string      `gorm:"column:namespace_id" json:"namespaceId"`
	Slug              string      `gorm:"column:slug" json:"slug"`
	DisplayName       string      `gorm:"column:display_name" json:"displayName"`
	Summary           string      `gorm:"column:summary" json:"summary"`
	OwnerId           string      `gorm:"column:owner_id" json:"ownerId"`
	OwnerName         string      `gorm:"-" json:"ownerName"`  // 回填：上传人姓名（aa_user.name）
	OwnerEmpNo        string      `gorm:"-" json:"ownerEmpNo"` // 回填：上传人工号（aa_user.employee_no）
	SourceSkillId     string      `gorm:"column:source_skill_id" json:"sourceSkillId"`
	Source            string      `gorm:"column:source" json:"source"`                  // 第三方市场来源标识（xfyun 等）；空=本地上传
	SourceUrl         string      `gorm:"column:source_url" json:"sourceUrl"`           // 该 skill 在来源站的详情链接
	UpstreamAuthor    string      `gorm:"column:upstream_author" json:"upstreamAuthor"` // 原作者（外部市场作者 handle，如 @user_xxx；或包内 SKILL.md author）
	MarketRef         string      `gorm:"column:market_ref" json:"marketRef"`           // 第三方市场溯源标识 {source}:{slug}（SourceSkillId varchar(32) 装不下）
	Visibility        uint        `gorm:"column:visibility" json:"visibility"`          // 0=PUBLIC 1=PRIVATE
	Status            uint        `gorm:"column:status" json:"status"`                  // 1=ACTIVE 0=INACTIVE
	LatestVersionId   string      `gorm:"column:latest_version_id" json:"latestVersionId"`
	Hidden            uint        `gorm:"column:hidden" json:"hidden"`
	HiddenAt          *types.Time `gorm:"column:hidden_at" json:"hiddenAt"`
	HiddenBy          string      `gorm:"column:hidden_by" json:"hiddenBy"`
	DownloadCount     int64       `gorm:"column:download_count" json:"downloadCount"`
	StarCount         int         `gorm:"column:star_count" json:"starCount"`
	RatingAvg         float64     `gorm:"column:rating_avg" json:"ratingAvg"`
	RatingCount       int         `gorm:"column:rating_count" json:"ratingCount"`
	SubscriptionCount int         `gorm:"column:subscription_count" json:"subscriptionCount"`
	CreatedBy         string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt         *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedBy         string      `gorm:"column:updated_by" json:"updatedBy"`
	UpdatedAt         *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt         *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (Skill) TableName() string { return "aa_skill" }
