package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/crypto"
	"aiarsenal/interfaces/dto"
)

// LLM 配置服务：凭据 / 模型 / 预算。
// 凭据的 API key 走 AES-256-GCM 加密落库（密钥取 jwt.encryptKey）。

type LLMConfigService struct {
	credRepo   repository.ILLMCredentialRepository
	modelRepo  repository.ILLMModelRepository
	budgetRepo repository.ILLMBudgetRepository
	groupRepo  repository.ILLMModelGroupRepository
	dailyRepo  repository.ILLMDailySpendRepository // 我的预算·历史周期消费聚合
	noticeSvc  *NoticeService                     // 预算低额/耗尽站内通知
}

func NewLLMConfigService(c repository.ILLMCredentialRepository, m repository.ILLMModelRepository, b repository.ILLMBudgetRepository, g repository.ILLMModelGroupRepository, n *NoticeService, ds repository.ILLMDailySpendRepository) *LLMConfigService {
	return &LLMConfigService{credRepo: c, modelRepo: m, budgetRepo: b, groupRepo: g, dailyRepo: ds, noticeSvc: n}
}

// ===== Credential =====

func (s *LLMConfigService) ListCredentials(ctx context.Context, name, provider string, pn, ps int) ([]*entity.LLMCredential, int64, error) {
	return s.credRepo.List(ctx, name, provider, pn, ps)
}

func (s *LLMConfigService) GetCredential(ctx context.Context, id string) (*entity.LLMCredential, error) {
	return s.credRepo.Get(ctx, id)
}

// GetCredentialAPIKey 解密返回 provider 明文 key（仅供 router 内部或"测试连通"调用，不暴露给前端 list）。
func (s *LLMConfigService) GetCredentialAPIKey(ctx context.Context, id string) (string, *entity.LLMCredential, error) {
	c, err := s.credRepo.Get(ctx, id)
	if err != nil {
		return "", nil, err
	}
	pt, err := crypto.Decrypt(c.CredentialValues, config.Get().Jwt.EncryptKey)
	if err != nil {
		return "", nil, err
	}
	return pt, c, nil
}

func (s *LLMConfigService) AddCredential(ctx context.Context, e *entity.LLMCredential, apiKey string) (string, error) {
	if apiKey != "" {
		enc, err := crypto.Encrypt(apiKey, config.Get().Jwt.EncryptKey)
		if err != nil {
			return "", err
		}
		e.CredentialValues = enc
	}
	if e.Status == 0 {
		e.Status = 1
	}
	return s.credRepo.Add(ctx, e)
}

func (s *LLMConfigService) EditCredential(ctx context.Context, e *entity.LLMCredential, apiKey string) error {
	if apiKey != "" {
		enc, err := crypto.Encrypt(apiKey, config.Get().Jwt.EncryptKey)
		if err != nil {
			return err
		}
		e.CredentialValues = enc
	}
	return s.credRepo.Edit(ctx, e)
}

func (s *LLMConfigService) DeleteCredentials(ctx context.Context, ids []string) error {
	return s.credRepo.Delete(ctx, ids)
}

// ===== Model =====

func (s *LLMConfigService) ListModels(ctx context.Context, name, provider, group string, status *uint, pn, ps int) ([]*entity.LLMModel, int64, error) {
	return s.modelRepo.List(ctx, name, provider, group, status, pn, ps)
}

func (s *LLMConfigService) AllModels(ctx context.Context) ([]*entity.LLMModel, error) {
	return s.modelRepo.All(ctx)
}

func (s *LLMConfigService) GetModel(ctx context.Context, id string) (*entity.LLMModel, error) {
	return s.modelRepo.Get(ctx, id)
}

func (s *LLMConfigService) AddModel(ctx context.Context, e *entity.LLMModel) (string, error) {
	if e.Status == 0 {
		e.Status = 1
	}
	if e.Weight == 0 {
		e.Weight = 1
	}
	return s.modelRepo.Add(ctx, e)
}

func (s *LLMConfigService) EditModel(ctx context.Context, e *entity.LLMModel) error {
	if err := s.modelRepo.Edit(ctx, e); err != nil {
		return err
	}
	// max_input/output_tokens 是模型固有属性：编辑后同步到同分组同模型的所有部署，
	// 避免 FetchLLMModels 富化时多个部署参差、取到不确定的值。
	return s.modelRepo.BatchSyncModelTokenLimits(ctx, e.ModelGroup, e.ModelName, e.MaxInputTokens, e.MaxOutputTokens, e.UpdatedBy)
}

func (s *LLMConfigService) DeleteModels(ctx context.Context, ids []string) error {
	return s.modelRepo.Delete(ctx, ids)
}

// DiscoverModels 用凭据调上游 /v1/models，把支持的模型批量导入为该 group 下的部署。
// 已存在的 (group, provider_model, credential) 跳过；provider 取凭据 ProviderType；
// providerModel/modelName 用上游返回的 id；定价留 0（导入后在模型页改）。
func (s *LLMConfigService) DiscoverModels(ctx context.Context, credentialId, modelGroup, userId string) (*dto.LLMDiscoverModelsRes, error) {
	if modelGroup == "" {
		modelGroup = "auto"
	}
	apiKey, cred, err := s.GetCredentialAPIKey(ctx, credentialId)
	if err != nil {
		slog.Error("导入模型：取凭据失败", "credentialId", credentialId, "err", err)
		return nil, errors.New("凭据不存在或解密失败")
	}
	provider := cred.ProviderType
	baseURL := cred.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(provider)
	}
	if baseURL == "" {
		return nil, errors.New("凭据未配置 base_url，且该 provider 无默认地址")
	}
	ids, err := fetchUpstreamModels(ctx, baseURL, provider, apiKey)
	if err != nil {
		slog.Error("导入模型：调上游 /v1/models 失败", "credentialId", credentialId, "baseURL", baseURL, "err", err)
		return nil, fmt.Errorf("调上游 /v1/models 失败: %w", err)
	}
	res := &dto.LLMDiscoverModelsRes{Models: []string{}}
	for _, id := range ids {
		exists, err := s.modelRepo.ExistsDeploy(ctx, modelGroup, id, credentialId)
		if err != nil {
			slog.Error("导入模型：查重失败", "group", modelGroup, "providerModel", id, "err", err)
			continue
		}
		if exists {
			res.Skipped++
			continue
		}
		m := &entity.LLMModel{
			ModelName:     id,
			ModelGroup:    modelGroup,
			Provider:      provider,
			ProviderModel: id,
			CredentialId:  credentialId,
			Weight:        1,
			Status:        1,
			CreatedBy:     userId,
			UpdatedBy:     userId,
		}
		if _, e := s.modelRepo.Add(ctx, m); e != nil {
			slog.Warn("导入模型：单条写入失败", "providerModel", id, "err", e)
			res.Skipped++
			continue
		}
		res.Imported++
		res.Models = append(res.Models, id)
	}
	slog.Info("导入模型完成", "credentialId", credentialId, "group", modelGroup, "upstream", len(ids), "imported", res.Imported, "skipped", res.Skipped)
	return res, nil
}

// fetchUpstreamModels 调上游 GET <baseURL>/v1/models 取支持的模型 id 列表。
// anthropic 用 x-api-key + anthropic-version；其余(openai 兼容族)用 Authorization: Bearer。
// 两者响应都是 {data:[{id,...}]}，取 data[].id。
func fetchUpstreamModels(ctx context.Context, baseURL, provider, apiKey string) ([]string, error) {
	url := strings.TrimRight(baseURL, "/") + "/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(provider, "anthropic") {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return nil, fmt.Errorf("上游 %s 返回 %d: %s", url, resp.StatusCode, snippet)
	}
	var parsed struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("解析 /v1/models 响应失败: %w", err)
	}
	ids := make([]string, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		if d.Id != "" {
			ids = append(ids, d.Id)
		}
	}
	return ids, nil
}

// defaultBaseURL 按 provider 给上游默认地址（base_url 空时兜底，取值对齐网关 handler.go）。
func defaultBaseURL(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "https://api.openai.com"
	case "anthropic":
		return "https://api.anthropic.com"
	case "deepseek":
		return "https://api.deepseek.com"
	case "moonshot":
		return "https://api.moonshot.cn"
	case "groq":
		return "https://api.groq.com/openai"
	case "openrouter":
		return "https://openrouter.ai/api"
	case "ollama":
		return "http://127.0.0.1:11434"
	}
	return ""
}

// ===== Budget =====

func (s *LLMConfigService) ListBudgets(ctx context.Context, name string, pn, ps int) ([]*entity.LLMBudget, int64, error) {
	return s.budgetRepo.List(ctx, name, pn, ps)
}

func (s *LLMConfigService) GetBudget(ctx context.Context, id string) (*entity.LLMBudget, error) {
	return s.budgetRepo.Get(ctx, id)
}

// normalizeBudgetRules 校验并规范化适用范围条件行（[]model.GrantRow JSON，可空），
// 与菜单 grant_rules 同一套校验；空/未填统一落 "[]"。
func normalizeBudgetRules(e *entity.LLMBudget) error {
	rows, err := model.ParseGrantRows(e.GrantRules)
	if err != nil {
		return err
	}
	e.GrantRules = model.GrantRowsJSON(rows)
	return nil
}

func (s *LLMConfigService) AddBudget(ctx context.Context, e *entity.LLMBudget) (string, error) {
	if err := normalizeBudgetRules(e); err != nil {
		return "", err
	}
	return s.budgetRepo.Add(ctx, e)
}

func (s *LLMConfigService) EditBudget(ctx context.Context, e *entity.LLMBudget) error {
	if err := normalizeBudgetRules(e); err != nil {
		return err
	}
	return s.budgetRepo.Edit(ctx, e)
}

func (s *LLMConfigService) DeleteBudgets(ctx context.Context, ids []string) error {
	return s.budgetRepo.Delete(ctx, ids)
}

// 预算低额/耗尽通知档位（与网关侧 llm/internal/notify 同步维护）。
const (
	budgetNoticeLevelLow       = "low"       // 剩余 <10%（spend/max ≥ 90%）
	budgetNoticeLevelExhausted = "exhausted" // 已用尽（≥100%）
)

// budgetWindowKey 通知去重的窗口标识：有周期预算用 reset_at（窗口滚存后自然换新 key 重新提醒）；
// 无周期（累计型）用当月，避免一次提醒后永久静默。与网关侧实现保持一致。
func budgetWindowKey(resetAt *types.Time) string {
	if resetAt != nil {
		return time.Time(*resetAt).Format(time.RFC3339)
	}
	return time.Now().Format("200601")
}

// notifyBudgetLevel 预算低额/耗尽时给 userId 发一条站内通知（右上角铃铛可见，
// 点击跳【我的预算】）。aa_llm_budget_notice 表按 user+budget+window+level 去重，
// 每窗口每人每预算每档至多一条；best-effort，失败只记日志不影响主流程。
func (s *LLMConfigService) notifyBudgetLevel(ctx context.Context, userId string, b *entity.LLMBudget, level string) {
	if s.noticeSvc == nil || userId == "" || b.MaxBudget == nil || *b.MaxBudget <= 0 {
		return
	}
	first, err := s.budgetRepo.TryBudgetNoticeDedup(ctx, userId, b.BudgetId, budgetWindowKey(b.BudgetResetAt), level)
	if err != nil || !first {
		if err != nil {
			slog.Warn("预算通知去重失败", "budgetId", b.BudgetId, "userId", userId, "err", err)
		}
		return
	}
	pct := float64(b.Spend) / float64(*b.MaxBudget) * 100
	remain := *b.MaxBudget - b.Spend
	if remain < 0 {
		remain = 0
	}
	var title, content, ntype string
	if level == budgetNoticeLevelExhausted {
		title, ntype = "预算已用尽", "error"
		content = fmt.Sprintf("预算「%s」本周期额度已用尽（$%.2f / $%.2f），命中此预算的 AI 调用将被拒绝。", b.BudgetName, b.Spend.Float(), b.MaxBudget.Float())
	} else {
		title, ntype = "预算余额不足", "warning"
		content = fmt.Sprintf("预算「%s」本周期仅剩 %.1f%%（剩余 $%.2f / 上限 $%.2f），用尽后命中此预算的 AI 调用将被拒绝。", b.BudgetName, 100-pct, remain.Float(), b.MaxBudget.Float())
	}
	if err := s.noticeSvc.Push(ctx, userId, title, content, "/llm/budget/mine", ntype, "0"); err != nil {
		slog.Warn("预算低额通知发送失败", "budgetId", b.BudgetId, "userId", userId, "err", err)
	}
}

// MyBudgets 我的预算：按当前用户六维身份匹配全部圈人预算（grant_rules 条件行，任一行命中即算），
// 返回本周期已消费/剩余 + 本人本月消费汇总（tokens/金额/次数）。命中集合先懒滚存再取值，
// 个人口径展示：Spend/Remaining/UsedPercent 全部 =「本人本周期消费 vs 预算上限」，只在展示层
// 拿预算配置减已消耗（周期内，user 维度日聚合）计算；预算行的池子 spend 不回显也不被修改——
// 预算是多人共用的，个人额度不能写回预算。命中 = 生效上限最大的一条（唯一展示/提醒对象），
// 低额/耗尽提醒只对它发（个人口径；网关计费路径是主通知生产者，此处兜底覆盖"别人花超、自己来看"）。
func (s *LLMConfigService) MyBudgets(ctx context.Context, t model.Identity) (*dto.LLMMyBudgetRes, error) {
	budgets, err := s.budgetRepo.ListGrantBudgets(ctx)
	if err != nil {
		return nil, err
	}
	ms := make([]myBudgetMatch, 0, len(budgets))
	for _, b := range budgets {
		rows, perr := model.ParseGrantRows(b.GrantRules)
		if perr != nil {
			continue
		}
		if !model.MatchAny(rows, t) {
			continue
		}
		// 窗口到期懒滚存；失败（如并发删除）退回未滚存值，不阻断展示。
		if rolled, rerr := s.budgetRepo.RollBudgetWindow(ctx, b.BudgetId); rerr == nil && rolled != nil {
			b = rolled
		}
		ms = append(ms, myBudgetMatch{b: b, mySpend: budgetWindowSpend(ctx, s.dailyRepo, t.UserId, b)})
	}
	hit := pickHitBudget(ms)
	out := make([]dto.LLMMyBudgetItem, 0, len(ms))
	for _, m := range ms {
		b := m.b
		item := dto.LLMMyBudgetItem{
			BudgetId:       b.BudgetId,
			BudgetName:     b.BudgetName,
			MaxBudget:      b.MaxBudget,
			SoftBudget:     b.SoftBudget,
			Spend:          m.mySpend,
			BudgetDuration: b.BudgetDuration,
			BudgetResetAt:  b.BudgetResetAt,
		}
		if b.MaxBudget != nil && *b.MaxBudget > 0 {
			remaining := *b.MaxBudget - m.mySpend
			if remaining < 0 {
				remaining = 0
			}
			pct := float64(m.mySpend) / float64(*b.MaxBudget) * 100
			item.Remaining = &remaining
			item.UsedPercent = pct
			item.IsExhausted = m.mySpend >= *b.MaxBudget
			item.IsLow = pct >= 90
		}
		out = append(out, item)
	}
	res := &dto.LLMMyBudgetRes{List: out, Usage: s.myUsageSummary(ctx, t.UserId)}
	if hit != nil {
		res.HitBudgetId = hit.b.BudgetId
		for _, item := range out {
			if item.BudgetId != hit.b.BudgetId {
				continue
			}
			if item.IsExhausted {
				s.notifyBudgetLevel(ctx, t.UserId, hit.b, budgetNoticeLevelExhausted)
			} else if item.IsLow {
				s.notifyBudgetLevel(ctx, t.UserId, hit.b, budgetNoticeLevelLow)
			}
		}
	}
	return res, nil
}

// myBudgetMatch 命中预算 + 本人本窗口消费（展示层计算中间量，毫）。
type myBudgetMatch struct {
	b       *entity.LLMBudget
	mySpend types.Milli
}

// pickHitBudget 命中预算：生效上限最大的一条（nil/0=不限额视作 ∞；并列取本人消费多的，
// 再按 budgetId 稳定）。与 resolveBase（临时预算基础预算）同口径。
func pickHitBudget(ms []myBudgetMatch) *myBudgetMatch {
	var best *myBudgetMatch
	for _, m := range ms {
		if best == nil {
			best = &m
			continue
		}
		mb, bb := budgetCap(m.b), budgetCap(best.b)
		if mb > bb || (mb == bb && m.mySpend > best.mySpend) ||
			(mb == bb && m.mySpend == best.mySpend && m.b.BudgetId < best.b.BudgetId) {
			best = &m
		}
	}
	return best
}

// budgetWindowSpend 本人在预算当前窗口内的消费（aa_llm_daily_spend user 维度按日聚合）：
// 窗口起点 = 重置时间 − 周期；无周期 = 累计全量；resetAt 未初始化回退预算创建时间。
// 纯只读求和，日粒度近似窗口边界。聚合失败返回 0（不阻断展示）。
// /mine 展示与临时预算申请（申请前提=命中预算本周期有消耗）共用本口径。
func budgetWindowSpend(ctx context.Context, dailyRepo repository.ILLMDailySpendRepository, userId string, b *entity.LLMBudget) types.Milli {
	start := "1970-01-01"
	if b.BudgetDuration != "" {
		anchor := b.BudgetResetAt
		if anchor == nil {
			anchor = b.CreatedAt
		}
		if anchor != nil {
			start = time.Time(*anchor).Add(-budgetDurationLen(b.BudgetDuration)).Format("2006-01-02")
		}
	}
	rows, err := dailyRepo.SumScopeDaily(ctx, "user", userId,
		[]string{start, time.Now().Format("2006-01-02")})
	if err != nil {
		return 0
	}
	var sum types.Milli
	for _, p := range rows {
		sum += p.Spend
	}
	return sum
}

// budgetDurationLen 周期字符串 → 时长（1d/7d，其余含 30d/1mo 按 30 天兜底）。
func budgetDurationLen(d string) time.Duration {
	switch d {
	case "1d":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	}
	return 30 * 24 * time.Hour
}

// myUsageSummary 本人自然月（1 号至今）真实消费汇总：与预算窗口解耦，tokens/金额/次数
// 全来自 aa_llm_daily_spend 的 user 维度日聚合。聚合失败不阻断页面，返回零值。
func (s *LLMConfigService) myUsageSummary(ctx context.Context, userId string) dto.LLMMyUsageSummary {
	now := time.Now()
	u := dto.LLMMyUsageSummary{Month: now.Format("2006-01")}
	rows, err := s.dailyRepo.SumScopeDaily(ctx, "user", userId, []string{now.Format("2006-01") + "-01", now.Format("2006-01-02")})
	if err != nil {
		return u
	}
	for _, p := range rows {
		u.Spend += p.Spend
		u.Requests += p.Requests
		u.PromptTokens += p.PromptTokens
		u.CompletionTokens += p.CompletionTokens
	}
	return u
}

// MySpendHistory 历史周期消费：本人（scope_type='user'）日粒度聚合再按周期分桶。
// day→近 31 天逐日 / week→近 90 天按 ISO 周桶（2026-W36）/ month→近 365 天按月桶（2026-09），
// 桶升序返回（日行本就升序，同桶连续聚合不影响有序）。
func (s *LLMConfigService) MySpendHistory(ctx context.Context, userId, period string) (*dto.LLMSpendHistoryRes, error) {
	if period != "day" && period != "week" && period != "month" {
		period = "month"
	}
	days := 365
	switch period {
	case "day":
		days = 31
	case "week":
		days = 90
	}
	dateRange := []string{
		time.Now().AddDate(0, 0, -days + 1).Format("2006-01-02"),
		time.Now().Format("2006-01-02"),
	}
	rows, err := s.dailyRepo.SumScopeDaily(ctx, "user", userId, dateRange)
	if err != nil {
		return nil, err
	}
	res := &dto.LLMSpendHistoryRes{Period: period, Points: make([]dto.LLMSpendHistoryPoint, 0, len(rows))}
	idx := make(map[string]int, len(rows))
	for _, p := range rows {
		if len(p.Date) < 10 {
			continue
		}
		key := p.Date
		if period == "week" {
			key = isoWeekKey(p.Date)
		} else if period == "month" {
			key = p.Date[:7]
		}
		if i, ok := idx[key]; ok {
			res.Points[i].Spend += p.Spend
			continue
		}
		idx[key] = len(res.Points)
		res.Points = append(res.Points, dto.LLMSpendHistoryPoint{Period: key, Spend: p.Spend})
	}
	return res, nil
}

// isoWeekKey YYYY-MM-DD → ISO 周桶标签（2026-W36）。
func isoWeekKey(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	y, w := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

// ===== ModelGroup（分组字典）=====

func (s *LLMConfigService) ListGroups(ctx context.Context, name string, pn, ps int) ([]*entity.LLMModelGroup, int64, error) {
	return s.groupRepo.List(ctx, name, pn, ps)
}

func (s *LLMConfigService) GetGroup(ctx context.Context, id string) (*entity.LLMModelGroup, error) {
	return s.groupRepo.Get(ctx, id)
}

func (s *LLMConfigService) AddGroup(ctx context.Context, e *entity.LLMModelGroup) (string, error) {
	if e.Status == 0 {
		e.Status = 1
	}
	return s.groupRepo.Add(ctx, e)
}

func (s *LLMConfigService) EditGroup(ctx context.Context, e *entity.LLMModelGroup) error {
	return s.groupRepo.Edit(ctx, e)
}

func (s *LLMConfigService) DeleteGroups(ctx context.Context, ids []string) error {
	return s.groupRepo.Delete(ctx, ids)
}
