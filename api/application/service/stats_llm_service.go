package service

import (
	"context"
	"errors"
	"math"
	"time"

	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/db"
)

// StatsLlmService LLM 数据看板（前端 /stats/llm 页面）：只读聚合，无写操作。
// 数据源 = aa_llm_spend_log（请求/token/花费，含失败请求）+ aa_llm_health_check（健康探测）；
// 聚合 SQL 直接走 db.LLMDB()（不建仓储），用户昵称富化跨库查主库 aa_user 回填。
// 时间口径：start_time 落的是服务本地墙钟（DSN 会话 TimeZone=Asia/Shanghai，见 config.tzParam），
// 故「今天 0 点」与按天分组均用服务本地时区。

type StatsLlmService struct{}

func NewStatsLlmService() *StatsLlmService { return &StatsLlmService{} }

// StatsLlmUsageSummary 一个时间窗的用量汇总（今日 / 近 N 天 / 趋势点共用形状）。
type StatsLlmUsageSummary struct {
	Requests         int64       `json:"requests"`
	PromptTokens     int64       `json:"promptTokens"`
	CompletionTokens int64       `json:"completionTokens"`
	TotalTokens      int64       `json:"totalTokens"`
	Spend            types.Milli `json:"spend"`
	ErrorCount       int64       `json:"errorCount"`
	// ErrorRate 错误率（%，0-100，保留 2 位小数）。
	ErrorRate float64 `json:"errorRate"`
}

// StatsLlmHealth 凭证健康卡（来自健康检查表）。
// 口径：网关 prober 以 health_check_id=model_id 对每个模型部署滚动 Upsert 一行
// （llm/internal/store UpsertHealthCheck），故每行=一个部署的最新探测结果：
// 总数=行数、健康/异常=按最近 status 计数、时延=各行 response_time_ms 平均。
type StatsLlmHealth struct {
	TotalCredentials     int64 `json:"totalCredentials"`
	HealthyCredentials   int64 `json:"healthyCredentials"`
	UnhealthyCredentials int64 `json:"unhealthyCredentials"`
	// AvgLatencyMs 平均响应时延（ms，四舍五入取整）。
	AvgLatencyMs int64       `json:"avgLatencyMs"`
	CheckedAt    *types.Time `json:"checkedAt,omitempty"`
}

// StatsLlmOverview GET /stats/llm/overview 响应。
type StatsLlmOverview struct {
	Today     StatsLlmUsageSummary `json:"today"`
	Last7Days StatsLlmUsageSummary `json:"last7Days"`
	Health    StatsLlmHealth       `json:"health"`
}

// StatsLlmTrendPoint GET /stats/llm/trend 单日聚合点（内嵌 UsageSummary 摊平输出）。
type StatsLlmTrendPoint struct {
	Date string `json:"date"`
	StatsLlmUsageSummary
}

// StatsLlmTopItem GET /stats/llm/top 单行。
type StatsLlmTopItem struct {
	// Key 维度值：模型 id / 虚拟 Key token_hash（密钥公开标识）/ 用户 id。
	Key string `json:"key"`
	// Name 展示名：模型名 / 虚拟 Key 别名 / 用户昵称（后端富化）。
	Name        string      `json:"name"`
	Requests    int64       `json:"requests"`
	TotalTokens int64       `json:"totalTokens"`
	Spend       types.Milli `json:"spend"`
	ErrorCount  int64       `json:"errorCount"`
	ErrorRate   float64     `json:"errorRate"`
}

// spendAggRow 时间窗聚合扫描行（spend 为 Milli 整数，::bigint 收敛 SUM 的 numeric）。
type spendAggRow struct {
	Requests         int64       `gorm:"column:requests"`
	PromptTokens     int64       `gorm:"column:prompt_tokens"`
	CompletionTokens int64       `gorm:"column:completion_tokens"`
	TotalTokens      int64       `gorm:"column:total_tokens"`
	Spend            types.Milli `gorm:"column:spend"`
	ErrorCount       int64       `gorm:"column:error_count"`
}

// spendDayAggRow 按日聚合扫描行（多一列 day）。
type spendDayAggRow struct {
	Day              string      `gorm:"column:day"`
	Requests         int64       `gorm:"column:requests"`
	PromptTokens     int64       `gorm:"column:prompt_tokens"`
	CompletionTokens int64       `gorm:"column:completion_tokens"`
	TotalTokens      int64       `gorm:"column:total_tokens"`
	Spend            types.Milli `gorm:"column:spend"`
	ErrorCount       int64       `gorm:"column:error_count"`
}

// spendAggSQL 窗口聚合 SQL：错误口径 = status <> 'success'
// （网关只落 success/failed 两种，见 llm/internal/proxy 的 bill 调用点）。
const spendAggSQL = `
SELECT COUNT(*) AS requests,
  COALESCE(SUM(prompt_tokens), 0)::bigint AS prompt_tokens,
  COALESCE(SUM(completion_tokens), 0)::bigint AS completion_tokens,
  COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(spend), 0)::bigint AS spend,
  COALESCE(SUM(CASE WHEN status <> 'success' THEN 1 ELSE 0 END), 0)::bigint AS error_count
FROM aa_llm_spend_log
WHERE start_time >= ? AND start_time < ?`

// Overview 今日 + 近 7 天（含今天）用量汇总 + 凭证健康卡。
func (s *StatsLlmService) Overview(ctx context.Context) (*StatsLlmOverview, error) {
	now := time.Now()
	today0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	g := db.LLMDB().WithContext(ctx)

	res := &StatsLlmOverview{}
	// 今日：本地今天 0 点起。
	var today spendAggRow
	if err := g.Raw(spendAggSQL, today0, today0.AddDate(0, 0, 1)).Scan(&today).Error; err != nil {
		return nil, err
	}
	res.Today = toUsageSummary(today)
	// 近 7 天：含今天共 7 个自然日。
	var last7 spendAggRow
	if err := g.Raw(spendAggSQL, today0.AddDate(0, 0, -6), today0.AddDate(0, 0, 1)).Scan(&last7).Error; err != nil {
		return nil, err
	}
	res.Last7Days = toUsageSummary(last7)

	// 健康卡：健康检查表全行滚动态（见 StatsLlmHealth 注释）。
	var h struct {
		Total       int64       `gorm:"column:total"`
		Healthy     int64       `gorm:"column:healthy"`
		Unhealthy   int64       `gorm:"column:unhealthy"`
		AvgLatency  *float64    `gorm:"column:avg_latency"`
		LastChecked *types.Time `gorm:"column:last_checked"`
	}
	err := g.Raw(`
SELECT COUNT(*) AS total,
  COALESCE(SUM(CASE WHEN status = 'healthy' THEN 1 ELSE 0 END), 0) AS healthy,
  COALESCE(SUM(CASE WHEN status = 'unhealthy' THEN 1 ELSE 0 END), 0) AS unhealthy,
  AVG(response_time_ms) AS avg_latency,
  MAX(checked_at) AS last_checked
FROM aa_llm_health_check`).Scan(&h).Error
	if err != nil {
		return nil, err
	}
	res.Health = StatsLlmHealth{
		TotalCredentials:     h.Total,
		HealthyCredentials:   h.Healthy,
		UnhealthyCredentials: h.Unhealthy,
		AvgLatencyMs:         roundFloatToInt(h.AvgLatency),
		CheckedAt:            h.LastChecked,
	}
	if res.Health.CheckedAt != nil && time.Time(*res.Health.CheckedAt).IsZero() {
		res.Health.CheckedAt = nil
	}
	return res, nil
}

// Trend 近 N 天按日趋势（N 上限 90，默认 7；含今天，空天补零）。
func (s *StatsLlmService) Trend(ctx context.Context, days int) ([]StatsLlmTrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	now := time.Now()
	today0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := today0.AddDate(0, 0, -(days - 1))

	var rows []spendDayAggRow
	err := db.LLMDB().WithContext(ctx).Raw(`
SELECT to_char(start_time, 'YYYY-MM-DD') AS day,
  COUNT(*) AS requests,
  COALESCE(SUM(prompt_tokens), 0)::bigint AS prompt_tokens,
  COALESCE(SUM(completion_tokens), 0)::bigint AS completion_tokens,
  COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(spend), 0)::bigint AS spend,
  COALESCE(SUM(CASE WHEN status <> 'success' THEN 1 ELSE 0 END), 0)::bigint AS error_count
FROM aa_llm_spend_log
WHERE start_time >= ? AND start_time < ?
GROUP BY day`, from, today0.AddDate(0, 0, 1)).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]spendAggRow, len(rows))
	for _, r := range rows {
		byDay[r.Day] = spendAggRow{
			Requests: r.Requests, PromptTokens: r.PromptTokens, CompletionTokens: r.CompletionTokens,
			TotalTokens: r.TotalTokens, Spend: r.Spend, ErrorCount: r.ErrorCount,
		}
	}
	// 逐日展开补零，保证趋势图横轴连续。
	points := make([]StatsLlmTrendPoint, 0, days)
	for i := 0; i < days; i++ {
		day := from.AddDate(0, 0, i).Format("2006-01-02")
		points = append(points, StatsLlmTrendPoint{Date: day, StatsLlmUsageSummary: toUsageSummary(byDay[day])})
	}
	return points, nil
}

// Top 维度 Top 榜：dimension ∈ model | virtualKey | user；days 上限 90（默认 7），limit 上限 50（默认 10）。
// 排序沿用花费归集报表口径（spend DESC）。
func (s *StatsLlmService) Top(ctx context.Context, dimension string, days, limit int) ([]StatsLlmTopItem, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	now := time.Now()
	today0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := today0.AddDate(0, 0, -(days - 1))

	switch dimension {
	case "model":
		return s.topModel(ctx, from, today0.AddDate(0, 0, 1), limit)
	case "virtualKey":
		return s.topVirtualKey(ctx, from, today0.AddDate(0, 0, 1), limit)
	case "user":
		return s.topUser(ctx, from, today0.AddDate(0, 0, 1), limit)
	}
	return nil, errors.New("dimension 须为 model/virtualKey/user")
}

// topModel 按模型聚合：key=model_id，name=模型表 model_name（富化；查不到回落请求里的 model 串）。
func (s *StatsLlmService) topModel(ctx context.Context, from, to time.Time, limit int) ([]StatsLlmTopItem, error) {
	var rows []struct {
		Key         string      `gorm:"column:key"`
		Model       string      `gorm:"column:model"`
		Requests    int64       `gorm:"column:requests"`
		TotalTokens int64       `gorm:"column:total_tokens"`
		Spend       types.Milli `gorm:"column:spend"`
		ErrorCount  int64       `gorm:"column:error_count"`
	}
	err := db.LLMDB().WithContext(ctx).Raw(`
SELECT model_id AS key, MAX(model) AS model,
  COUNT(*) AS requests,
  COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(spend), 0)::bigint AS spend,
  COALESCE(SUM(CASE WHEN status <> 'success' THEN 1 ELSE 0 END), 0)::bigint AS error_count
FROM aa_llm_spend_log
WHERE start_time >= ? AND start_time < ? AND model_id <> ''
GROUP BY model_id
ORDER BY spend DESC LIMIT ?`, from, to, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	// 模型名富化：同库 aa_llm_model（含软删行——仅取名，不筛状态）。
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.Key)
	}
	names := make(map[string]string, len(rows))
	if len(ids) > 0 {
		var ms []*struct {
			ModelId   string `gorm:"column:model_id"`
			ModelName string `gorm:"column:model_name"`
		}
		if err := db.LLMDB().WithContext(ctx).
			Table("aa_llm_model").
			Select("model_id, model_name").
			Where("model_id in ?", ids).
			Scan(&ms).Error; err == nil {
			for _, m := range ms {
				names[m.ModelId] = m.ModelName
			}
		}
	}
	list := make([]StatsLlmTopItem, 0, len(rows))
	for _, r := range rows {
		name := r.Model
		if n, ok := names[r.Key]; ok && n != "" {
			name = n
		}
		list = append(list, StatsLlmTopItem{
			Key: r.Key, Name: name, Requests: r.Requests,
			TotalTokens: r.TotalTokens, Spend: r.Spend,
			ErrorCount: r.ErrorCount, ErrorRate: errRatePct(r.ErrorCount, r.Requests),
		})
	}
	return list, nil
}

// topVirtualKeyRow 虚拟 Key 榜扫描行（与出参解耦，别名富化在 SQL 内完成）。
type topVirtualKeyRow struct {
	Key         string      `gorm:"column:key"`
	Name        string      `gorm:"column:name"`
	Requests    int64       `gorm:"column:requests"`
	TotalTokens int64       `gorm:"column:total_tokens"`
	Spend       types.Milli `gorm:"column:spend"`
	ErrorCount  int64       `gorm:"column:error_count"`
}

// topVirtualKey 按虚拟 Key 聚合：key=token_hash（密钥公开标识，spend_log.api_key 即它），
// name=key_alias（同库 LEFT JOIN 富化；别名空取 hash 前 12 位加省略号兜底）。
func (s *StatsLlmService) topVirtualKey(ctx context.Context, from, to time.Time, limit int) ([]StatsLlmTopItem, error) {
	var rows []topVirtualKeyRow
	err := db.LLMDB().WithContext(ctx).Raw(`
SELECT d.api_key AS key,
  COALESCE(NULLIF(v.key_alias, ''), LEFT(d.api_key, 12) || '…') AS name,
  COUNT(*) AS requests,
  COALESCE(SUM(d.total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(d.spend), 0)::bigint AS spend,
  COALESCE(SUM(CASE WHEN d.status <> 'success' THEN 1 ELSE 0 END), 0)::bigint AS error_count
FROM aa_llm_spend_log d
LEFT JOIN aa_llm_virtual_key v ON v.token_hash = d.api_key
WHERE d.start_time >= ? AND d.start_time < ? AND d.api_key <> ''
GROUP BY d.api_key, v.key_alias
ORDER BY spend DESC LIMIT ?`, from, to, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	list := make([]StatsLlmTopItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, StatsLlmTopItem{
			Key: r.Key, Name: r.Name, Requests: r.Requests,
			TotalTokens: r.TotalTokens, Spend: r.Spend,
			ErrorCount: r.ErrorCount, ErrorRate: errRatePct(r.ErrorCount, r.Requests),
		})
	}
	return list, nil
}

// topUser 按用户聚合：key=spend_log.user_id（即 aa_user.user_id），
// 昵称富化走主库 aa_user（跨库不可 join，批量回填）。
func (s *StatsLlmService) topUser(ctx context.Context, from, to time.Time, limit int) ([]StatsLlmTopItem, error) {
	var rows []StatsLlmTopItem
	err := db.LLMDB().WithContext(ctx).Raw(`
SELECT user_id AS key,
  COUNT(*) AS requests,
  COALESCE(SUM(total_tokens), 0)::bigint AS total_tokens,
  COALESCE(SUM(spend), 0)::bigint AS spend,
  COALESCE(SUM(CASE WHEN status <> 'success' THEN 1 ELSE 0 END), 0)::bigint AS error_count
FROM aa_llm_spend_log
WHERE start_time >= ? AND start_time < ? AND user_id <> ''
GROUP BY user_id
ORDER BY spend DESC LIMIT ?`, from, to, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	// 昵称富化：主库 aa_user（TOP 行数 ≤50，一把 IN 查回）。
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.Key)
	}
	names := make(map[string]string, len(rows))
	if len(ids) > 0 {
		var us []*struct {
			UserId       string `gorm:"column:user_id"`
			UserNickname string `gorm:"column:user_nickname"`
		}
		if err := db.DB().WithContext(ctx).
			Table("aa_user").
			Select("user_id, user_nickname").
			Where("user_id in ?", ids).
			Scan(&us).Error; err == nil {
			for _, u := range us {
				names[u.UserId] = u.UserNickname
			}
		}
	}
	for i := range rows {
		rows[i].Name = rows[i].Key
		if n, ok := names[rows[i].Key]; ok && n != "" {
			rows[i].Name = n
		}
		rows[i].ErrorRate = errRatePct(rows[i].ErrorCount, rows[i].Requests)
	}
	return rows, nil
}

// toUsageSummary 聚合行 → 汇总出参（错误率在此统一补算）。
func toUsageSummary(r spendAggRow) StatsLlmUsageSummary {
	return StatsLlmUsageSummary{
		Requests:         r.Requests,
		PromptTokens:     r.PromptTokens,
		CompletionTokens: r.CompletionTokens,
		TotalTokens:      r.TotalTokens,
		Spend:            r.Spend,
		ErrorCount:       r.ErrorCount,
		ErrorRate:        errRatePct(r.ErrorCount, r.Requests),
	}
}

// errRatePct 错误率（%）：保留 2 位小数；无请求恒 0。
func errRatePct(errCount, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(errCount)/float64(total)*10000) / 100
}

// roundFloatToInt 四舍五入取整；nil 视为无数据取 0。
func roundFloatToInt(v *float64) int64 {
	if v == nil {
		return 0
	}
	return int64(math.Round(*v))
}
