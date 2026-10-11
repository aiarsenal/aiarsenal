package service

import (
	"context"
	"math"
	"time"

	"aiarsenal/infrastructure/db"
)

// StatsSkillService 技能数据看板（前端 /stats/skill 页面）：只读聚合，无写操作。
// 口径涉及三个库：技能/版本/审计 = db.SkillDB()（aa_skill 库），
// 实例安装关联 = db.AgentDB()（aa_agent 库，跨库不可 join，分查），来源无跨库引用。

type StatsSkillService struct{}

func NewStatsSkillService() *StatsSkillService { return &StatsSkillService{} }

// StatsSkillSecurity 安全扫描卡。
// 口径：按版本取「最新一次审计」（同版本多次重扫取 id 最大者，同 SkillSecurityRepository
// LatestAuditsByVersions 的后写覆盖语义），避免历史重扫记录混入计数。
type StatsSkillSecurity struct {
	// Scanned 已有审计结论（安全/不安全）的版本数。
	Scanned int64 `json:"scanned"`
	// Passed 审计结论=安全（entity.AuditStatusPassed=1）。
	Passed int64 `json:"passed"`
	// Failed 审计结论=不安全（entity.AuditStatusFailed=2）。
	Failed int64 `json:"failed"`
	// Scanning 扫描中（entity.AuditStatusScanning=3）。
	Scanning int64 `json:"scanning"`
	// PassRate 通过率（%，passed/scanned，保留 1 位小数；无结论恒 0）。
	PassRate float64 `json:"passRate"`
}

// StatsSkillSource 来源分布行（饼图）。
type StatsSkillSource struct {
	// Source 来源标识：local（本地创建，即 aa_skill.source 为空串）/ 市场标识（xfyun 等）。
	Source string `json:"source"`
	// Label 展示名（后端富化：本地创建 / 市场:<市场标识>）。
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// StatsSkillOverview GET /stats/skill/overview 响应。
type StatsSkillOverview struct {
	TotalSkills int64 `json:"totalSkills"`
	// PublishedSkills 存在已发布版本（aa_skill_version.status=1）的技能数（去重口径，不限最新版本）。
	PublishedSkills int64 `json:"publishedSkills"`
	// DraftSkills 未发布过任何版本的技能数 = totalSkills - publishedSkills（含 0 版本技能）。
	DraftSkills int64 `json:"draftSkills"`
	// PendingReview 待出审计结论的版本数（最新审计 ∈ 待扫描0/扫描中3）。
	PendingReview int64 `json:"pendingReview"`
	// TotalVersions 在用技能的版本总数（不含已删技能的版本）。
	TotalVersions int64 `json:"totalVersions"`
	// InstalledCount 实例侧当前安装关系总数（aa_agent_instance_skill 行数）。
	InstalledCount int64              `json:"installedCount"`
	Security       StatsSkillSecurity `json:"security"`
	Sources        []StatsSkillSource `json:"sources"`
}

// StatsSkillInstallTrendPoint GET /stats/skill/install-trend 单日聚合点。
type StatsSkillInstallTrendPoint struct {
	Date string `json:"date"`
	// Installs 当日安装次数（按 aa_agent_instance_skill.created_at 分组；升级不新增行、不重复计）。
	Installs int64 `json:"installs"`
	// Uninstalls 当日卸载次数。卸载随 uninstall_skill done 物理删行（AgentRuntimeService
	// applyCommandSideEffect → DeleteByInstanceSkill），无卸载时间列、无法按日归集，恒 0。
	Uninstalls int64 `json:"uninstalls"`
}

// Overview 技能计数卡 + 安全扫描卡 + 来源分布。
func (s *StatsSkillService) Overview(ctx context.Context) (*StatsSkillOverview, error) {
	g := db.SkillDB().WithContext(ctx)
	res := &StatsSkillOverview{}

	// 技能总数（软删除外）。
	if err := g.Table("aa_skill").
		Where("deleted_at is null").
		Count(&res.TotalSkills).Error; err != nil {
		return nil, err
	}

	// 已发布 / 版本总数：join 在用技能，避免已删技能的版本混入。
	var published, versions int64
	if err := g.Table("aa_skill_version v").
		Joins("join aa_skill s on s.skill_id = v.skill_id and s.deleted_at is null").
		Where("v.status = 1").
		Count(&published).Error; err != nil {
		return nil, err
	}
	res.PublishedSkills = published
	if err := g.Table("aa_skill_version v").
		Joins("join aa_skill s on s.skill_id = v.skill_id and s.deleted_at is null").
		Count(&versions).Error; err != nil {
		return nil, err
	}
	res.TotalVersions = versions
	res.DraftSkills = res.TotalSkills - res.PublishedSkills

	// 安全审计：按版本取最新一次审计的状态分布（窗口函数 ROW_NUMBER），
	// 一次查询同时喂 pendingReview 与 security 卡。
	var auditRows []struct {
		Status int64 `gorm:"column:status"`
		Cnt    int64 `gorm:"column:cnt"`
	}
	err := g.Raw(`
SELECT t.status, COUNT(*) AS cnt FROM (
  SELECT status, ROW_NUMBER() OVER (PARTITION BY skill_version_id ORDER BY id DESC) AS rn
  FROM aa_skill_security_audit
) t WHERE t.rn = 1 GROUP BY t.status`).Scan(&auditRows).Error
	if err != nil {
		return nil, err
	}
	auditCnt := map[int64]int64{}
	for _, r := range auditRows {
		auditCnt[r.Status] = r.Cnt
	}
	passed := auditCnt[1]                      // AuditStatusPassed
	failed := auditCnt[2]                      // AuditStatusFailed
	scanning := auditCnt[3]                    // AuditStatusScanning
	res.PendingReview = auditCnt[0] + scanning // AuditStatusPending + Scanning
	res.Security = StatsSkillSecurity{
		Scanned:  passed + failed,
		Passed:   passed,
		Failed:   failed,
		Scanning: scanning,
		PassRate: passRatePct(passed, passed+failed),
	}

	// 来源分布：aa_skill.source 空串=本地上传 → 归一为 local；市场标识富化为「市场:<标识>」。
	var srcRows []struct {
		Source string `gorm:"column:source"`
		Count  int64  `gorm:"column:count"`
	}
	if err := g.Raw(`
SELECT COALESCE(NULLIF(source, ''), 'local') AS source, COUNT(*) AS count
FROM aa_skill WHERE deleted_at is null
GROUP BY 1 ORDER BY count DESC`).Scan(&srcRows).Error; err != nil {
		return nil, err
	}
	res.Sources = make([]StatsSkillSource, 0, len(srcRows))
	for _, r := range srcRows {
		label := "本地创建"
		if r.Source != "local" {
			label = "市场:" + r.Source
		}
		res.Sources = append(res.Sources, StatsSkillSource{Source: r.Source, Label: label, Count: r.Count})
	}

	// 实例安装总量：agent 库（跨库不可 join，单独查）。
	if err := db.AgentDB().WithContext(ctx).
		Table("aa_agent_instance_skill").
		Count(&res.InstalledCount).Error; err != nil {
		return nil, err
	}
	return res, nil
}

// InstallTrend 近 N 天安装趋势（N 上限 90，默认 30；含今天，空天补零）。
// 卸载恒 0：见 StatsSkillInstallTrendPoint.Uninstalls 注释。
func (s *StatsSkillService) InstallTrend(ctx context.Context, days int) ([]StatsSkillInstallTrendPoint, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}
	now := time.Now()
	today0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := today0.AddDate(0, 0, -(days - 1))

	var rows []struct {
		Day      string `gorm:"column:day"`
		Installs int64  `gorm:"column:installs"`
	}
	err := db.AgentDB().WithContext(ctx).Raw(`
SELECT to_char(created_at, 'YYYY-MM-DD') AS day, COUNT(*) AS installs
FROM aa_agent_instance_skill
WHERE created_at >= ? AND created_at < ?
GROUP BY day`, from, today0.AddDate(0, 0, 1)).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]int64, len(rows))
	for _, r := range rows {
		byDay[r.Day] = r.Installs
	}
	points := make([]StatsSkillInstallTrendPoint, 0, days)
	for i := 0; i < days; i++ {
		day := from.AddDate(0, 0, i).Format("2006-01-02")
		points = append(points, StatsSkillInstallTrendPoint{Date: day, Installs: byDay[day]})
	}
	return points, nil
}

// passRatePct 通过率（%）：保留 1 位小数；无结论（分母 0）恒 0。
func passRatePct(passed, scanned int64) float64 {
	if scanned <= 0 {
		return 0
	}
	return math.Round(float64(passed)/float64(scanned)*1000) / 10
}
