package controller

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

type UserController struct{ svc *service.UserService }

func NewUserController(s *service.UserService) *UserController { return &UserController{svc: s} }

func (h *UserController) List(c *gin.Context) {
	var req dto.UserSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	slog.Debug("user 列表请求", "keyWords", req.KeyWords, "deptId", req.DeptId, "mobile", req.Mobile, "status", req.Status, "pageNum", req.PageNum, "pageSize", req.PageSize)
	list, total, err := h.svc.List(c, req.DeptId, req.Mobile, req.Status, req.KeyWords, req.PageNum, req.PageSize)
	if err != nil {
		slog.Error("user 列表失败", "keyWords", req.KeyWords, "status", req.Status, "err", err)
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.UserSearchRes{UserList: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// SearchOptions 人员选择器搜索 GET /api/v1/system/user/search
// 姓名 / 工号 / 手机号 / 邮箱 四字段 OR，返回精简用户列表（用于负责人等人员选择）。
func (h *UserController) SearchOptions(c *gin.Context) {
	var req dto.UserSearchOptionsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.SearchOptions(c, req.KeyWords, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.UserSearchOptionsRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

// ResolveOptions 按 id 批量解析 GET /api/v1/system/user/resolve（授权条件行已选用户回显）。
func (h *UserController) ResolveOptions(c *gin.Context) {
	var req dto.UserResolveReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	ids := make([]string, 0, 32)
	seen := make(map[string]bool)
	for _, id := range strings.Split(req.Ids, ",") {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) > 200 {
		middleware.Fail(c, "单次最多解析 200 个用户")
		return
	}
	list, err := h.svc.ResolveOptions(c, ids)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.UserResolveRes{List: list})
}

func (h *UserController) GetParams(c *gin.Context) {
	roles, posts, err := h.svc.GetParams(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.UserGetParamsRes{RoleList: roles, Posts: posts})
}

func (h *UserController) Add(c *gin.Context) {
	var req dto.UserAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.SetUserReq
	u := &entity.User{
		UserName: req.UserName, EmployeeNo: r.EmployeeNo, Name: r.Name,
		Mobile: r.Mobile, UserNickname: r.NickName,
		UserStatus: r.Status, UserEmail: r.Email, Sex: r.Sex, DeptId: r.DeptId,
		Remark: r.Remark, IsAdmin: r.IsAdmin,
	}
	if err := h.svc.Add(c, u, req.Password, r.RoleIds, r.PostIds); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *UserController) Edit(c *gin.Context) {
	var req dto.UserEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.SetUserReq
	u := &entity.User{
		UserId: req.UserId, EmployeeNo: r.EmployeeNo, Name: r.Name,
		Mobile: r.Mobile, UserNickname: r.NickName,
		UserStatus: r.Status, UserEmail: r.Email, Sex: r.Sex, DeptId: r.DeptId,
		Remark: r.Remark, IsAdmin: r.IsAdmin,
	}
	if err := h.svc.Edit(c, u, r.RoleIds, r.PostIds); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *UserController) GetEdit(c *gin.Context) {
	var req dto.UserGetEditReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	user, roleIds, postIds, err := h.svc.GetEditUser(c, req.Id)
	if err != nil {
		middleware.Fail(c, "获取用户数据失败")
		return
	}
	middleware.Success(c, dto.UserGetEditRes{User: user, CheckedRoleIds: roleIds, CheckedPosts: postIds})
}

func (h *UserController) ResetPwd(c *gin.Context) {
	var req dto.UserResetPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.ResetPwd(c, req.UserId, req.Password); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *UserController) SetStatus(c *gin.Context) {
	var req dto.UserStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.ChangeStatus(c, req.UserId, req.UserStatus); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *UserController) Delete(c *gin.Context) {
	var req dto.UserDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Delete(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *UserController) GetUsers(c *gin.Context) {
	var req dto.UserGetByIdsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, err := h.svc.GetUsersByIds(c, req.Ids)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, dto.UserGetByIdsRes{List: list})
}
