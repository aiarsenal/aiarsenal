package dto

// 日程模块请求结构。时间串一律 "2006-01-02 15:04:05"（也接受 "2006-01-02 15:04" / "2006-01-02"）。

// ScheduleEventAddReq 创建/编辑日程（编辑时带 eventId）
type ScheduleEventAddReq struct {
	EventId         string   `json:"eventId" form:"eventId"`
	Title           string   `json:"title" form:"title" binding:"required"`
	EventType       int      `json:"eventType" form:"eventType"` // 0日程 1线下会议 2线上会议 3线上+线下
	StartAt         string   `json:"startAt" form:"startAt" binding:"required"`
	EndAt           string   `json:"endAt" form:"endAt" binding:"required"`
	RRule           string   `json:"rrule" form:"rrule"` // 空=单场；{"freq":"WEEKLY","byday":["FR"],"interval":1,"until":"2026-12-31","count":20}
	RoomId          string   `json:"roomId" form:"roomId"`
	OnlineProvider  string   `json:"onlineProvider" form:"onlineProvider"`
	OnlineUrl       string   `json:"onlineUrl" form:"onlineUrl"`
	Location        string   `json:"location" form:"location"`
	Attendees       []string `json:"attendees" form:"attendees"` // user_id 列表
	Remark          string   `json:"remark" form:"remark"`
	IgnoreConflicts bool     `json:"ignoreConflicts" form:"ignoreConflicts"`
	DryRun          bool     `json:"dryRun" form:"dryRun"` // true=只预检（冲突+摘要）不落库
}

// ScheduleEventListReq 我的日程窗口
type ScheduleEventListReq struct {
	From string `json:"from" form:"from"` // 默认今天 00:00
	To   string `json:"to" form:"to"`     // 默认 7 天后
}

type ScheduleEventDetailReq struct {
	EventId string `json:"eventId" form:"eventId" binding:"required"`
}

type ScheduleIdsReq struct {
	Ids []string `json:"ids"`
}

// ScheduleExceptionReq 单场取消/改期
type ScheduleExceptionReq struct {
	EventId    string `json:"eventId" form:"eventId" binding:"required"`
	Date       string `json:"date" form:"date" binding:"required"` // YYYY-MM-DD
	Status     int    `json:"status" form:"status" binding:"required"`
	NewStartAt string `json:"newStartAt" form:"newStartAt"`
	NewEndAt   string `json:"newEndAt" form:"newEndAt"`
	NewRoomId  string `json:"newRoomId" form:"newRoomId"`
}

// ScheduleRespondReq 参会回执
type ScheduleRespondReq struct {
	EventId string `json:"eventId" form:"eventId" binding:"required"`
	Status  int    `json:"status" form:"status" binding:"required"` // 1接受 2拒绝
}

// ScheduleFreeBusyReq 忙闲/找空闲
type ScheduleFreeBusyReq struct {
	UserIds        []string `json:"userIds" form:"userIds"`
	From           string   `json:"from" form:"from"`
	To             string   `json:"to" form:"to"`
	DurationMinute int      `json:"durationMinute" form:"durationMinute"`
}

// ScheduleRoomReq 会议室增删改
type ScheduleRoomReq struct {
	RoomId    string `json:"roomId" form:"roomId"`
	Name      string `json:"name" form:"name" binding:"required"`
	Location  string `json:"location" form:"location"`
	Capacity  int    `json:"capacity" form:"capacity"`
	Equipment string `json:"equipment" form:"equipment"` // jsonb ["电视","白板"]
	Enabled   int    `json:"enabled" form:"enabled"`
	Remark    string `json:"remark" form:"remark"`
}

type ScheduleRoomListReq struct {
	Name        string `json:"name" form:"name"`
	EnabledOnly bool   `json:"enabledOnly" form:"enabledOnly"`
	PageReq
}

type ScheduleRoomDayReq struct {
	Date string `json:"date" form:"date"` // 默认今天
}
