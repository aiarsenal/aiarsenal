package entity

import "aiarsenal/domain/types"

// 日程模块实体（aa_schedule 库，表前缀 aa_schedule_ / aa_meeting_room）。
// 约定与 project 一致：id bigserial PK + <实体>_id varchar(32) UNIQUE（idgen 生成）；
//   时间 *types.Time（本地时刻 "2006-01-02 15:04:05"，GORM 不自动回填，repo 层显式盖）；
//   跨库软引用 organizer_id/user_id → aiarsenal.aa_user.user_id（varchar(32) biz_id，无 FK）；
//   jsonb 列用 string（自行解析）。
//
// 周期模型：series（母事件 + rrule）+ exception（单场取消/改期），不物化每场实例。
// 展开是纯函数（schedule_service.expandEvent），日历视图按查询窗口现算。

// ============ 事件/会议（series 母事件）============
type ScheduleEvent struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	EventId        string      `gorm:"column:event_id;uniqueIndex" json:"eventId"`
	Title          string      `gorm:"column:title" json:"title"`
	EventType      int         `gorm:"column:event_type" json:"eventType"` // 0普通日程 1线下会议 2线上会议 3线上+线下
	StartAt        *types.Time `gorm:"column:start_at" json:"startAt"`     // 首场开始（锚点，本地时刻）
	EndAt          *types.Time `gorm:"column:end_at" json:"endAt"`         // 首场结束（时长=End-Start）
	IsRecurring    int         `gorm:"column:is_recurring" json:"isRecurring"`
	RRule          string      `gorm:"column:rrule;type:jsonb" json:"rrule"` // {"freq":"WEEKLY","byday":["FR"],"interval":1,"until":"2026-12-31","count":20}
	Tz             string      `gorm:"column:tz" json:"tz"`                  // 展开时区，默认 Asia/Shanghai
	RoomId         string      `gorm:"column:room_id" json:"roomId"`         // 软引用 aa_meeting_room.room_id
	OnlineProvider string      `gorm:"column:online_provider" json:"onlineProvider"`
	OnlineUrl      string      `gorm:"column:online_url" json:"onlineUrl"`
	Location       string      `gorm:"column:location" json:"location"` // 会议室之外的自定义地点
	OrganizerId    string      `gorm:"column:organizer_id" json:"organizerId"`
	Remark         string      `gorm:"column:remark" json:"remark"`
	CreatedBy      string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt      *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt      *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ScheduleEvent) TableName() string { return "aa_schedule_event" }

// ============ 参会人 ============
type ScheduleAttendee struct {
	Id          uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AttendeeId  string      `gorm:"column:attendee_id;uniqueIndex" json:"attendeeId"`
	EventId     string      `gorm:"column:event_id" json:"eventId"`
	UserId      string      `gorm:"column:user_id" json:"userId"` // 软引用 aa_user.user_id
	UserName    string      `gorm:"-" json:"userName"`            // 跨库回填 aa_user.name，展示用
	Status      int         `gorm:"column:status" json:"status"` // 0待确认 1接受 2拒绝
	RespondedAt *types.Time `gorm:"column:responded_at" json:"respondedAt"`
	CreatedAt   *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt   *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (ScheduleAttendee) TableName() string { return "aa_schedule_attendee" }

// ============ 单场例外（不物化场次，只记差异）============
type ScheduleException struct {
	Id             uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	ExceptionId    string      `gorm:"column:exception_id;uniqueIndex" json:"exceptionId"`
	EventId        string      `gorm:"column:event_id" json:"eventId"`
	OccurrenceDate string      `gorm:"column:occurrence_date" json:"occurrenceDate"` // 该场次日期 YYYY-MM-DD
	Status         int         `gorm:"column:status" json:"status"`                  // 0取消该场 1该场改期
	NewStartAt     *types.Time `gorm:"column:new_start_at" json:"newStartAt"`
	NewEndAt       *types.Time `gorm:"column:new_end_at" json:"newEndAt"`
	NewRoomId      string      `gorm:"column:new_room_id" json:"newRoomId"`
	NewOnlineUrl   string      `gorm:"column:new_online_url" json:"newOnlineUrl"`
	CreatedAt      *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (ScheduleException) TableName() string { return "aa_schedule_exception" }

// ============ 会议室（资源，冲突=同房间区间重叠）============
type MeetingRoom struct {
	Id        uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RoomId    string      `gorm:"column:room_id;uniqueIndex" json:"roomId"`
	Name      string      `gorm:"column:name" json:"name"`
	Location  string      `gorm:"column:location" json:"location"`
	Capacity  int         `gorm:"column:capacity" json:"capacity"`
	Equipment string      `gorm:"column:equipment;type:jsonb" json:"equipment"` // ["电视","白板"]
	Enabled   int         `gorm:"column:enabled" json:"enabled"`
	Remark    string      `gorm:"column:remark" json:"remark"`
	CreatedBy string      `gorm:"column:created_by" json:"createdBy"`
	CreatedAt *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (MeetingRoom) TableName() string { return "aa_meeting_room" }
