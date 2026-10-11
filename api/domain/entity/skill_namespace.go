package entity

import "aiarsenal/domain/types"

// SkillNamespace 对应表 aa_skill_namespace（命名空间）。
// NamespaceId 为 32 位业务 id；CreatedBy 引用 aa_user.user_id。
type SkillNamespace struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	NamespaceId string      `gorm:"column:namespace_id;uniqueIndex" json:"namespaceId"`
	Slug        string      `gorm:"column:slug" json:"slug"`
	DisplayName string      `gorm:"column:display_name" json:"displayName"`
	Type        string      `gorm:"column:type" json:"type"` // GLOBAL / TEAM
	Description string      `gorm:"column:description" json:"description"`
	Avatar      string      `gorm:"column:avatar" json:"avatar"`
	Status      uint        `gorm:"column:status" json:"status"`
	CreatedBy   string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SkillNamespace) TableName() string { return "aa_skill_namespace" }

// SkillNamespaceMember 对应表 aa_skill_namespace_member。
// MemberId 为业务 id；NamespaceId 引用 aa_skill_namespace.namespace_id；
// UserId 引用 aa_user.user_id。
type SkillNamespaceMember struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	MemberId    string      `gorm:"column:member_id;uniqueIndex" json:"memberId"`
	NamespaceId string      `gorm:"column:namespace_id" json:"namespaceId"`
	UserId      string      `gorm:"column:user_id" json:"userId"`
	Role        string      `gorm:"column:role" json:"role"` // OWNER / ADMIN / MEMBER
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (SkillNamespaceMember) TableName() string { return "aa_skill_namespace_member" }
