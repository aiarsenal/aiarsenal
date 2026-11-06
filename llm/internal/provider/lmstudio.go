package provider

import (
	"context"
)

// LMStudioProvider LM Studio 的本机 OpenAI 兼容接口。
// 与 openai provider 共用请求/响应转换，但本地服务通常不要求 API Key，
// 因此仅在显式配置 Key 时才发送 Authorization 头。
type LMStudioProvider struct {
	OpenAIProvider
}

func (LMStudioProvider) Name() string { return "lmstudio" }

func (p LMStudioProvider) BuildUpstream(ctx context.Context, req Request) (string, string, map[string]string, []byte, error) {
	url, method, headers, body, err := p.OpenAIProvider.BuildUpstream(ctx, req)
	if err != nil {
		return "", "", nil, nil, err
	}
	if req.APIKey == "" {
		delete(headers, "Authorization")
	}
	return url, method, headers, body, nil
}
