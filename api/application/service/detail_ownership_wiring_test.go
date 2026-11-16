package service

import (
	"context"
	"testing"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// F-017b 接线层测试（#69 审计变异 M3b/M4 的杀手）：谓词表测只钉纯函数，
// service.Get 的成员扫描/超管传参链路要用 fake repo 行为级验证。

type fakeNSRepo struct {
	repository.ISkillNamespaceRepository
	ns        *entity.SkillNamespace
	memberIds map[string]bool
}

func (f *fakeNSRepo) Get(ctx context.Context, namespaceId string) (*entity.SkillNamespace, error) {
	if f.ns == nil || f.ns.NamespaceId != namespaceId {
		return nil, context.DeadlineExceeded
	}
	return f.ns, nil
}
func (f *fakeNSRepo) ListMembers(ctx context.Context, namespaceId string) ([]*entity.SkillNamespaceMember, error) {
	var out []*entity.SkillNamespaceMember
	for uid := range f.memberIds {
		out = append(out, &entity.SkillNamespaceMember{UserId: uid})
	}
	return out, nil
}

func TestSkillNamespaceGetOwnership(t *testing.T) {
	svc := NewSkillNamespaceService(&fakeNSRepo{
		ns:        &entity.SkillNamespace{NamespaceId: "ns1", CreatedBy: "creator"},
		memberIds: map[string]bool{"mate": true},
	})
	if _, err := svc.Get(context.Background(), "ns1", "creator", false); err != nil {
		t.Fatalf("创建者应可读: %v", err)
	}
	if _, err := svc.Get(context.Background(), "ns1", "mate", false); err != nil {
		t.Fatalf("成员应可读: %v", err)
	}
	if _, err := svc.Get(context.Background(), "ns1", "anyone", true); err != nil {
		t.Fatalf("超管应可读: %v", err)
	}
	// 变异 M3b（成员扫描短路）与 M4 类（isAdmin 恒真）在本格必红
	if _, err := svc.Get(context.Background(), "ns1", "stranger", false); err == nil {
		t.Fatal("陌生应「未找到」——成员扫描/谓词被短路")
	}
}

type fakeMcpRepo struct {
	repository.IMcpServerRepository
	servers map[string]*entity.McpServer
}

func (f *fakeMcpRepo) Get(ctx context.Context, id string) (*entity.McpServer, error) {
	if sv, ok := f.servers[id]; ok {
		return sv, nil
	}
	return nil, context.DeadlineExceeded
}

func TestMcpServerGetOwnership(t *testing.T) {
	svc := NewMcpServerService(&fakeMcpRepo{
		servers: map[string]*entity.McpServer{
			"srv1": {ServerId: "srv1", CreatedBy: "owner"},
		},
	})
	if _, err := svc.Get(context.Background(), "srv1", "owner", false); err != nil {
		t.Fatalf("创建者应可读: %v", err)
	}
	if _, err := svc.Get(context.Background(), "srv1", "anyone", true); err != nil {
		t.Fatalf("超管应可读: %v", err)
	}
	if _, err := svc.Get(context.Background(), "srv1", "stranger", false); err == nil {
		t.Fatal("陌生应「未找到」——接线层 isAdmin 恒真变异在此必红（M4 杀手）")
	}
}
