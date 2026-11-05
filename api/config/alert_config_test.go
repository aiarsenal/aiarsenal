package config

import (
	"testing"

	"aiarsenal/infrastructure/alert"
)

// applyAlertDefaults 归一表测：非法/零值回落默认（与 applyPasswordDefaults 同款语义）。
func TestApplyAlertDefaults(t *testing.T) {
	cases := []struct {
		name    string
		in      AlertConfig
		want    AlertConfig
		wantLog string // 非空=断言 WARN 提及该串（slog 全局输出，这里只校验行为不校验日志）
	}{
		{
			name: "全零值回落默认",
			in:   AlertConfig{},
			want: AlertConfig{QueueSize: 64, RetryMax: 5, ThrottleMinutes: 0},
		},
		{
			name: "非法负值归一",
			in:   AlertConfig{QueueSize: -1, RetryMax: -3, ThrottleMinutes: -10},
			want: AlertConfig{QueueSize: 64, RetryMax: 5, ThrottleMinutes: 0},
		},
		{
			name: "显式合法值保留",
			in:   AlertConfig{QueueSize: 128, RetryMax: 9, ThrottleMinutes: 30},
			want: AlertConfig{QueueSize: 128, RetryMax: 9, ThrottleMinutes: 30},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Alert: tc.in}
			c.applyAlertDefaults()
			got := c.Alert
			if got.QueueSize != tc.want.QueueSize || got.RetryMax != tc.want.RetryMax || got.ThrottleMinutes != tc.want.ThrottleMinutes {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

// SchemeSupported 是 config 预校验的单一真相源：白名单内外 scheme 判定必须与
// alert.senderByScheme 一致（变异点：新增/删除白名单项只应改 alert 包一处）。
func TestAlertSchemeWhitelistAligned(t *testing.T) {
	for _, s := range []string{"lark", "wecom", "smtp", "generic"} {
		if !alert.SchemeSupported(s) {
			t.Fatalf("scheme %q 应在白名单", s)
		}
	}
	for _, s := range []string{"slack", "telegram", "discord", "bark"} {
		if alert.SchemeSupported(s) {
			t.Fatalf("scheme %q 不应在白名单（shoutrrr 有 service 但未显式登记）", s)
		}
	}
	// 大小写归一：SchemeSupported 与 sendShoutrrr 消费侧同做 ToLower，两侧必须一致
	if !alert.SchemeSupported("LARK") {
		t.Fatal("白名单判定未做大小写归一，与 sendShoutrrr 消费侧行为不一致")
	}
}

