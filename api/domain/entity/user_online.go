package entity

import "aiarsenal/domain/types"

// UserOnline 对应表 aa_user_online。Id 为内部 bigint 自增主键；
// uuid 为会话 token 的 md5 哈希（32 位字符串），作为在线会话的业务标识（对外 = id）。
type UserOnline struct {
	Id         uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	Uuid       string      `gorm:"column:uuid;uniqueIndex" json:"id"`
	Token      string      `gorm:"column:token" json:"token"`
	CreateTime *types.Time `gorm:"column:create_time" json:"createTime"`
	UserName   string      `gorm:"column:user_name" json:"userName"`
	Ip         string      `gorm:"column:ip" json:"ip"`
	Explorer   string      `gorm:"column:explorer" json:"explorer"`
	Os         string      `gorm:"column:os" json:"os"`
}

func (UserOnline) TableName() string { return "aa_user_online" }
