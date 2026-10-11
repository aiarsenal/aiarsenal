package auth

import (
	"crypto/md5"
	"encoding/hex"
)

// md5Hex 标准库 md5 十六进制
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// EncryptPassword 复刻旧 libUtils.EncryptPassword：md5(md5(password)+md5(salt))
// 兼容存量用户密码，不加盐强化。
func EncryptPassword(password, salt string) string {
	return md5Hex(md5Hex(password) + md5Hex(salt))
}
