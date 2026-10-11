package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// 日程控制器：我的日程/事件 CRUD/单场例外/参会回执/忙闲 + 会议室管理与当日时间轴。
// 视图结构（ScheduleOccurrenceView 等）定义在 service 层，REST 与 MCP 共用。

type ScheduleController struct {
	svc *service.ScheduleService
}

func NewScheduleController(svc *service.ScheduleService) *ScheduleController {
	return &ScheduleController{svc: svc}
}

func (h *ScheduleController) eventInput(req dto.ScheduleEventAddReq) (service.ScheduleEventInput, error) {
	startAt, err := service.ParseScheduleTime(req.StartAt)
	if err != nil {
		return service.ScheduleEventInput{}, err
	}
	endAt, err := service.ParseScheduleTime(req.EndAt)
	if err != nil {
		return service.ScheduleEventInput{}, err
	}
	return service.ScheduleEventInput{
		Title: req.Title, EventType: req.EventType,
		StartAt: startAt, EndAt: endAt, RRule: req.RRule,
		RoomId: req.RoomId, OnlineProvider: req.OnlineProvider, OnlineUrl: req.OnlineUrl,
		Location: req.Location, Attendees: req.Attendees, Remark: req.Remark,
		IgnoreConflicts: req.IgnoreConflicts, DryRun: req.DryRun,
	}, nil
}

// List 我的日程（窗口内场次展开）
func (h *ScheduleController) List(c *gin.Context) {
	var req dto.ScheduleEventListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	from, to, err := scheduleWindow(req.From, req.To)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.ListMyOccurrences(c, middleware.GetUserId(c), from, to)
	if err != nil {
		middleware.Fail(c, "获取数据失败："+err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *ScheduleController) Detail(c *gin.Context) {
	var req dto.ScheduleEventDetailReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	d, err := h.svc.GetDetail(c, req.EventId, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, d)
}

func (h *ScheduleController) Add(c *gin.Context) {
	var req dto.ScheduleEventAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	in, err := h.eventInput(req)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	p, err := h.svc.CreateEvent(c, middleware.GetUserId(c), in)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, p)
}

func (h *ScheduleController) Edit(c *gin.Context) {
	var req dto.ScheduleEventAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.EventId == "" {
		middleware.Fail(c, "eventId 不能为空")
		return
	}
	in, err := h.eventInput(req)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	p, err := h.svc.UpdateEvent(c, middleware.GetUserId(c), req.EventId, in)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, p)
}

func (h *ScheduleController) Delete(c *gin.Context) {
	var req dto.ScheduleIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	for _, id := range req.Ids {
		if err := h.svc.CancelEvent(c, middleware.GetUserId(c), id); err != nil {
			middleware.Fail(c, err.Error())
			return
		}
	}
	middleware.Success(c, nil)
}

func (h *ScheduleController) Exception(c *gin.Context) {
	var req dto.ScheduleExceptionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	in := service.ExceptionInput{EventId: req.EventId, Date: req.Date, Status: req.Status, NewRoomId: req.NewRoomId}
	if req.NewStartAt != "" {
		t, err := service.ParseScheduleTime(req.NewStartAt)
		if err != nil {
			middleware.Fail(c, err.Error())
			return
		}
		in.NewStartAt = t
	}
	if req.NewEndAt != "" {
		t, err := service.ParseScheduleTime(req.NewEndAt)
		if err != nil {
			middleware.Fail(c, err.Error())
			return
		}
		in.NewEndAt = t
	}
	if err := h.svc.AddException(c, middleware.GetUserId(c), in); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, nil)
}

func (h *ScheduleController) Respond(c *gin.Context) {
	var req dto.ScheduleRespondReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Respond(c, req.EventId, middleware.GetUserId(c), req.Status); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, nil)
}

func (h *ScheduleController) FreeBusy(c *gin.Context) {
	var req dto.ScheduleFreeBusyReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	from, to, err := scheduleWindow(req.From, req.To)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.FreeBusy(c, req.UserIds, from, to)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// FindFreeSlots 共同空闲（约会助手）
func (h *ScheduleController) FindSlots(c *gin.Context) {
	var req dto.ScheduleFreeBusyReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	from, to, err := scheduleWindow(req.From, req.To)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	slots, err := h.svc.FindFreeSlots(c, req.UserIds, from, to, req.DurationMinute)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": slots})
}

// ===================== 会议室 =====================

func (h *ScheduleController) RoomList(c *gin.Context) {
	var req dto.ScheduleRoomListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.RoomList(c, req.Name, req.EnabledOnly, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, gin.H{"list": list, "currentPage": pn, "total": total})
}

func (h *ScheduleController) RoomOverview(c *gin.Context) {
	var req dto.ScheduleRoomDayReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.Date == "" {
		req.Date = time.Now().Format("2006-01-02")
	}
	list, err := h.svc.RoomDayOverview(c, req.Date)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list, "date": req.Date})
}

func (h *ScheduleController) RoomAdd(c *gin.Context) {
	var req dto.ScheduleRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.RoomAdd(c, middleware.GetUserId(c), req.Name, req.Location, req.Capacity, req.Equipment, req.Enabled, req.Remark); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, nil)
}

func (h *ScheduleController) RoomEdit(c *gin.Context) {
	var req dto.ScheduleRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.RoomId == "" {
		middleware.Fail(c, "roomId 不能为空")
		return
	}
	if err := h.svc.RoomEdit(c, req.RoomId, req.Name, req.Location, req.Capacity, req.Equipment, req.Enabled, req.Remark); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, nil)
}

func (h *ScheduleController) RoomDelete(c *gin.Context) {
	var req dto.ScheduleIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.RoomDelete(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, nil)
}

// scheduleWindow 解析窗口参数；缺省今天 00:00 → 7 天后。
func scheduleWindow(fromStr, toStr string) (time.Time, time.Time, error) {
	from := time.Now()
	if fromStr != "" {
		t, err := service.ParseScheduleTime(fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		from = t
	} else {
		from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	}
	to := from.AddDate(0, 0, 7)
	if toStr != "" {
		t, err := service.ParseScheduleTime(toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		to = t
	}
	return from, to, nil
}
