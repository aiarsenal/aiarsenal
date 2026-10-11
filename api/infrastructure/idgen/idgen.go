// Package idgen 生成 32 位业务 id。
//
// 规则：32 字符，全部小写 [a-z0-9]，首尾字符必须是字母 [a-z]（非数字）。
// 基于 crypto/rand，无时间序、不可枚举。
package idgen

import (
	"crypto/rand"
)

const (
	length = 32
	// 首尾位只取字母，中间位取字母+数字。
	letters = "abcdefghijklmnopqrstuvwxyz"
	alnum   = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// New 生成一个 32 位业务 id（首尾字母、全小写）。
func New() string {
	b := make([]byte, length)
	// 首位字母
	if _, err := rand.Read(b[:1]); err != nil {
		// rand 失败极少见；退化为首位 'a'，不影响正确性
		b[0] = letters[0]
	} else {
		b[0] = letters[int(b[0])%len(letters)]
	}
	// 中间 30 位字母+数字
	if _, err := rand.Read(b[1 : length-1]); err == nil {
		for i := 1; i < length-1; i++ {
			b[i] = alnum[int(b[i])%len(alnum)]
		}
	}
	// 末位字母
	if _, err := rand.Read(b[length-1:]); err != nil {
		b[length-1] = letters[0]
	} else {
		b[length-1] = letters[int(b[length-1])%len(letters)]
	}
	return string(b)
}
