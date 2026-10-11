package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/middleware"
)

// agent_usage.go — 运行时用量上报（容器内 agent 控制面）与查询（管理端）。
// 请求/响应结构体内联在本文件（dto 目录不归本任务改）。

// AgentUsageController 用量控制器。
type AgentUsageController struct{ svc *service.AgentUsageService }

// NewAgentUsageController 构造。
func NewAgentUsageController(svc *service.AgentUsageService) *AgentUsageController {
	return &AgentUsageController{svc: svc}
}

// UsageReportReq 上报请求体（InstanceAuth 路由：instanceId 取中间件注入，不由客户端传）。
type UsageReportReq struct {
	Usage []service.UsageReportItem `json:"usage"`
	Logs  []string                  `json:"logs"`
}

// Report 运行时上报（POST /agent/runtime/report）。
// 与心跳（POST /agent/runtime/heartbeat）同走 agRuntime（InstanceAuth）分组，
// 计量与心跳分端点：心跳保持轻量不动，计量批量可大。
func (h *AgentUsageController) Report(c *gin.Context) {
	var req UsageReportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	instanceId := middleware.GetInstanceId(c)
	if instanceId == "" {
		middleware.Fail(c, "缺少实例身份")
		return
	}
	inserted, err := h.svc.Report(c, instanceId, service.UsageReport{Usage: req.Usage, Logs: req.Logs})
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"inserted": inserted})
}

// List 用量查询（GET /agent/usage/list?instanceId=&start=&end=&pageNum=&pageSize=）。
// start/end 接 RFC3339 或 yyyy-MM-dd（本地时区零点），空=不限。
func (h *AgentUsageController) List(c *gin.Context) {
	instanceId := c.Query("instanceId")
	start := parseTimeParam(c.Query("start"), false)
	end := parseTimeParam(c.Query("end"), true)
	page, err := h.svc.List(c, instanceId, start, end, queryInt(c, "pageNum", 1), queryInt(c, "pageSize", 20))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, page)
}

// parseTimeParam 时间参数解析：RFC3339 优先，其次 yyyy-MM-dd（end 取当日 23:59:59，
// start 取零点），空/非法返回零值（repo 侧 IsZero 跳过条件）。
func parseTimeParam(s string, endOfDay bool) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		if endOfDay {
			return t.Add(24*time.Hour - time.Second)
		}
		return t
	}
	return time.Time{}
}
