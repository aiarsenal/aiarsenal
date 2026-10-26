package auth

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"aiarsenal/config"
)

// TestMain 把工作目录切到 api/：config.Get() 按 config/config.yaml 相对路径加载，
// 单测要拿到与运行时一致的归一化默认值（其余用例不改配置的也顺带受益）。
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// TestHashPasswordRoundTrip 默认档位派生→校验往返：格式、段数、迭代数、b64 合法性。
func TestHashPasswordRoundTrip(t *testing.T) {
	stored, err := HashPassword("Aiarsenal@2026")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(stored, "$")
	if len(parts) != 5 || parts[0] != "" {
		t.Fatalf("存储串段数错误：%q", stored)
	}
	if parts[1] != "pbkdf2-sm3" {
		t.Fatalf("默认算法应为 pbkdf2-sm3，实际 %q", parts[1])
	}
	if parts[2] != strconv.Itoa(config.Get().Password.Hash.Iterations) {
		t.Fatalf("串内迭代数应与配置一致：%q vs %d", parts[2], config.Get().Password.Hash.Iterations)
	}
	if !VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("正确口令校验失败")
	}
	if VerifyPassword("wrong-password", stored) {
		t.Fatal("错误口令竟然通过")
	}
}

// TestHashPasswordSha512Algo 切档后新口令用新算法落串，且新串仍可校验。
func TestHashPasswordSha512Algo(t *testing.T) {
	c := config.Get()
	oldAlgo, oldIter := c.Password.Hash.Algorithm, c.Password.Hash.Iterations
	defer func() { c.Password.Hash.Algorithm, c.Password.Hash.Iterations = oldAlgo, oldIter }()
	c.Password.Hash.Algorithm = "pbkdf2-sha512"
	c.Password.Hash.Iterations = 1000 // 测试提速；串内自描述，校验不受影响

	stored, err := HashPassword("Aiarsenal@2026")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$pbkdf2-sha512$1000$") {
		t.Fatalf("切档后存储串前缀错误：%q", stored)
	}
	if !VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("sha512 档位往返校验失败")
	}
	// 回到默认档后，sha512 存量串照常校验（换档不迁移）
	c.Password.Hash.Algorithm, c.Password.Hash.Iterations = oldAlgo, oldIter
	if !VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("换回默认档后存量 sha512 串必须照常校验")
	}
}

// TestVerifyPasswordRejectsMalformed 畸形串/旧格式串/篡改段一律拒绝——闸门零值必须
// 是拒绝。特别覆盖 md5 时代存量串（硬切换语义：旧格式必须死）。
func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	stored, err := HashPassword("Aiarsenal@2026")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(stored, "$")

	cases := []struct {
		name   string
		stored string
	}{
		{"空串", ""},
		{"缺段", "$pbkdf2-sm3$60000$abcdef"},
		{"裸 md5 旧串（32 位 hex）", "9b1d4744de72012b9390a000e6a8070a"},
		{"未知算法", "$pbkdf2-md5$60000$" + parts[3] + "$" + parts[4]},
		{"迭代数非数字", "$pbkdf2-sm3$abc$" + parts[3] + "$" + parts[4]},
		{"迭代数为零", "$pbkdf2-sm3$0$" + parts[3] + "$" + parts[4]},
		{"迭代数负", "$pbkdf2-sm3$-1$" + parts[3] + "$" + parts[4]},
		{"迭代数超上限（CPU 放大面截断）", "$pbkdf2-sm3$100000000$" + parts[3] + "$" + parts[4]},
		{"盐段为空", "$pbkdf2-sm3$60000$$" + parts[4]},
		{"盐段短于 RFC 8018 下限", "$pbkdf2-sm3$60000$" + b64.EncodeToString(make([]byte, 4)) + "$" + parts[4]},
		{"盐段损坏", "$pbkdf2-sm3$60000$!!notb64!!$" + parts[4]},
		{"哈希段损坏", "$pbkdf2-sm3$60000$" + parts[3] + "$!!notb64!!"},
		{"哈希段截短（长度不符直接不等）", "$pbkdf2-sm3$60000$" + parts[3] + "$" + parts[4][:8]},
	}
	for _, tc := range cases {
		if VerifyPassword("Aiarsenal@2026", tc.stored) {
			t.Fatalf("%s 竟然通过校验", tc.name)
		}
	}
}

// TestVerifyPasswordUnknownAlgoAfterRegistryMutate 反向变异：把 sm3 从注册表摘掉，
// 存量 sm3 串必须拒绝——证明「能认」也「不乱认」（SKILL：只做正向变异证据只有一半）。
func TestVerifyPasswordUnknownAlgoAfterRegistryMutate(t *testing.T) {
	stored, err := HashPassword("Aiarsenal@2026")
	if err != nil {
		t.Fatal(err)
	}
	saved := hashRegistry["pbkdf2-sm3"]
	delete(hashRegistry, "pbkdf2-sm3")
	defer func() { hashRegistry["pbkdf2-sm3"] = saved }()
	if VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("注册表摘除算法后存量串必须拒绝")
	}
}

// BenchmarkHashPasswordSM3 口令派生基准：部署方在目标机上用它复测定档——
//
//	go test ./infrastructure/auth/ -bench=HashPasswordSM3 -benchtime=5x
//
// 单次耗时应在 50~100ms；偏低则上调 config.yaml 的 password.hash.iterations，
// 实测值回写 README（定档口径见 README「鉴权与安全」）。
func BenchmarkHashPasswordSM3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := HashPassword("Benchmark@2026pw"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHashPasswordSha512 备选档基准：与 SM3 档同口径，供部署方在目标机上
// 对比两档延迟后定档（README「鉴权与安全」）。
func BenchmarkHashPasswordSha512(b *testing.B) {
	c := config.Get()
	oldAlgo, oldIter := c.Password.Hash.Algorithm, c.Password.Hash.Iterations
	c.Password.Hash.Algorithm, c.Password.Hash.Iterations = "pbkdf2-sha512", 220000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := HashPassword("Benchmark@2026pw"); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	c.Password.Hash.Algorithm, c.Password.Hash.Iterations = oldAlgo, oldIter
}

// setPepper 临时配置 pepper 并在用例结束后恢复（全局单例，测试间互不污染）。
func setPepper(t *testing.T, pepper string) {
	t.Helper()
	old := config.Get().Password.Hash.Pepper
	config.Get().Password.Hash.Pepper = pepper
	t.Cleanup(func() { config.Get().Password.Hash.Pepper = old })
}

// TestPepperedRoundTrip 配置 pepper 后：新口令落 peppered 档、正确口令可验、
// 错误口令拒；同口令在掺与不掺下串不同（pepper 真实生效）；关掉 pepper 后
// peppered 串拒绝（配置丢失的明确失败），重新配上又恢复。
func TestPepperedRoundTrip(t *testing.T) {
	setPepper(t, "prverify-pepper-0123456789abcdef0123456789abcdef")

	stored, err := HashPassword("Aiarsenal@2026")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "$pbkdf2-sm3-peppered$") {
		t.Fatalf("配置 pepper 后应落 peppered 档：%q", stored[:30])
	}
	if !VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("peppered 串正确口令校验失败")
	}
	if VerifyPassword("Aiarsenal@2026x", stored) {
		t.Fatal("peppered 串错误口令通过")
	}

	// 未掺 pepper 的同口令串必须不同（pepper 真实参与派生）
	setPepper(t, "")
	plain, err := HashPassword("Aiarsenal@2026")
	if err != nil {
		t.Fatal(err)
	}
	if plain == stored {
		t.Fatal("掺 pepper 与不掺生成了相同串")
	}
	if !strings.HasPrefix(plain, "$pbkdf2-sm3$") {
		t.Fatalf("无 pepper 时应回落普通档：%q", plain[:24])
	}

	// 存量普通串在 pepper 开启后照常校验（开关不影响既有账号）
	if !VerifyPassword("Aiarsenal@2026", plain) {
		t.Fatal("开启 pepper 后存量普通串必须照常校验")
	}

	// peppered 串在 pepper 缺失下拒绝（配置丢失 = 明确失败，不是放行）
	setPepper(t, "prverify-pepper-0123456789abcdef0123456789abcdef")
	if !VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("pepper 配回后应恢复校验")
	}
	setPepper(t, "")
	if VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("pepper 缺失时 peppered 串必须拒绝")
	}
}

// TestPepperedRoundTrip 的收尾追加两格（审计指出的验证侧缺口）：
// 换值到不同 pepper 时旧 peppered 串必须拒绝（「不乱认」的半边证据），
// 以及 sha512-peppered 档整行往返 + 换值拒绝。
func TestPepperedRotateAndSha512(t *testing.T) {
	setPepper(t, "prverify-pepper-0123456789abcdef0123456789abcdef")
	stored, err := HashPassword("Aiarsenal@2026")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("原 pepper 下应通过")
	}
	// 换值：旧串必须拒绝
	setPepper(t, "prverify-pepper-ffffffffffffffffffffffffffffffff")
	if VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("换 pepper 后旧串必须拒绝（不乱认）")
	}
	// 换回原值恢复
	setPepper(t, "prverify-pepper-0123456789abcdef0123456789abcdef")
	if !VerifyPassword("Aiarsenal@2026", stored) {
		t.Fatal("恢复原 pepper 后应重新通过")
	}
	// sha512-peppered 档整行
	c := config.Get()
	oldAlgo := c.Password.Hash.Algorithm
	defer func() { c.Password.Hash.Algorithm = oldAlgo }()
	c.Password.Hash.Algorithm = "pbkdf2-sha512"
	stored2, err := HashPassword("Aiarsenal@2026")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored2, "$pbkdf2-sha512-peppered$") {
		t.Fatalf("sha512 档应落 sha512-peppered：%q", stored2[:36])
	}
	if !VerifyPassword("Aiarsenal@2026", stored2) {
		t.Fatal("sha512-peppered 往返失败")
	}
	setPepper(t, "prverify-pepper-ffffffffffffffffffffffffffffffff")
	if VerifyPassword("Aiarsenal@2026", stored2) {
		t.Fatal("sha512-peppered 换值后旧串必须拒绝")
	}
}
