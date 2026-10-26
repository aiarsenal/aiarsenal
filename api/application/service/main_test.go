package service

import (
	"os"
	"testing"
)

// TestMain 把工作目录切到 api/：Add/ResetPwd 现在会经 auth 的口令策略与哈希，
// 它们读 config.Get()（按 config/config.yaml 相对路径加载），单测要拿到与运行时
// 一致的配置。与 infrastructure/auth 包的 TestMain 同一机制。
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
