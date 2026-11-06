package config

import (
	"os"
	"reflect"
	"testing"
)

// ---- TRUSTED_PROXIES 环境变量解析（F-022 llm 侧）----
// llm 网关的用量/计费记录带 ClientIP；默认信任 none，挂反代时经本变量显式声明。

func TestSplitTrustedProxies(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"127.0.0.1", []string{"127.0.0.1"}},
		{"127.0.0.1,10.0.0.0/8", []string{"127.0.0.1", "10.0.0.0/8"}},
		{" 127.0.0.1 , 10.0.0.0/8 ", []string{"127.0.0.1", "10.0.0.0/8"}},
	}
	for _, tc := range cases {
		if got := splitTrustedProxies(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("splitTrustedProxies(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestTrustedProxiesEnvWiring(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8")
	if got := Load().Server.TrustedProxies; !reflect.DeepEqual(got, []string{"127.0.0.1", "10.0.0.0/8"}) {
		t.Fatalf("Load 未读取 TRUSTED_PROXIES: %v", got)
	}
	os.Unsetenv("TRUSTED_PROXIES")
	if got := Load().Server.TrustedProxies; got != nil {
		t.Fatalf("默认应为信任 none（nil）: %v", got)
	}
}
