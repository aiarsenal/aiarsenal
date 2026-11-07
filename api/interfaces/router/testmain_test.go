package router

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestMain chdir 到 api/：auditChain 里的 CORS 中间件经 config.Get() 读跨域白名单，
// viper AddConfigPath 相对 cwd，本测试目录下找不到 config/config.yaml（与
// service/config/middleware 包的 chdir 模式同源）。
func TestMain(m *testing.M) {
	if _, filename, _, ok := runtime.Caller(0); ok {
		_ = os.Chdir(filepath.Join(filepath.Dir(filename), "..", ".."))
	}
	os.Exit(m.Run())
}
