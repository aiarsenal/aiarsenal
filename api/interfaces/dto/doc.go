package dto

import "aiarsenal/domain/types"

// DocSearchReq 文档列表查询
type DocSearchReq struct {
	KeyWords string `form:"keyWords"`
	FileType string `form:"fileType"`
	DirId    string `form:"dirId"` // ''=全部；选中目录=含子树
	PageNum  int    `form:"pageNum"`
	PageSize int    `form:"pageSize"`
}

// DocListRes 文档列表
type DocListRes struct {
	List interface{} `json:"list"`
	ListRes
}

// DocEditReq 编辑标题/摘要（编辑后自动重嵌）
type DocEditReq struct {
	DocId   string `json:"docId" binding:"required"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// DocDeleteReq 删除文档
type DocDeleteReq struct {
	Ids []string `json:"ids" binding:"required"`
}

// ---- 目录管理 ----

// DocDirNode 目录树节点（service 组装，含计数）
type DocDirNode struct {
	DirId       string        `json:"dirId"`
	Name        string        `json:"name"`
	ParentId    string        `json:"parentId"`
	Visibility  uint          `json:"visibility"` // 0私有 1按条件 2全员（徽标）
	CanWrite    bool          `json:"canWrite"`   // 当前身份可写（改名/删除/配授权按钮）
	Kind        string        `json:"kind"`       // ''=普通 project_space/personal_space=固定根 project/personal=系统托管
	RefId       string        `json:"refId"`      // kind=project→projectId；kind=personal→userId
	DocCount    int           `json:"docCount"`   // 直属文档数
	TotalCount  int           `json:"totalCount"` // 子树文档总数（含直属）
	CreatorName string        `json:"creatorName"`
	CreatedAt   *types.Time   `json:"createdAt"`
	Children    []*DocDirNode `json:"children"`
}

// DocDirCreateReq 新建目录（parentId ”=根）。visibility/readRules/writeRules = 创建即配授权。
type DocDirCreateReq struct {
	ParentId   string `json:"parentId"`
	Name       string `json:"name" binding:"required"`
	Visibility uint   `json:"visibility"`                   // 0私有 1按条件 2全员（缺省 0 按零值=私有，前端必传）
	ReadRules  string `json:"readRules" form:"readRules"`   // GrantRow[] JSON
	WriteRules string `json:"writeRules" form:"writeRules"` // GrantRow[] JSON
}

// DocDirRenameReq 目录重命名
type DocDirRenameReq struct {
	DirId string `json:"dirId" binding:"required"`
	Name  string `json:"name" binding:"required"`
}

// DocDirMoveReq 移动目录（parentId ”=根；不得移入自身子树）
type DocDirMoveReq struct {
	DirId    string `json:"dirId" binding:"required"`
	ParentId string `json:"parentId"`
}

// DocDirDeleteReq 删除目录（须空目录）
type DocDirDeleteReq struct {
	DirId string `json:"dirId" binding:"required"`
}

// ---- 文档目录操作 / 重嵌 / 访问记录 ----

// DocMoveReq 批量移动文档（dirId ”=移回根）
type DocMoveReq struct {
	DocIds []string `json:"docIds" binding:"required"`
	DirId  string   `json:"dirId"`
}

// DocEmbedReq 重新嵌入：docIds 指定文档 / dirId 目录（含子树）
type DocEmbedReq struct {
	DocIds []string `json:"docIds"`
	DirId  string   `json:"dirId"`
}

// DocAccessReq 访问记录查询
type DocAccessReq struct {
	Action   string `form:"action"` // ''=全部 search|read
	DocId    string `form:"docId"`  // ''=全部
	PageNum  int    `form:"pageNum"`
	PageSize int    `form:"pageSize"`
}

// ---- 组合授权（文档/目录共用；读写分离） ----

// DocGrantGetReq 查询目标的授权配置（回显与保存同门槛：超管或目标 canWrite）
type DocGrantGetReq struct {
	TargetType string `form:"targetType" binding:"required"` // doc | dir
	TargetId   string `form:"targetId" binding:"required"`
}

// DocGrantSaveReq 保存目标的可见性档位 + 读/写条件行（整体重写）
type DocGrantSaveReq struct {
	TargetType string `json:"targetType" binding:"required"` // doc | dir
	TargetId   string `json:"targetId" binding:"required"`
	Visibility uint   `json:"visibility"` // 0私有 1按条件 2全员
	ReadRules  string `json:"readRules"`  // GrantRow[] JSON（''=空行集）
	WriteRules string `json:"writeRules"` // GrantRow[] JSON
}
