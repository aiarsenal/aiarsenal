package provider

import (
	"context"
	"io"
	"net/http"
)

// Usage token 用量(provider 无关)。
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	CacheReadTokens  int
	CacheCreateTokens int
}

// Request 一次上游调用的全部上下文。
type Request struct {
	Body          []byte            // 客户端原始 JSON body（OpenAI 格式）
	Stream        bool
	APIKey        string
	BaseURL       string
	ProviderModel string            // 上游真实模型名
	ExtraHeaders  map[string]string
}

// Provider 翻译层接口：transform_request/transform_response。
// BuildUpstream 把 OpenAI 请求翻成上游请求；TransformResponse/TransformStream 把上游响应翻回 OpenAI。
type Provider interface {
	Name() string
	BuildUpstream(ctx context.Context, req Request) (url, method string, headers map[string]string, body []byte, err error)
	// TransformResponse 转换非流式上游 body → OpenAI body，并提取用量。
	TransformResponse(upstreamBody []byte, req Request) (openaiBody []byte, usage Usage, err error)
	// TransformStream 读上游流，写 OpenAI SSE 到 w，返回用量。
	TransformStream(upstream io.Reader, w http.ResponseWriter, req Request) (usage Usage, err error)
	UpstreamURL(baseURL, path string) string
}

// Registry provider 注册表。
type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry { return &Registry{providers: map[string]Provider{}} }

func (r *Registry) Register(p Provider) { r.providers[p.Name()] = p }

func (r *Registry) Get(name string) Provider { return r.providers[name] }

// Pick 默认回退 openai 兼容。
func (r *Registry) Pick(name string) Provider {
	if p, ok := r.providers[name]; ok {
		return p
	}
	return r.providers["openai"]
}

func copyHeaders(dst http.Header, src map[string]string) {
	for k, v := range src {
		dst.Set(k, v)
	}
}
