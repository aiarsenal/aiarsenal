package entity

import "aiarsenal/domain/types"

// PermGroup 对应表 aa_perm_group（权限分组：可复用的组合授权词条）。
// Terms 为词条行 JSON（[]model.GrantRow：六维任意组合、行内 AND，多行 OR）；
// 引用在鉴权时经 grantmeta 快照实时解析，改词条即时生效。
type PermGroup struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	GroupId   string      `gorm:"column:group_id;uniqueIndex" json:"groupId"`
	Name      string      `gorm:"column:name" json:"name"`
	Remark    string      `gorm:"column:remark" json:"remark"`
	Terms     string      `gorm:"column:terms" json:"terms"` // jsonb 数组文本
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	// 回填：创建人「姓名（工号）」（服务层批量解析 aa_user，非库列）
	CreatedByDesc string `gorm:"-" json:"createdByDesc"`
}

func (PermGroup) TableName() string { return "aa_perm_group" }

// EmpSet 对应表 aa_emp_set（员工集合：工号数组，内部或关系）。
type EmpSet struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	SetId     string      `gorm:"column:set_id;uniqueIndex" json:"setId"`
	Name      string      `gorm:"column:name" json:"name"`
	Remark    string      `gorm:"column:remark" json:"remark"`
	EmpNos    string      `gorm:"column:emp_nos" json:"empNos"` // jsonb 数组文本
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	// 回填：创建人「姓名（工号）」（服务层批量解析 aa_user，非库列）
	CreatedByDesc string `gorm:"-" json:"createdByDesc"`
}

func (EmpSet) TableName() string { return "aa_emp_set" }
