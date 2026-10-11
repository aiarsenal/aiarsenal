package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

// LLM 网关相关加解密工具：
// - RandomKey：生成 virtual key 明文（前缀 + 48 位随机字符，含特殊字符 -_.~）
// - HashToken：sha256(明文) hex，落库与查询均用哈希
// - Encrypt/Decrypt：AES-256-GCM，用于 provider 凭据密文落库

// keyAlphabet 虚拟 key 随机字符表：字母+数字+特殊字符 -_.~（均为 URL/header 安全字符）。
var keyAlphabet = []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_.~")

// RandomKey 生成 virtual key 明文。prefix 如 "sk-aa"。48 位随机字符，含特殊字符。
func RandomKey(prefix string) (string, error) {
	const n = 48
	rb := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, rb); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i, v := range rb {
		out[i] = keyAlphabet[int(v)%len(keyAlphabet)]
	}
	return prefix + "-" + string(out), nil
}

// HashToken 返回明文 key 的 sha256 hex。
func HashToken(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// key32 把任意长度密钥 sha256 成 32 字节（AES-256）。
func key32(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// Encrypt AES-256-GCM 加密，返回 base64？这里直接返回 hex（落 jsonb 字符串列）。
func Encrypt(plaintext, secret string) (string, error) {
	block, err := aes.NewCipher(key32(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ct), nil
}

// Decrypt 解密 Encrypt 产出的 hex 密文。
func Decrypt(cipherHex, secret string) (string, error) {
	if cipherHex == "" {
		return "", nil
	}
	raw, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key32(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
