package middleware

import (
	"github.com/gin-gonic/gin"

	"aiarsenal/consts"
	"aiarsenal/domain/model"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/grantmeta"
)

const ctxUserKey = "currentUser"

// SetUser 把当前登录用户写入 gin.Context（由 JWTAuth 中间件调用）
func SetUser(c *gin.Context, u *model.LoginUserRes) {
	c.Set(ctxUserKey, u)
}

// GetUser 取当前登录用户
func GetUser(c *gin.Context) *model.LoginUserRes {
	v, ok := c.Get(ctxUserKey)
	if !ok {
		return nil
	}
	u, _ := v.(*model.LoginUserRes)
	return u
}

// GetUserId 当前登录用户 id（业务 id 字符串）
func GetUserId(c *gin.Context) string {
	if u := GetUser(c); u != nil {
		return u.Id
	}
	return ""
}

// GetDeptId 当前登录用户所属部门 id（aa_dept.dept_id）。
// 团队实体 = 部门：LLM 密钥的 team_id / org_id 均软引用该值。
func GetDeptId(c *gin.Context) string {
	if u := GetUser(c); u != nil {
		return u.DeptId
	}
	return ""
}

// GetIdentity 当前登录用户的权限身份（组合授权判定主体）。
// 角色维每次经 casbin enforcer 实时解析（内存 g 表，免查库且角色变更即时生效）；
// 部门/岗位/工号维来自登录 claims（登录时新鲜解析，换部门/岗位后重新登录生效）；
// 分组/集合维经 grantmeta 快照实时解析（改分组内容即时生效）。
func GetIdentity(c *gin.Context) model.Identity {
	u := GetUser(c)
	if u == nil {
		return model.Identity{}
	}
	t := model.Identity{
		UserId:     u.Id,
		DeptId:     u.DeptId,
		PostIds:    u.PostIds,
		IsAdmin:    u.IsAdmin == 1,
		EmployeeNo: u.EmployeeNo,
	}
	if t.UserId != "" {
		if e, err := auth.Enforcer(); err == nil {
			if roles, rerr := e.GetImplicitRolesForUser(consts.CasBinUserPrefix + t.UserId); rerr == nil {
				t.RoleIds = roles
			}
		}
	}
	grantmeta.ResolveIdentity(&t)
	if t.RoleIds == nil {
		t.RoleIds = []string{}
	}
	if t.PostIds == nil {
		t.PostIds = []string{}
	}
	return t
}

// GetIdentityPtr 同 GetIdentity，指针形态（service 层 nil=系统路径全放行的约定参数）。
// Web 控制器经 JWTAuth 后必有用户，正常恒非-nil。
func GetIdentityPtr(c *gin.Context) *model.Identity {
	t := GetIdentity(c)
	return &t
}
