package dto

import "aiarsenal/domain/types"

// KbSearchReq 文档列表查询
type KbSearchReq struct {
	KeyWords string `form:"keyWords"`
	FileType string `form:"fileType"`
	DirId    string `form:"dirId"` // ''=全部；选中目录=含子树
	PageNum  int    `form:"pageNum"`
	PageSize int    `form:"pageSize"`
}

// KbListRes 文档列表
type KbListRes struct {
	List interface{} `json:"list"`
	ListRes
}

// KbEditReq 编辑标题/摘要（编辑后自动重嵌）
type KbEditReq struct {
	DocId   string `json:"docId" binding:"required"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// KbDeleteReq 删除文档
type KbDeleteReq struct {
	Ids []string `json:"ids" binding:"required"`
}

// ---- 目录管理 ----

// KbDirNode 目录树节点（service 组装，含计数）
type KbDirNode struct {
	DirId       string       `json:"dirId"`
	Name        string       `json:"name"`
	ParentId    string       `json:"parentId"`
	Visibility  uint         `json:"visibility"` // 0私有 1按条件 2全员（徽标）
	CanWrite    bool         `json:"canWrite"`   // 当前身份可写（改名/删除/配授权按钮）
	DocCount    int          `json:"docCount"`   // 直属文档数
	TotalCount  int          `json:"totalCount"` // 子树文档总数（含直属）
	CreatorName string       `json:"creatorName"`
	CreatedAt   *types.Time  `json:"createdAt"`
	Children    []*KbDirNode `json:"children"`
}

// KbDirCreateReq 新建目录（parentId ”=根）。visibility/readRules/writeRules = 创建即配授权。
type KbDirCreateReq struct {
	ParentId   string `json:"parentId"`
	Name       string `json:"name" binding:"required"`
	Visibility uint   `json:"visibility"`                   // 0私有 1按条件 2全员（缺省 0 按零值=私有，前端必传）
	ReadRules  string `json:"readRules" form:"readRules"`   // GrantRow[] JSON
	WriteRules string `json:"writeRules" form:"writeRules"` // GrantRow[] JSON
}

// KbDirRenameReq 目录重命名
type KbDirRenameReq struct {
	DirId string `json:"dirId" binding:"required"`
	Name  string `json:"name" binding:"required"`
}

// KbDirMoveReq 移动目录（parentId ”=根；不得移入自身子树）
type KbDirMoveReq struct {
	DirId    string `json:"dirId" binding:"required"`
	ParentId string `json:"parentId"`
}

// KbDirDeleteReq 删除目录（须空目录）
type KbDirDeleteReq struct {
	DirId string `json:"dirId" binding:"required"`
}

// ---- 文档目录操作 / 重嵌 / 访问记录 ----

// KbMoveReq 批量移动文档（dirId ”=移回根）
type KbMoveReq struct {
	DocIds []string `json:"docIds" binding:"required"`
	DirId  string   `json:"dirId"`
}

// KbEmbedReq 重新嵌入：docIds 指定文档 / dirId 目录（含子树）
type KbEmbedReq struct {
	DocIds []string `json:"docIds"`
	DirId  string   `json:"dirId"`
}

// KbAccessReq 访问记录查询
type KbAccessReq struct {
	Action   string `form:"action"` // ''=全部 search|read
	DocId    string `form:"docId"`  // ''=全部
	PageNum  int    `form:"pageNum"`
	PageSize int    `form:"pageSize"`
}

// ---- 组合授权（文档/目录共用；读写分离） ----

// KbGrantGetReq 查询目标的授权配置（回显与保存同门槛：超管或目标 canWrite）
type KbGrantGetReq struct {
	TargetType string `form:"targetType" binding:"required"` // doc | dir
	TargetId   string `form:"targetId" binding:"required"`
}

// KbGrantSaveReq 保存目标的可见性档位 + 读/写条件行（整体重写）
type KbGrantSaveReq struct {
	TargetType string `json:"targetType" binding:"required"` // doc | dir
	TargetId   string `json:"targetId" binding:"required"`
	Visibility uint   `json:"visibility"` // 0私有 1按条件 2全员
	ReadRules  string `json:"readRules"`  // GrantRow[] JSON（''=空行集）
	WriteRules string `json:"writeRules"` // GrantRow[] JSON
}
