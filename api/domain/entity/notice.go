package entity

import "aiarsenal/domain/types"

// Notice 对应表 aa_notice。个人消息通知，user_id 软引用 aa_user.user_id。
type Notice struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	NoticeId  string      `gorm:"column:notice_id;uniqueIndex" json:"noticeId"`
	UserId    string      `gorm:"column:user_id" json:"userId"`
	Title     string      `gorm:"column:title" json:"title"`
	Content   string      `gorm:"column:content" json:"content"`
	Link      string      `gorm:"column:link" json:"link"`      // http*=外链 /xxx=内部路由 #xxx=弹窗
	Type      string      `gorm:"column:type" json:"type"`      // info/warning/success/error
	IsRead    uint        `gorm:"column:is_read" json:"isRead"` // 0=未读 1=已读
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (Notice) TableName() string { return "aa_notice" }
