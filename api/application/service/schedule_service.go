package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// 日程服务（aa_schedule 库）。核心三件事：
//  1. rrule 展开：series 母事件 + 单场例外，按查询窗口现算场次（不物化）；
//  2. 资源冲突：同房间时间区间重叠即冲突，创建/改期/编辑时预检，冲突默认拒绝（可 force）；
//  3. 双通道通知：邀请/变更/取消/响应，经 ChannelNoticeService（nil 安全，未装配=跳过）。
//
// 时间语义：全库本地时刻（types.Time "2006-01-02 15:04:05"）。周期重建按事件 tz（默认 Asia/Shanghai），
// 展开结果仍是该时区的钟表时刻，JSON 输出保持钟表时间不变。

const (
	scheduleDateLayout  = "2006-01-02"
	scheduleTimeLayout  = "2006-01-02 15:04:05"
	scheduleMinuteLayout = "2006-01-02 15:04"

	// 周期冲突检测视野：无限 series 只预检未来 180 天内的场次
	scheduleConflictHorizon = 180 * 24 * time.Hour
	// 展开硬上限（防病态 rrule 死循环）
	expandMaxDays = 20000
	// 单次接口返回场次上限
	maxOccurrences = 1000

	ScheduleExceptionCancel = 0 // 单场取消
	ScheduleExceptionMove   = 1 // 单场改期

	ScheduleAttendeePending = 0
	ScheduleAttendeeAccept  = 1
	ScheduleAttendeeDecline = 2
)

// ===================== RRule =====================

// RRule 周期规则（jsonb 子集，未上完整 RFC 5545）：
// freq: DAILY/WEEKLY/MONTHLY；byday: MO..SU（WEEKLY 用，空=锚点星期）；
// interval: 默认 1；until: YYYY-MM-DD 含当天；count: 总场数含首场（until/count 可并存，先到先停）。
type RRule struct {
	Freq     string   `json:"freq,omitempty"`
	ByDay    []string `json:"byday,omitempty"`
	Interval int      `json:"interval,omitempty"`
	Until    string   `json:"until,omitempty"`
	Count    int      `json:"count,omitempty"`
}

var rruleWeekdays = map[string]time.Weekday{"MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday, "SU": time.Sunday}
var weekdayShort = map[time.Weekday]string{time.Monday: "MO", time.Tuesday: "TU", time.Wednesday: "WE", time.Thursday: "TH", time.Friday: "FR", time.Saturday: "SA", time.Sunday: "SU"}
var weekdayName = map[string]string{"MO": "一", "TU": "二", "WE": "三", "TH": "四", "FR": "五", "SA": "六", "SU": "日"}

// parseRRule 解析事件的 rrule 列；空串=单场。
func parseRRule(s string) (RRule, error) {
	r := RRule{}
	if strings.TrimSpace(s) == "" || s == "{}" {
		return r, nil
	}
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return RRule{}, fmt.Errorf("rrule 格式错误：%w", err)
	}
	return r, nil
}

// validateRRule 校验规则合法性（anchor=首场锚点）。freq 空=单场（非周期），合法放行——
// 这里若按 default 分支拒绝，CreateEvent/UpdateEvent 对一切单场事件直接报错
// （网页 repeat=none 送 rrule=""、MCP repeat 不传同理；hermes 曾被迫 weekly&count=1 绕行）。
func validateRRule(r RRule) error {
	if strings.TrimSpace(r.Freq) == "" {
		return nil
	}
	switch strings.ToUpper(r.Freq) {
	case "DAILY", "MONTHLY":
	case "WEEKLY":
		for _, d := range r.ByDay {
			if _, ok := rruleWeekdays[strings.ToUpper(d)]; !ok {
				return fmt.Errorf("byday 取值：MO/TU/WE/TH/FR/SA/SU")
			}
		}
	default:
		return fmt.Errorf("freq 取值：DAILY/WEEKLY/MONTHLY")
	}
	if r.Interval < 0 {
		return errors.New("interval 须为正整数")
	}
	if r.Until != "" {
		if _, err := time.Parse(scheduleDateLayout, r.Until); err != nil {
			return fmt.Errorf("until 须为 YYYY-MM-DD")
		}
	}
	if r.Count < 0 {
		return errors.New("count 须为正整数")
	}
	return nil
}

func (r RRule) normalized() RRule {
	r.Freq = strings.ToUpper(r.Freq)
	if r.Interval < 1 {
		r.Interval = 1
	}
	for i, d := range r.ByDay {
		r.ByDay[i] = strings.ToUpper(d)
	}
	return r
}

// describeRRule 中文摘要（不含首场具体日期，配合时间区间展示）。
func describeRRule(rule RRule, anchor time.Time) string {
	if rule.Freq == "" {
		return ""
	}
	r := rule.normalized()
	var freq string
	switch r.Freq {
	case "DAILY":
		if r.Interval == 1 {
			freq = "每天"
		} else {
			freq = fmt.Sprintf("每 %d 天", r.Interval)
		}
	case "WEEKLY":
		days := r.ByDay
		if len(days) == 0 {
			days = []string{weekdayShort[anchor.Weekday()]}
		}
		var names []string
		// 按 MO..SU 固定顺序输出
		for _, k := range []string{"MO", "TU", "WE", "TH", "FR", "SA", "SU"} {
			for _, d := range days {
				if strings.ToUpper(d) == k {
					names = append(names, weekdayName[k])
				}
			}
		}
		prefix := "每周"
		if r.Interval > 1 {
			prefix = fmt.Sprintf("每 %d 周", r.Interval)
		}
		freq = prefix + "周" + strings.Join(names, "、")
	case "MONTHLY":
		freq = fmt.Sprintf("每月 %d 日", anchor.Day())
		if r.Interval > 1 {
			freq = fmt.Sprintf("每 %d 月的 %d 日", r.Interval, anchor.Day())
		}
	}
	if r.Count > 0 {
		freq += fmt.Sprintf("，共 %d 场", r.Count)
	} else if r.Until != "" {
		freq += "，直到 " + r.Until
	}
	return freq
}

// ===================== 场次展开 =====================

// Occurrence 展开出的一场（内部结构，视图组装用）。
type Occurrence struct {
	EventId   string
	Title     string
	Start     time.Time
	End       time.Time
	Date      string // 场次标识日（规则展开日 YYYY-MM-DD，非改期后日期）
	RoomId    string
	OnlineUrl string
	Modified  bool // 命中改期例外
}

// expandEvent 在 [from, to) 窗口内展开事件场次，应用例外（取消/改期）。
// 锚点=首场 start/end；周期场次的钟表时刻取锚点时刻，按事件 tz 重建。
func expandEvent(e *entity.ScheduleEvent, rule RRule, exceptions map[string]*entity.ScheduleException, from, to time.Time) []Occurrence {
	if e.StartAt == nil || e.EndAt == nil {
		return nil
	}
	loc := eventLoc(e.Tz)
	anchorStart := time.Time(*e.StartAt).In(loc)
	anchorEnd := time.Time(*e.EndAt).In(loc)
	duration := anchorEnd.Sub(anchorStart)
	if duration <= 0 {
		duration = 30 * time.Minute
	}

	var out []Occurrence
	appendOcc := func(start time.Time, dateKey string) {
		end := start.Add(duration)
		if !start.Before(to) || !end.After(from) {
			return
		}
		o := Occurrence{EventId: e.EventId, Title: e.Title, Start: start, End: end, Date: dateKey, RoomId: e.RoomId, OnlineUrl: e.OnlineUrl}
		if x, ok := exceptions[dateKey]; ok {
			if x.Status == ScheduleExceptionCancel {
				return
			}
			if x.Status == ScheduleExceptionMove && x.NewStartAt != nil && x.NewEndAt != nil {
				o.Start = time.Time(*x.NewStartAt)
				o.End = time.Time(*x.NewEndAt)
				if x.NewRoomId != "" {
					o.RoomId = x.NewRoomId
				}
				if x.NewOnlineUrl != "" {
					o.OnlineUrl = x.NewOnlineUrl
				}
				o.Modified = true
				// 改期后重新判窗口
				if !o.Start.Before(to) || !o.End.After(from) {
					return
				}
			}
		}
		out = append(out, o)
	}

	if e.IsRecurring != 1 || rule.Freq == "" {
		appendOcc(anchorStart, anchorStart.Format(scheduleDateLayout))
		return out
	}

	r := rule.normalized()
	untilBound := time.Time{}
	if r.Until != "" {
		if u, err := time.ParseInLocation(scheduleDateLayout, r.Until, loc); err == nil {
			untilBound = u.AddDate(0, 0, 1) // until 含当天 → 排他上界
		}
	}
	emitted := 0 // 自锚点起总场数（含窗口外，count 语义）
	day := time.Date(anchorStart.Year(), anchorStart.Month(), anchorStart.Day(), 0, 0, 0, 0, loc)
	for i := 0; i < expandMaxDays; i++ {
		if !untilBound.IsZero() && !day.Before(untilBound) {
			break
		}
		if r.Count > 0 && emitted >= r.Count {
			break
		}
		if matchRRuleDay(r, anchorStart, day) {
			appendOcc(time.Date(day.Year(), day.Month(), day.Day(), anchorStart.Hour(), anchorStart.Minute(), anchorStart.Second(), 0, loc), day.Format(scheduleDateLayout))
			emitted++
		}
		day = day.AddDate(0, 0, 1)
	}
	return out
}

// matchRRuleDay 判断 day（锚点同地午夜）是否命中规则。
func matchRRuleDay(r RRule, anchor, day time.Time) bool {
	days := calendarDayDiff(anchor, day)
	if days < 0 {
		return false
	}
	interval := r.Interval
	if interval < 1 {
		interval = 1
	}
	switch r.Freq {
	case "DAILY":
		return days%interval == 0
	case "WEEKLY":
		days_ := r.ByDay
		if len(days_) == 0 {
			days_ = []string{weekdayShort[anchor.Weekday()]}
		}
		match := false
		for _, d := range days_ {
			if rruleWeekdays[d] == day.Weekday() {
				match = true
				break
			}
		}
		if !match {
			return false
		}
		weeks := calendarDayDiff(weekStart(anchor), day) / 7
		return weeks%interval == 0 && weeks >= 0
	case "MONTHLY":
		if day.Day() != anchor.Day() {
			return false
		}
		months := (day.Year()-anchor.Year())*12 + int(day.Month()) - int(anchor.Month())
		return months%interval == 0
	}
	return false
}

// calendarDayDiff 日历日差 b-a（用 UTC 午夜算，规避 DST 影响）。
func calendarDayDiff(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return int(time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC).Sub(time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)).Hours() / 24)
}

// weekStart t 所在周的周一 0 点。
func weekStart(t time.Time) time.Time {
	wd := t.Weekday()
	if wd == time.Sunday {
		wd = 7
	}
	y, m, d := t.Date()
	return time.Date(y, m, d-int(wd)+1, 0, 0, 0, 0, t.Location())
}

func eventLoc(tz string) *time.Location {
	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			return l
		}
	}
	return time.Local
}

// parseScheduleTime 宽松时间解析：YYYY-MM-DD[ HH[:MM[:SS]]] / RFC3339。
func parseScheduleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("时间不能为空")
	}
	for _, layout := range []string{scheduleTimeLayout, scheduleMinuteLayout, scheduleDateLayout, time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("时间格式须为 %s", scheduleTimeLayout)
}

func formatScheduleTime(t time.Time) string { return t.Format(scheduleTimeLayout) }

// ParseScheduleTime 供 controller / MCP 工具层复用的宽松时间解析。
var ParseScheduleTime = parseScheduleTime

// ===================== 服务 =====================

type ScheduleService struct {
	eventRepo     repository.IScheduleEventRepository
	attendeeRepo  repository.IScheduleAttendeeRepository
	exceptionRepo repository.IScheduleExceptionRepository
	roomRepo      repository.IMeetingRoomRepository
	userRepo      repository.IUserRepository
	channelNotice *ChannelNoticeService // 双通道通知；nil=未装配，静默跳过
}

func NewScheduleService(er repository.IScheduleEventRepository, ar repository.IScheduleAttendeeRepository,
	xr repository.IScheduleExceptionRepository, rr repository.IMeetingRoomRepository, ur repository.IUserRepository) *ScheduleService {
	return &ScheduleService{eventRepo: er, attendeeRepo: ar, exceptionRepo: xr, roomRepo: rr, userRepo: ur}
}

// SetChannelNotice 接入双通道通知（装配期补注，防构造循环；nil 安全）
func (s *ScheduleService) SetChannelNotice(c *ChannelNoticeService) { s.channelNotice = c }

// Bootstrap api 启动对账：会议室全空时灌 3 间默认房（幂等 best-effort，只 api 主进程跑）。
func (s *ScheduleService) Bootstrap(ctx context.Context) {
	_, total, err := s.roomRepo.List(ctx, "", false, 1, 1)
	if err != nil || total > 0 {
		return
	}
	defaults := []entity.MeetingRoom{
		{Name: "小会议室", Location: "默认楼层", Capacity: 6, Equipment: `["电视","白板"]`, Enabled: 1, Remark: "种子预置，可在会议室管理中修改"},
		{Name: "中会议室", Location: "默认楼层", Capacity: 12, Equipment: `["电视","投影","视频会议"]`, Enabled: 1, Remark: "种子预置，可在会议室管理中修改"},
		{Name: "大会议室", Location: "默认楼层", Capacity: 30, Equipment: `["投影","视频会议","音响"]`, Enabled: 1, Remark: "种子预置，可在会议室管理中修改"},
	}
	for i := range defaults {
		if _, err := s.roomRepo.Add(ctx, &defaults[i]); err != nil {
			return
		}
	}
}

// ===================== 输入/视图结构 =====================

// ScheduleEventInput 创建/编辑事件的统一入参（REST 与 MCP 共用）。
type ScheduleEventInput struct {
	Title           string
	EventType       int
	StartAt         time.Time
	EndAt           time.Time
	RRule           string // 空=单场
	RoomId          string
	OnlineProvider  string
	OnlineUrl       string
	Location        string
	Attendees       []string // user_id 列表（组织者自动剔除）
	Remark          string
	IgnoreConflicts bool
	DryRun          bool // true=只预检（摘要+冲突），不落库不发通知
}

type AttendeeView struct {
	UserId   string `json:"userId"`
	UserName string `json:"userName"`
	Status   int    `json:"status"`
}

type ConflictView struct {
	EventId  string `json:"eventId"`
	Title    string `json:"title"`
	RoomId   string `json:"roomId"`
	RoomName string `json:"roomName"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type ScheduleEventPreview struct {
	EventId    string         `json:"eventId"`
	Title      string         `json:"title"`
	EventType  int            `json:"eventType"`
	StartAt    string         `json:"startAt"`
	EndAt      string         `json:"endAt"`
	RepeatDesc string         `json:"repeatDesc"`
	RoomId     string         `json:"roomId"`
	RoomName   string         `json:"roomName"`
	OnlineUrl  string         `json:"onlineUrl"`
	Location   string         `json:"location"`
	Attendees  []AttendeeView `json:"attendees"`
	Conflicts  []ConflictView `json:"conflicts"`
	Created    bool           `json:"created"`
}

type ScheduleOccurrenceView struct {
	EventId        string         `json:"eventId"`
	Title          string         `json:"title"`
	EventType      int            `json:"eventType"`
	Start          string         `json:"start"`
	End            string         `json:"end"`
	Date           string         `json:"date"`
	RoomId         string         `json:"roomId"`
	RoomName       string         `json:"roomName"`
	Location       string         `json:"location"`
	OnlineProvider string         `json:"onlineProvider"`
	OnlineUrl      string         `json:"onlineUrl"`
	OrganizerId    string         `json:"organizerId"`
	OrganizerName  string         `json:"organizerName"`
	Attendees      []AttendeeView `json:"attendees"`
	IsRecurring    int            `json:"isRecurring"`
	RepeatDesc     string         `json:"repeatDesc"`
	RRule          string         `json:"rrule,omitempty"` // 周期规则原文（编辑回填用）
	Modified       bool           `json:"modified"`
	MyStatus       int            `json:"myStatus"` // 我作为参会人的响应状态；非参会人=-1
	IsOrganizer    bool           `json:"isOrganizer"`
	Remark         string         `json:"remark"`
}

type ScheduleEventDetailView struct {
	ScheduleOccurrenceView
	Exceptions []ExceptionView `json:"exceptions"`
}

type ExceptionView struct {
	Date        string `json:"date"`
	Status      int    `json:"status"`
	NewStartAt  string `json:"newStartAt"`
	NewEndAt    string `json:"newEndAt"`
	NewRoomId   string `json:"newRoomId"`
	NewRoomName string `json:"newRoomName"`
}

type RoomDayOccView struct {
	EventId       string `json:"eventId"`
	Title         string `json:"title"`
	Start         string `json:"start"`
	End           string `json:"end"`
	OrganizerName string `json:"organizerName"`
}

type RoomDayView struct {
	RoomId    string           `json:"roomId"`
	Name      string           `json:"name"`
	Location  string           `json:"location"`
	Capacity  int              `json:"capacity"`
	Equipment string           `json:"equipment"`
	Enabled   int              `json:"enabled"`
	Remark    string           `json:"remark"`
	Busy      []RoomDayOccView `json:"busy"`
}

type BusyInterval struct {
	UserId   string `json:"userId"`
	UserName string `json:"userName"`
	Title    string `json:"title"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type FreeSlot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// ===================== 查询 =====================

// ListMyOccurrences 我的日程：我是组织者或参会人的事件在窗口内展开的场次。
func (s *ScheduleService) ListMyOccurrences(ctx context.Context, userID string, from, to time.Time) ([]ScheduleOccurrenceView, error) {
	events, err := s.eventRepo.ListByUser(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	return s.buildOccurrenceViews(ctx, events, from, to, userID)
}

func (s *ScheduleService) buildOccurrenceViews(ctx context.Context, events []*entity.ScheduleEvent, from, to time.Time, viewerID string) ([]ScheduleOccurrenceView, error) {
	if len(events) == 0 {
		return []ScheduleOccurrenceView{}, nil
	}
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.EventId)
	}
	exceptions, err := s.exceptionRepo.ListByEvents(ctx, ids)
	if err != nil {
		return nil, err
	}
	exMap := map[string]map[string]*entity.ScheduleException{}
	for _, x := range exceptions {
		if exMap[x.EventId] == nil {
			exMap[x.EventId] = map[string]*entity.ScheduleException{}
		}
		exMap[x.EventId][x.OccurrenceDate] = x
	}
	attendees, err := s.attendeeRepo.ListByEvents(ctx, ids)
	if err != nil {
		return nil, err
	}
	attMap := map[string][]*entity.ScheduleAttendee{}
	userIds := make([]string, 0, len(attendees)+len(events))
	for _, a := range attendees {
		attMap[a.EventId] = append(attMap[a.EventId], a)
		userIds = append(userIds, a.UserId)
	}
	for _, e := range events {
		userIds = append(userIds, e.OrganizerId)
	}
	rooms, _, err := s.roomRepo.List(ctx, "", false, 1, 1000)
	if err != nil {
		return nil, err
	}
	roomName := map[string]string{}
	for _, r := range rooms {
		roomName[r.RoomId] = r.Name
	}
	names := userNameMap(ctx, s.userRepo, dedupeStrings(userIds))

	var out []ScheduleOccurrenceView
	for _, e := range events {
		rule, _ := parseRRule(e.RRule)
		anchor := time.Time{}
		if e.StartAt != nil {
			anchor = time.Time(*e.StartAt)
		}
		repeatDesc := describeRRule(rule, anchor)
		atts := make([]AttendeeView, 0, len(attMap[e.EventId]))
		myStatus := -1
		for _, a := range attMap[e.EventId] {
			atts = append(atts, AttendeeView{UserId: a.UserId, UserName: names[a.UserId], Status: a.Status})
			if viewerID != "" && a.UserId == viewerID {
				myStatus = a.Status
			}
		}
		occ := expandEvent(e, rule, exMap[e.EventId], from, to)
		for _, o := range occ {
			out = append(out, ScheduleOccurrenceView{
				EventId: e.EventId, Title: e.Title, EventType: e.EventType,
				Start: formatScheduleTime(o.Start), End: formatScheduleTime(o.End), Date: o.Date,
				RoomId: o.RoomId, RoomName: roomName[o.RoomId], Location: e.Location,
				OnlineProvider: e.OnlineProvider, OnlineUrl: o.OnlineUrl,
				OrganizerId: e.OrganizerId, OrganizerName: names[e.OrganizerId], Attendees: atts,
				IsRecurring: e.IsRecurring, RepeatDesc: repeatDesc, RRule: e.RRule, Modified: o.Modified,
				MyStatus: myStatus, IsOrganizer: viewerID != "" && e.OrganizerId == viewerID,
				Remark: e.Remark,
			})
		}
		if len(out) > maxOccurrences {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	if len(out) > maxOccurrences {
		out = out[:maxOccurrences]
	}
	return out, nil
}

// GetDetail 事件详情（编辑弹窗/详情抽屉用）。
func (s *ScheduleService) GetDetail(ctx context.Context, eventID, viewerID string) (*ScheduleEventDetailView, error) {
	e, err := s.eventRepo.Get(ctx, eventID)
	if err != nil {
		return nil, errors.New("事件不存在")
	}
	views, err := s.buildOccurrenceViews(ctx, []*entity.ScheduleEvent{e}, time.Time{}, time.Now().AddDate(100, 0, 0), viewerID)
	if err != nil {
		return nil, err
	}
	detail := &ScheduleEventDetailView{}
	if len(views) > 0 {
		// 详情取首场（buildOccurrenceViews 已按开始时间排序）
		detail.ScheduleOccurrenceView = views[0]
	} else {
		// 窗口内无场次（如全部取消）时仍返回基本信息
		rule, _ := parseRRule(e.RRule)
		anchor := time.Time{}
		if e.StartAt != nil {
			anchor = time.Time(*e.StartAt)
		}
		detail.ScheduleOccurrenceView = ScheduleOccurrenceView{
			EventId: e.EventId, Title: e.Title, EventType: e.EventType,
			RepeatDesc: describeRRule(rule, anchor), IsRecurring: e.IsRecurring, RRule: e.RRule,
			OrganizerId: e.OrganizerId, Location: e.Location, OnlineProvider: e.OnlineProvider,
			OnlineUrl: e.OnlineUrl, RoomId: e.RoomId, Remark: e.Remark, MyStatus: -1,
		}
	}
	atts, _ := s.attendeeRepo.ListByEvent(ctx, eventID)
	names := userNameMap(ctx, s.userRepo, func() []string {
		ids := []string{e.OrganizerId}
		for _, a := range atts {
			ids = append(ids, a.UserId)
		}
		return ids
	}())
	avs := make([]AttendeeView, 0, len(atts))
	for _, a := range atts {
		avs = append(avs, AttendeeView{UserId: a.UserId, UserName: names[a.UserId], Status: a.Status})
		if viewerID != "" && a.UserId == viewerID {
			detail.MyStatus = a.Status
		}
	}
	detail.Attendees = avs
	detail.OrganizerName = names[e.OrganizerId]
	exs, _ := s.exceptionRepo.ListByEvent(ctx, eventID)
	rooms, _, _ := s.roomRepo.List(ctx, "", false, 1, 1000)
	roomName := map[string]string{}
	for _, r := range rooms {
		roomName[r.RoomId] = r.Name
	}
	evs := make([]ExceptionView, 0, len(exs))
	for _, x := range exs {
		ev := ExceptionView{Date: x.OccurrenceDate, Status: x.Status, NewRoomId: x.NewRoomId, NewRoomName: roomName[x.NewRoomId]}
		if x.NewStartAt != nil {
			ev.NewStartAt = formatScheduleTime(time.Time(*x.NewStartAt))
		}
		if x.NewEndAt != nil {
			ev.NewEndAt = formatScheduleTime(time.Time(*x.NewEndAt))
		}
		evs = append(evs, ev)
	}
	detail.Exceptions = evs
	return detail, nil
}

// RoomDayOverview 会议室当日时间轴：启用房间 + 每房的当日场次。
func (s *ScheduleService) RoomDayOverview(ctx context.Context, date string) ([]RoomDayView, error) {
	day, err := time.ParseInLocation(scheduleDateLayout, date, time.Local)
	if err != nil {
		return nil, fmt.Errorf("date 须为 YYYY-MM-DD")
	}
	from := day
	to := day.AddDate(0, 0, 1)
	rooms, err := s.roomRepo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	events, err := s.eventRepo.ListRoomEvents(ctx, "", from, to, true)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(events))
	orgIds := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.EventId)
		orgIds = append(orgIds, e.OrganizerId)
	}
	exList, _ := s.exceptionRepo.ListByEvents(ctx, ids)
	exMap := map[string]map[string]*entity.ScheduleException{}
	for _, x := range exList {
		if exMap[x.EventId] == nil {
			exMap[x.EventId] = map[string]*entity.ScheduleException{}
		}
		exMap[x.EventId][x.OccurrenceDate] = x
	}
	names := userNameMap(ctx, s.userRepo, dedupeStrings(orgIds))
	busyByRoom := map[string][]RoomDayOccView{}
	for _, e := range events {
		rule, _ := parseRRule(e.RRule)
		for _, o := range expandEvent(e, rule, exMap[e.EventId], from, to) {
			busyByRoom[o.RoomId] = append(busyByRoom[o.RoomId], RoomDayOccView{
				EventId: o.EventId, Title: o.Title,
				Start: formatScheduleTime(o.Start), End: formatScheduleTime(o.End),
				OrganizerName: names[e.OrganizerId],
			})
		}
	}
	out := make([]RoomDayView, 0, len(rooms))
	for _, r := range rooms {
		busy := busyByRoom[r.RoomId]
		if busy == nil {
			busy = []RoomDayOccView{}
		}
		sort.SliceStable(busy, func(i, j int) bool { return busy[i].Start < busy[j].Start })
		out = append(out, RoomDayView{
			RoomId: r.RoomId, Name: r.Name, Location: r.Location, Capacity: r.Capacity,
			Equipment: r.Equipment, Enabled: r.Enabled, Remark: r.Remark, Busy: busy,
		})
	}
	return out, nil
}

// FreeBusy 用户忙闲：窗口内每人的占用区间（含标题，供"他 3 点有会"解释）。
func (s *ScheduleService) FreeBusy(ctx context.Context, userIDs []string, from, to time.Time) ([]BusyInterval, error) {
	if len(userIDs) == 0 {
		return []BusyInterval{}, nil
	}
	userIDs = dedupeStrings(userIDs)
	events, err := s.eventRepo.ListByUsers(ctx, userIDs, from, to)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.EventId)
	}
	atts, _ := s.attendeeRepo.ListByEvents(ctx, ids)
	userSet := map[string]bool{}
	for _, u := range userIDs {
		userSet[u] = true
	}
	eventUsers := map[string][]string{} // eventId → involved user_ids
	for _, a := range atts {
		if userSet[a.UserId] {
			eventUsers[a.EventId] = append(eventUsers[a.EventId], a.UserId)
		}
	}
	exList, _ := s.exceptionRepo.ListByEvents(ctx, ids)
	exMap := map[string]map[string]*entity.ScheduleException{}
	for _, x := range exList {
		if exMap[x.EventId] == nil {
			exMap[x.EventId] = map[string]*entity.ScheduleException{}
		}
		exMap[x.EventId][x.OccurrenceDate] = x
	}
	names := userNameMap(ctx, s.userRepo, userIDs)
	var out []BusyInterval
	for _, e := range events {
		involved := eventUsers[e.EventId]
		if userSet[e.OrganizerId] {
			involved = append(involved, e.OrganizerId)
		}
		if len(involved) == 0 {
			continue
		}
		rule, _ := parseRRule(e.RRule)
		for _, o := range expandEvent(e, rule, exMap[e.EventId], from, to) {
			for _, uid := range dedupeStrings(involved) {
				out = append(out, BusyInterval{
					UserId: uid, UserName: names[uid], Title: e.Title,
					Start: formatScheduleTime(o.Start), End: formatScheduleTime(o.End),
				})
			}
		}
	}
	return out, nil
}

// FindFreeSlots 共同空闲：[from,to] 内所有人合忙的补集，留 ≥durationMin 的空档。
func (s *ScheduleService) FindFreeSlots(ctx context.Context, userIDs []string, from, to time.Time, durationMin int) ([]FreeSlot, error) {
	if durationMin <= 0 {
		durationMin = 30
	}
	busy, err := s.FreeBusy(ctx, userIDs, from, to)
	if err != nil {
		return nil, err
	}
	type iv struct{ s, e time.Time }
	ivs := make([]iv, 0, len(busy))
	for _, b := range busy {
		st, err1 := parseScheduleTime(b.Start)
		et, err2 := parseScheduleTime(b.End)
		if err1 != nil || err2 != nil {
			continue
		}
		ivs = append(ivs, iv{st, et})
	}
	sort.SliceStable(ivs, func(i, j int) bool { return ivs[i].s.Before(ivs[j].s) })
	// 合并重叠
	merged := make([]iv, 0, len(ivs))
	for _, v := range ivs {
		if n := len(merged); n > 0 && !v.s.After(merged[n-1].e) {
			if v.e.After(merged[n-1].e) {
				merged[n-1].e = v.e
			}
			continue
		}
		merged = append(merged, v)
	}
	// 走空档
	var out []FreeSlot
	need := time.Duration(durationMin) * time.Minute
	cur := from
	for _, m := range merged {
		if m.s.Sub(cur) >= need {
			out = append(out, FreeSlot{Start: formatScheduleTime(cur), End: formatScheduleTime(m.s)})
		}
		if m.e.After(cur) {
			cur = m.e
		}
	}
	if to.Sub(cur) >= need {
		out = append(out, FreeSlot{Start: formatScheduleTime(cur), End: formatScheduleTime(to)})
	}
	return out, nil
}

// ===================== 写入（创建/编辑/取消/例外/响应）=====================

// CreateEvent 创建日程/会议。冲突且未 IgnoreConflicts → 不落库，返回预览（Created=false + Conflicts）。
func (s *ScheduleService) CreateEvent(ctx context.Context, organizer string, in ScheduleEventInput) (*ScheduleEventPreview, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, errors.New("title 不能为空")
	}
	if !in.StartAt.Before(in.EndAt) {
		return nil, errors.New("开始时间须早于结束时间")
	}
	rule, err := parseRRule(in.RRule)
	if err != nil {
		return nil, err
	}
	if err := validateRRule(rule); err != nil {
		return nil, err
	}
	if in.RoomId != "" {
		room, err := s.roomRepo.Get(ctx, in.RoomId)
		if err != nil {
			return nil, errors.New("会议室不存在")
		}
		if room.Enabled != 1 {
			return nil, errors.New("会议室已停用")
		}
	}
	attendees := dedupeStrings(in.Attendees)
	filtered := attendees[:0:0]
	for _, uid := range attendees {
		if uid != organizer && uid != "" {
			filtered = append(filtered, uid)
		}
	}

	tentative := &entity.ScheduleEvent{
		Title: in.Title, EventType: in.EventType,
		StartAt: tp(in.StartAt), EndAt: tp(in.EndAt),
		IsRecurring: boolInt(rule.Freq != ""), RRule: canonicalRRule(rule),
		Tz: "Asia/Shanghai", RoomId: in.RoomId,
		OnlineProvider: in.OnlineProvider, OnlineUrl: in.OnlineUrl, Location: in.Location,
		OrganizerId: organizer, Remark: in.Remark, CreatedBy: organizer,
	}

	conflicts, err := s.checkRoomConflicts(ctx, tentative, rule, "")
	if err != nil {
		return nil, err
	}
	preview, err := s.buildPreview(ctx, tentative, rule, filtered)
	if err != nil {
		return nil, err
	}
	preview.Conflicts = conflicts
	if len(conflicts) > 0 && !in.IgnoreConflicts {
		preview.Created = false
		return preview, nil
	}
	if in.DryRun {
		preview.Created = false
		return preview, nil
	}
	eventID, err := s.eventRepo.Add(ctx, tentative)
	if err != nil {
		return nil, err
	}
	for _, uid := range filtered {
		_, _ = s.attendeeRepo.Add(ctx, &entity.ScheduleAttendee{EventId: eventID, UserId: uid, Status: ScheduleAttendeePending})
	}
	preview.EventId = eventID
	preview.Created = true
	// 邀请通知（双通道，fail-open）
	if content, ok := s.inviteContent(ctx, organizer, preview, filtered); ok {
		s.notifyUsers(ctx, filtered, "info", "📅 会议邀请："+in.Title, content, "/schedule/my")
	}
	return preview, nil
}

// GetEventInput 读取事件当前编辑入参（MCP schedule_update 局部修改：旧值打底、调用方覆盖）。
func (s *ScheduleService) GetEventInput(ctx context.Context, eventID string) (ScheduleEventInput, error) {
	e, err := s.eventRepo.Get(ctx, eventID)
	if err != nil {
		return ScheduleEventInput{}, errors.New("事件不存在")
	}
	if e.StartAt == nil || e.EndAt == nil {
		return ScheduleEventInput{}, errors.New("事件时间缺失")
	}
	atts, _ := s.attendeeRepo.ListByEvent(ctx, eventID)
	ids := make([]string, 0, len(atts))
	for _, a := range atts {
		ids = append(ids, a.UserId)
	}
	return ScheduleEventInput{
		Title: e.Title, EventType: e.EventType,
		StartAt: time.Time(*e.StartAt), EndAt: time.Time(*e.EndAt),
		RRule: e.RRule, RoomId: e.RoomId, OnlineProvider: e.OnlineProvider, OnlineUrl: e.OnlineUrl,
		Location: e.Location, Attendees: ids, Remark: e.Remark,
	}, nil
}

// UpdateEvent 整个 series 编辑（例外保留；改锚点星期后旧例外若不再命中自然失效）。
func (s *ScheduleService) UpdateEvent(ctx context.Context, operator, eventID string, in ScheduleEventInput) (*ScheduleEventPreview, error) {
	old, err := s.eventRepo.Get(ctx, eventID)
	if err != nil {
		return nil, errors.New("事件不存在")
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, errors.New("title 不能为空")
	}
	if !in.StartAt.Before(in.EndAt) {
		return nil, errors.New("开始时间须早于结束时间")
	}
	rule, err := parseRRule(in.RRule)
	if err != nil {
		return nil, err
	}
	if err := validateRRule(rule); err != nil {
		return nil, err
	}
	if in.RoomId != "" {
		room, err := s.roomRepo.Get(ctx, in.RoomId)
		if err != nil {
			return nil, errors.New("会议室不存在")
		}
		if room.Enabled != 1 {
			return nil, errors.New("会议室已停用")
		}
	}
	newAttendees := dedupeStrings(in.Attendees)
	keep := newAttendees[:0:0]
	for _, uid := range newAttendees {
		if uid != old.OrganizerId && uid != "" {
			keep = append(keep, uid)
		}
	}
	next := *old
	next.Title = in.Title
	next.EventType = in.EventType
	next.StartAt = tp(in.StartAt)
	next.EndAt = tp(in.EndAt)
	next.IsRecurring = boolInt(rule.Freq != "")
	next.RRule = canonicalRRule(rule)
	next.RoomId = in.RoomId
	next.OnlineProvider = in.OnlineProvider
	next.OnlineUrl = in.OnlineUrl
	next.Location = in.Location
	next.Remark = in.Remark

	conflicts, err := s.checkRoomConflicts(ctx, &next, rule, old.EventId)
	if err != nil {
		return nil, err
	}
	preview, err := s.buildPreview(ctx, &next, rule, keep)
	if err != nil {
		return nil, err
	}
	preview.EventId = old.EventId
	preview.Conflicts = conflicts
	if len(conflicts) > 0 && !in.IgnoreConflicts {
		preview.Created = false
		return preview, nil
	}
	if in.DryRun {
		preview.Created = false
		return preview, nil
	}
	if err := s.eventRepo.Edit(ctx, &next); err != nil {
		return nil, err
	}
	// 参会人差集：新加=邀请、移除=取消通知、保留=变更通知（响应状态保留）
	oldAtts, _ := s.attendeeRepo.ListByEvent(ctx, eventID)
	oldSet := map[string]bool{}
	for _, a := range oldAtts {
		oldSet[a.UserId] = true
	}
	newSet := map[string]bool{}
	for _, uid := range keep {
		newSet[uid] = true
	}
	var added, removed, kept []string
	for _, uid := range keep {
		if oldSet[uid] {
			kept = append(kept, uid)
		} else {
			added = append(added, uid)
		}
	}
	for _, a := range oldAtts {
		if !newSet[a.UserId] {
			removed = append(removed, a.UserId)
		}
	}
	_ = s.attendeeRepo.DeleteByEventUsers(ctx, eventID, removed)
	for _, uid := range added {
		_, _ = s.attendeeRepo.Add(ctx, &entity.ScheduleAttendee{EventId: eventID, UserId: uid, Status: ScheduleAttendeePending})
	}
	if content, ok := s.inviteContent(ctx, old.OrganizerId, preview, keep); ok {
		s.notifyUsers(ctx, added, "info", "📅 会议邀请："+next.Title, content, "/schedule/my")
		s.notifyUsers(ctx, kept, "warning", "📅 会议变更："+next.Title, content, "/schedule/my")
		s.notifyUsers(ctx, removed, "warning", "📅 已将你移出会议："+next.Title, "你已不是「"+next.Title+"」的参会人。", "/schedule/my")
	}
	preview.Created = true
	return preview, nil
}

// CancelEvent 取消整个 series（软删 + 通知参会人）。
func (s *ScheduleService) CancelEvent(ctx context.Context, operator, eventID string) error {
	e, err := s.eventRepo.Get(ctx, eventID)
	if err != nil {
		return errors.New("事件不存在")
	}
	if err := s.eventRepo.Delete(ctx, []string{eventID}); err != nil {
		return err
	}
	atts, _ := s.attendeeRepo.ListByEvent(ctx, eventID)
	var uids []string
	for _, a := range atts {
		uids = append(uids, a.UserId)
	}
	rule, _ := parseRRule(e.RRule)
	desc := describeRRule(rule, time.Time(*e.StartAt))
	if desc != "" {
		desc = "（" + desc + "）"
	}
	s.notifyUsers(ctx, uids, "warning", "📅 会议取消："+e.Title,
		fmt.Sprintf("「%s」%s 已取消。\n原时间：%s ~ %s", e.Title, desc,
			formatScheduleTime(time.Time(*e.StartAt)), formatScheduleTime(time.Time(*e.EndAt))), "/schedule/my")
	return nil
}

// ExceptionInput 单场例外入参。
type ExceptionInput struct {
	EventId   string
	Date      string // YYYY-MM-DD 该场次日期
	Status    int    // 0取消该场 1该场改期
	NewStartAt, NewEndAt time.Time
	NewRoomId string
}

// AddException 单场取消/改期（周期事件专用）。
func (s *ScheduleService) AddException(ctx context.Context, operator string, in ExceptionInput) error {
	e, err := s.eventRepo.Get(ctx, in.EventId)
	if err != nil {
		return errors.New("事件不存在")
	}
	if e.IsRecurring != 1 {
		return errors.New("非周期事件，请直接编辑或取消")
	}
	if _, err := time.Parse(scheduleDateLayout, in.Date); err != nil {
		return errors.New("date 须为 YYYY-MM-DD")
	}
	if in.Status == ScheduleExceptionMove {
		if !in.NewStartAt.Before(in.NewEndAt) {
			return errors.New("改期时间：开始须早于结束")
		}
		roomID := in.NewRoomId
		if roomID == "" {
			roomID = e.RoomId
		}
		if roomID != "" {
			if err := s.checkOccurrenceConflict(ctx, roomID, in.EventId, in.Date, in.NewStartAt, in.NewEndAt); err != nil {
				return err
			}
		}
	}
	ex := &entity.ScheduleException{
		EventId: in.EventId, OccurrenceDate: in.Date, Status: in.Status,
		NewRoomId: in.NewRoomId,
	}
	if in.Status == ScheduleExceptionMove {
		ex.NewStartAt = tp(in.NewStartAt)
		ex.NewEndAt = tp(in.NewEndAt)
	}
	if _, err := s.exceptionRepo.Upsert(ctx, ex); err != nil {
		return err
	}
	// 通知参会人
	atts, _ := s.attendeeRepo.ListByEvent(ctx, in.EventId)
	var uids []string
	for _, a := range atts {
		uids = append(uids, a.UserId)
	}
	if in.Status == ScheduleExceptionCancel {
		s.notifyUsers(ctx, uids, "warning", "📅 单场取消："+e.Title,
			fmt.Sprintf("「%s」%s 这一場已取消，其余场次不变。", e.Title, in.Date), "/schedule/my")
	} else {
		s.notifyUsers(ctx, uids, "warning", "📅 单场改期："+e.Title,
			fmt.Sprintf("「%s」%s 一场改至：%s ~ %s，其余场次不变。", e.Title, in.Date,
				formatScheduleTime(in.NewStartAt), formatScheduleTime(in.NewEndAt)), "/schedule/my")
	}
	return nil
}

// Respond 参会人接受/拒绝邀请（通知组织者）。
func (s *ScheduleService) Respond(ctx context.Context, eventID, userID string, status int) error {
	if status != ScheduleAttendeeAccept && status != ScheduleAttendeeDecline {
		return errors.New("status 取值：1=接受 2=拒绝")
	}
	a, err := s.attendeeRepo.GetByEventUser(ctx, eventID, userID)
	if err != nil {
		return errors.New("你不是该事件的参会人")
	}
	a.Status = status
	t := time.Now()
	a.RespondedAt = &types.Time{}
	*a.RespondedAt = types.Time(t)
	if err := s.attendeeRepo.Edit(ctx, a); err != nil {
		return err
	}
	e, err := s.eventRepo.Get(ctx, eventID)
	if err == nil {
		verdict := "已接受"
		if status == ScheduleAttendeeDecline {
			verdict = "已拒绝"
		}
		name := userID
		if m := userNameMap(ctx, s.userRepo, []string{userID}); m[userID] != "" {
			name = m[userID]
		}
		s.notifyUsers(ctx, []string{e.OrganizerId}, "info", "📅 参会回执："+e.Title,
			fmt.Sprintf("%s %s「%s」的邀请。", name, verdict, e.Title), "/schedule/my")
	}
	return nil
}

// ===================== 会议室 CRUD（薄包装）=====================

func (s *ScheduleService) RoomList(ctx context.Context, keyword string, enabledOnly bool, pn, ps int) ([]*entity.MeetingRoom, int64, error) {
	return s.roomRepo.List(ctx, keyword, enabledOnly, pn, ps)
}

func (s *ScheduleService) RoomAdd(ctx context.Context, operator, name, location string, capacity int, equipment string, enabled int, remark string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name 不能为空")
	}
	if equipment == "" {
		equipment = "[]"
	}
	_, err := s.roomRepo.Add(ctx, &entity.MeetingRoom{
		Name: name, Location: location, Capacity: capacity, Equipment: equipment,
		Enabled: boolInt(enabled != 0), Remark: remark, CreatedBy: operator,
	})
	return err
}

func (s *ScheduleService) RoomEdit(ctx context.Context, roomID, name, location string, capacity int, equipment string, enabled int, remark string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name 不能为空")
	}
	if equipment == "" {
		equipment = "[]"
	}
	return s.roomRepo.Edit(ctx, &entity.MeetingRoom{
		RoomId: roomID, Name: name, Location: location, Capacity: capacity,
		Equipment: equipment, Enabled: boolInt(enabled != 0), Remark: remark,
	})
}

func (s *ScheduleService) RoomDelete(ctx context.Context, ids []string) error {
	// 房间删除后，既有事件的 room_id 变悬空引用（展开仍占时段但不显示房名）；
	// 展示层 roomName 取不到时回退 room_id，行为可接受。
	return s.roomRepo.Delete(ctx, ids)
}

// ===================== 房间冲突 =====================

// checkRoomConflicts 校验 tentative（含周期展开）在冲突视野内与既有占用的重叠。
// excludeEventID：编辑场景排除自身 series。
func (s *ScheduleService) checkRoomConflicts(ctx context.Context, tentative *entity.ScheduleEvent, rule RRule, excludeEventID string) ([]ConflictView, error) {
	if tentative.RoomId == "" {
		return nil, nil
	}
	from := time.Time(*tentative.StartAt)
	to := from.Add(scheduleConflictHorizon)
	if r := rule.normalized(); r.Until != "" {
		if u, err := time.ParseInLocation(scheduleDateLayout, r.Until, from.Location()); err == nil && u.Before(to) {
			to = u.AddDate(0, 0, 1)
		}
	}
	occ := expandEvent(tentative, rule, nil, from, to)
	if len(occ) == 0 {
		return nil, nil
	}
	return s.roomConflictsBetween(ctx, tentative.RoomId, excludeEventID, occ)
}

// roomConflictsBetween 给定请求占用的场次集，找同房间的既有冲突。
func (s *ScheduleService) roomConflictsBetween(ctx context.Context, roomID, excludeEventID string, reqOcc []Occurrence) ([]ConflictView, error) {
	candidates, err := s.eventRepo.ListRoomEvents(ctx, roomID, reqOcc[0].Start, reqOcc[len(reqOcc)-1].End, true)
	if err != nil {
		return nil, err
	}
	filtered := candidates[:0:0]
	for _, e := range candidates {
		if e.EventId != excludeEventID {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(filtered))
	for _, e := range filtered {
		ids = append(ids, e.EventId)
	}
	exList, _ := s.exceptionRepo.ListByEvents(ctx, ids)
	exMap := map[string]map[string]*entity.ScheduleException{}
	for _, x := range exList {
		if exMap[x.EventId] == nil {
			exMap[x.EventId] = map[string]*entity.ScheduleException{}
		}
		exMap[x.EventId][x.OccurrenceDate] = x
	}
	rooms, _, _ := s.roomRepo.List(ctx, "", false, 1, 1000)
	roomName := map[string]string{}
	for _, r := range rooms {
		roomName[r.RoomId] = r.Name
	}
	var conflicts []ConflictView
	seen := map[string]bool{}
	for _, e := range filtered {
		rule, _ := parseRRule(e.RRule)
		for _, o := range expandEvent(e, rule, exMap[e.EventId], reqOcc[0].Start, reqOcc[len(reqOcc)-1].End) {
			for _, q := range reqOcc {
				if q.Start.Before(o.End) && q.End.After(o.Start) {
					key := o.EventId + o.Date + formatScheduleTime(o.Start)
					if !seen[key] {
						seen[key] = true
						conflicts = append(conflicts, ConflictView{
							EventId: o.EventId, Title: o.Title, RoomId: o.RoomId, RoomName: roomName[o.RoomId],
							Start: formatScheduleTime(o.Start), End: formatScheduleTime(o.End),
						})
					}
					break
				}
			}
		}
	}
	sort.SliceStable(conflicts, func(i, j int) bool { return conflicts[i].Start < conflicts[j].Start })
	if conflicts == nil {
		conflicts = []ConflictView{}
	}
	return conflicts, nil
}

// checkOccurrenceConflict 单场改期冲突：新时段 vs 同房既有占用（含自己 series 的其他场次）。
func (s *ScheduleService) checkOccurrenceConflict(ctx context.Context, roomID, ownEventID, replaceDate string, newStart, newEnd time.Time) error {
	pad := 24 * time.Hour
	candidates, err := s.eventRepo.ListRoomEvents(ctx, roomID, newStart.Add(-pad), newEnd.Add(pad), true)
	if err != nil {
		return err
	}
	rooms, _, _ := s.roomRepo.List(ctx, "", false, 1, 1000)
	roomName := map[string]string{}
	for _, r := range rooms {
		roomName[r.RoomId] = r.Name
	}
	var hit string
	for _, e := range candidates {
		rule, _ := parseRRule(e.RRule)
		exs, _ := s.exceptionRepo.ListByEvent(ctx, e.EventId)
		exMap := map[string]*entity.ScheduleException{}
		for _, x := range exs {
			exMap[x.OccurrenceDate] = x
		}
		for _, o := range expandEvent(e, rule, exMap, newStart.Add(-pad), newEnd.Add(pad)) {
			if e.EventId == ownEventID && o.Date == replaceDate {
				continue // 被替换的那场
			}
			if newStart.Before(o.End) && newEnd.After(o.Start) {
				hit = fmt.Sprintf("「%s」%s ~ %s 已占用%s", o.Title,
					formatScheduleTime(o.Start), formatScheduleTime(o.End), roomLabel(roomName[o.RoomId], o.RoomId))
				break
			}
		}
		if hit != "" {
			break
		}
	}
	if hit != "" {
		return fmt.Errorf("会议室冲突：%s", hit)
	}
	return nil
}

// ===================== 内部工具 =====================

// buildPreview 组装创建/编辑的结果摘要（DryRun 与 MCP 确认卡共用）。
func (s *ScheduleService) buildPreview(ctx context.Context, e *entity.ScheduleEvent, rule RRule, attendeeIds []string) (*ScheduleEventPreview, error) {
	anchor := time.Time(*e.StartAt)
	end := time.Time(*e.EndAt)
	p := &ScheduleEventPreview{
		Title: e.Title, EventType: e.EventType,
		StartAt: formatScheduleTime(anchor), EndAt: formatScheduleTime(end),
		RepeatDesc: describeRRule(rule, anchor),
		RoomId: e.RoomId, OnlineUrl: e.OnlineUrl, Location: e.Location,
		Attendees: []AttendeeView{}, Conflicts: []ConflictView{},
	}
	ids := append([]string{e.OrganizerId}, attendeeIds...)
	names := userNameMap(ctx, s.userRepo, dedupeStrings(ids))
	p.Attendees = append(p.Attendees, AttendeeView{UserId: e.OrganizerId, UserName: names[e.OrganizerId], Status: ScheduleAttendeeAccept})
	for _, uid := range attendeeIds {
		p.Attendees = append(p.Attendees, AttendeeView{UserId: uid, UserName: names[uid], Status: ScheduleAttendeePending})
	}
	if e.RoomId != "" {
		if room, err := s.roomRepo.Get(ctx, e.RoomId); err == nil {
			p.RoomName = room.Name
		}
	}
	return p, nil
}

// inviteContent 邀请/变更通知正文。
func (s *ScheduleService) inviteContent(ctx context.Context, organizerID string, p *ScheduleEventPreview, attendeeIds []string) (string, bool) {
	names := userNameMap(ctx, s.userRepo, append([]string{organizerID}, attendeeIds...))
	organizer := names[organizerID]
	if organizer == "" {
		organizer = organizerID
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s 邀请你参加「%s」\n时间：%s ~ %s", organizer, p.Title, p.StartAt, strings.Split(p.EndAt, " ")[1]))
	if p.RepeatDesc != "" {
		b.WriteString("（" + p.RepeatDesc + "）")
	}
	place := roomLabel(p.RoomName, p.RoomId)
	if place == "" {
		place = p.Location
	}
	if p.OnlineUrl != "" {
		if place != "" {
			place += "；"
		}
		place += "线上：" + p.OnlineUrl
	}
	if place != "" {
		b.WriteString("\n地点：" + place)
	}
	if len(attendeeIds) > 0 {
		var ns []string
		for _, uid := range attendeeIds {
			if n := names[uid]; n != "" {
				ns = append(ns, n)
			}
		}
		if len(ns) > 0 {
			b.WriteString("\n参会人：" + strings.Join(ns, "、"))
		}
	}
	return b.String(), true
}

func (s *ScheduleService) notifyUsers(ctx context.Context, userIds []string, ntype, title, content, link string) {
	if s.channelNotice == nil {
		return
	}
	for _, uid := range userIds {
		if uid == "" {
			continue
		}
		s.channelNotice.Push(ctx, uid, title, content, link, ntype, "schedule")
	}
}

func roomLabel(name, roomID string) string {
	if name != "" {
		return name
	}
	if roomID != "" {
		return "会议室 " + roomID
	}
	return ""
}

// canonicalRRule 归一化后回写 jsonb（空规则 → "{}"）。
func canonicalRRule(r RRule) string {
	n := r.normalized()
	if n.Freq == "" {
		return "{}"
	}
	// DAILY/MONTHLY 不落 byday
	if n.Freq != "WEEKLY" {
		n.ByDay = nil
	}
	b, err := json.Marshal(n)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func tp(t time.Time) *types.Time {
	v := types.Time(t)
	return &v
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
