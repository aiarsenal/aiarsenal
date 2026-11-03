package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aiarsenal/infrastructure/crypto"
)

// TestMain chdir 到 api/（本文件上一级）：Load 的 viper AddConfigPath("config")
// 相对 cwd，测试目录里没有 config/config.yaml——与 service 包的 chdir 模式同源。
func TestMain(m *testing.M) {
	if _, filename, _, ok := runtime.Caller(0); ok {
		_ = os.Chdir(filepath.Join(filepath.Dir(filename), ".."))
	}
	os.Exit(m.Run())
}

// ---- DataEncryptKey 决策收口（变异锚点：删回退分支或删优先分支必有格红）----

func TestDataEncryptKey(t *testing.T) {
	cases := []struct {
		name    string
		dataKey string
		jwtKey  string
		want    string
	}{
		{"留空回退 jwt 密钥（存量部署零变化）", "", "jwt-key", "jwt-key"},
		{"配置了独立密钥则优先", "data-key", "jwt-key", "data-key"},
		{"两者都空返回空串（不 panic）", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Security: SecurityConfig{DataKey: tc.dataKey}}
			c.Jwt.EncryptKey = tc.jwtKey
			if got := c.DataEncryptKey(); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

// ---- dataKey 真值入库判断（pepper 同判据：空值=功能关，不算告警对象）----

func TestSecurityDataKeyFromFileJudgment(t *testing.T) {
	cases := []struct {
		name             string
		effective, yamlV string
		want             bool
	}{
		{"非空且取自 yaml=告警", "real-key", "real-key", true},
		{"非空但经 env 覆盖=不告警", "real-key", "", false},
		{"空值=留空是文档化正常态，不告警", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pepperFromConfigFile(tc.effective, tc.yamlV); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

// rebase 接线钉子：insecureDefaults 列表项必须都有告警文案——#40 的
// EmitInsecureDefaultAlerts 逐项 Dispatch，漏 case 会发出空标题空正文的告警。
func TestInsecureDefaultNoticeHasDataKeyCopy(t *testing.T) {
	for _, item := range (&Config{dataKeyFromFile: true}).insecureDefaults() {
		title, body := insecureDefaultNotice(item)
		if title == "" || body == "" {
			t.Fatalf("insecureDefaultNotice(%q) 文案为空：title=%q body=%q", item, title, body)
		}
	}
}

func TestInsecureDefaultsContainsDataKey(t *testing.T) {
	c := &Config{}
	c.dataKeyFromFile = true
	found := false
	for _, item := range c.insecureDefaults() {
		if item == "security.dataKey" {
			found = true
		}
	}
	if !found {
		t.Fatal("dataKeyFromFile=true 时 insecureDefaults 必须包含 security.dataKey")
	}
}

// ---- 平滑启用路径的语义钉子：回退密钥加密的数据在显式 dataKey 下不可解，
// 反之亦然（GCM 认证错暴露密钥错配，绝不静默给错数据）----

func TestDataKeySeparationRoundTrip(t *testing.T) {
	jwtKey, dataKey := "jwt-signing-key", "dedicated-data-key"

	// 存量语义：dataKey 留空，数据在 jwt 密钥下加密
	legacy := &Config{}
	legacy.Jwt.EncryptKey = jwtKey
	ct, err := crypto.Encrypt("sk-provider-secret", legacy.DataEncryptKey())
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// 启用路径 1：dataKey 设为当前 jwt 密钥值 → 存量密文照常解密（平滑启用）
	adopt := &Config{Security: SecurityConfig{DataKey: jwtKey}}
	if pt, err := crypto.Decrypt(ct, adopt.DataEncryptKey()); err != nil || pt != "sk-provider-secret" {
		t.Fatalf("平滑启用路径解密失败: pt=%q err=%v", pt, err)
	}

	// 启用路径 2：dataKey 是全新随机值 → 旧密文必须解密失败（不可静默）
	fresh := &Config{Security: SecurityConfig{DataKey: dataKey}}
	if _, err := crypto.Decrypt(ct, fresh.DataEncryptKey()); err == nil {
		t.Fatal("换新 dataKey 后旧密文应当解密失败（GCM 认证错），实际成功")
	}

	// 新密钥下写入的数据在新密钥下往返一致，且不再受 jwt 密钥影响
	ct2, err := crypto.Encrypt("sk-new", fresh.DataEncryptKey())
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if pt, err := crypto.Decrypt(ct2, fresh.DataEncryptKey()); err != nil || pt != "sk-new" {
		t.Fatalf("新 dataKey 往返失败: pt=%q err=%v", pt, err)
	}
}

// ---- 回退 WARN 双向钉住（变异锚点：删 WARN 块或改触发条件必有格红）----

func TestFallbackWarnEmitted(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	c := &Config{}
	c.Jwt.EncryptKey = "jwt-key"
	c.warnInsecureDefaults()
	if !strings.Contains(buf.String(), "security.dataKey.fallback") {
		t.Fatal("dataKey 留空时必须输出密钥未分离 WARN")
	}

	buf.Reset()
	c.Security.DataKey = "separated"
	c.warnInsecureDefaults()
	if strings.Contains(buf.String(), "security.dataKey.fallback") {
		t.Fatal("配置独立 dataKey 后不应再有回退 WARN")
	}
}

// ---- Load 级：dataKeyFromFile 接线（变异 M3 补网：判据函数直测对 Load 接线
// 不可见——判据被偷换成空值也算取自 yaml 时，只有 Load 级断言能红）----

func TestLoadDataKeyFromFileWiring(t *testing.T) {
	// 空值语义：仓库 config.yaml 的 dataKey 留空（功能关），Load 后不得标记为取自文件
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Security.DataKey != "" {
		t.Fatalf("前提失效：仓库 config.yaml 的 dataKey 应为空，实际 %q", c.Security.DataKey)
	}
	if c.dataKeyFromFile {
		t.Fatal("空 dataKey 不得判定为取自 config.yaml（留空=文档化正常态）")
	}

	// 真值语义：yaml 带 dataKey 真值 → Load 后必须标记（触发 insecureDefaults 告警）
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "jwt:\n  encryptKey: \"\"\nsecurity:\n  dataKey: \"truth-in-yaml\"\n"
	if err := os.WriteFile(filepath.Join(tmp, "config", "config.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	pwd, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(pwd) }()
	c2, err := Load()
	if err != nil {
		t.Fatalf("Load(tmp): %v", err)
	}
	if c2.Security.DataKey != "truth-in-yaml" {
		t.Fatalf("yaml 真值未加载: %q", c2.Security.DataKey)
	}
	if !c2.dataKeyFromFile {
		t.Fatal("yaml 真值必须判定为取自 config.yaml（insecureDefaults 告警前提）")
	}
}

// ---- Load 级：env 覆盖生效 + yaml 真值入库判断 ----

func TestLoadSecurityDataKeyEnvOverride(t *testing.T) {
	t.Setenv("AIARSENAL_SECURITY_DATA_KEY", "env-injected-data-key")
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Security.DataKey != "env-injected-data-key" {
		t.Fatalf("env 覆盖未生效: %q", c.Security.DataKey)
	}
	if c.DataEncryptKey() != "env-injected-data-key" {
		t.Fatal("DataEncryptKey 未取到 env 注入值")
	}
	if c.dataKeyFromFile {
		t.Fatal("env 注入值不应被判定为取自 config.yaml")
	}
}
