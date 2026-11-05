package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

// ---- AIARSENAL_SERVER_TRUSTED_PROXIES 环境变量注入（F-022）----
// 变异锚点：删注入分支或改 Split 分隔符 → 对应格红。

func TestApplyTrustedProxyEnvOverride(t *testing.T) {
	t.Setenv("AIARSENAL_SERVER_TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8")
	v := viper.New()
	v.Set("server.trustedProxies", []string{})
	applyTrustedProxyEnvOverride(v)
	got := v.GetStringSlice("server.trustedProxies")
	if !reflect.DeepEqual(got, []string{"127.0.0.1", "10.0.0.0/8"}) {
		t.Fatalf("可信代理清单未被环境变量覆盖: %v", got)
	}

	os.Unsetenv("AIARSENAL_SERVER_TRUSTED_PROXIES")
	v2 := viper.New()
	v2.Set("server.trustedProxies", []string{"127.0.0.1"})
	applyTrustedProxyEnvOverride(v2)
	if got := v2.GetStringSlice("server.trustedProxies"); !reflect.DeepEqual(got, []string{"127.0.0.1"}) {
		t.Fatalf("无环境变量时应保持 yaml 原值: %v", got)
	}
}
