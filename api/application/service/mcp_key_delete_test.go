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
	// 模拟真实删除：从 existing 移除（TOCTOU 复核用 existing 是否仍在判定删后状态）
	for _, id := range ids {
		delete(f.existing, id)
	}
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

// ---- F-033 补口（TOCTOU 收口）：删后复核——并发删除使 repo.Delete 零行生效时，
// 服务层必须报「删除未生效」而非静默成功。变异锚点：删掉复核循环 → 本格红。 ----

type staleFakeRepo struct {
	fakeBearerKeyRepo
}

// Delete 不移除 existing（模拟预检后、删除前被并发变更的极端形态）
func (f *staleFakeRepo) Delete(ctx context.Context, ids []string) error {
	f.deleted = append(f.deleted, ids)
	return nil
}

func TestMcpKeyDeletePostVerify(t *testing.T) {
	repo := &staleFakeRepo{fakeBearerKeyRepo: fakeBearerKeyRepo{
		existing: map[string]*entity.McpBearerKey{"k1": {KeyId: "k1"}},
	}}
	svc := NewMcpKeyService(repo)
	err := svc.Delete(context.Background(), []string{"k1"})
	if err == nil || !strings.Contains(err.Error(), "删除未生效") {
		t.Fatalf("删后仍存在的密钥必须报删除未生效: %v", err)
	}
	// 对照：正常删除（fake 移除 existing）→ 复核通过
	ok := &fakeBearerKeyRepo{existing: map[string]*entity.McpBearerKey{"k2": {KeyId: "k2"}}}
	svc2 := NewMcpKeyService(ok)
	if err := svc2.Delete(context.Background(), []string{"k2"}); err != nil {
		t.Fatalf("正常删除不应报错: %v", err)
	}
}
