package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/interfaces/middleware"
)

// StatsController 数据看板（前端 /stats 页面）：LLM 用量看板 + 技能资产看板。
// 全部只读聚合；参数容错（缺省/越界）由 service 层收口。

type StatsController struct {
	llm   *service.StatsLlmService
	skill *service.StatsSkillService
}

func NewStatsController(l *service.StatsLlmService, s *service.StatsSkillService) *StatsController {
	return &StatsController{llm: l, skill: s}
}

// ===== LLM 看板 =====

// LlmOverview GET /stats/llm/overview：今日 + 近 7 天用量 + 凭证健康卡。
func (h *StatsController) LlmOverview(c *gin.Context) {
	res, err := h.llm.Overview(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, res)
}

// LlmTrend GET /stats/llm/trend?days=7：按日用量趋势。
func (h *StatsController) LlmTrend(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	points, err := h.llm.Trend(c, days)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"points": points})
}

// LlmTop GET /stats/llm/top?dimension=model|virtualKey|user&days=7&limit=10：用量 Top 榜。
func (h *StatsController) LlmTop(c *gin.Context) {
	dimension := c.Query("dimension")
	switch dimension {
	case "model", "virtualKey", "user":
	default:
		middleware.Fail(c, "参数错误")
		return
	}
	days, _ := strconv.Atoi(c.Query("days"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	list, err := h.llm.Top(c, dimension, days, limit)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"list": list})
}

// ===== 技能看板 =====

// SkillOverview GET /stats/skill/overview：技能计数 + 安全扫描 + 来源分布。
func (h *StatsController) SkillOverview(c *gin.Context) {
	res, err := h.skill.Overview(c)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, res)
}

// SkillInstallTrend GET /stats/skill/install-trend?days=30：按日安装趋势。
func (h *StatsController) SkillInstallTrend(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	points, err := h.skill.InstallTrend(c, days)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	middleware.Success(c, gin.H{"points": points})
}
