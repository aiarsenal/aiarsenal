package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"aiarsenal/domain/entity"
)

// ---- McpKeyService.Delete 存在性确认（F-033）----
// 变异锚点：删存在性预检循环 → 缺失格红；删空清单拒绝 → 空清单格红。

type fakeBearerKeyRepo struct {
	existing map[string]*entity.McpBearerKey
	deleted  [][]string
}

func (f *fakeBearerKeyRepo) List(ctx context.Context, name string, pn, ps int) ([]*entity.McpBearerKey, int64, error) {
	return nil, 0, nil
}
func (f *fakeBearerKeyRepo) Get(ctx context.Context, id string) (*entity.McpBearerKey, error) {
	if e, ok := f.existing[id]; ok {
		return e, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeBearerKeyRepo) GetByHash(ctx context.Context, h string) (*entity.McpBearerKey, error) {
	return nil, errors.New("not found")
}
func (f *fakeBearerKeyRepo) Add(ctx context.Context, e *entity.McpBearerKey) (string, error) {
	return "", nil
}
func (f *fakeBearerKeyRepo) Edit(ctx context.Context, e *entity.McpBearerKey) error { return nil }
func (f *fakeBearerKeyRepo) Delete(ctx context.Context, ids []string) error {
	f.deleted = append(f.deleted, ids)
	return nil
}

func TestMcpKeyDeleteExistenceCheck(t *testing.T) {
	repo := &fakeBearerKeyRepo{existing: map[string]*entity.McpBearerKey{
		"key-1": {KeyId: "key-1"},
	}}
	s := NewMcpKeyService(repo)

	if err := s.Delete(context.Background(), nil); err == nil {
		t.Fatal("空清单应拒绝（参数形状错误此前静默成功）")
	}
	err := s.Delete(context.Background(), []string{"key-1", "key-missing"})
	if err == nil || !strings.Contains(err.Error(), "key-missing") {
		t.Fatalf("不存在的 keyId 应拒并指名: %v", err)
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("存在性确认失败时不得落到仓库层删除: %v", repo.deleted)
	}
	if err := s.Delete(context.Background(), []string{"key-1"}); err != nil {
		t.Fatalf("存在的 keyId 应正常删除: %v", err)
	}
	if len(repo.deleted) != 1 || len(repo.deleted[0]) != 1 {
		t.Fatalf("正常删除应透传仓库层: %v", repo.deleted)
	}
}
