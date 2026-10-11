package entity

import "aiarsenal/domain/types"

// Dept 对应表 aa_dept。Id 为内部 bigint 自增主键；
// DeptId 为 32 位业务 id（对外 = deptId），ParentId 引用父部门的 dept_id。
type Dept struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	DeptId    string      `gorm:"column:dept_id;uniqueIndex" json:"deptId"`
	ParentId  string      `gorm:"column:parent_id" json:"parentId"`
	Ancestors string      `gorm:"column:ancestors" json:"ancestors"`
	DeptName  string      `gorm:"column:dept_name" json:"deptName"`
	OrderNum  int         `gorm:"column:order_num" json:"orderNum"`
	LeaderId  string      `gorm:"column:leader_id" json:"leaderId"`
	Status    uint        `gorm:"column:status" json:"status"`
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy string      `gorm:"column:updated_by" json:"updatedBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (Dept) TableName() string { return "aa_dept" }
