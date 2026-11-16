package service

import (
	"context"
	"testing"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// F-017b 接线层行为测试：同级面收口的六类门禁各有变异钉（审计变异 M1/M3/M5c/M6）。

// ---- M1：MCP AllForCaller 的 createdBy 过滤（删过滤=非超管见全量） ----

type fakeMcpRepoForAll struct {
	repository.IMcpServerRepository
	allRows   []*entity.McpServer // 系统路径 All 的全量
	ownRows   []*entity.McpServer // AllByCreator 的过滤结果
	byCreator string
}

func (f *fakeMcpRepoForAll) All(ctx context.Context) ([]*entity.McpServer, error) {
	return f.allRows, nil
}
func (f *fakeMcpRepoForAll) AllByCreator(ctx context.Context, createdBy string) ([]*entity.McpServer, error) {
	f.byCreator = createdBy
	return f.ownRows, nil
}

func TestMcpAllForCallerFilters(t *testing.T) {
	own := []*entity.McpServer{{ServerId: "own"}}
	all := []*entity.McpServer{{ServerId: "own"}, {ServerId: "other"}, {ServerId: "seeded"}}
	svc := NewMcpServerService(&fakeMcpRepoForAll{allRows: all, ownRows: own})

	got, err := svc.AllForCaller(context.Background(), "u1", false)
	if err != nil || len(got) != 1 || got[0].ServerId != "own" {
		t.Fatalf("非超管 /all 应走 AllByCreator 仅见自己: n=%d err=%v", len(got), err)
	}
	got2, _ := svc.AllForCaller(context.Background(), "u1", true)
	if len(got2) != 3 {
		t.Fatalf("超管 /all 应全量: n=%d", len(got2))
	}
	// 变异 M1（AllForCaller 回 repo.All）：非超管格拿到的应是 1 条——上面第一断言即杀。
}

// ---- M3：EnsureSkillReadable（readableGate 的 service 核心；删谓词=陌生可读） ----

type fakePkgRepo struct {
	repository.ISkillRepository
	skills map[string]*entity.Skill
}

func (f *fakePkgRepo) Get(ctx context.Context, skillId string) (*entity.Skill, error) {
	if sk, ok := f.skills[skillId]; ok {
		return sk, nil
	}
	return nil, context.DeadlineExceeded
}

type fakePkgNS struct {
	repository.ISkillNamespaceRepository
	memberIds map[string]bool
}

func (f *fakePkgNS) ListMembers(ctx context.Context, namespaceId string) ([]*entity.SkillNamespaceMember, error) {
	var out []*entity.SkillNamespaceMember
	for uid := range f.memberIds {
		out = append(out, &entity.SkillNamespaceMember{UserId: uid})
	}
	return out, nil
}

func TestEnsureSkillReadable(t *testing.T) {
	svc := NewSkillPackageService(
		&fakePkgRepo{skills: map[string]*entity.Skill{
			"priv": {SkillId: "priv", OwnerId: "owner", Visibility: 1, NamespaceId: "ns1"},
			"pub":  {SkillId: "pub", OwnerId: "owner", Visibility: 0, NamespaceId: "ns1"},
		}},
		&fakePkgNS{memberIds: map[string]bool{"mate": true}},
		nil, nil, nil, nil,
	)
	if err := svc.EnsureSkillReadable(context.Background(), "priv", "owner", false); err != nil {
		t.Fatalf("属主应可读: %v", err)
	}
	if err := svc.EnsureSkillReadable(context.Background(), "priv", "mate", false); err != nil {
		t.Fatalf("成员应可读: %v", err)
	}
	if err := svc.EnsureSkillReadable(context.Background(), "priv", "stranger", true); err != nil {
		t.Fatalf("超管应可读: %v", err)
	}
	if err := svc.EnsureSkillReadable(context.Background(), "priv", "stranger", false); err == nil {
		t.Fatal("陌生读 PRIVATE 应拒——EnsureSkillReadable 谓词被短路（M3 类变异在此必红）")
	}
	if err := svc.EnsureSkillReadable(context.Background(), "pub", "stranger", false); err != nil {
		t.Fatalf("PUBLIC 应可读（市场流）: %v", err)
	}
}

// ---- M5c：ListVersions 归属门禁（删门禁=他人版本 id 枚举复活） ----

type fakeSkillRepoForVer struct {
	repository.ISkillRepository
	skills   map[string]*entity.Skill
	versions map[string][]*entity.SkillVersion
}

func (f *fakeSkillRepoForVer) Get(ctx context.Context, skillId string) (*entity.Skill, error) {
	if sk, ok := f.skills[skillId]; ok {
		return sk, nil
	}
	return nil, context.DeadlineExceeded
}
func (f *fakeSkillRepoForVer) ListVersions(ctx context.Context, skillId string) ([]*entity.SkillVersion, error) {
	return f.versions[skillId], nil
}
func (f *fakeSkillRepoForVer) ListMembers(ctx context.Context, namespaceId string) ([]*entity.SkillNamespaceMember, error) {
	return nil, nil
}

func TestSkillListVersionsGate(t *testing.T) {
	owner := NewSkillService(
		&fakeSkillRepoForVer{
			skills:   map[string]*entity.Skill{"priv1": {SkillId: "priv1", OwnerId: "owner", Visibility: 1, NamespaceId: "nsx"}},
			versions: map[string][]*entity.SkillVersion{"priv1": {{VersionId: "v1"}}},
		},
		&fakePkgNS{memberIds: map[string]bool{}}, nil, nil,
	)
	if _, err := owner.ListVersions(context.Background(), "priv1", "owner", false); err != nil {
		t.Fatalf("属主应可枚举: %v", err)
	}
	if _, err := owner.ListVersions(context.Background(), "priv1", "stranger", false); err == nil {
		t.Fatal("陌生枚举他人 PRIVATE 版本应拒——ListVersions 门禁被删（M5c 在此必红）")
	}
}

// ---- B-03：GetVersion 门禁（pair 审计指出的「无测试钉住」缺口）----

type fakeSkillRepoForGetVer struct {
	repository.ISkillRepository
	skills   map[string]*entity.Skill
	versions map[string]*entity.SkillVersion
}

func (f *fakeSkillRepoForGetVer) Get(ctx context.Context, skillId string) (*entity.Skill, error) {
	if sk, ok := f.skills[skillId]; ok {
		return sk, nil
	}
	return nil, context.DeadlineExceeded
}
func (f *fakeSkillRepoForGetVer) GetVersion(ctx context.Context, versionId string) (*entity.SkillVersion, error) {
	if v, ok := f.versions[versionId]; ok {
		return v, nil
	}
	return nil, context.DeadlineExceeded
}
func (f *fakeSkillRepoForGetVer) ListMembers(ctx context.Context, namespaceId string) ([]*entity.SkillNamespaceMember, error) {
	return nil, nil
}

func TestSkillGetVersionGate(t *testing.T) {
	svc := NewSkillService(
		&fakeSkillRepoForGetVer{
			skills: map[string]*entity.Skill{
				"priv1": {SkillId: "priv1", OwnerId: "owner", Visibility: 1, NamespaceId: "nsy"},
				"pub1":  {SkillId: "pub1", OwnerId: "owner", Visibility: 0, NamespaceId: "nsy"},
			},
			versions: map[string]*entity.SkillVersion{
				"v-priv": {VersionId: "v-priv", SkillId: "priv1", ManifestJson: []byte(`{"k":"secret"}`)},
				"v-pub":  {VersionId: "v-pub", SkillId: "pub1"},
			},
		},
		&fakePkgNS{memberIds: map[string]bool{}}, nil, nil,
	)
	if _, err := svc.GetVersion(context.Background(), "v-priv", "owner", false); err != nil {
		t.Fatalf("属主应可读版本行: %v", err)
	}
	if _, err := svc.GetVersion(context.Background(), "v-priv", "anyone", true); err != nil {
		t.Fatalf("超管应可读: %v", err)
	}
	if _, err := svc.GetVersion(context.Background(), "v-priv", "stranger", false); err == nil {
		t.Fatal("陌生读他人 PRIVATE 版本行（含 ManifestJson）应拒——B-03 门禁被拆（变异在此必红）")
	}
	if _, err := svc.GetVersion(context.Background(), "v-pub", "stranger", false); err != nil {
		t.Fatalf("PUBLIC 技能版本应可读: %v", err)
	}
}

// ---- A-04：ListTags 归属门禁（标签/存在性不被非属主探测） ----
type fakeTagRepo struct {
	repository.ISkillRepository
	skills map[string]*entity.Skill
}

func (f *fakeTagRepo) Get(ctx context.Context, skillId string) (*entity.Skill, error) {
	if sk, ok := f.skills[skillId]; ok {
		return sk, nil
	}
	return nil, context.DeadlineExceeded
}
func (f *fakeTagRepo) ListTags(ctx context.Context, skillId string) ([]*entity.SkillTag, error) {
	return []*entity.SkillTag{{TagName: "t1"}}, nil
}

func TestSkillListTagsGate(t *testing.T) {
	svc := NewSkillService(
		&fakeTagRepo{skills: map[string]*entity.Skill{
			"priv1": {SkillId: "priv1", OwnerId: "owner", Visibility: 1, NamespaceId: "nsz"},
			"pub1":  {SkillId: "pub1", OwnerId: "owner", Visibility: 0, NamespaceId: "nsz"},
		}},
		&fakePkgNS{memberIds: map[string]bool{}}, nil, nil,
	)
	if _, err := svc.ListTags(context.Background(), "priv1", "owner", false); err != nil {
		t.Fatalf("属主应可读标签: %v", err)
	}
	if _, err := svc.ListTags(context.Background(), "priv1", "stranger", false); !IsDetailNotFound(err) {
		t.Fatalf("陌生探测他人私有技能标签应拒: %v", err)
	}
	if _, err := svc.ListTags(context.Background(), "pub1", "stranger", false); err != nil {
		t.Fatalf("PUBLIC 技能标签应可读: %v", err)
	}
}

// ---- M6：probe 双门禁（删归属检查=借他人凭据配置连调） ----

type fakeProbeRepo struct {
	repository.IMcpServerRepository
	servers map[string]*entity.McpServer
}

func (f *fakeProbeRepo) Get(ctx context.Context, id string) (*entity.McpServer, error) {
	if sv, ok := f.servers[id]; ok {
		return sv, nil
	}
	return nil, context.DeadlineExceeded
}

func TestMcpProbeOwnershipGate(t *testing.T) {
	probe := NewMcpProbeService(&fakeProbeRepo{
		servers: map[string]*entity.McpServer{"srv": {ServerId: "srv", CreatedBy: "owner", Type: "sse"}},
	}, nil) // activityRepo：Test/CallTool 门禁路径不触达，nil=哨兵
	if _, _, err := probe.Test(context.Background(), "srv", nil, "stranger", false); err == nil {
		t.Fatal("陌生 probe 他人 server 应拒——Test 门禁被删（M6 在此必红）")
	}
	if _, _, err := probe.CallTool(context.Background(), "srv", "t", nil, "stranger", false); err == nil {
		t.Fatal("陌生 CallTool 他人 server 应拒——CallTool 门禁被删（M6 在此必红）")
	}
	// 属主过门禁后进入校验/探针层（nil form 报字段校验错属正常）——关键是不得返回「未找到」
	if _, _, err := probe.Test(context.Background(), "srv", nil, "owner", false); err != nil && IsDetailNotFound(err) {
		t.Fatalf("属主 probe 被归属谓词误拒: %v", err)
	}
	// G3：ad-hoc 表单（serverId 空）非超管拒、超管放行（变异锚点：删该分支 →  strangers 格红）
	if _, _, err := probe.Test(context.Background(), "", nil, "stranger", false); !IsDetailNotFound(err) {
		t.Fatalf("非超管 ad-hoc 表单探测应拒: %v", err)
	}
	if _, _, err := probe.Test(context.Background(), "", nil, "anyone", true); err != nil && IsDetailNotFound(err) {
		t.Fatalf("超管 ad-hoc 表单不应被该分支拒: %v", err)
	}
}

// ---- G2：机器取包的归属绑定（可注入队列校验）----
func TestEnsureMachinePackageBinding(t *testing.T) {
	svc := NewSkillPackageService(nil, nil, nil, nil, nil, nil)
	// nil 注入=部分装配路径放行（既有 nil-guard 语义，正文披露）
	if err := svc.EnsureAgentPackageBinding(context.Background(), "inst1", "v1"); err != nil {
		t.Fatalf("未注入时应放行: %v", err)
	}
	// 队列命中 → 放行；未命中 → 拒（变异锚点：删 false 分支 → 未命中格红）
	svc.SetMachineInstallBindings(
		func(ctx context.Context, instanceId, versionId string) (bool, error) {
			return instanceId == "inst1" && versionId == "v1", nil
		},
		func(ctx context.Context, deviceId, versionId string) (bool, error) {
			return deviceId == "dev1" && versionId == "v1", nil
		},
	)
	if err := svc.EnsureAgentPackageBinding(context.Background(), "inst1", "v1"); err != nil {
		t.Fatalf("队列命中应放行: %v", err)
	}
	if err := svc.EnsureAgentPackageBinding(context.Background(), "inst2", "v1"); !IsDetailNotFound(err) {
		t.Fatalf("未下达安装命令的实例应拒: %v", err)
	}
	if err := svc.EnsureDevicePackageBinding(context.Background(), "dev1", "v1"); err != nil {
		t.Fatalf("设备队列命中应放行: %v", err)
	}
	if err := svc.EnsureDevicePackageBinding(context.Background(), "dev2", "v1"); !IsDetailNotFound(err) {
		t.Fatalf("未下达安装命令的设备应拒: %v", err)
	}
}
