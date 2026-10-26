package model

import "aiarsenal/domain/types"

// LoginUserRes 登录返回的用户信息（aa_user 的投影，对应旧 model.LoginUserRes）
type LoginUserRes struct {
	Id           string   `json:"id"`
	UserName     string   `json:"userName"`
	EmployeeNo   string   `json:"employeeNo"`
	Name         string   `json:"name"`
	UserNickname string   `json:"userNickname"`
	UserPassword string   `json:"userPassword"`
	UserSalt     string   `json:"userSalt"`
	UserStatus   uint     `json:"userStatus"`
	IsAdmin      int      `json:"isAdmin"`
	Avatar       string   `json:"avatar"`
	DeptId       string   `json:"deptId"`
	PostIds      []string `json:"postIds,omitempty"` // 权限五元组-岗位（来自登录 claims）
	// PasswordChangedAt 供登录口令过期判定（rotationDays>0 时）
	PasswordChangedAt *types.Time `json:"-"`
	// TOTPEnabled/TOTPSecret 登录时判定是否需要动态码并验码（secret 响应前必须清空）
	TOTPEnabled uint        `json:"-"`
	TOTPSecret  string      `json:"-"`
	DeviceId          string      `json:"deviceId,omitempty"` // 桌面 App 登录携带；web 端为空
}

// MenuInfoRes 菜单/规则投影（对应旧 model.MenuInfoRes）
type MenuInfoRes struct {
	Id        string `json:"id"`
	Pid       string `json:"pid"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	EnName    string `json:"enName"`
	Icon      string `json:"icon"`
	Condition string `json:"condition"`
	Remark    string `json:"remark"`
	MenuType  uint   `json:"menuType"`
	Weigh     int    `json:"weigh"`
	IsHide    uint   `json:"isHide"`
	IsCached  uint   `json:"isCached"`
	IsAffix   uint   `json:"isAffix"`
	Path      string `json:"path"`
	Redirect  string `json:"redirect"`
	Component string `json:"component"`
	IsIframe  uint   `json:"isIframe"`
	IsLink    uint   `json:"isLink"`
	LinkUrl   string `json:"linkUrl"`
}

// MenuMeta 前端菜单 meta
type MenuMeta struct {
	Icon        string `json:"icon"`
	Title       string `json:"title"`
	EnName      string `json:"enName"`
	IsLink      string `json:"isLink"`
	IsHide      bool   `json:"isHide"`
	IsKeepAlive bool   `json:"isKeepAlive"`
	IsAffix     bool   `json:"isAffix"`
	IsIframe    bool   `json:"isIframe"`
}

// UserMenu 前端菜单项（*MenuMeta 匿名嵌入，json 输出 meta，且可直接 m.MenuMeta 访问）
type UserMenu struct {
	Id        string `json:"id"`
	Pid       string `json:"pid"`
	Name      string `json:"name"`
	Component string `json:"component"`
	Path      string `json:"path"`
	*MenuMeta `json:"meta"`
}

// UserMenus 前端菜单树
type UserMenus struct {
	*UserMenu
	Children []*UserMenus `json:"children"`
}

// LoginLogParams 登录日志写入参数
type LoginLogParams struct {
	Status    int
	Username  string
	Ip        string
	UserAgent string
	Msg       string
	Module    string
}

// UserOnlineParams 用户在线状态写入参数
type UserOnlineParams struct {
	UserAgent string
	Uuid      string
	Token     string
	Username  string
	Ip        string
}

// ContextUser 上下文中的当前用户
type ContextUser struct {
	*LoginUserRes
}

// Context 请求上下文（当前登录用户）
type Context struct {
	User *ContextUser
}

// ===== 部门 =====

// MenuTreeRes 菜单树（管理页用，嵌套 children）
type MenuTreeRes struct {
	*MenuInfoRes
	Children []*MenuTreeRes `json:"children"`
}

// DeptTreeNode 部门树节点
type DeptTreeNode struct {
	DeptId   string          `json:"deptId"`
	ParentId string          `json:"parentId"`
	DeptName string          `json:"deptName"`
	Children []*DeptTreeNode `json:"children"`
}

// DeptTreeRes 部门树
type DeptTreeRes struct {
	*DeptInfo
	Children []*DeptTreeRes `json:"children"`
}

// DeptInfo 部门投影（列表/树通用）。LeaderName/LeaderEmployeeNo 由 leader_id 关联 aa_user 带出。
type DeptInfo struct {
	DeptId           string `json:"deptId"`
	ParentId         string `json:"parentId"`
	DeptName         string `json:"deptName"`
	OrderNum         int    `json:"orderNum"`
	LeaderId         string `json:"leaderId"`
	LeaderName       string `json:"leaderName"`
	LeaderEmployeeNo string `json:"leaderEmployeeNo"`
	Status           uint   `json:"status"`
	CreatedAt        string `json:"createdAt"`
}

// UserOptionRes 用户精简投影（人员选择器 / 负责人搜索）。
// 搜索条件：name / employee_no / mobile / user_email 四字段 OR。
type UserOptionRes struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	EmployeeNo string `json:"employeeNo"`
	Mobile     string `json:"mobile"`
	UserEmail  string `json:"userEmail"`
	DeptId     string `json:"deptId"`
	DeptName   string `json:"deptName"` // 部门完整层级路径「根 / … / 末级」（repo 回填；无部门空串）
}

// ===== 字典 =====

// DictTypeRes 字典类型简（前端 getDictData 用）
type DictTypeRes struct {
	DictName string `json:"name"`
	Remark   string `json:"remark"`
}

// DictDataRes 字典数据简（前端 getDictData 用）
type DictDataRes struct {
	DictValue string `json:"key"`
	DictLabel string `json:"value"`
	IsDefault int    `json:"isDefault"`
	Remark    string `json:"remark"`
}

// ===== 操作日志 =====

// OperLogListRes 操作日志列表项
type OperLogListRes struct {
	OperId        string `json:"operId"`
	Title         string `json:"title"`
	RequestMethod string `json:"requestMethod"`
	OperName      string `json:"operName"`
	DeptName      string `json:"deptName"`
	OperUrl       string `json:"operUrl"`
	OperIp        string `json:"operIp"`
	OperLocation  string `json:"operLocation"`
	OperParam     string `json:"operParam"`
	OperTime      string `json:"operTime"`
}

// OperLogInfoRes 操作日志详情（含 BusinessType/Method/OperatorType/ErrorMsg）
type OperLogInfoRes struct {
	OperId        string `json:"operId"`
	Title         string `json:"title"`
	BusinessType  int    `json:"businessType"`
	Method        string `json:"method"`
	RequestMethod string `json:"requestMethod"`
	OperatorType  int    `json:"operatorType"`
	OperName      string `json:"operName"`
	DeptName      string `json:"deptName"`
	OperUrl       string `json:"operUrl"`
	OperIp        string `json:"operIp"`
	OperLocation  string `json:"operLocation"`
	OperParam     string `json:"operParam"`
	ErrorMsg      string `json:"errorMsg"`
	OperTime      string `json:"operTime"`
}

// ===== 用户关联 =====

// UserRoleInfoRes 用户角色简
type UserRoleInfoRes struct {
	RoleId string `json:"roleId"`
	Name   string `json:"name"`
}

// UserPostInfoRes 用户岗位简
type UserPostInfoRes struct {
	PostId   string `json:"postId"`
	PostName string `json:"postName"`
}

// UserSimpleRes 用户简（多用户查询）
type UserSimpleRes struct {
	Id           string `json:"id"`
	Avatar       string `json:"avatar"`
	Sex          int    `json:"sex"`
	UserName     string `json:"userName"`
	UserNickname string `json:"userNickname"`
	Name         string `json:"name"`
	EmployeeNo   string `json:"employeeNo"`
}

// MemberInfoRes 员工集合成员明细（工号 → 姓名/部门；解析不到的在册外工号由前端标注）
type MemberInfoRes struct {
	EmployeeNo string `json:"employeeNo"`
	Name       string `json:"name"`
	DeptId     string `json:"deptId"`
	DeptName   string `json:"deptName"` // 部门完整层级路径「根 / … / 末级」（repo 回填）
}
