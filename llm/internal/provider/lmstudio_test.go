package provider

import (
	"context"
	"testing"
)

func TestLMStudioBuildUpstreamWithoutKey(t *testing.T) {
	url, method, headers, _, err := (LMStudioProvider{}).BuildUpstream(context.Background(), Request{
		Body:          []byte(`{"model":"old"}`),
		APIKey:        "",
		BaseURL:       "http://127.0.0.1:1234/v1",
		ProviderModel: "qwen3.8-27b-mlx@8bit",
	})
	if err != nil {
		t.Fatalf("BuildUpstream() error = %v", err)
	}
	if url != "http://127.0.0.1:1234/v1/chat/completions" {
		t.Fatalf("unexpected URL: %s", url)
	}
	if method != "POST" {
		t.Fatalf("unexpected method: %s", method)
	}
	if _, ok := headers["Authorization"]; ok {
		t.Fatal("local LM Studio request without key must not send Authorization")
	}
}

func TestLMStudioBuildUpstreamWithKey(t *testing.T) {
	_, _, headers, _, err := (LMStudioProvider{}).BuildUpstream(context.Background(), Request{
		Body:    []byte(`{"model":"old"}`),
		APIKey:  "local-token",
		BaseURL: "http://127.0.0.1:1234/v1",
	})
	if err != nil {
		t.Fatalf("BuildUpstream() error = %v", err)
	}
	if headers["Authorization"] != "Bearer local-token" {
		t.Fatalf("unexpected Authorization header: %q", headers["Authorization"])
	}
}
