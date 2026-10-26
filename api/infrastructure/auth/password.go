package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"hash"
	"log/slog"
	"strconv"
	"strings"

	"aiarsenal/config"
)

// password.go — 口令派生与校验（PBKDF2 框架，存储串自描述算法与参数）。
//
// 存储串格式：$<算法id>$<迭代次数>$<盐b64>$<派生键b64>，如
//
//	$pbkdf2-sm3$100000$<22 字符>$<43 字符>
//
// 盐/派生键用 base64 RawStdEncoding（RFC 4648 无 padding，对齐 PHC string format
// 惯例）。该格式是 PHC 的退化子集（无版本段、无参数名段）：唯一消费者是本解析器、
// 无跨库互操作需求，砍掉两个段把解析器控制在 ~30 行（argon2id 的 parse 要 ~75 行）。
//
// 「换档不迁移」：校验时算法与迭代次数一律取自存量串本身，与当前配置无关——部署方
// 换 algorithm/iterations 只影响之后新产生的口令，存量照常校验。
//
// 硬切换（本 PR）：md5 旧路径（EncryptPassword）已删除，md5 时代的存量串按未知算法
// 处理一律校验失败——升级部署必须先执行 README 的存量口令重置 SOP（行为变化之首）。

// registryAlgo 注册表条目：newFn 供 PBKDF2 与 HMAC 复用，pepper 标记派生前是否与
// 系统侧密钥混合。
type registryAlgo struct {
	newFn  func() hash.Hash
	keyLen int
	pepper bool
}

// hashRegistry 算法注册表：新增档位只改这里一行。SM3 为本地实现（见 sm3.go），
// SHA-512 用标准库。pepper=true 的档位在派生前先把口令与系统侧密钥（HMAC）混合：
// 拖库者拿到库拿不到 env 密钥，离线爆破不可行；只有服务器连同密钥一起失守才退回
// 拼迭代数（README 写明这层权衡）。peppered 与非 peppered 是不同 id——存量串按串
// 内 id 校验，开关 pepper 不影响既有口令。
var hashRegistry = map[string]registryAlgo{
	"pbkdf2-sm3":             {sm3New, 32, false},
	"pbkdf2-sha512":          {sha512.New, 64, false},
	"pbkdf2-sm3-peppered":    {sm3New, 32, true},
	"pbkdf2-sha512-peppered": {sha512.New, 64, true},
}

// saltLen 新口令盐长度（字节）。RFC 8018 要求 ≥8，这里取 16。
const saltLen = 16

// storedIterCap 校验侧接受的串内迭代上限：正常路径迭代数只来自 config，但只要存在
// 任何能把任意串写进口令列的路径（SQL 注入、误操作导回），无上限的迭代数就是逐请求
// CPU 放大面（1 亿迭代 ≈ 155s/次）。上限远高于任何合理配置，只截断滥用。
const storedIterCap = 2_000_000

// b64 存储串内二进制段编码（无 padding，PHC 惯例）。
var b64 = base64.RawStdEncoding

// effectiveAlgoId 当前配置对应的存储串算法 id：配置了 pepper 就落到 peppered 档。
func effectiveAlgoId(c *config.Config) string {
	if c.Password.Hash.Pepper != "" {
		return c.Password.Hash.Algorithm + "-peppered"
	}
	return c.Password.Hash.Algorithm
}

// applyPepper peppered 档位把口令替换为 HMAC(系统密钥, 口令) 再进 PBKDF2——
// 密钥不落库，拖库场景下缺一味料就无法离线枚举。非 peppered 档原样返回。
func applyPepper(a registryAlgo, password string) (string, error) {
	if !a.pepper {
		return password, nil
	}
	pepper := config.Get().Password.Hash.Pepper
	if pepper == "" {
		// 配置侧移除/丢失 pepper 后，peppered 存量串在数学上不可校验——明确拒绝，
		// 让运维从 ERROR 日志立刻知道是配置丢了，而不是口令错了
		return "", errors.New("口令串为 peppered 档位但系统未配置 pepper")
	}
	mac := hmac.New(a.newFn, []byte(pepper))
	mac.Write([]byte(password))
	return string(mac.Sum(nil)), nil
}

// HashPassword 用当前配置档位派生口令存储串。错误只可能来自配置与随机源，由
// 调用方（建档/改密路径）透传给请求方。
func HashPassword(password string) (string, error) {
	c := config.Get()
	id := effectiveAlgoId(c)
	algo, ok := hashRegistry[id]
	if !ok {
		// Load 时已归一化，这里再拦一道：未知算法宁可拒绝建档，也不落一个验不了的串
		return "", errors.New("口令哈希算法配置非法")
	}
	pw, err := applyPepper(algo, password)
	if err != nil {
		return "", err
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(algo.newFn, pw, salt, c.Password.Hash.Iterations, algo.keyLen)
	if err != nil {
		return "", err
	}
	return "$" + id + "$" + strconv.Itoa(c.Password.Hash.Iterations) +
		"$" + b64.EncodeToString(salt) + "$" + b64.EncodeToString(key), nil
}

// VerifyPassword 校验口令与存量串。算法/迭代次数/盐全部取自存量串，与当前配置无关
// （「换档不迁移」的机制核心）。任何格式异常（含 md5 时代的旧串、被篡改的段）一律
// false——闸门的零值必须是拒绝。
func VerifyPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 5 || parts[0] != "" {
		return false
	}
	algo, ok := hashRegistry[parts[1]]
	if !ok {
		return false
	}
	iter, err := strconv.Atoi(parts[2])
	if err != nil || iter <= 0 || iter > storedIterCap {
		return false
	}
	salt, err := b64.DecodeString(parts[3])
	// 盐至少 8 字节（RFC 8018 下限）：空/超短盐在格式层就拒，不浪费一次派生
	if err != nil || len(salt) < 8 {
		return false
	}
	want, err := b64.DecodeString(parts[4])
	// 哈希段长度必须等于算法键长：既挡格式畸形，也挡「截短哈希段仍可校验」的
	// 非标准串（PBKDF2 输出前缀与全长派生共享前缀，放行会让存储串长度失去意义）
	if err != nil || len(want) != algo.keyLen {
		return false
	}
	pw, err := applyPepper(algo, password)
	if err != nil {
		// peppered 串遇到未配置 pepper：配置丢失，不是口令错——ERROR 级让运维立刻看见
		slog.Error("peppered 口令串无法校验", "err", err)
		return false
	}
	key, err := pbkdf2.Key(algo.newFn, pw, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(key, want) == 1
}
