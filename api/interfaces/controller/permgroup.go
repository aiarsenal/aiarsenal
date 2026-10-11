package controller

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// PermGroupController 权限分组/员工集合两字典管理（系统管理 → 权限分组管理）。
// options 瘦投影端点供 GrantEditor 拉选项（登录即可见，无词条明细）。
type PermGroupController struct {
	svc *service.PermGroupService
}

func NewPermGroupController(s *service.PermGroupService) *PermGroupController {
	return &PermGroupController{svc: s}
}

// ===== 权限分组 =====

func (h *PermGroupController) GroupList(c *gin.Context) {
	var req dto.PermGroupListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.ListGroups(c, req.Name)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *PermGroupController) GroupSave(c *gin.Context) {
	var req dto.PermGroupSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	g := &entity.PermGroup{GroupId: req.Id, Name: req.Name, Remark: req.Remark, CreatedBy: middleware.GetUserId(c)}
	if err := h.svc.SaveGroup(c, g, req.Terms); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"id": g.GroupId})
}

func (h *PermGroupController) GroupDelete(c *gin.Context) {
	var req dto.PermGroupDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteGroups(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// ===== 员工集合 =====

func (h *PermGroupController) SetList(c *gin.Context) {
	var req dto.EmpSetListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.ListSets(c, req.Name)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

func (h *PermGroupController) SetSave(c *gin.Context) {
	var req dto.EmpSetSaveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	st := &entity.EmpSet{SetId: req.Id, Name: req.Name, Remark: req.Remark, CreatedBy: middleware.GetUserId(c)}
	if err := h.svc.SaveSet(c, st, req.Members); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"id": st.SetId})
}

func (h *PermGroupController) SetDelete(c *gin.Context) {
	var req dto.EmpSetDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.DeleteSets(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

// SetResolve 工号批量解析姓名+部门（集合弹窗成员明细确认；不在册工号不返回）
func (h *PermGroupController) SetResolve(c *gin.Context) {
	var req dto.EmpSetResolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.ResolveMembers(c, req.EmpNos)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if list == nil {
		list = []*model.MemberInfoRes{}
	}
	middleware.Success(c, gin.H{"list": list})
}

// Options 瘦投影（GrantEditor 选择器；登录即可见）
func (h *PermGroupController) Options(c *gin.Context) {
	groups, sets, err := h.svc.Options(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	if groups == nil {
		groups = []map[string]string{}
	}
	if sets == nil {
		sets = []map[string]string{}
	}
	middleware.Success(c, dto.PermGroupOptionsRes{Groups: groups, Sets: sets})
}
