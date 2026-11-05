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
