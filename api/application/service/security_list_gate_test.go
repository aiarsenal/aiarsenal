package service

import (
	"context"
	"testing"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// F-017b A-01 接线测试：/skill/security/list 的 ID oracle 收口——变异锚点为
// 「删空 VersionId 拒绝分支」（跨技能枚举复活）与「删技能可读判定」。

type fakeSecRepoList struct {
	repository.ISkillSecurityRepository
	rows []*entity.SkillSecurityAudit
}

func (f *fakeSecRepoList) ListAudits(ctx context.Context, versionId, status string, pageNum, pageSize int) ([]*entity.SkillSecurityAudit, int64, error) {
	return f.rows, int64(len(f.rows)), nil
}

type fakeSecSkillRepo struct {
	repository.ISkillRepository
	skills map[string]*entity.Skill
}

func (f *fakeSecSkillRepo) Get(ctx context.Context, skillId string) (*entity.Skill, error) {
	if sk, ok := f.skills[skillId]; ok {
		return sk, nil
	}
	return nil, context.DeadlineExceeded
}
func (f *fakeSecSkillRepo) GetVersion(ctx context.Context, versionId string) (*entity.SkillVersion, error) {
	if versionId == "v-of-u2-priv" {
		return &entity.SkillVersion{VersionId: versionId, SkillId: "u2priv"}, nil
	}
	if versionId == "v-of-viewer" {
		return &entity.SkillVersion{VersionId: versionId, SkillId: "viewer-own"}, nil
	}
	return nil, context.DeadlineExceeded
}
func (f *fakeSecSkillRepo) ListMembers(ctx context.Context, namespaceId string) ([]*entity.SkillNamespaceMember, error) {
	return nil, nil
}

func TestSecurityListIDOracleGate(t *testing.T) {
	svc := NewSkillSecurityService(
		&fakeSecRepoList{rows: []*entity.SkillSecurityAudit{{AuditId: "a1"}}},
		&fakeSecSkillRepo{skills: map[string]*entity.Skill{
			"u2priv":     {SkillId: "u2priv", OwnerId: "u2", Visibility: 1, NamespaceId: "ns-x"},
			"viewer-own": {SkillId: "viewer-own", OwnerId: "viewer", Visibility: 1, NamespaceId: "ns-x"},
		}},
		&fakePkgNS{memberIds: map[string]bool{}},
		nil,
	)
	// 变异格：删空 VersionId 拒绝分支 → 非超管跨技能枚举复活，本格红
	if _, _, err := svc.ListAudits(context.Background(), "", "", 1, 20, "viewer", false); err == nil {
		t.Fatal("非超管空 VersionId 枚举应拒——ID oracle 收口被拆")
	}
	// 他人 PRIVATE 技能的审计 → 未找到
	if _, _, err := svc.ListAudits(context.Background(), "v-of-u2-priv", "", 1, 20, "viewer", false); err == nil {
		t.Fatal("他人 PRIVATE 技能的审计应拒")
	}
	// 自己 PRIVATE 技能的审计 → 放行
	if _, _, err := svc.ListAudits(context.Background(), "v-of-viewer", "", 1, 20, "viewer", false); err != nil {
		t.Fatalf("属主应可读自己技能的审计: %v", err)
	}
	// 超管空 VersionId → 放行（管理面）
	if _, _, err := svc.ListAudits(context.Background(), "", "", 1, 20, "anyone", true); err != nil {
		t.Fatalf("超管跨技能枚举应放行: %v", err)
	}
}
