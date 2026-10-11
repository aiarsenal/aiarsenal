package controller

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/middleware"
)

// agent_user.go — Agent 平台用户管理（v2 占位菜单：aa_agent_user 新表）。
// 契约与前端 agent/user.ts 对齐：userId/enabled(1|0)/instanceNames(富化)，
// 新建与重置口令均返回一次性明文（落库仅哈希）。
// 与主库 aa_user 解耦（username 软引用），增删改查仅本表。

// AgentUserController Agent 平台用户控制器。
type AgentUserController struct {
	svc     *service.AgentUserService
	instSvc *service.AgentInstanceService // instanceIds → 实例名富化
}

// NewAgentUserController 构造。
func NewAgentUserController(svc *service.AgentUserService, instSvc *service.AgentInstanceService) *AgentUserController {
	return &AgentUserController{svc: svc, instSvc: instSvc}
}

// List 用户分页（GET /agent/user/list?keyword=&status=-1&pageNum=&pageSize=）。
func (h *AgentUserController) List(c *gin.Context) {
	keyword := c.Query("keyword")
	status := queryInt(c, "status", -1)
	pageNum := queryInt(c, "pageNum", 1)
	pageSize := queryInt(c, "pageSize", 20)
	list, total, err := h.svc.List(c, keyword, status, pageNum, pageSize)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, u := range list {
		items = append(items, agentUserView(c, h, u))
	}
	middleware.Success(c, gin.H{"list": items, "total": total, "currentPage": pageNum})
}

// agentUserUserReq 新建/编辑请求体（前端 UserReq：enabled 1|0）。
type agentUserUserReq struct {
	UserId      string   `json:"userId"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Role        string   `json:"role"`
	Enabled     uint     `json:"enabled"`
	InstanceIds []string `json:"instanceIds"`
	Remark      string   `json:"remark"`
}

// Add 新建用户（POST /agent/user/add；明文初始口令仅本次返回）。
func (h *AgentUserController) Add(c *gin.Context) {
	var req agentUserUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	id, initialPassword, err := h.svc.Add(c, service.AgentUserUpsert{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Role:        req.Role,
		InstanceIds: req.InstanceIds,
		IsEnabled:   req.Enabled,
		Remark:      req.Remark,
	}, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"userId": id, "initialPassword": initialPassword})
}

// Edit 编辑用户（PUT /agent/user/edit；body 带 userId，enabled=0 也生效）。
func (h *AgentUserController) Edit(c *gin.Context) {
	var req agentUserUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if req.UserId == "" {
		middleware.Fail(c, "缺少 userId")
		return
	}
	if err := h.svc.Edit(c, req.UserId, service.AgentUserUpsert{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Role:        req.Role,
		InstanceIds: req.InstanceIds,
		IsEnabled:   req.Enabled,
		Remark:      req.Remark,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// Delete 删用户（DELETE /agent/user/delete，body {ids:[...]}）。
func (h *AgentUserController) Delete(c *gin.Context) {
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

// ResetPassword 重置口令（PUT /agent/user/reset-password，body {userId}；新明文仅本次返回）。
func (h *AgentUserController) ResetPassword(c *gin.Context) {
	var req struct {
		UserId string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	password, err := h.svc.ResetPassword(c, req.UserId)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"password": password})
}

// agentUserView 实体 → 前端行视图：enabled 转 1|0、instanceIds 解 jsonb、
// instanceNames 按实例逐个富化（查不到的 id 静默跳过——实例可能已删）。
func agentUserView(c *gin.Context, h *AgentUserController, u *entity.AgentUser) gin.H {
	enabled := 0
	if u.IsEnabled == 1 {
		enabled = 1
	}
	ids := []string{}
	_ = json.Unmarshal([]byte(u.InstanceIds), &ids)
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if ins, err := h.instSvc.Get(c, id); err == nil && ins != nil {
			names = append(names, ins.Name)
		}
	}
	return gin.H{
		"userId":        u.AgentUserId,
		"username":      u.Username,
		"displayName":   u.DisplayName,
		"role":          u.Role,
		"enabled":       enabled,
		"instanceIds":   ids,
		"instanceNames": names,
		"remark":        u.Remark,
		"createdBy":     u.CreatedBy,
		"createdAt":     u.CreatedAt,
		"updatedAt":     u.UpdatedAt,
	}
}
