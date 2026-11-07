package middleware

import (
	"testing"

	"aiarsenal/domain/model"
)

// ---- menuFor URL→菜单映射（变异锚点：删斜杠归一必有格红；F-14 的闸门恢复靠它）----

func menuSlice(names ...string) []*model.MenuInfoRes {
	out := make([]*model.MenuInfoRes, 0, len(names))
	for _, n := range names {
		out = append(out, &model.MenuInfoRes{Name: n, Id: "menu-" + n})
	}
	return out
}

func TestMenuFor(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		names   []string
		wantHit string // 期望命中的原始菜单名；""=不应命中
	}{
		{
			name:    "斜杠前缀记录命中归一后的路径（F-14 主场景：role/edit 闸门恢复）",
			url:     "api/v1/system/role/edit",
			names:   []string{"api/v1/system/role/list", "/api/v1/system/role/edit"},
			wantHit: "/api/v1/system/role/edit",
		},
		{
			name:    "无斜杠记录照常命中（回归保护）",
			url:     "api/v1/system/user/edit",
			names:   []string{"api/v1/system/user/edit"},
			wantHit: "api/v1/system/user/edit",
		},
		{
			name:    "带查询串的记录按 ? 前段命中",
			url:     "api/v1/system/user/list",
			names:   []string{"api/v1/system/user/list?accessParams=x"},
			wantHit: "api/v1/system/user/list?accessParams=x",
		},
		{
			name:    "首条命中优先（repo 列表序）",
			url:     "api/v1/x",
			names:   []string{"api/v1/x", "/api/v1/x"},
			wantHit: "api/v1/x",
		},
		{
			name:    "无记录返回 nil（menu==nil 分支的既有语义不变）",
			url:     "api/v1/system/monitor/server",
			names:   []string{"api/v1/system/user/edit"},
			wantHit: "",
		},
		{
			name:    "子路径不误命中（前缀≠全串）",
			url:     "api/v1/system/role/edit/extra",
			names:   []string{"api/v1/system/role/edit"},
			wantHit: "",
		},
		{
			// 旧实现有「菜单全名（含查询串）与 url 等值」的第二分支，menuFor 收口后
			// 删除：名字经查询串剥离后与带 accessParams 的 url 永不相等。方向是收紧
			// 非放松（不命中→menu==nil 既有分支），此处钉住新语义防回弹。
			name:    "名字与 url 均带查询串不命中（旧第二分支已收口）",
			url:     "api/v1/x?a=b",
			names:   []string{"api/v1/x?a=b"},
			wantHit: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := menuFor(tc.url, menuSlice(tc.names...))
			if tc.wantHit == "" {
				if got != nil {
					t.Fatalf("不应命中，实际 %q", got.Name)
				}
				return
			}
			if got == nil || got.Name != tc.wantHit {
				t.Fatalf("got %v want %q", got, tc.wantHit)
			}
		})
	}
}
