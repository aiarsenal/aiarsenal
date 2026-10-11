package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// 日程模块仓储实现（aa_schedule 库）。统一走 db.ScheduleDB()。
// 写入时 idgen.New() 生成业务 id；*types.Time 不自动回填，显式盖 created_at/updated_at；
// 软删表 Where deleted_at is null；删除=置 deleted_at；Edit 用显式 map 避免 GORM 丢零值。

// ===================== Event =====================

type ScheduleEventRepository struct{ db *gorm.DB }

func NewScheduleEventRepository() *ScheduleEventRepository {
	return &ScheduleEventRepository{db: db.ScheduleDB()}
}

func (r *ScheduleEventRepository) List(ctx context.Context, keyword string, pageNum, pageSize int) (list []*entity.ScheduleEvent, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ScheduleEvent{}).Where("deleted_at is null")
	if keyword != "" {
		q = q.Where("title like ? or remark like ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id desc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *ScheduleEventRepository) ListByIds(ctx context.Context, ids []string) (list []*entity.ScheduleEvent, err error) {
	if len(ids) == 0 {
		return
	}
	err = r.db.WithContext(ctx).Where("event_id in ? and deleted_at is null", ids).Order("id asc").Find(&list).Error
	return
}

func (r *ScheduleEventRepository) ListOverlapping(ctx context.Context, from, to time.Time) (list []*entity.ScheduleEvent, err error) {
	err = r.db.WithContext(ctx).
		Where("deleted_at is null and is_recurring = 0 and end_at > ? and start_at < ?", from, to).
		Order("start_at asc").Find(&list).Error
	return
}

func (r *ScheduleEventRepository) ListRoomEvents(ctx context.Context, roomID string, from, to time.Time, includeRecurring bool) (list []*entity.ScheduleEvent, err error) {
	q := r.db.WithContext(ctx).Model(&entity.ScheduleEvent{}).
		Where("deleted_at is null and room_id <> ''")
	if roomID != "" {
		q = q.Where("room_id = ?", roomID)
	}
	if includeRecurring {
		// 周期 series 不看 end_at；单场仍与窗口粗筛
		q = q.Where("is_recurring = 1 or (is_recurring = 0 and end_at > ? and start_at < ?)", from, to)
	} else {
		q = q.Where("is_recurring = 0 and end_at > ? and start_at < ?", from, to)
	}
	err = q.Order("start_at asc").Find(&list).Error
	return
}

func (r *ScheduleEventRepository) ListByUser(ctx context.Context, userID string, from, to time.Time) (list []*entity.ScheduleEvent, err error) {
	err = r.db.WithContext(ctx).Model(&entity.ScheduleEvent{}).
		Where("deleted_at is null and (organizer_id = ? or event_id in (select event_id from aa_schedule_attendee where user_id = ? and deleted_at is null))"+
			" and (is_recurring = 1 or (is_recurring = 0 and end_at > ? and start_at < ?))", userID, userID, from, to).
		Order("start_at asc").Find(&list).Error
	return
}

func (r *ScheduleEventRepository) ListByUsers(ctx context.Context, userIDs []string, from, to time.Time) (list []*entity.ScheduleEvent, err error) {
	if len(userIDs) == 0 {
		return
	}
	err = r.db.WithContext(ctx).Model(&entity.ScheduleEvent{}).
		Where("deleted_at is null and (organizer_id in ? or event_id in (select event_id from aa_schedule_attendee where user_id in ? and deleted_at is null))"+
			" and (is_recurring = 1 or (is_recurring = 0 and end_at > ? and start_at < ?))", userIDs, userIDs, from, to).
		Order("start_at asc").Find(&list).Error
	return
}

func (r *ScheduleEventRepository) Get(ctx context.Context, id string) (*entity.ScheduleEvent, error) {
	var e entity.ScheduleEvent
	if err := r.db.WithContext(ctx).Where("event_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ScheduleEventRepository) Add(ctx context.Context, e *entity.ScheduleEvent) (string, error) {
	e.EventId = idgen.New()
	now := nowPtr()
	if e.CreatedAt == nil {
		e.CreatedAt = now
	}
	if e.UpdatedAt == nil {
		e.UpdatedAt = now
	}
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.EventId, nil
}

func (r *ScheduleEventRepository) Edit(ctx context.Context, e *entity.ScheduleEvent) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ScheduleEvent{}).Where("event_id = ?", e.EventId).Updates(map[string]any{
		"title":           e.Title,
		"event_type":      e.EventType,
		"start_at":        e.StartAt,
		"end_at":          e.EndAt,
		"is_recurring":    e.IsRecurring,
		"rrule":           e.RRule,
		"room_id":         e.RoomId,
		"online_provider": e.OnlineProvider,
		"online_url":      e.OnlineUrl,
		"location":        e.Location,
		"organizer_id":    e.OrganizerId,
		"remark":          e.Remark,
		"updated_at":      e.UpdatedAt,
	}).Error
}

func (r *ScheduleEventRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ScheduleEvent{}).
		Where("event_id in ?", ids).Update("deleted_at", time.Now()).Error
}

// ===================== Attendee =====================

type ScheduleAttendeeRepository struct{ db *gorm.DB }

func NewScheduleAttendeeRepository() *ScheduleAttendeeRepository {
	return &ScheduleAttendeeRepository{db: db.ScheduleDB()}
}

func (r *ScheduleAttendeeRepository) ListByEvent(ctx context.Context, eventID string) (list []*entity.ScheduleAttendee, err error) {
	err = r.db.WithContext(ctx).Where("event_id = ? and deleted_at is null", eventID).Order("id asc").Find(&list).Error
	return
}

func (r *ScheduleAttendeeRepository) ListByEvents(ctx context.Context, eventIDs []string) (list []*entity.ScheduleAttendee, err error) {
	if len(eventIDs) == 0 {
		return
	}
	err = r.db.WithContext(ctx).Where("event_id in ? and deleted_at is null", eventIDs).Order("id asc").Find(&list).Error
	return
}

func (r *ScheduleAttendeeRepository) GetByEventUser(ctx context.Context, eventID, userID string) (*entity.ScheduleAttendee, error) {
	var e entity.ScheduleAttendee
	if err := r.db.WithContext(ctx).Where("event_id = ? and user_id = ? and deleted_at is null", eventID, userID).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ScheduleAttendeeRepository) Add(ctx context.Context, e *entity.ScheduleAttendee) (string, error) {
	e.AttendeeId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.AttendeeId, nil
}

func (r *ScheduleAttendeeRepository) Edit(ctx context.Context, e *entity.ScheduleAttendee) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.ScheduleAttendee{}).Where("attendee_id = ?", e.AttendeeId).Updates(map[string]any{
		"status":       e.Status,
		"responded_at": e.RespondedAt,
		"updated_at":   e.UpdatedAt,
	}).Error
}

func (r *ScheduleAttendeeRepository) DeleteByEvent(ctx context.Context, eventID string) error {
	return r.db.WithContext(ctx).Model(&entity.ScheduleAttendee{}).
		Where("event_id = ?", eventID).Update("deleted_at", time.Now()).Error
}

func (r *ScheduleAttendeeRepository) DeleteByEventUsers(ctx context.Context, eventID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.ScheduleAttendee{}).
		Where("event_id = ? and user_id in ?", eventID, userIDs).Update("deleted_at", time.Now()).Error
}

// ===================== Exception =====================

type ScheduleExceptionRepository struct{ db *gorm.DB }

func NewScheduleExceptionRepository() *ScheduleExceptionRepository {
	return &ScheduleExceptionRepository{db: db.ScheduleDB()}
}

func (r *ScheduleExceptionRepository) ListByEvent(ctx context.Context, eventID string) (list []*entity.ScheduleException, err error) {
	err = r.db.WithContext(ctx).Where("event_id = ?", eventID).Order("occurrence_date asc").Find(&list).Error
	return
}

func (r *ScheduleExceptionRepository) ListByEvents(ctx context.Context, eventIDs []string) (list []*entity.ScheduleException, err error) {
	if len(eventIDs) == 0 {
		return
	}
	err = r.db.WithContext(ctx).Where("event_id in ?", eventIDs).Order("occurrence_date asc").Find(&list).Error
	return
}

func (r *ScheduleExceptionRepository) GetByEventDate(ctx context.Context, eventID, date string) (*entity.ScheduleException, error) {
	var e entity.ScheduleException
	if err := r.db.WithContext(ctx).Where("event_id = ? and occurrence_date = ?", eventID, date).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ScheduleExceptionRepository) Upsert(ctx context.Context, e *entity.ScheduleException) (string, error) {
	var exist entity.ScheduleException
	err := r.db.WithContext(ctx).Where("event_id = ? and occurrence_date = ?", e.EventId, e.OccurrenceDate).First(&exist).Error
	now := nowPtr()
	if err == nil {
		e.ExceptionId = exist.ExceptionId
		e.UpdatedAt = now
		if err := r.db.WithContext(ctx).Model(&entity.ScheduleException{}).Where("exception_id = ?", exist.ExceptionId).Updates(map[string]any{
			"status":         e.Status,
			"new_start_at":   e.NewStartAt,
			"new_end_at":     e.NewEndAt,
			"new_room_id":    e.NewRoomId,
			"new_online_url": e.NewOnlineUrl,
			"updated_at":     e.UpdatedAt,
		}).Error; err != nil {
			return "", err
		}
		return exist.ExceptionId, nil
	}
	e.ExceptionId = idgen.New()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.ExceptionId, nil
}

func (r *ScheduleExceptionRepository) DeleteByEventDate(ctx context.Context, eventID, date string) error {
	return r.db.WithContext(ctx).Where("event_id = ? and occurrence_date = ?", eventID, date).Delete(&entity.ScheduleException{}).Error
}

// ===================== MeetingRoom =====================

type MeetingRoomRepository struct{ db *gorm.DB }

func NewMeetingRoomRepository() *MeetingRoomRepository {
	return &MeetingRoomRepository{db: db.ScheduleDB()}
}

func (r *MeetingRoomRepository) List(ctx context.Context, keyword string, enabledOnly bool, pageNum, pageSize int) (list []*entity.MeetingRoom, total int64, err error) {
	q := r.db.WithContext(ctx).Model(&entity.MeetingRoom{}).Where("deleted_at is null")
	if keyword != "" {
		q = q.Where("name like ? or location like ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabledOnly {
		q = q.Where("enabled = 1")
	}
	if err = q.Count(&total).Error; err != nil {
		return
	}
	off, lim := paginate(pageNum, pageSize)
	err = q.Order("id asc").Offset(off).Limit(lim).Find(&list).Error
	return
}

func (r *MeetingRoomRepository) ListEnabled(ctx context.Context) (list []*entity.MeetingRoom, err error) {
	err = r.db.WithContext(ctx).Where("deleted_at is null and enabled = 1").Order("id asc").Find(&list).Error
	return
}

func (r *MeetingRoomRepository) Get(ctx context.Context, id string) (*entity.MeetingRoom, error) {
	var e entity.MeetingRoom
	if err := r.db.WithContext(ctx).Where("room_id = ? and deleted_at is null", id).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *MeetingRoomRepository) Add(ctx context.Context, e *entity.MeetingRoom) (string, error) {
	e.RoomId = idgen.New()
	now := nowPtr()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := r.db.WithContext(ctx).Create(e).Error; err != nil {
		return "", err
	}
	return e.RoomId, nil
}

func (r *MeetingRoomRepository) Edit(ctx context.Context, e *entity.MeetingRoom) error {
	e.UpdatedAt = nowPtr()
	return r.db.WithContext(ctx).Model(&entity.MeetingRoom{}).Where("room_id = ?", e.RoomId).Updates(map[string]any{
		"name":      e.Name,
		"location":  e.Location,
		"capacity":  e.Capacity,
		"equipment": e.Equipment,
		"enabled":   e.Enabled,
		"remark":    e.Remark,
		"updated_at": e.UpdatedAt,
	}).Error
}

func (r *MeetingRoomRepository) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&entity.MeetingRoom{}).
		Where("room_id in ?", ids).Update("deleted_at", time.Now()).Error
}
