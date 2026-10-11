package dto

import "aiarsenal/domain/entity"

// ===== 个人消息通知 =====
type NoticeListReq struct {
	IsRead    string `json:"isRead" form:"isRead"`       // 0=未读 1=已读，空=全部
	Type      string `json:"type" form:"type"`           // info/warning/success/error，空=全部
	StartTime string `json:"startTime" form:"startTime"` // 创建时间起（含），YYYY-MM-DD 或 YYYY-MM-DD HH:MM:SS
	EndTime   string `json:"endTime" form:"endTime"`     // 创建时间止（含）
	PageReq
}
type NoticeListRes struct {
	ListRes
	NoticeList []*entity.Notice `json:"noticeList"`
}
type NoticeReadReq struct {
	NoticeId string `json:"noticeId" form:"noticeId" binding:"required"`
}
type NoticeDeleteReq struct {
	Ids []string `json:"ids" form:"ids"`
}
