package entity

import "aiarsenal/domain/types"

// Role 对应表 aa_role。Id 为内部 bigint 自增主键，RoleId 为 32 位业务 id（对外 = id）。
type Role struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RoleId    string      `gorm:"column:role_id;uniqueIndex" json:"id"`
	Status    uint        `gorm:"column:status" json:"status"`
	ListOrder uint        `gorm:"column:list_order" json:"listOrder"`
	Name      string      `gorm:"column:name" json:"name"`
	Remark    string      `gorm:"column:remark" json:"remark"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (Role) TableName() string { return "aa_role" }
