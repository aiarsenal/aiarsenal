package repository

import (
	"context"
	"time"

	"aiarsenal/domain/entity"
)

// 日程模块仓储接口（aa_schedule 库）。
// 事件按窗口查询的语义：is_recurring=1 的 series 不看 end_at（可能无限延伸），
// 由 service 层展开 rrule 后再与窗口求交；单场仍用 end_at > from and start_at < to 粗筛。

type IScheduleEventRepository interface {
	List(ctx context.Context, keyword string, pageNum, pageSize int) (list []*entity.ScheduleEvent, total int64, err error)
	ListByIds(ctx context.Context, ids []string) (list []*entity.ScheduleEvent, err error)
	// ListOverlapping 单场事件粗筛：deleted 且 [start_at,end_at) 与窗口相交
	ListOverlapping(ctx context.Context, from, to time.Time) (list []*entity.ScheduleEvent, err error)
	// ListRoomEvents 房间视角候选：指定房间（或全部带房间事件），含全部周期 series
	ListRoomEvents(ctx context.Context, roomID string, from, to time.Time, includeRecurring bool) (list []*entity.ScheduleEvent, err error)
	// ListByUser 用户视角候选：我是组织者或参会人，含全部周期 series
	ListByUser(ctx context.Context, userID string, from, to time.Time) (list []*entity.ScheduleEvent, err error)
	// ListByUsers 批量用户视角候选（freebusy/找空闲用）
	ListByUsers(ctx context.Context, userIDs []string, from, to time.Time) (list []*entity.ScheduleEvent, err error)
	Get(ctx context.Context, id string) (*entity.ScheduleEvent, error)
	Add(ctx context.Context, e *entity.ScheduleEvent) (string, error)
	Edit(ctx context.Context, e *entity.ScheduleEvent) error
	Delete(ctx context.Context, ids []string) error
}

type IScheduleAttendeeRepository interface {
	ListByEvent(ctx context.Context, eventID string) (list []*entity.ScheduleAttendee, err error)
	ListByEvents(ctx context.Context, eventIDs []string) (list []*entity.ScheduleAttendee, err error)
	GetByEventUser(ctx context.Context, eventID, userID string) (*entity.ScheduleAttendee, error)
	Add(ctx context.Context, e *entity.ScheduleAttendee) (string, error)
	Edit(ctx context.Context, e *entity.ScheduleAttendee) error
	DeleteByEvent(ctx context.Context, eventID string) error
	DeleteByEventUsers(ctx context.Context, eventID string, userIDs []string) error
}

type IScheduleExceptionRepository interface {
	ListByEvent(ctx context.Context, eventID string) (list []*entity.ScheduleException, err error)
	ListByEvents(ctx context.Context, eventIDs []string) (list []*entity.ScheduleException, err error)
	GetByEventDate(ctx context.Context, eventID, date string) (*entity.ScheduleException, error)
	// Upsert 单场例外按 (event_id, occurrence_date) 幂等覆盖
	Upsert(ctx context.Context, e *entity.ScheduleException) (string, error)
	DeleteByEventDate(ctx context.Context, eventID, date string) error
}

type IMeetingRoomRepository interface {
	List(ctx context.Context, keyword string, enabledOnly bool, pageNum, pageSize int) (list []*entity.MeetingRoom, total int64, err error)
	ListEnabled(ctx context.Context) (list []*entity.MeetingRoom, err error)
	Get(ctx context.Context, id string) (*entity.MeetingRoom, error)
	Add(ctx context.Context, e *entity.MeetingRoom) (string, error)
	Edit(ctx context.Context, e *entity.MeetingRoom) error
	Delete(ctx context.Context, ids []string) error
}
