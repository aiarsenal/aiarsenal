package entity

import "aiarsenal/domain/types"

// SkillLabelDefinition 对应表 aa_skill_label_definition（标签定义）。
type SkillLabelDefinition struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	LabelId         string      `gorm:"column:label_id;uniqueIndex" json:"labelId"`
	Slug            string      `gorm:"column:slug" json:"slug"`
	Type            string      `gorm:"column:type" json:"type"` // RECOMMENDED / PRIVILEGED
	VisibleInFilter uint        `gorm:"column:visible_in_filter" json:"visibleInFilter"`
	SortOrder       int         `gorm:"column:sort_order" json:"sortOrder"`
	CreatedBy       string      `gorm:"column:created_by" json:"createdBy"`
	CreatorName     string      `gorm:"-" json:"creatorName"` // 由 service 跨库回填 aa_user.name，非列
	CreatedAt       *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SkillLabelDefinition) TableName() string { return "aa_skill_label_definition" }

// SkillLabelTranslation 对应表 aa_skill_label_translation（标签多语言）。
type SkillLabelTranslation struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	TransId     string      `gorm:"column:trans_id;uniqueIndex" json:"transId"`
	LabelId     string      `gorm:"column:label_id" json:"labelId"`
	Locale      string      `gorm:"column:locale" json:"locale"`
	DisplayName string      `gorm:"column:display_name" json:"displayName"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SkillLabelTranslation) TableName() string { return "aa_skill_label_translation" }

// SkillLabel 对应表 aa_skill_label（技能-标签关联）。
type SkillLabel struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	SkillLabelId string      `gorm:"column:skill_label_id;uniqueIndex" json:"skillLabelId"`
	SkillId      string      `gorm:"column:skill_id" json:"skillId"`
	LabelId      string      `gorm:"column:label_id" json:"labelId"`
	CreatedBy    string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
}

func (SkillLabel) TableName() string { return "aa_skill_label" }
