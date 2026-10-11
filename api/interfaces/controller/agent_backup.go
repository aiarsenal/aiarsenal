package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/middleware"
)

// agent_backup.go — 实例备份/恢复 + 定时备份计划（localdocker）。
// 请求/响应结构体内联在本文件（dto 目录不归本任务改）。

// AgentBackupController 备份控制器。
type AgentBackupController struct {
	svc     *service.AgentBackupService
	instSvc *service.AgentInstanceService // owner 校验
}

// NewAgentBackupController 构造。
func NewAgentBackupController(svc *service.AgentBackupService, instSvc *service.AgentInstanceService) *AgentBackupController {
	return &AgentBackupController{svc: svc, instSvc: instSvc}
}

// checkOwner 实例存在 + 归属校验（备份按实例维度操作）。
func (h *AgentBackupController) checkOwner(c *gin.Context, instanceId string) bool {
	uid := middleware.GetUserId(c)
	ins, err := h.instSvc.Get(c, instanceId)
	if err != nil || ins == nil {
		middleware.Fail(c, "未找到实例")
		return false
	}
	if ins.UserId != uid {
		middleware.Fail(c, "无权访问该实例")
		return false
	}
	return true
}

// List 备份分页（GET /agent/backup/list?instanceId=&pageNum=&pageSize=；instanceId 空=全部，管理端视角）。
func (h *AgentBackupController) List(c *gin.Context) {
	list, total, err := h.svc.List(c, c.Query("instanceId"), queryInt(c, "pageNum", 1), queryInt(c, "pageSize", 20))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list, "total": total})
}

// BackupCreateReq 手动备份请求体。
type BackupCreateReq struct {
	InstanceId    string `json:"instanceId" binding:"required"`
	BackupName    string `json:"backupName"`
	RetentionDays int    `json:"retentionDays"` // >0 时 expires = now + N 天；0=永久保留
}

// Create 手动备份（POST /agent/backup/create）：受理即返回，执行异步。
func (h *AgentBackupController) Create(c *gin.Context) {
	var req BackupCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if !h.checkOwner(c, req.InstanceId) {
		return
	}
	var expires *time.Time
	if req.RetentionDays > 0 {
		t := time.Now().AddDate(0, 0, req.RetentionDays)
		expires = &t
	}
	id, err := h.svc.Create(c, &entity.AgentBackup{
		InstanceId: req.InstanceId,
		BackupName: req.BackupName,
		CreatedBy:  middleware.GetUserId(c),
	}, expires)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"backupId": id})
}

// Get 备份详情（GET /agent/backup/:id）。
func (h *AgentBackupController) Get(c *gin.Context) {
	row, err := h.svc.Get(c, c.Param("id"))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, row)
}

// Delete 删备份（POST /agent/backup/delete {ids:[...]}）：物理文件一起删。
func (h *AgentBackupController) Delete(c *gin.Context) {
	var req struct {
		Ids []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// Restore 恢复（POST /agent/backup/:id/restore）：停实例→解包 /aa→按原状态启停。
func (h *AgentBackupController) Restore(c *gin.Context) {
	row, err := h.svc.Get(c, c.Param("id"))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if !h.checkOwner(c, row.InstanceId) {
		return
	}
	if err := h.svc.Restore(c, c.Param("id"), middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// ==================== 备份计划 ====================

// ListSchedules 计划分页（GET /agent/backup/schedule/list?instanceId=&pageNum=&pageSize=）。
func (h *AgentBackupController) ListSchedules(c *gin.Context) {
	list, total, err := h.svc.ListSchedules(c, c.Query("instanceId"), queryInt(c, "pageNum", 1), queryInt(c, "pageSize", 20))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list, "total": total})
}

// ScheduleReq 计划新建/编辑请求体。
type ScheduleReq struct {
	InstanceId     string `json:"instanceId"`
	ScheduleName   string `json:"scheduleName"`
	CronExpression string `json:"cronExpression" binding:"required"` // 5 字段标准 cron
	RetentionDays  int    `json:"retentionDays"`
	IsActive       uint   `json:"isActive"`
}

// AddSchedule 新建计划（POST /agent/backup/schedule/create）。
func (h *AgentBackupController) AddSchedule(c *gin.Context) {
	var req ScheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if !h.checkOwner(c, req.InstanceId) {
		return
	}
	id, err := h.svc.AddSchedule(c, &entity.AgentBackupSchedule{
		InstanceId:     req.InstanceId,
		ScheduleName:   req.ScheduleName,
		CronExpression: req.CronExpression,
		RetentionDays:  req.RetentionDays,
		IsActive:       req.IsActive,
	})
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"backupScheduleId": id})
}

// EditSchedule 编辑计划（POST /agent/backup/schedule/:id/edit；isActive=0 停用即摘调度）。
func (h *AgentBackupController) EditSchedule(c *gin.Context) {
	var req ScheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	id := c.Param("id")
	old, err := h.svc.GetSchedule(c, id)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if !h.checkOwner(c, old.InstanceId) {
		return
	}
	if err := h.svc.EditSchedule(c, &entity.AgentBackupSchedule{
		BackupScheduleId: id,
		InstanceId:       req.InstanceId,
		ScheduleName:     req.ScheduleName,
		CronExpression:   req.CronExpression,
		RetentionDays:    req.RetentionDays,
		IsActive:         req.IsActive,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// DeleteSchedule 删计划（POST /agent/backup/schedule/delete {ids:[...]}）。
func (h *AgentBackupController) DeleteSchedule(c *gin.Context) {
	var req struct {
		Ids []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteSchedule(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}
