package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

// ---- MCP 批量上限配置（F-032）----
// 变异锚点：删 applyMcpDefaults 回落 → 零值/负值格红；删 env 注入分支 → env 格红。

func TestApplyMcpDefaults(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{name: "未配置回落默认", in: 0, want: DefaultMaxBatchItems},
		{name: "负值回落默认", in: -5, want: DefaultMaxBatchItems},
		{name: "显式合法值保留", in: 50, want: 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{}
			c.Mcp.MaxBatchItems = tc.in
			c.applyMcpDefaults()
			if c.Mcp.MaxBatchItems != tc.want {
				t.Fatalf("in=%d got %d want %d", tc.in, c.Mcp.MaxBatchItems, tc.want)
			}
		})
	}
}

func TestApplyMcpDefaultsMaxRequestBytes(t *testing.T) {
	c := &Config{}
	c.applyMcpDefaults()
	if c.Mcp.MaxRequestBytes != DefaultMaxRequestBytes {
		t.Fatalf("未配置应回落默认 2MB, got %d", c.Mcp.MaxRequestBytes)
	}
	c2 := &Config{}
	c2.Mcp.MaxRequestBytes = 0
	c2.applyMcpDefaults()
	if c2.Mcp.MaxRequestBytes != DefaultMaxRequestBytes {
		t.Fatalf("显式 0 应回落默认, got %d", c2.Mcp.MaxRequestBytes)
	}
	c3 := &Config{}
	c3.Mcp.MaxRequestBytes = 4096
	c3.applyMcpDefaults()
	if c3.Mcp.MaxRequestBytes != 4096 {
		t.Fatalf("显式正值应保留, got %d", c3.Mcp.MaxRequestBytes)
	}
}

func TestApplyMcpEnvOverrideRequestBytes(t *testing.T) {
	t.Setenv("AIARSENAL_MCP_MAX_REQUEST_BYTES", "8192")
	v := viper.New()
	applyMcpEnvOverride(v)
	c := &Config{}
	if err := v.Unmarshal(c); err != nil {
		t.Fatal(err)
	}
	c.applyMcpDefaults()
	if c.Mcp.MaxRequestBytes != 8192 {
		t.Fatalf("env 注入应生效, got %d", c.Mcp.MaxRequestBytes)
	}
	os.Unsetenv("AIARSENAL_MCP_MAX_REQUEST_BYTES")
}

func TestApplyMcpEnvOverride(t *testing.T) {
	t.Setenv("AIARSENAL_MCP_MAX_BATCH_ITEMS", "7")
	v := viper.New()
	applyMcpEnvOverride(v)
	c := &Config{}
	if err := v.Unmarshal(c); err != nil {
		t.Fatal(err)
	}
	c.applyMcpDefaults()
	if c.Mcp.MaxBatchItems != 7 {
		t.Fatalf("env 覆盖未生效: %d", c.Mcp.MaxBatchItems)
	}

	os.Unsetenv("AIARSENAL_MCP_MAX_BATCH_ITEMS")
	v2 := viper.New()
	applyMcpEnvOverride(v2)
	c2 := &Config{}
	if err := v2.Unmarshal(c2); err != nil {
		t.Fatal(err)
	}
	c2.applyMcpDefaults()
	if c2.Mcp.MaxBatchItems != DefaultMaxBatchItems {
		t.Fatalf("无 env 应回落默认: %d", c2.Mcp.MaxBatchItems)
	}
}
