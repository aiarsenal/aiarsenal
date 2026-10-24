package service

import (
	"context"
	"errors"
	"testing"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
)

// fakeUserRepo 只实现用例会走到的方法，其余走嵌入的 nil 接口。
// 嵌入 nil 接口的方法一旦被调用会 panic——这正是想要的：悄悄多查一次库、多写一张表，
// 用例当场炸出来，而不是让"顺手多做了点什么"混过去。
type fakeUserRepo struct {
	repository.IUserRepository
	added  *entity.User
	edited *entity.User
	// editSetAdmin 记录 Edit 收到的 isAdmin 指针：nil=仓储不写该列，非 nil=显式落库。
	editSetAdmin *int
	// entities 是"数据库里现存"的用户记录（按 user_id 取），编辑路径的现状比对与
	// 操作者身份都从这查——操作者与目标必须能区分，闸门测试才立得住。
	curUsers           map[string]*entity.User
	curRoles, curPosts map[string][]string
	getEditCalls       int
	opErr              error
	resetTarget        string
	statusTarget       string
	deletedIds         [][]string
}

func (f *fakeUserRepo) GetUserByUsername(ctx context.Context, userName string) (*model.LoginUserRes, error) {
	return nil, nil
}

func (f *fakeUserRepo) Add(ctx context.Context, u *entity.User, roleIds, postIds []string) (string, error) {
	f.added = u
	return "uid-1", nil
}

func (f *fakeUserRepo) Edit(ctx context.Context, u *entity.User, roleIds, postIds []string, setAdmin *int) error {
	f.edited = u
	f.editSetAdmin = setAdmin
	return nil
}

func (f *fakeUserRepo) GetEditUser(ctx context.Context, id string) (*entity.User, []string, []string, error) {
	f.getEditCalls++
	u, ok := f.curUsers[id]
	if !ok {
		u = &entity.User{UserId: id}
	}
	return u, f.curRoles[id], f.curPosts[id], nil
}

func (f *fakeUserRepo) GetEntityById(ctx context.Context, userId string) (*entity.User, error) {
	if f.opErr != nil {
		return nil, f.opErr
	}
	u, ok := f.curUsers[userId]
	if !ok {
		return nil, errors.New("record not found")
	}
	return u, nil
}

func (f *fakeUserRepo) ResetPwd(ctx context.Context, userId string, pwd, salt string) error {
	f.resetTarget = userId
	return nil
}

func (f *fakeUserRepo) ChangeStatus(ctx context.Context, userId string, status uint) error {
	f.statusTarget = userId
	return nil
}

func (f *fakeUserRepo) Delete(ctx context.Context, userIds []string) error {
	f.deletedIds = append(f.deletedIds, userIds)
	return nil
}

func newTestUserService(repo *fakeUserRepo) *UserService {
	return NewUserService(repo, nil, nil)
}

func ordinaryUser(id string) *entity.User { return &entity.User{UserId: id, IsAdmin: 0} }

func superUser(id string) *entity.User { return &entity.User{UserId: id, IsAdmin: 1} }

func ip(v int) *int { return &v }

// TestAddRejectsAdminFlagFromNonAdmin R1 的建档侧回归用例。
//
// 攻击面是"请求体里的 isAdmin 被原样绑定进 entity 再 Updates 落库"：任意登录用户
// POST /system/user/add 带 isAdmin=1 就是超管（is_admin 让 casbin.go:21 直接 c.Next()）。
// 断言的不只是"返回错误"，还有"根本没走到仓储"——授权检查必须在校验/写库之前。
func TestAddRejectsAdminFlagFromNonAdmin(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-1": ordinaryUser("u-1")}}
	svc := newTestUserService(repo)

	err := svc.Add(context.Background(),
		&entity.User{UserName: "attacker", IsAdmin: 1}, "pwd", nil, nil, "u-1")

	if err == nil {
		t.Fatal("非超管携带 isAdmin=1 建档应当被拒，实际通过")
	}
	if repo.added != nil {
		t.Fatal("权限检查应发生在写库之前，实际已调用仓储 Add")
	}
}

// TestAddAllowsGrantsFromEditor R4：持有用户编辑菜单权限的人建档时可以分配角色/岗位，
// 目标是新账号（普通用户），不构成越权——这层能力归菜单授权管，服务层不再收紧。
func TestAddAllowsGrantsFromEditor(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-1": ordinaryUser("u-1")}}
	svc := newTestUserService(repo)

	if err := svc.Add(context.Background(),
		&entity.User{UserName: "newbie"}, "pwd", []string{"role-1"}, []string{"post-1"}, "u-1"); err != nil {
		t.Fatalf("编辑权限者建档并分配角色被误拦: %v", err)
	}
	if repo.added == nil {
		t.Fatal("建档未落库")
	}
}

// TestAddAllowsAdminFlagFromAdmin 超管给新员工开户时仍然能勾选管理员标识与角色。
func TestAddAllowsAdminFlagFromAdmin(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-9": superUser("u-9")}}
	svc := newTestUserService(repo)

	err := svc.Add(context.Background(),
		&entity.User{UserName: "newadmin", IsAdmin: 1}, "pwd", []string{"role-1"}, []string{"post-1"}, "u-9")

	if err != nil {
		t.Fatalf("超管设置 isAdmin=1 并分配角色应当通过，实际 %v", err)
	}
	if repo.added == nil || repo.added.IsAdmin != 1 {
		t.Fatalf("管理员标识未落库: %+v", repo.added)
	}
	if repo.getEditCalls != 0 {
		t.Fatalf("超管路径不应为了比对而多查一次库，实际调用 GetEditUser %d 次", repo.getEditCalls)
	}
}

// TestEditRejectsSettingAdminFlagFromNonSuper R1：非超管把 isAdmin 改成 1 应当被拒，
// 且拒绝发生在写库之前。
func TestEditRejectsSettingAdminFlagFromNonSuper(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-1": ordinaryUser("u-1"),
		"u-2": ordinaryUser("u-2"),
	}}
	svc := newTestUserService(repo)

	err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-2"}, nil, nil, ip(1), "u-1")

	if err == nil {
		t.Fatal("非超管把普通用户改成超管应当被拒，实际通过")
	}
	if repo.edited != nil {
		t.Fatal("权限检查应发生在写库之前，实际已调用仓储 Edit")
	}
}

// TestEditRejectsModifyingAdminFromNonSuper R3 目标保护：编辑权限者不能修改管理员
// （含改档、改角色、改 isAdmin 任何组合）。请求形状按 UI 的来：表单回填白名单
// 不含 isAdmin，编辑请求体永远缺省该字段。
func TestEditRejectsModifyingAdminFromNonSuper(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-1": ordinaryUser("u-1"),
		"u-9": superUser("u-9"),
	}}
	svc := newTestUserService(repo)

	err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-9"}, nil, nil, nil, "u-1")

	if err == nil {
		t.Fatal("非超管修改管理员账号应当被拒，实际通过")
	}
	if repo.edited != nil {
		t.Fatal("目标保护应发生在写库之前，实际已调用仓储 Edit")
	}
}

// TestEditRejectsSelfGrantChange R2：编辑权限者可以给别人分配角色，但不能借编辑接口
// 改自己的——把 roleIds 换成另一个角色，等于拿到那个角色的全部菜单权限。
func TestEditRejectsSelfGrantChange(t *testing.T) {
	repo := &fakeUserRepo{
		curUsers: map[string]*entity.User{"u-1": ordinaryUser("u-1")},
		curRoles: map[string][]string{"u-1": {"role-a"}},
	}
	svc := newTestUserService(repo)

	err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-1"}, []string{"role-b"}, nil, nil, "u-1")

	if err == nil {
		t.Fatal("借编辑接口改自己的角色应当被拒，实际通过")
	}
	if repo.edited != nil {
		t.Fatal("自助保护应发生在写库之前，实际已调用仓储 Edit")
	}
}

// TestEditAllowsGrantChangeOnOthers R4：编辑权限者给普通用户分配角色照旧可用。
func TestEditAllowsGrantChangeOnOthers(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-1": ordinaryUser("u-1"),
		"u-2": ordinaryUser("u-2"),
	}}
	svc := newTestUserService(repo)

	err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-2", Mobile: "13800000000"}, []string{"role-b"}, nil, nil, "u-1")

	if err != nil {
		t.Fatalf("编辑权限者给普通用户分配角色被误拦: %v", err)
	}
	if repo.edited == nil {
		t.Fatal("角色分配未落库")
	}
}

// TestEditAllowsUnchangedSelfGrants R2 的边界：编辑表单会把当前角色/岗位原样回传，
// 改自己的普通档案（手机号等）必须照旧可用，否则一次正常改名会被误杀。
func TestEditAllowsUnchangedSelfGrants(t *testing.T) {
	repo := &fakeUserRepo{
		curUsers: map[string]*entity.User{"u-1": ordinaryUser("u-1")},
		curRoles: map[string][]string{"u-1": {"role-a", "role-b"}},
		curPosts: map[string][]string{"u-1": {"post-1"}},
	}
	svc := newTestUserService(repo)

	err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-1", Mobile: "13800000000"},
		[]string{"role-b", "role-a"}, []string{"post-1"}, nil, "u-1")

	if err != nil {
		t.Fatalf("自己改普通档案、角色岗位原样回传被误拦: %v", err)
	}
	if repo.edited == nil {
		t.Fatal("普通档案修改未落库")
	}
}

// TestEditAllowsAdminFlagFromAdmin 超管编辑任意账号（含显式授予 isAdmin）不受影响，
// 且不做多余查询。
func TestEditAllowsAdminFlagFromAdmin(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-9": superUser("u-9"),
		"u-2": ordinaryUser("u-2"),
	}}
	svc := newTestUserService(repo)

	err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-2"}, []string{"role-x"}, nil, ip(1), "u-9")

	if err != nil {
		t.Fatalf("超管授予 isAdmin 应当通过，实际 %v", err)
	}
	if repo.getEditCalls != 0 {
		t.Fatalf("超管路径不应为了比对而多查一次库，实际调用 GetEditUser %d 次", repo.getEditCalls)
	}
	if repo.edited == nil || repo.editSetAdmin == nil || *repo.editSetAdmin != 1 {
		t.Fatalf("isAdmin 未显式落库: edited=%+v setAdmin=%v", repo.edited, repo.editSetAdmin)
	}
}

// TestEditOmittingAdminFlagKeepsFlag UI 回归用例（评审抓出的静默降级）：
// 编辑表单回填白名单不含 isAdmin，请求体永远缺省该字段。仓储对 nil 不得写
// is_admin 列——否则超管在界面上改任何管理员的手机号都会把账号降级为普通用户，
// 且基线种子只有一个超管，界面上授不回来。
func TestEditOmittingAdminFlagKeepsFlag(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-9": superUser("u-9"),
	}}
	svc := newTestUserService(repo)

	err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-9", Mobile: "13900000000"}, nil, nil, nil, "u-9")

	if err != nil {
		t.Fatalf("超管编辑管理员档案（缺省 isAdmin）被误拦: %v", err)
	}
	if repo.edited == nil {
		t.Fatal("档案修改未落库")
	}
	if repo.editSetAdmin != nil {
		t.Fatal("请求体缺省 isAdmin 时仓储不得写该列，实际收到了显式写")
	}
}

// TestEditExplicitZeroDemotesAdmin 收权路径必须保持可达：超管显式传 0 才落 0——
// 这是对「GORM 零值跳过导致收回管理员从未真正落库」缺陷的修复语义。
func TestEditExplicitZeroDemotesAdmin(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-9": superUser("u-9"),
	}}
	svc := newTestUserService(repo)

	err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-9"}, nil, nil, ip(0), "u-9")

	if err != nil {
		t.Fatalf("超管显式收权被误拦: %v", err)
	}
	if repo.editSetAdmin == nil || *repo.editSetAdmin != 0 {
		t.Fatalf("显式 0 未落库: setAdmin=%v", repo.editSetAdmin)
	}
}

// TestCallerWithoutRecordRejected 操作者读不到记录（含零值 callerId）时必须拒绝：
// "查无此人"不能被当成放行，降权/注销后的残留登录态也走不到这条缝。
func TestCallerWithoutRecordRejected(t *testing.T) {
	svc := newTestUserService(&fakeUserRepo{})

	if err := svc.Add(context.Background(),
		&entity.User{UserName: "x"}, "pwd", nil, nil, "ghost"); err == nil {
		t.Fatal("操作者记录缺失时建档应当被拒")
	}
	if err := svc.Edit(context.Background(),
		&entity.User{UserId: "u-2"}, nil, nil, nil, ""); err == nil {
		t.Fatal("零值 callerId 不应被当成系统调用放行")
	}
}

// TestDeleteRejectsAdminTarget R3 的删除侧：批量删除混入管理员账号时整批拒绝。
func TestDeleteRejectsAdminTarget(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-1": ordinaryUser("u-1"),
		"u-2": ordinaryUser("u-2"),
		"u-9": superUser("u-9"),
	}}
	svc := newTestUserService(repo)

	if err := svc.Delete(context.Background(), "u-1", []string{"u-2", "u-9"}); err == nil {
		t.Fatal("非超管批量删除混入管理员账号应当被拒")
	}
	if len(repo.deletedIds) != 0 {
		t.Fatalf("拒绝路径不应触发仓储删除，实际 %v", repo.deletedIds)
	}
}

// TestChangeStatusRejectsAdminTarget R3 的停用侧：编辑权限者不能冻结管理员。
func TestChangeStatusRejectsAdminTarget(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{
		"u-1": ordinaryUser("u-1"),
		"u-9": superUser("u-9"),
	}}
	svc := newTestUserService(repo)

	if err := svc.ChangeStatus(context.Background(), "u-1", "u-9", 0); err == nil {
		t.Fatal("非超管停用管理员账号应当被拒")
	}
	if repo.statusTarget != "" {
		t.Fatal("拒绝路径不应写库")
	}
}

// TestResetPwdRejectsResettingOthersFromNonSuper 密码重置的归属校验（手册程序4）：
// 旧实现只校验密码非空，任意登录用户可重置任意账号的口令。归属校验必须发生在写库前。
func TestResetPwdRejectsResettingOthersFromNonSuper(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-1": ordinaryUser("u-1")}}
	svc := newTestUserService(repo)

	err := svc.ResetPwd(context.Background(), "u-1", "u-2", "new-pwd")

	if err == nil {
		t.Fatal("非超管重置他人口令应当被拒，实际通过")
	}
	if repo.resetTarget != "" {
		t.Fatal("归属校验应发生在写库之前，实际已调用仓储 ResetPwd")
	}
}

// TestResetPwdAllowsSelfForNonSuper 非超管重置本人口令必须照旧可用——
// 个人中心自助改密走 personal 路由，管理端入口不应对本人失效。
func TestResetPwdAllowsSelfForNonSuper(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-1": ordinaryUser("u-1")}}
	svc := newTestUserService(repo)

	if err := svc.ResetPwd(context.Background(), "u-1", "u-1", "new-pwd"); err != nil {
		t.Fatalf("非超管重置本人密码被误拦: %v", err)
	}
	if repo.resetTarget != "u-1" {
		t.Fatalf("本人重置未落库，实际目标 %q", repo.resetTarget)
	}
}

// TestResetPwdAllowsSuperResettingOthers 超管重置他人口令照旧可用（忘记密码的运维场景）。
func TestResetPwdAllowsSuperResettingOthers(t *testing.T) {
	repo := &fakeUserRepo{curUsers: map[string]*entity.User{"u-9": superUser("u-9")}}
	svc := newTestUserService(repo)

	if err := svc.ResetPwd(context.Background(), "u-9", "u-2", "new-pwd"); err != nil {
		t.Fatalf("超管重置他人口令被误拦: %v", err)
	}
	if repo.resetTarget != "u-2" {
		t.Fatalf("目标口令未落库，实际目标 %q", repo.resetTarget)
	}
}

// TestResetPwdFailsClosedWithoutOperatorRecord 操作者读不到记录时必须拒绝：
// "查无此人"不能被当成放行，否则任何登录态都能把自己当超管。
func TestResetPwdFailsClosedWithoutOperatorRecord(t *testing.T) {
	repo := &fakeUserRepo{opErr: errors.New("record not found")}
	svc := newTestUserService(repo)

	if err := svc.ResetPwd(context.Background(), "ghost", "ghost", "new-pwd"); err == nil {
		t.Fatal("操作者记录缺失时应当拒绝，实际通过")
	}
	if repo.resetTarget != "" {
		t.Fatal("拒绝路径不应写库")
	}
}

// TestSameStringSet 比对函数的边界：nil 与空切片等价、重复元素不改变判定。
func TestSameStringSet(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want bool
	}{
		{"双 nil", nil, nil, true},
		{"nil 与空切片", nil, []string{}, true},
		{"顺序不同", []string{"a", "b"}, []string{"b", "a"}, true},
		{"元素不同", []string{"a"}, []string{"b"}, false},
		{"长度不同", []string{"a"}, []string{"a", "b"}, false},
		{"空与非空", nil, []string{"a"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sameStringSet(c.a, c.b); got != c.want {
				t.Fatalf("sameStringSet(%v,%v)=%v，期望 %v", c.a, c.b, got, c.want)
			}
		})
	}
}
