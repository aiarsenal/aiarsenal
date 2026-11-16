package service

import (
	"context"
	"errors"
	"testing"
)

// F-002 第二轮审计缺口 2：候选用户名选择的行为级表测。
func TestPickAdminName(t *testing.T) {
	// hi 空闲 → hi
	name, err := pickAdminName(context.Background(), func(_ context.Context, name string) (bool, error) {
		return name != "hi", nil
	})
	if err != nil || name != "hi" {
		t.Fatalf("want hi, got %v/%v", name, err)
	}
	// hi 占用 → admin 顺延
	name, err = pickAdminName(context.Background(), func(_ context.Context, name string) (bool, error) {
		return name == "hi", nil
	})
	if err != nil || name != "admin" {
		t.Fatalf("want admin 顺延, got %v/%v", name, err)
	}
	// hi+admin 占用 → admin2；hi+admin+admin2 占用 → admin3（深顺延）
	taken := map[string]bool{"hi": true, "admin": true}
	name, err = pickAdminName(context.Background(), func(_ context.Context, name string) (bool, error) {
		return taken[name], nil
	})
	if err != nil || name != "admin2" {
		t.Fatalf("want admin2 顺延, got %v/%v", name, err)
	}
	taken["admin2"] = true
	name, err = pickAdminName(context.Background(), func(_ context.Context, name string) (bool, error) {
		return taken[name], nil
	})
	if err != nil || name != "admin3" {
		t.Fatalf("want admin3 顺延, got %v/%v", name, err)
	}
	// 全占用 → 报错
	if _, err = pickAdminName(context.Background(), func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}); err == nil {
		t.Fatal("全占用应报错")
	}
	// 查询异常透传
	if _, err = pickAdminName(context.Background(), func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("db down")
	}); err == nil {
		t.Fatal("查询异常应透传")
	}
}
