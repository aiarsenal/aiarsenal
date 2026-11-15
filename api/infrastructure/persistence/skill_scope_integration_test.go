package persistence

import (
	"context"
	"os"
	"strings"
	"testing"

	"aiarsenal/domain/entity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// F-017b 列表可见性 scope 与向量检索 scope 的真库集成测试（审计变异 M2/M4 的杀手——
// SQL 拼装正确性只有真库能验：Or 优先级、占位符与参数配对）。
// 运行方式：SKILL_TEST_DSN='host=127.0.0.1 port=5434 user=aiarsenal password=aiarsenal123
// dbname=aa_skill sslmode=disable' go test ./infrastructure/persistence/ -run TestSkillScope
func TestSkillListAndSearchScope(t *testing.T) {
	dsn := os.Getenv("SKILL_TEST_DSN")
	if dsn == "" || testing.Short() {
		t.Skip("SKILL_TEST_DSN 未设置或 short 模式：跳过真库集成")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("DSN 不可达，跳过: %v", err)
	}
	ctx := context.Background()
	r := &SkillRepository{db: gdb}
	nsr := &SkillNamespaceRepository{db: gdb}

	// 种子：三技能（u1 私有 / u1 公开 / u2 私有）+ u1 的成员命名空间
	gdb.Exec(`DELETE FROM aa_skill WHERE skill_id LIKE 'scope-test-%'`)
	gdb.Exec(`DELETE FROM aa_skill_namespace_member WHERE user_id = 'scope-test-viewer'`)
	gdb.Exec(`DELETE FROM aa_skill_namespace WHERE namespace_id = 'scope-test-ns'`)
	cleanup := func() {
		gdb.Exec(`DELETE FROM aa_skill WHERE skill_id LIKE 'scope-test-%'`)
		gdb.Exec(`DELETE FROM aa_skill_namespace_member WHERE user_id = 'scope-test-viewer'`)
		gdb.Exec(`DELETE FROM aa_skill_namespace WHERE namespace_id = 'scope-test-ns'`)
	}
	defer cleanup()
	rows := []struct {
		id, slug  string
		vis       int
		owner, ns string
	}{
		{"scope-test-a", "scope-test-a", 1, "u1", "scope-test-ns"},
		{"scope-test-b", "scope-test-b", 0, "u1", "scope-test-ns"},
		{"scope-test-c", "scope-test-c", 1, "u2", "scope-test-ns"},
		{"scope-test-d", "scope-test-d", 1, "u2", "scope-test-other-ns"},
	}
	for _, rw := range rows {
		if err := gdb.Exec(`INSERT INTO aa_skill (skill_id, slug, display_name, visibility, owner_id, namespace_id, status, deleted_at)
			VALUES (?, ?, ?, ?, ?, ?, 1, null)`, rw.id, rw.slug, rw.slug, rw.vis, rw.owner, rw.ns).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := gdb.Exec(`INSERT INTO aa_skill_namespace (namespace_id, slug, display_name) VALUES ('scope-test-ns', 'scope-test-ns', 'x')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Exec(`INSERT INTO aa_skill_namespace_member (member_id, namespace_id, user_id, role) VALUES ('scope-test-m1', 'scope-test-ns', 'scope-test-viewer', 'MEMBER')`).Error; err != nil {
		t.Fatal(err)
	}
	memberNs, _ := nsr.MemberNamespaceIds(ctx, "scope-test-viewer")
	if len(memberNs) != 1 || memberNs[0] != "scope-test-ns" {
		t.Fatalf("MemberNamespaceIds 前置失败: %v", memberNs)
	}

	// T1（M2/B-01 杀手）：成员视角无 namespaceId 查询 → 恰见 a（成员命名空间私有）+
	// b（公开）+ c（同成员命名空间的他人私有）；不见 d（非成员命名空间的他人私有）。
	// 不带 namespaceId 过滤才能暴露 scope 删除（M2）——否则既有 namespaceId 条件
	// 会把 d 排除、差异不可见。软删行复活与关键词过滤同链破坏由 T1b/T2 钉。
	list, total, err := r.List(ctx, "", "", "", "", "", 1, 20, "scope-test-viewer", false, memberNs)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, x := range list {
		ids[x.SkillId] = true
	}
	// 环境可能含其他 PUBLIC 技能（合法可见），断言集合语义：a/b/c 必在、d 必不在
	// （d 在=M2 scope 删除或 Or 逃逸）。
	if !ids["scope-test-a"] || !ids["scope-test-b"] || !ids["scope-test-c"] || ids["scope-test-d"] {
		t.Fatalf("T1 可见集应含 {a,b,c} 且不含 d, total=%d ids=%v——scope 删除或 Or 逃逸", total, ids)
	}
	gdb.Exec(`UPDATE aa_skill SET deleted_at = now() WHERE skill_id = 'scope-test-a'`)
	list2, _, err := r.List(ctx, "", "", "", "", "", 1, 20, "scope-test-viewer", false, memberNs)
	if err != nil {
		t.Fatal(err)
	}
	ids2 := map[string]bool{}
	for _, x := range list2 {
		ids2[x.SkillId] = true
	}
	if ids2["scope-test-a"] || !ids2["scope-test-b"] || !ids2["scope-test-c"] {
		t.Fatalf("T1b 软删 a 后应仍见 b/c 不见 a——Or 优先级复活软删（B-01 回归）, ids=%v", ids2)
	}
	// T2：关键词过滤与 scope 同链（B-01 的功能破坏格）
	list3, _, err := r.List(ctx, "", "", "", "", "scope-test-b", 1, 20, "scope-test-viewer", false, memberNs)
	if err != nil {
		t.Fatal(err)
	}
	if len(list3) != 1 || list3[0].SkillId != "scope-test-b" {
		t.Fatalf("T2 关键词过滤失效（scope Or 逃逸）：得 %d 行", len(list3))
	}

	dim1536 := "[" + strings.Repeat("0,", 1535) + "1]"
	// T3（M4/B-02 杀手）：SearchByVector 各分支占位符/参数配对——admin/匿名/成员三态均不得报错
	// （B-02 实证：admin 分支 4 args vs 3 占位符 → pgx 报错）。semantic_vector 为 NULL 时
	// 不会命中行，但 SQL 必须可执行（占位符错配在 Scan 前即报错）。
	for _, viewer := range []struct {
		id    string
		admin bool
	}{{"", true}, {"", false}, {"scope-test-viewer", false}} {
		if _, err := r.SearchByVector(ctx, dim1536, 5, viewer.id, viewer.admin, memberNs); err != nil {
			t.Fatalf("T3 viewer(id=%q admin=%v) 检索报错: %v", viewer.id, viewer.admin, err)
		}
	}
}

// F-017b G1（成对终验缺口2）：namespace List/AllForCaller 的可见性 scope 真库测试
// ——变异锚点：删 repo 的 if !viewerAdmin 块（scope 全开）→ 本测试必红。
func TestNamespaceListAndAllScope(t *testing.T) {
	dsn := os.Getenv("SKILL_TEST_DSN")
	if dsn == "" || testing.Short() {
		t.Skip("SKILL_TEST_DSN 未设置或 short 模式：跳过真库集成")
	}
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("DSN 不可达，跳过: %v", err)
	}
	ctx := context.Background()
	r := &SkillNamespaceRepository{db: gdb}
	nsr := &SkillNamespaceRepository{db: gdb}

	cleanup := func() {
		gdb.Exec(`DELETE FROM aa_skill_namespace WHERE namespace_id LIKE 'ns-scope-%'`)
		gdb.Exec(`DELETE FROM aa_skill_namespace_member WHERE namespace_id LIKE 'ns-scope-%'`)
	}
	cleanup()
	defer cleanup()
	// 四种命名空间：GLOBAL（他人创建）/ 自己的 TEAM / 成员 TEAM / 他人 TEAM
	rows := []struct{ id, typ, owner string }{
		{"ns-scope-global", "GLOBAL", "u-stranger"},
		{"ns-scope-own", "TEAM", "scope-test-viewer"},
		{"ns-scope-member", "TEAM", "u-member-owner"},
		{"ns-scope-other", "TEAM", "u-stranger"},
	}
	for _, rw := range rows {
		if err := gdb.Exec(`INSERT INTO aa_skill_namespace (namespace_id, slug, display_name, type, status, created_by)
			VALUES (?, ?, ?, ?, 1, ?)`, rw.id, rw.id, rw.id, rw.typ, rw.owner).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if err := gdb.Exec(`INSERT INTO aa_skill_namespace_member (member_id, namespace_id, user_id, role)
		VALUES ('ns-scope-m1', 'ns-scope-member', 'scope-test-viewer', 'MEMBER')`).Error; err != nil {
		t.Fatal(err)
	}
	memberNs, _ := nsr.MemberNamespaceIds(ctx, "scope-test-viewer")
	if len(memberNs) != 1 || memberNs[0] != "ns-scope-member" {
		t.Fatalf("MemberNamespaceIds 前置失败: %v", memberNs)
	}

	ids := func(list []*entity.SkillNamespace) map[string]bool {
		out := map[string]bool{}
		for _, n := range list {
			out[n.NamespaceId] = true
		}
		return out
	}
	// 成员视角 List：GLOBAL + 自己的 + 成员；不见他人 TEAM（变异：scope 删除 → other 现身）
	list, _, err := r.List(ctx, "", "", "", "", 1, 50, "scope-test-viewer", false, memberNs)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(list)
	if !got["ns-scope-global"] || !got["ns-scope-own"] || !got["ns-scope-member"] || got["ns-scope-other"] {
		t.Fatalf("成员视角 List 集合不符（应 GLOBAL+own+member，不应 other）: %v", got)
	}
	// 成员视角 AllForCaller：同一 scope
	all, err := r.AllForCaller(ctx, "scope-test-viewer", false, memberNs)
	if err != nil {
		t.Fatal(err)
	}
	gotAll := ids(all)
	if !gotAll["ns-scope-global"] || !gotAll["ns-scope-member"] || gotAll["ns-scope-other"] {
		t.Fatalf("AllForCaller 集合不符: %v", gotAll)
	}
	// 非超管无任何命名空间归属：只见 GLOBAL
	list2, _, err := r.List(ctx, "", "", "", "", 1, 50, "u-nobody", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	got2 := ids(list2)
	if !got2["ns-scope-global"] || got2["ns-scope-own"] || got2["ns-scope-other"] {
		t.Fatalf("无归属用户应只见 GLOBAL: %v", got2)
	}
	// 超管：全量
	list3, _, err := r.List(ctx, "", "", "", "", 1, 50, "u-admin", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	got3 := ids(list3)
	if !got3["ns-scope-global"] || !got3["ns-scope-own"] || !got3["ns-scope-member"] || !got3["ns-scope-other"] {
		t.Fatalf("超管应全量: %v", got3)
	}
}
