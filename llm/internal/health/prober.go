package health

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"aiarsenal/llm/internal/model"
	"aiarsenal/llm/internal/provider"
	"aiarsenal/llm/internal/routing"
	"aiarsenal/llm/internal/store"
)

// Prober 后台部署健康探测：周期性对每个活跃部署发一次最小探测请求（max_tokens=1），
// 结果落 aa_llm_health_check（UI「健康监测」页直接可见），并反馈路由——
// 连续失败达到阈值 → strategy.MarkUnhealthy（路由跳过），探测恢复 → ClearUnhealthy（重新参与）。
// 与请求失败触发的冷却互补：冷却是传输层短期退避（自动过期），不健康是探测确认的持续态（显式清除）。
type Prober struct {
	st       *store.Store
	reg      *provider.Registry
	strat    routing.Strategy
	interval time.Duration
	client   *http.Client

	mu       sync.Mutex
	failStreak map[string]int // model_id → 连续失败次数（成功即清零）
}

// New 探测器。interval 来自 env LLM_HEALTH_INTERVAL_SECONDS（默认 300s；设 0 关闭探测）。
func New(st *store.Store, reg *provider.Registry, strat routing.Strategy) *Prober {
	return &Prober{
		st: st, reg: reg, strat: strat,
		interval: intervalFromEnv(),
		client:   &http.Client{Timeout: 15 * time.Second},
		failStreak: map[string]int{},
	}
}

func intervalFromEnv() time.Duration {
	if s := os.Getenv("LLM_HEALTH_INTERVAL_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return 300 * time.Second
}

// Run 阻塞循环（由 main 起 goroutine 调用），ctx 取消即退出。interval<=0 时不探测。
func (p *Prober) Run(ctx context.Context) {
	if p.interval <= 0 {
		log.Printf("[health] 探测已关闭（LLM_HEALTH_INTERVAL_SECONDS=%s）", os.Getenv("LLM_HEALTH_INTERVAL_SECONDS"))
		return
	}
	log.Printf("[health] 部署健康探测启动，间隔 %s", p.interval)
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.probeAll(ctx)
		}
	}
}

// probeAll 对全部活跃部署各探一次。单部署失败不阻断其它（含 DB 读失败——下轮再试）。
func (p *Prober) probeAll(ctx context.Context) {
	models, err := p.st.AllModels(ctx)
	if err != nil {
		log.Printf("[health] 拉取部署列表失败: %v", err)
		return
	}
	for _, m := range models {
		if ctx.Err() != nil {
			return
		}
		if m.CredentialId == "" {
			continue // 无凭据的部署本就不参与路由，不探
		}
		p.probeOne(ctx, m)
	}
}

// probeOne 探单个部署：发最小请求，记 DB，按连续失败阈值联动路由健康标记。
func (p *Prober) probeOne(ctx context.Context, m *model.Model) {
	rtt, err := p.probe(ctx, m)
	status, errMsg := "healthy", ""
	if err != nil {
		status, errMsg = "unhealthy", truncate(err.Error(), 500)
	}
	if dbErr := p.st.UpsertHealthCheck(ctx, m.ModelId, m.ModelName, status, errMsg, rtt); dbErr != nil {
		log.Printf("[health] 写探测结果失败 model=%s: %v", m.ModelId, dbErr)
	}

	p.mu.Lock()
	if err != nil {
		p.failStreak[m.ModelId]++
		streak := p.failStreak[m.ModelId]
		p.mu.Unlock()
		if streak >= failThreshold() {
			p.strat.MarkUnhealthy(ctx, m.ModelId)
			log.Printf("[health] 部署连续 %d 次探测失败，路由跳过 model=%s (%s): %s", streak, m.ModelId, m.ProviderModel, errMsg)
		}
		return
	}
	delete(p.failStreak, m.ModelId)
	p.mu.Unlock()
	p.strat.ClearUnhealthy(ctx, m.ModelId)
}

// probe 发探测请求：OpenAI 格式最小 body 经各 provider 的 BuildUpstream 翻成上游格式
// （openai 族 → /chat/completions，anthropic 族 → /v1/messages），2xx 即健康。返回往返毫秒。
func (p *Prober) probe(ctx context.Context, m *model.Model) (float64, error) {
	apiKey, cred, err := p.st.GetCredentialAPIKey(ctx, m.CredentialId)
	if err != nil {
		return 0, fmt.Errorf("取凭据: %w", err)
	}
	prov := p.reg.Pick(m.Provider)
	body := []byte(`{"model":` + strconv.Quote(m.ProviderModel) + `,"max_tokens":1,"messages":[{"role":"user","content":"ping"}]}`)
	url, method, headers, upBody, err := prov.BuildUpstream(ctx, provider.Request{
		Body: body, APIKey: apiKey, BaseURL: provider.DefaultBaseURL(m.Provider, cred.BaseURL), ProviderModel: m.ProviderModel,
	})
	if err != nil {
		return 0, fmt.Errorf("构造探测请求: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(upBody))
	if err != nil {
		return 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	start := time.Now()
	resp, err := p.client.Do(req)
	rtt := float64(time.Since(start).Milliseconds())
	if err != nil {
		return rtt, fmt.Errorf("请求上游: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return rtt, fmt.Errorf("上游返回 status %d", resp.StatusCode)
	}
	return rtt, nil
}

// failThreshold 连续失败多少次才置不健康（env LLM_HEALTH_FAIL_THRESHOLD 覆盖，默认 2）：
// 单次失败可能只是抖动，连续失败才判定真不可用。
func failThreshold() int {
	if s := os.Getenv("LLM_HEALTH_FAIL_THRESHOLD"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 2
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
