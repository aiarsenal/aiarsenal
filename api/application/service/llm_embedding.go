package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// 真实语义嵌入：替换/升级 skill_embedding.go 的离线 LocalEmbedding（FNV 哈希）。
// 经 llm 网关 POST /v1/embeddings（OpenAI embeddings 协议，网关代理到 openai 兼容族上游
// embedding 模型）取向量。配置缺失/调用失败/维度不符自动回落 LocalEmbedding（同维度）——
// 嵌入只影响检索排序质量，降级不应打断建库/检索主流程。

// defaultEmbeddingDim 默认维度：与既有 LocalEmbedding(1536) 对齐，向量列里新旧向量同维共存。
const defaultEmbeddingDim = 1536

// LLMEmbeddingConfig llm 网关语义嵌入配置（config.yaml llm_embedding 段 + AIARSENAL_LLM_EMBEDDING_* 覆盖）。
type LLMEmbeddingConfig struct {
	BaseURL        string // llm 网关地址（如 https://llm.aiarsenal.cc），内部拼 /v1/embeddings
	APIKey         string // 虚拟 Key 明文（Authorization: Bearer）
	Model          string // embedding 调用 id：裸分组或 group/model（同 /v1/models 口径）
	Dim            int    // 期望维度；须与向量列既有维度一致（LocalEmbedding 1536），不符即回落
	TimeoutSeconds int    // 单次请求超时，0→30s
}

// LoadLLMEmbeddingConfig 读 config/config.yaml 的 llm_embedding 段（viper，路径/优先级与
// config.Load 一致），环境变量 AIARSENAL_LLM_EMBEDDING_* 优先（与 jwt.encryptKey 同一套覆盖模式）。
// config 文件读不到返回零值配置（NewEmbeddingProvider 据此回落离线嵌入，不 panic）。
func LoadLLMEmbeddingConfig() LLMEmbeddingConfig {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("config")
	v.AddConfigPath("./config")
	if pwd, err := os.Getwd(); err == nil {
		v.AddConfigPath(pwd + "/config")
	}
	var c LLMEmbeddingConfig
	if err := v.ReadInConfig(); err != nil {
		slog.Warn("llm 嵌入配置读取失败（回落离线 LocalEmbedding）", "err", err)
		return c
	}
	_ = v.UnmarshalKey("llm_embedding", &c)
	// 密钥/地址类环境变量覆盖：生产经 systemd EnvironmentFile 注入，不入库不提交 git
	if s := os.Getenv("AIARSENAL_LLM_EMBEDDING_BASE_URL"); s != "" {
		c.BaseURL = s
	}
	if s := os.Getenv("AIARSENAL_LLM_EMBEDDING_API_KEY"); s != "" {
		c.APIKey = s
	}
	if s := os.Getenv("AIARSENAL_LLM_EMBEDDING_MODEL"); s != "" {
		c.Model = s
	}
	if s := os.Getenv("AIARSENAL_LLM_EMBEDDING_DIM"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			c.Dim = n
		}
	}
	if s := os.Getenv("AIARSENAL_LLM_EMBEDDING_TIMEOUT_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			c.TimeoutSeconds = n
		}
	}
	return c
}

// NewEmbeddingProvider 嵌入装配入口（cmd/main.go 把 NewLocalEmbedding 的注入点换成这里）：
//   - base_url / api_key / model 任一未配置 → 直接返回离线 LocalEmbedding（返回值即降级结果）
//   - 配置齐全 → 返回网关语义嵌入（调用失败/维度不符时 Embed 内部自动回落离线向量）
func NewEmbeddingProvider(cfg LLMEmbeddingConfig) EmbeddingProvider {
	if cfg.Dim <= 0 {
		cfg.Dim = defaultEmbeddingDim
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 30
	}
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		slog.Info("llm 网关嵌入未配置（llm_embedding.base_url/api_key/model 缺一即视为未配置），使用离线 LocalEmbedding", "dim", cfg.Dim)
		return NewLocalEmbedding(cfg.Dim)
	}
	slog.Info("llm 网关语义嵌入已启用", "base_url", cfg.BaseURL, "model", cfg.Model, "dim", cfg.Dim, "timeout_seconds", cfg.TimeoutSeconds)
	return &GatewayEmbedding{
		cfg:      cfg,
		httpc:    &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
		fallback: NewLocalEmbedding(cfg.Dim),
	}
}

// GatewayEmbedding 网关语义嵌入：多输入一次请求（上游 /v1/embeddings 支持 input 数组，
// llm 网关对 body 只重写 model 后透传）。任何失败回落 fallback（离线哈希，同维度）。
type GatewayEmbedding struct {
	cfg      LLMEmbeddingConfig
	httpc    *http.Client
	fallback *LocalEmbedding
}

func (e *GatewayEmbedding) Dim() int { return e.cfg.Dim }

func (e *GatewayEmbedding) Embed(text string) ([]float32, error) {
	vecs, err := e.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedBatch 批量嵌入：一次请求多输入（省往返），失败整批回落离线向量（返回值不带 error——
// 调用方拿到的一定是同维度可用向量，降级只体现在日志里）。
func (e *GatewayEmbedding) EmbedBatch(texts []string) ([][]float32, error) {
	vecs, err := e.gatewayBatch(texts)
	if err != nil {
		slog.Warn("llm 网关嵌入调用失败，回落离线 LocalEmbedding（fallback）", "model", e.cfg.Model, "texts", len(texts), "err", err)
		return e.fallbackBatch(texts), nil
	}
	return vecs, nil
}

// gatewayBatch 单次网关调用：POST {base_url}/v1/embeddings {"model":..,"input":[..]}，
// 校验返回条数与维度后按 index 归位。
func (e *GatewayEmbedding) gatewayBatch(texts []string) ([][]float32, error) {
	payload, err := json.Marshal(map[string]any{"model": e.cfg.Model, "input": texts})
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(e.cfg.BaseURL, "/") + "/v1/embeddings"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	resp, err := e.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20)) // 封顶 32MB，防上游异常巨包拖死进程
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gateway embeddings http %d: %s", resp.StatusCode, capErrBody(body))
	}
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode embeddings response: %w", err)
	}
	if out.Error != nil && out.Error.Message != "" {
		return nil, fmt.Errorf("gateway embeddings error: %s", out.Error.Message)
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings count mismatch: got %d want %d", len(out.Data), len(texts))
	}
	// 维度不符整批拒绝：真语义向量混进哈希向量列会把余弦相似度搅浑，宁可整批回落
	for _, d := range out.Data {
		if e.cfg.Dim > 0 && len(d.Embedding) != e.cfg.Dim {
			return nil, fmt.Errorf("embedding dim mismatch: got %d want %d", len(d.Embedding), e.cfg.Dim)
		}
	}
	vecs := make([][]float32, len(texts))
	for _, d := range out.Data {
		idx := d.Index
		if idx < 0 || idx >= len(texts) {
			return nil, fmt.Errorf("embeddings index out of range: %d", idx)
		}
		vecs[idx] = d.Embedding
	}
	for i := range vecs {
		if vecs[i] == nil { // 上游漏序时兜底（不应发生）
			return nil, fmt.Errorf("embeddings missing vector for input %d", i)
		}
	}
	return vecs, nil
}

// fallbackBatch 离线哈希嵌入整批（LocalEmbedding.Embed 恒成功，不再带错误上抛）。
func (e *GatewayEmbedding) fallbackBatch(texts []string) [][]float32 {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := e.fallback.Embed(t)
		if err != nil {
			v = make([]float32, e.cfg.Dim) // 理论不可达（哈希嵌入无失败路径），兜底零向量保维
		}
		out[i] = v
	}
	return out
}

// capErrBody 错误响应体截断到 4KB（只进日志/错误信息，不落库）。
func capErrBody(b []byte) string {
	const n = 4 * 1024
	if len(b) > n {
		return string(b[:n]) + "...[truncated]"
	}
	return string(b)
}
