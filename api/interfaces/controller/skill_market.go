package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/middleware"
)

// SkillMarketController 第三方 skill 市场：链接导入 + 外部市场目录 CRUD
// （站内搜索/一键同步已随「讯飞」页下线；适配器与 service 层保留，供链接导入解析与 MCP 工具使用）。
type SkillMarketController struct {
	svc     *service.SkillMarketService
	siteSvc *service.SkillMarketSiteService
}

func NewSkillMarketController(svc *service.SkillMarketService, siteSvc *service.SkillMarketSiteService) *SkillMarketController {
	return &SkillMarketController{svc: svc, siteSvc: siteSvc}
}

// Import 链接导入：粘贴第三方市场 skill 的 zip 直链或详情页链接，保存到本地技能库。
// POST /api/v1/skill/market/import  JSON: {url,namespaceId,upstreamAuthor,tags}
func (h *SkillMarketController) Import(c *gin.Context) {
	var req service.MarketImportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	res, err := h.svc.ImportURL(c, &req, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, res)
}

// Sources 可用市场源列表（搜索页来源选择器）。
// GET /api/v1/skill/market/sources
func (h *SkillMarketController) Sources(c *gin.Context) {
	middleware.Success(c, gin.H{"list": h.svc.ListSources()})
}

// Search 站内搜索市场 skill（经适配器透传上游搜索接口，带限流与缓存）。
// GET /api/v1/skill/market/search?source=xfyun&keyword=xx&page=1&limit=12
func (h *SkillMarketController) Search(c *gin.Context) {
	source := c.Query("source")
	keyword := c.Query("keyword")
	if source == "" || keyword == "" {
		middleware.Fail(c, "source/keyword 不能为空")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "12"))
	items, total, err := h.svc.Search(c, source, keyword, page, limit)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": items, "total": total})
}

// Sync 一键同步：从市场下载技能包并落地为本地技能（DRAFT，待审核发布）。
// POST /api/v1/skill/market/sync  JSON: {source,slug,version,namespaceId}
func (h *SkillMarketController) Sync(c *gin.Context) {
	var req service.MarketSyncReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	res, err := h.svc.Sync(c, &req, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, res)
}

// ===== 外部市场目录（aa_skill_market_site）=====

// SiteList 目录管理分页列表。
// GET /api/v1/skill/market/site/list?keyword=&status=&pageNum=&pageSize=
func (h *SkillMarketController) SiteList(c *gin.Context) {
	var req service.MarketSiteListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.siteSvc.List(c, &req)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list, "total": total})
}

// SiteAll 启用中的目录全量（others 页卡片展示用，不分页）。
// GET /api/v1/skill/market/site/all
func (h *SkillMarketController) SiteAll(c *gin.Context) {
	list, err := h.siteSvc.All(c)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// SiteAdd 新增目录项。
// POST /api/v1/skill/market/site/add  JSON: {name,siteUrl,description,icon,logoUrl,themeColor,status,sort}
func (h *SkillMarketController) SiteAdd(c *gin.Context) {
	var req service.MarketSiteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	marketId, err := h.siteSvc.Add(c, &req, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"marketId": marketId})
}

// SiteEdit 编辑目录项。
// POST /api/v1/skill/market/site/edit  JSON: {marketId,name,siteUrl,...}
func (h *SkillMarketController) SiteEdit(c *gin.Context) {
	var req service.MarketSiteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.siteSvc.Edit(c, &req, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}

// SiteDelete 删除目录项。
// POST /api/v1/skill/market/site/delete  JSON: {marketId}
func (h *SkillMarketController) SiteDelete(c *gin.Context) {
	var req service.MarketSiteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.siteSvc.Delete(c, req.MarketId); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{})
}
