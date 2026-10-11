package dto

// PageReq 公共分页请求参数（对应旧 common.PageReq）
type PageReq struct {
	DateRange []string `json:"dateRange" form:"dateRange"`
	PageNum   int      `json:"pageNum" form:"pageNum"`
	PageSize  int      `json:"pageSize" form:"pageSize"`
}

// ListRes 公共列表返回（对应旧 common.ListRes）
type ListRes struct {
	CurrentPage int         `json:"currentPage"`
	Total       interface{} `json:"total"`
}
