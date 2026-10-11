package entity

import (
	"encoding/json"

	"aiarsenal/domain/types"
)

// SkillVersion 对应表 aa_skill_version（技能版本）。
// VersionId 为业务 id；SkillId 引用 aa_skill.skill_id。
type SkillVersion struct {
	Id                 uint            `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	VersionId          string          `gorm:"column:version_id;uniqueIndex" json:"versionId"`
	SkillId            string          `gorm:"column:skill_id" json:"skillId"`
	Version            string          `gorm:"column:version" json:"version"`
	Status             uint            `gorm:"column:status" json:"status"` // 0=DRAFT 1=PUBLISHED 2=YANKED
	Changelog          string          `gorm:"column:changelog" json:"changelog"`
	ParsedMetadataJson json.RawMessage `gorm:"column:parsed_metadata_json;type:jsonb" json:"parsedMetadataJson,omitempty"`
	ManifestJson       json.RawMessage `gorm:"column:manifest_json;type:jsonb" json:"manifestJson,omitempty"`
	FileCount          int             `gorm:"column:file_count" json:"fileCount"`
	TotalSize          int64           `gorm:"column:total_size" json:"totalSize"`
	PublishedAt        *types.Time     `gorm:"column:published_at" json:"publishedAt"`
	YankedAt           *types.Time     `gorm:"column:yanked_at" json:"yankedAt"`
	YankedBy           string          `gorm:"column:yanked_by" json:"yankedBy"`
	YankReason         string          `gorm:"column:yank_reason" json:"yankReason"`
	BundleReady        uint            `gorm:"column:bundle_ready" json:"bundleReady"`
	DownloadReady      uint            `gorm:"column:download_ready" json:"downloadReady"`
	CreatedBy          string          `gorm:"column:created_by" json:"createdBy"`
	CreatedAt          *types.Time     `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt          *types.Time     `gorm:"column:updated_at" json:"updatedAt"`
	// AuditStatus 该版本最新安全审计状态（entity.AuditStatus*，非持久化列——
	// ListVersions 出参前由 service 批量回填；nil=未回填/无审计记录）。
	AuditStatus *uint `gorm:"-" json:"auditStatus,omitempty"`
}

func (SkillVersion) TableName() string { return "aa_skill_version" }

// SkillFile 对应表 aa_skill_file。VersionId 引用 aa_skill_version.version_id。
type SkillFile struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	FileId      string      `gorm:"column:file_id;uniqueIndex" json:"fileId"`
	VersionId   string      `gorm:"column:version_id" json:"versionId"`
	FilePath    string      `gorm:"column:file_path" json:"filePath"`
	FileSize    int64       `gorm:"column:file_size" json:"fileSize"`
	ContentType string      `gorm:"column:content_type" json:"contentType"`
	Sha256      string      `gorm:"column:sha256" json:"sha256"`
	StorageKey  string      `gorm:"column:storage_key" json:"storageKey"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (SkillFile) TableName() string { return "aa_skill_file" }

// SkillTag 对应表 aa_skill_tag。
type SkillTag struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TagId     string      `gorm:"column:tag_id;uniqueIndex" json:"tagId"`
	SkillId   string      `gorm:"column:skill_id" json:"skillId"`
	TagName   string      `gorm:"column:tag_name" json:"tagName"`
	VersionId string      `gorm:"column:version_id" json:"versionId"`
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SkillTag) TableName() string { return "aa_skill_tag" }

// SkillVersionStats 对应表 aa_skill_version_stats（版本下载统计）。
type SkillVersionStats struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	VersionId     string      `gorm:"column:version_id;uniqueIndex" json:"versionId"`
	SkillId       string      `gorm:"column:skill_id" json:"skillId"`
	DownloadCount int64       `gorm:"column:download_count" json:"downloadCount"`
	UpdatedAt     *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SkillVersionStats) TableName() string { return "aa_skill_version_stats" }
