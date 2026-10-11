package entity

import "aiarsenal/domain/types"

// agent_user.go — Agent 平台用户（v2 占位菜单：Agent 用户管理）。
//
// 与主库 aa_user 解耦的软引用模型：username 对应 aa_user.username（便于
// 检索/展示，不存 FK），本表自有角色/口令哈希/备注与实例授权（instance_ids
// jsonb 数组，元素为 aa_agent_instance.instance_id）。后续 v2 权限体系（如按
// 用户授权可访问哪些实例、配额复用等）以此表为锚点扩展。

// AgentUser Agent 平台用户。
type AgentUser struct {
	Id           uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AgentUserId  string      `gorm:"column:agent_user_id;uniqueIndex" json:"agentUserId"`
	Username     string      `gorm:"column:username;uniqueIndex" json:"username"` // 软引用 aa_user.username（唯一，避免同名重复授权）
	DisplayName  string      `gorm:"column:display_name" json:"displayName"`      // 展示名（富化自 aa_user 或手填）
	Role         string      `gorm:"column:role" json:"role"`                     // admin / operator / member
	InstanceIds  string      `gorm:"column:instance_ids;type:jsonb" json:"instanceIds"`
	IsEnabled    uint        `gorm:"column:is_enabled" json:"isEnabled"`
	PasswordSalt string      `gorm:"column:password_salt" json:"-"` // 口令盐（md5(md5(pw)+md5(salt))，同 aa_user 口径；明文仅创建/重置时一次性返回）
	PasswordHash string      `gorm:"column:password_hash" json:"-"`
	Remark       string      `gorm:"column:remark" json:"remark"`
	CreatedBy    string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt    *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (AgentUser) TableName() string { return "aa_agent_user" }
