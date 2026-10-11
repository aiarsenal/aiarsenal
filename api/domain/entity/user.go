package entity

import "aiarsenal/domain/types"

// User 对应表 aa_user。Id 为内部 bigint 自增主键（不对外）；
// UserId 为 32 位业务 id（对外 = id），DeptId 引用 aa_dept.dept_id。
type User struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	UserId        string      `gorm:"column:user_id;uniqueIndex" json:"id"`
	UserName      string      `gorm:"column:user_name" json:"userName"`
	EmployeeNo    string      `gorm:"column:employee_no" json:"employeeNo"`
	Name          string      `gorm:"column:name" json:"name"`
	Mobile        string      `gorm:"column:mobile" json:"mobile"`
	UserNickname  string      `gorm:"column:user_nickname" json:"userNickname"`
	Birthday      int         `gorm:"column:birthday" json:"birthday"`
	UserPassword  string      `gorm:"column:user_password" json:"userPassword"`
	UserSalt      string      `gorm:"column:user_salt" json:"userSalt"`
	UserStatus    uint        `gorm:"column:user_status" json:"userStatus"`
	UserEmail     string      `gorm:"column:user_email" json:"userEmail"`
	Sex           int         `gorm:"column:sex" json:"sex"`
	Avatar        string      `gorm:"column:avatar" json:"avatar"`
	DeptId        string      `gorm:"column:dept_id" json:"deptId"`
	Remark        string      `gorm:"column:remark" json:"remark"`
	IsAdmin       int         `gorm:"column:is_admin" json:"isAdmin"`
	Address       string      `gorm:"column:address" json:"address"`
	Describe      string      `gorm:"column:describe" json:"describe"`
	LastLoginIp   string      `gorm:"column:last_login_ip" json:"lastLoginIp"`
	LastLoginTime *types.Time `gorm:"column:last_login_time" json:"lastLoginTime"`
	CreatedAt     *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt     *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (User) TableName() string { return "aa_user" }
