package config

import "testing"

// 回归：密钥来源判定只看「生效值是否等于读入 yaml 时的原值」，不比对具体内容，
// 因此 config.yaml 换掉占位值时告警不会失效。

func TestJwtKeyFromConfigFile(t *testing.T) {
	const committedKey = "fc6eab236b50ac1ba29f968e0c80dcea"
	cases := []struct {
		name      string
		effective string
		fromFile  string
		want      bool
	}{
		{
			name:      "环境变量已覆盖，不应告警",
			effective: "9f2c4a1d8e7b6c5f4a3b2c1d0e9f8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f",
			fromFile:  committedKey,
			want:      false,
		},
		{
			name:      "未覆盖，沿用仓库占位值，应告警",
			effective: committedKey,
			fromFile:  committedKey,
			want:      true,
		},
		{
			name:      "yaml 换成自有密钥但未走环境变量，仍应告警",
			effective: "my-own-secret-key",
			fromFile:  "my-own-secret-key",
			want:      true,
		},
		{
			name:      "环境变量与 yaml 恰好同值（值仍随仓库公开），应告警",
			effective: committedKey,
			fromFile:  committedKey,
			want:      true,
		},
		{
			name:      "两边都是空串，算作取自该文件",
			effective: "",
			fromFile:  "",
			want:      true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := jwtKeyFromConfigFile(c.effective, c.fromFile); got != c.want {
				t.Fatalf("jwtKeyFromConfigFile(%q, %q) = %v，期望 %v", c.effective, c.fromFile, got, c.want)
			}
		})
	}
}

// 上报口径：只有确实沿用了仓库默认值时才出现在 insecureDefaults，避免误报。
func TestInsecureDefaults(t *testing.T) {
	if got := (&Config{}).insecureDefaults(); len(got) != 0 {
		t.Fatalf("密钥已由环境变量覆盖时不应上报，实际 %v", got)
	}
	got := (&Config{jwtKeyFromFile: true}).insecureDefaults()
	if len(got) != 1 || got[0] != "jwt.encryptKey" {
		t.Fatalf("沿用仓库默认值时应上报 jwt.encryptKey，实际 %v", got)
	}
}

// pepper 来源判定与 jwt 同判据但空值语义不同：默认留空是文档化的正常态，
// 只有非空真值进了仓库 yaml 才值得告警。
func TestPepperFromConfigFile(t *testing.T) {
	const committedPepper = "committed-pepper-value"
	cases := []struct {
		name      string
		effective string
		fromFile  string
		want      bool
	}{
		{"默认留空（功能关），不告警", "", "", false},
		{"yaml 真值未走环境变量，应告警", committedPepper, committedPepper, true},
		{"环境变量已覆盖 yaml 真值，不告警", "real-pepper-from-env-0123456789abcdef", committedPepper, false},
		{"环境变量注入（yaml 本就为空），不告警", "real-pepper-from-env-0123456789abcdef", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pepperFromConfigFile(c.effective, c.fromFile); got != c.want {
				t.Fatalf("pepperFromConfigFile(%q, %q) = %v，期望 %v", c.effective, c.fromFile, got, c.want)
			}
		})
	}
}
