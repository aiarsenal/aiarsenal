package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentTeamController Team 协作：team / member / task / event / work-item。
type AgentTeamController struct{ svc *service.AgentTeamService }

func NewAgentTeamController(s *service.AgentTeamService) *AgentTeamController {
	return &AgentTeamController{svc: s}
}

// ===== Team =====
func (h *AgentTeamController) ListTeams(c *gin.Context) {
	var req dto.AgentTeamSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	uid := middleware.GetUserId(c)
	list, total, err := h.svc.ListTeams(c, uid, req.Name, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentTeamSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentTeamController) GetTeam(c *gin.Context) {
	id := c.Query("teamId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.GetTeam(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentTeamController) AddTeam(c *gin.Context) {
	var req dto.AgentTeamAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentTeamReq
	uid := middleware.GetUserId(c)
	if _, err := h.svc.AddTeam(c, &entity.AgentTeam{
		UserId: uid, Name: r.Name, Description: r.Description, CommunicationMode: r.CommunicationMode,
		SharedVolumeName: r.SharedVolumeName, SharedMountPath: r.SharedMountPath, RedisUrl: r.RedisUrl, TeamToken: r.TeamToken,
		CreatedBy: uid,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentTeamController) EditTeam(c *gin.Context) {
	var req dto.AgentTeamEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentTeamReq
	if err := h.svc.EditTeam(c, &entity.AgentTeam{
		TeamId: req.TeamId, Name: r.Name, Description: r.Description, CommunicationMode: r.CommunicationMode,
		SharedVolumeName: r.SharedVolumeName, SharedMountPath: r.SharedMountPath, RedisUrl: r.RedisUrl, TeamToken: r.TeamToken,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentTeamController) DeleteTeams(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteTeams(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== Member =====
func (h *AgentTeamController) ListMembers(c *gin.Context) {
	var req dto.AgentTeamMemberSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListMembers(c, req.TeamId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentTeamMemberSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentTeamController) AddMember(c *gin.Context) {
	var req dto.AgentTeamMemberAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentTeamMemberReq
	if _, err := h.svc.AddMember(c, &entity.AgentTeamMember{
		TeamId: r.TeamId, InstanceId: r.InstanceId, UserId: r.UserId, MemberKey: r.MemberKey,
		DisplayName: r.DisplayName, Role: r.Role, Description: r.Description,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentTeamController) EditMember(c *gin.Context) {
	var req dto.AgentTeamMemberEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentTeamMemberReq
	if err := h.svc.EditMember(c, &entity.AgentTeamMember{
		TeamMemberId: req.TeamMemberId, TeamId: r.TeamId, InstanceId: r.InstanceId, UserId: r.UserId,
		MemberKey: r.MemberKey, DisplayName: r.DisplayName, Role: r.Role, Description: r.Description,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentTeamController) DeleteMembers(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteMembers(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== Task =====
func (h *AgentTeamController) ListTasks(c *gin.Context) {
	var req dto.AgentTeamTaskSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListTasks(c, req.TeamId, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentTeamTaskSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentTeamController) DispatchTask(c *gin.Context) {
	var req dto.AgentTeamTaskDispatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	uid := middleware.GetUserId(c)
	if _, err := h.svc.DispatchTask(c, &entity.AgentTeamTask{
		TeamId: req.TeamId, TargetMemberId: req.TargetMemberId, CreatedBy: uid,
		PayloadJson: req.PayloadJson, Status: 0,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== Event =====
func (h *AgentTeamController) ListEvents(c *gin.Context) {
	var req dto.AgentTeamEventSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListEvents(c, req.TeamId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentTeamEventSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// ===== WorkItem =====
func (h *AgentTeamController) ListWorkItems(c *gin.Context) {
	var req dto.AgentTeamWorkItemSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListWorkItems(c, req.RootTaskId, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentTeamWorkItemSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentTeamController) AddWorkItem(c *gin.Context) {
	var req dto.AgentTeamWorkItemAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.AgentTeamWorkItemReq
	if _, err := h.svc.AddWorkItem(c, &entity.AgentTeamWorkItem{
		TeamId: r.TeamId, RootTaskId: r.RootTaskId, WorkId: r.WorkId, OwnerMemberId: r.OwnerMemberId,
		Title: r.Title, DependsOnJson: r.DependsOnJson,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentTeamController) EditWorkItem(c *gin.Context) {
	var req dto.AgentTeamWorkItemEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.EditWorkItem(c, &entity.AgentTeamWorkItem{
		WorkItemId: req.WorkItemId, Status: req.Status, ResultJson: req.ResultJson,
	}); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentTeamController) DeleteWorkItems(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteWorkItems(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}
