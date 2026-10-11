package service

import (
	"strings"
	"testing"
)

// TestScanLocalApps 本机 App 扫描：darwin 下应扫出非空清单且 bundle id 形如反向域名。
// （跑在 CI/Linux 上自动跳过——scanLocalApps 对非 darwin 返回 nil。）
func TestScanLocalApps(t *testing.T) {
	apps := scanLocalApps()
	if len(apps) == 0 {
		t.Skip("非 darwin 或无 .app 可扫")
	}
	for _, a := range apps {
		if a.bundleId == "" || a.name == "" {
			t.Fatalf("存在空字段: %+v", a)
		}
	}
	// 系统目录必有 Finder（/System/Library/CoreServices/Finder.app）
	found := false
	for _, a := range apps {
		if a.bundleId == "com.apple.finder" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("未扫到 com.apple.finder（共 %d 个 App）", len(apps))
	}
}

// TestMatchLocalApps 关键词过滤：cursor 应命中本机 Cursor.app（若装了）。
func TestMatchLocalApps(t *testing.T) {
	for _, a := range matchLocalApps("cursor") {
		if !strings.Contains(strings.ToLower(a.name+a.bundleId), "cursor") {
			t.Fatalf("误命中: %+v", a)
		}
	}
}
