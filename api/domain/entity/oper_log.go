package entity

import "aiarsenal/domain/types"

// OperLog 对应表 aa_oper_log。Id 为内部 bigint 自增主键，OperId 为 32 位业务 id（对外 = operId）。
type OperLog struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	OperId        string      `gorm:"column:oper_id;uniqueIndex" json:"operId"`
	Title         string      `gorm:"column:title" json:"title"`
	BusinessType  int         `gorm:"column:business_type" json:"businessType"`
	Method        string      `gorm:"column:method" json:"method"`
	RequestMethod string      `gorm:"column:request_method" json:"requestMethod"`
	OperatorType  int         `gorm:"column:operator_type" json:"operatorType"`
	OperName      string      `gorm:"column:oper_name" json:"operName"`
	DeptName      string      `gorm:"column:dept_name" json:"deptName"`
	OperUrl       string      `gorm:"column:oper_url" json:"operUrl"`
	OperIp        string      `gorm:"column:oper_ip" json:"operIp"`
	OperLocation  string      `gorm:"column:oper_location" json:"operLocation"`
	OperParam     string      `gorm:"column:oper_param" json:"operParam"`
	ErrorMsg      string      `gorm:"column:error_msg" json:"errorMsg"`
	OperTime      *types.Time `gorm:"column:oper_time" json:"operTime"`
}

func (OperLog) TableName() string { return "aa_oper_log" }
