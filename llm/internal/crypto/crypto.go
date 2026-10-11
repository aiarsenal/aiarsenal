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

// AES-256-GCM，与 api/infrastructure/crypto 保持一致。
// router 用 Decrypt 解密 aa_llm_credential.credential_values。

func key32(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

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

// HashToken sha256(明文 key) hex——与 api 落库一致。
func HashToken(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

var _ = io.ReadFull
var _ = rand.Reader
