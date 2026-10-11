package entity

import "aiarsenal/domain/types"

// LoginLog 对应表 aa_login_log。Id 为内部 bigint 自增主键，InfoId 为 32 位业务 id（对外 = infoId）。
type LoginLog struct {
	Id            uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	InfoId        string      `gorm:"column:info_id;uniqueIndex" json:"infoId"`
	LoginName     string      `gorm:"column:login_name" json:"loginName"`
	Ipaddr        string      `gorm:"column:ipaddr" json:"ipaddr"`
	LoginLocation string      `gorm:"column:login_location" json:"loginLocation"`
	Browser       string      `gorm:"column:browser" json:"browser"`
	Os            string      `gorm:"column:os" json:"os"`
	Status        int         `gorm:"column:status" json:"status"`
	Msg           string      `gorm:"column:msg" json:"msg"`
	LoginTime     *types.Time `gorm:"column:login_time" json:"loginTime"`
	Module        string      `gorm:"column:module" json:"module"`
}

func (LoginLog) TableName() string { return "aa_login_log" }
