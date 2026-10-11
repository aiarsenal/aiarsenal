package ops

// 日程域（aa_schedule 库）：我的日程/会议室只读 + 日程会议创建修改取消写入。
// 读端点 platform-schedule（/mcp/schedule）、写端点 platform-schedule-write（/mcp/schedule-write）。
// 与 Web【日程】模块同一套 ScheduleService；身份锁定同预算域（actor 锁定本人，系统身份显式 user_id）。
//
// 写入走「提案→确认→执行」：confirmed 缺省 false 只预检（返回摘要+冲突明细，不落库不发通知），
// AI 把摘要呈给用户认可后带 confirmed=true 重调才真正写入；房间冲突仍拒绝时须用户明示
// ignore_conflicts=true。周期规则子集：repeat=none|daily|weekly|monthly（weekly 配 byday），
// interval/until/count 限定范围，不自动跳过冲突场次。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
)

// scheduleSelfIdentity 调用方身份：actor 锁定本人；系统身份显式 user_id。
func scheduleSelfIdentity(ctx context.Context, a map[string]any) (string, error) {
	if t := identityFromCtx(ctx); t != nil {
		return actorUserId(t), nil
	}
	uid := argStr(a, "user_id", "")
	if uid == "" {
		return "", fmt.Errorf("缺少调用方身份，须显式传 user_id")
	}
	return uid, nil
}

// scheduleTypeName 事件类型枚举词 → 数值（缺省按 room/online 推断）。
func scheduleTypeName(a map[string]any) int {
	switch argStr(a, "event_type", "") {
	case "personal":
		return 0
	case "meeting":
		return 1
	case "online":
		return 2
	case "hybrid":
		return 3
	}
	if argStr(a, "room_id", "") != "" && argStr(a, "online_url", "") != "" {
		return 3
	}
	if argStr(a, "room_id", "") != "" {
		return 1
	}
	if argStr(a, "online_url", "") != "" {
		return 2
	}
	return 0
}

var scheduleTypeNameMap = map[int]string{0: "普通日程", 1: "线下会议", 2: "线上会议", 3: "线上+线下"}
var scheduleStatusNameMap = map[int]string{0: "待确认", 1: "已接受", 2: "已拒绝"}

// scheduleRRule 把 repeat/byday/interval/until/count 参数折成 rrule json（空规则→""）。
func scheduleRRule(a map[string]any) (string, error) {
	repeat := strings.ToLower(argStr(a, "repeat", "none"))
	if repeat == "" || repeat == "none" || repeat == "once" || repeat == "单场" {
		return "", nil
	}
	r := service.RRule{Interval: 1}
	switch repeat {
	case "daily", "weekly", "monthly":
		r.Freq = strings.ToUpper(repeat)
	default:
		return "", fmt.Errorf("repeat 取值：none|daily|weekly|monthly")
	}
	if r.Freq == "WEEKLY" {
		if byday := argStr(a, "byday", ""); byday != "" {
			for _, d := range strings.Split(byday, ",") {
				d = strings.ToUpper(strings.TrimSpace(d))
				switch d {
				case "MO", "TU", "WE", "TH", "FR", "SA", "SU":
					r.ByDay = append(r.ByDay, d)
				default:
					return "", fmt.Errorf("byday 取值：MO/TU/WE/TH/FR/SA/SU（逗号分隔）")
				}
			}
		}
	}
	if v := argInt(a, "interval", 0); v > 1 {
		r.Interval = v
	}
	if v := argStr(a, "until", ""); v != "" {
		if _, err := time.ParseInLocation("2006-01-02", v, time.Local); err != nil {
			return "", fmt.Errorf("until 须为 YYYY-MM-DD")
		}
		r.Until = v
	}
	if v := argInt(a, "count", 0); v > 0 {
		r.Count = v
	}
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// schedulePlace 一行地点摘要（房间名/自定义地点/线上链接）。
func schedulePlace(roomName, roomID, location, onlineURL string) string {
	var parts []string
	if roomName != "" {
		parts = append(parts, roomName)
	} else if roomID != "" {
		parts = append(parts, "会议室 "+roomID)
	}
	if location != "" {
		parts = append(parts, location)
	}
	if onlineURL != "" {
		parts = append(parts, "线上："+onlineURL)
	}
	return strings.Join(parts, "；")
}

// schedulePreviewOut 创建/编辑预检结果 → 呈给用户确认的摘要。
func schedulePreviewOut(p *service.ScheduleEventPreview, confirmed bool) string {
	place := schedulePlace(p.RoomName, p.RoomId, p.Location, p.OnlineUrl)
	var names []string
	for _, at := range p.Attendees {
		if at.UserName != "" {
			names = append(names, at.UserName)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "「%s」%s\n时间：%s ~ %s", p.Title, scheduleTypeNameMap[p.EventType], p.StartAt, strings.Split(p.EndAt, " ")[1])
	if p.RepeatDesc != "" {
		b.WriteString("（" + p.RepeatDesc + "）")
	}
	if place != "" {
		b.WriteString("\n地点：" + place)
	}
	if len(names) > 0 {
		b.WriteString("\n参会人：" + strings.Join(names, "、"))
	}
	if len(p.Conflicts) > 0 {
		b.WriteString(fmt.Sprintf("\n⚠ 会议室冲突 %d 处：", len(p.Conflicts)))
		for i, c := range p.Conflicts {
			if i >= 5 {
				fmt.Fprintf(&b, "\n  …等共 %d 处", len(p.Conflicts))
				break
			}
			fmt.Fprintf(&b, "\n  %s：%s ~ %s（%s）", c.Title, c.Start, c.End, c.RoomName)
		}
	}
	if p.Created {
		if len(p.Conflicts) > 0 {
			b.WriteString("\n已按用户明示忽略冲突创建")
		} else {
			b.WriteString("\n已创建并通知参会人")
		}
		return b.String()
	}
	switch {
	case len(p.Conflicts) > 0 && confirmed:
		return b.String() + "\n存在冲突且已带 ignore_conflicts 才会创建；请与用户确认后重调（必要时 ignore_conflicts=true）"
	case len(p.Conflicts) > 0:
		return b.String() + "\n以上为预检结果，未创建；请与用户确认（换时间/换房间，或明示 ignore_conflicts=true 强制预订）"
	default:
		return b.String() + "\n以上为预检摘要，未写入；用户认可后带 confirmed=true 重调即可创建"
	}
}

// ===== 读 server（platform-schedule，/mcp/schedule） =====

func buildScheduleServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-schedule", Version: "1.0"}, nil)

	opsTool(srv, "schedule_list", "查我的日程（真实数据）：当前调用方组织或参与的会议/日程场次，含周期展开、地点、参会人及我的响应状态；用户问「我明天有什么安排/这周几场会」用它",
		[]toolArg{{"from", "起始日 YYYY-MM-DD（缺省今天）", false}, {"to", "结束日 YYYY-MM-DD（缺省 from 后 7 天，含 to 当天）", false}, {"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := scheduleSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			from, to, err := scheduleWindowOf(a)
			if err != nil {
				return "", err
			}
			list, err := d.Schedule.ListMyOccurrences(ctx, uid, from, to)
			if err != nil {
				return "", err
			}
			if len(list) == 0 {
				return "（该时间段内没有日程）", nil
			}
			items := make([]map[string]any, 0, len(list))
			for _, o := range list {
				var atts []string
				for _, at := range o.Attendees {
					if at.UserName != "" {
						atts = append(atts, at.UserName+":"+scheduleStatusNameMap[at.Status])
					}
				}
				myStatus := ""
				if o.IsOrganizer {
					myStatus = "组织者"
				} else if n, ok := scheduleStatusNameMap[o.MyStatus]; ok {
					myStatus = n
				}
				items = append(items, map[string]any{
					"eventId": o.EventId, "date": o.Date, "start": o.Start, "end": o.End,
					"title": o.Title, "type": scheduleTypeNameMap[o.EventType],
					"place":   schedulePlace(o.RoomName, o.RoomId, o.Location, o.OnlineUrl),
					"recurring": map[bool]string{true: o.RepeatDesc, false: ""}[o.IsRecurring == 1],
					"modified": o.Modified, "myStatus": myStatus,
					"isOrganizer": o.IsOrganizer, "attendees": atts, "remark": o.Remark,
				})
			}
			return jsonOutAny(items)
		})

	opsTool(srv, "schedule_find_slots", "找共同空闲时段（真实数据）：给定人员（缺省=本人）在工作日 08:00-20:00 内都空闲、且时长足够的档期；约跨团队会议前先用它选时间",
		[]toolArg{{"user_ids", "参与人 user_id 逗号分隔（缺省=只查本人；可先用 user_search 查花名册拿 user_id）", false}, {"date", "起始日 YYYY-MM-DD（缺省今天）", false}, {"days", "向后查几天，1-14，默认 3", false}, {"duration_minute", "需要的时长（分钟），默认 30", false}, {"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := scheduleSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			uids := []string{uid}
			if v := argStr(a, "user_ids", ""); v != "" {
				for _, s := range strings.Split(v, ",") {
					if s = strings.TrimSpace(s); s != "" {
						uids = append(uids, s)
					}
				}
			}
			day, err := time.ParseInLocation("2006-01-02", argStr(a, "date", time.Now().Format("2006-01-02")), time.Local)
			if err != nil {
				return "", fmt.Errorf("date 须为 YYYY-MM-DD")
			}
			days := clampInt(argInt(a, "days", 3), 1, 14)
			dur := argInt(a, "duration_minute", 30)
			var items []map[string]any
			for i := 0; i < days; i++ {
				dt := day.AddDate(0, 0, i)
				from := time.Date(dt.Year(), dt.Month(), dt.Day(), 8, 0, 0, 0, time.Local)
				to := time.Date(dt.Year(), dt.Month(), dt.Day(), 20, 0, 0, 0, time.Local)
				slots, err := d.Schedule.FindFreeSlots(ctx, uids, from, to, dur)
				if err != nil {
					return "", err
				}
				for _, s := range slots {
					items = append(items, map[string]any{"date": dt.Format("2006-01-02"), "start": s.Start, "end": s.End})
				}
			}
			if len(items) == 0 {
				return fmt.Sprintf("（%s 起 %d 天内 08:00-20:00 没有满足 %d 分钟的共同空闲档期）", day.Format("2006-01-02"), days, dur), nil
			}
			return jsonOutAny(items)
		})

	opsTool(srv, "schedule_rooms", "查会议室列表（真实数据）：容量/位置/设备/启停状态；订会前用它按人数和设备选房间拿 room_id",
		[]toolArg{{"name", "名称/位置关键词过滤（可选）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			rooms, _, err := d.Schedule.RoomList(ctx, argStr(a, "name", ""), true, 1, 100)
			if err != nil {
				return "", err
			}
			if len(rooms) == 0 {
				return "（暂无启用的会议室）", nil
			}
			items := make([]map[string]any, 0, len(rooms))
			for _, r := range rooms {
				items = append(items, roomOut(r))
			}
			return jsonOutAny(items)
		})

	return srv
}

// roomOut 会议室 → 规整 map（设备 jsonb 转顿号串）。
func roomOut(r *entity.MeetingRoom) map[string]any {
	var equip []string
	_ = json.Unmarshal([]byte(r.Equipment), &equip)
	m := map[string]any{
		"roomId": r.RoomId, "name": r.Name, "location": r.Location,
		"capacity": r.Capacity, "enabled": r.Enabled == 1, "remark": r.Remark,
	}
	if len(equip) > 0 {
		m["equipment"] = strings.Join(equip, "、")
	}
	return m
}

// scheduleWindowOf from/to 参数（YYYY-MM-DD，to 含当天）→ 窗口；缺省今天 → 7 天后。
func scheduleWindowOf(a map[string]any) (time.Time, time.Time, error) {
	from, err := time.ParseInLocation("2006-01-02", argStr(a, "from", time.Now().Format("2006-01-02")), time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("from 须为 YYYY-MM-DD")
	}
	to := from.AddDate(0, 0, 7)
	if v := argStr(a, "to", ""); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("to 须为 YYYY-MM-DD")
		}
		to = t.AddDate(0, 0, 1) // 含当天 → 排他上界
	}
	return from, to, nil
}

// ===== 写 server（platform-schedule-write，/mcp/schedule-write） =====

func buildScheduleWriteServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-schedule-write", Version: "1.0"}, nil)

	opsTool(srv, "schedule_create", "创建日程/会议（写真实库，先预检后确认）：支持周期（如「每周五 14:00-15:00 周会」repeat=weekly&byday=FR&count=20）；confirmed 缺省 false 只返回摘要+会议室冲突不写入，用户认可后带 confirmed=true 重调才创建并通知参会人；冲突时换时间/房间或明示 ignore_conflicts=true",
		[]toolArg{
			{"title", "标题（必填）", true},
			{"start_at", "首场开始 YYYY-MM-DD HH:MM（必填）", true},
			{"end_at", "结束 YYYY-MM-DD HH:MM（与 duration_minute 二选一）", false},
			{"duration_minute", "时长分钟（end_at 缺省时用它，默认 60）", false},
			{"repeat", "周期：none|daily|weekly|monthly（默认 none 单场）", false},
			{"byday", "weekly 的星期：MO/TU/WE/TH/FR/SA/SU 逗号分隔（缺省=start_at 的星期）", false},
			{"interval", "间隔数（如每 2 周=2），默认 1", false},
			{"count", "总场数（含首场，如 20 场）；与 until 二选一，都不传=持续到规则上限", false},
			{"until", "截止日 YYYY-MM-DD（含当天）", false},
			{"room_id", "会议室 id（schedule_rooms 查得；线下会议用）", false},
			{"attendees", "参会人 user_id 逗号分隔（可先用 user_search 查花名册；组织者=当前用户自动加入）", false},
			{"event_type", "personal|meeting|online|hybrid（缺省按 room_id/online_url 推断）", false},
			{"online_url", "线上会议链接（线上会用）", false},
			{"location", "自定义地点（无会议室时）", false},
			{"remark", "备注/议程", false},
			{"ignore_conflicts", "true=忽略会议室冲突强制创建（用户明示后才可传）", false},
			{"confirmed", "true=确认执行写入（缺省只预检）", false},
			{"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false},
		},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := scheduleSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			in, err := scheduleInputOf(a, nil)
			if err != nil {
				return "", err
			}
			confirmed := argStr(a, "confirmed", "") == "true"
			in.IgnoreConflicts = argStr(a, "ignore_conflicts", "") == "true"
			in.DryRun = !confirmed
			p, err := d.Schedule.CreateEvent(ctx, uid, in)
			if err != nil {
				return "", err
			}
			return schedulePreviewOut(p, confirmed), nil
		})

	opsTool(srv, "schedule_update", "修改日程/会议（写真实库，改整个周期系列；单场调整用 schedule_cancel 的单场取消后重建，或先咨询用户）：只传要改的字段，其余保持原样；confirmed 缺省 false 只返回变更后摘要+冲突不写入",
		[]toolArg{
			{"event_id", "事件 id（schedule_list 查得，必填）", true},
			{"title", "新标题（可选）", false},
			{"start_at", "新首场开始 YYYY-MM-DD HH:MM（可选）", false},
			{"end_at", "新结束 YYYY-MM-DD HH:MM（可选；与 duration_minute 二选一）", false},
			{"duration_minute", "新时长分钟（可选）", false},
			{"repeat", "新周期 none|daily|weekly|monthly（可选；传了才改周期）", false},
			{"byday", "weekly 的星期 MO..SU 逗号分隔（与 repeat 同用）", false},
			{"count", "新总场数（与 repeat 同用）", false},
			{"until", "新截止日 YYYY-MM-DD（与 repeat 同用）", false},
			{"room_id", "新会议室 id（可选）", false},
			{"attendees", "新参会人 user_id 逗号分隔（可选；整表覆盖）", false},
			{"online_url", "新线上链接（可选）", false},
			{"location", "新地点（可选）", false},
			{"remark", "新备注（可选）", false},
			{"ignore_conflicts", "true=忽略会议室冲突强制保存（用户明示后才可传）", false},
			{"confirmed", "true=确认执行写入（缺省只预检）", false},
			{"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false},
		},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := scheduleSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			eventID := argStr(a, "event_id", "")
			if eventID == "" {
				return "", fmt.Errorf("event_id 不能为空")
			}
			base, err := d.Schedule.GetEventInput(ctx, eventID)
			if err != nil {
				return "", err
			}
			in, err := scheduleInputOf(a, &base)
			if err != nil {
				return "", err
			}
			confirmed := argStr(a, "confirmed", "") == "true"
			in.IgnoreConflicts = argStr(a, "ignore_conflicts", "") == "true"
			in.DryRun = !confirmed
			p, err := d.Schedule.UpdateEvent(ctx, uid, eventID, in)
			if err != nil {
				return "", err
			}
			return schedulePreviewOut(p, confirmed), nil
		})

	opsTool(srv, "schedule_cancel", "取消日程/会议（写真实库）：不传 date 取消整个周期系列并通知参会人；传 date（该场次日期）只取消周期里的那一场，其余场次不变；confirmed 缺省 false 只返回将被取消的内容不写入",
		[]toolArg{
			{"event_id", "事件 id（schedule_list 查得，必填）", true},
			{"date", "单场取消时传该场次日期 YYYY-MM-DD（周期事件专用；缺省=取消整个系列）", false},
			{"confirmed", "true=确认执行写入（缺省只预检）", false},
			{"user_id", "系统身份显式指定（网关调用自动锁定本人，勿传）", false},
		},
		func(ctx context.Context, a map[string]any) (string, error) {
			uid, err := scheduleSelfIdentity(ctx, a)
			if err != nil {
				return "", err
			}
			eventID := argStr(a, "event_id", "")
			if eventID == "" {
				return "", fmt.Errorf("event_id 不能为空")
			}
			date := argStr(a, "date", "")
			det, err := d.Schedule.GetDetail(ctx, eventID, uid)
			if err != nil {
				return "", err
			}
			if argStr(a, "confirmed", "") != "true" {
				scope := "整个系列"
				if date != "" {
					scope = date + " 这一场"
				}
				return fmt.Sprintf("预检：将取消「%s」（%s ~ %s%s）的%s，不写入；用户认可后带 confirmed=true 重调",
					det.Title, det.Start, strings.Split(det.End, " ")[1], map[bool]string{true: "，" + det.RepeatDesc, false: ""}[det.IsRecurring == 1], scope), nil
			}
			if date != "" {
				if err := d.Schedule.AddException(ctx, uid, service.ExceptionInput{
					EventId: eventID, Date: date, Status: service.ScheduleExceptionCancel,
				}); err != nil {
					return "", err
				}
				return fmt.Sprintf("已取消「%s」%s 这一场（其余场次不变），已通知参会人", det.Title, date), nil
			}
			if err := d.Schedule.CancelEvent(ctx, uid, eventID); err != nil {
				return "", err
			}
			return fmt.Sprintf("已取消「%s」整个系列，已通知参会人", det.Title), nil
		})

	return srv
}

// scheduleInputOf 工具参数 → ScheduleEventInput。base 非空（编辑）时未出现的字段保持原值。
func scheduleInputOf(a map[string]any, base *service.ScheduleEventInput) (service.ScheduleEventInput, error) {
	in := service.ScheduleEventInput{}
	if base != nil {
		in = *base
	}
	if v := argStr(a, "title", ""); v != "" || base == nil {
		in.Title = v
	}
	startAt, err := service.ParseScheduleTime(argStr(a, "start_at", ""))
	if err != nil {
		if base == nil {
			return in, fmt.Errorf("start_at：%w", err)
		}
		startAt = in.StartAt // 编辑未传 → 保持原值
	} else {
		in.StartAt = startAt
	}
	if v := argStr(a, "end_at", ""); v != "" {
		endAt, err := service.ParseScheduleTime(v)
		if err != nil {
			return in, fmt.Errorf("end_at：%w", err)
		}
		in.EndAt = endAt
	} else if dm := argInt(a, "duration_minute", 0); dm > 0 {
		in.EndAt = in.StartAt.Add(time.Duration(dm) * time.Minute)
	} else if base == nil {
		in.EndAt = in.StartAt.Add(60 * time.Minute)
	}
	if _, ok := a["repeat"]; ok || base == nil {
		rule, err := scheduleRRule(a)
		if err != nil {
			return in, err
		}
		in.RRule = rule
	}
	if _, ok := a["room_id"]; ok {
		in.RoomId = argStr(a, "room_id", "")
	}
	if _, ok := a["online_url"]; ok {
		in.OnlineUrl = argStr(a, "online_url", "")
	}
	if _, ok := a["location"]; ok {
		in.Location = argStr(a, "location", "")
	}
	if _, ok := a["remark"]; ok {
		in.Remark = argStr(a, "remark", "")
	}
	if v, ok := a["attendees"]; ok {
		if s, isStr := v.(string); isStr {
			in.Attendees = nil
			for _, p := range strings.Split(s, ",") {
				if p = strings.TrimSpace(p); p != "" {
					in.Attendees = append(in.Attendees, p)
				}
			}
		}
	}
	if base == nil || argStr(a, "event_type", "") != "" {
		in.EventType = scheduleTypeName(a)
	}
	return in, nil
}
