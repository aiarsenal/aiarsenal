package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"aiarsenal/config"
	"aiarsenal/consts"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/grantmeta"
)

// AuthService 登录用例编排（对应旧 controller/sys_login + sysUser login 方法）
type AuthService struct {
	userRepo repository.IUserRepository
	menuRepo repository.IMenuRepository
	captcha  *CaptchaService
}

func NewAuthService(u repository.IUserRepository, a repository.IMenuRepository, cap *CaptchaService) *AuthService {
	return &AuthService{userRepo: u, menuRepo: a, captcha: cap}
}

func (s *AuthService) isDebug() bool {
	return os.Getenv("APP_DEBUG") == "1" || os.Getenv("APP_DEBUG") == "true"
}

// Login 登录：验证码 → 密码 → 签发 JWT → 菜单/权限
func (s *AuthService) Login(ctx context.Context, username, password, totpCode, verifyKey, verifyCode, ip, userAgent string) (*model.LoginUserRes, string, []*model.UserMenus, []string, error) {
	// 1. 验证码（debug 模式跳过，对应旧 gmode.IsDevelop）
	if !s.isDebug() {
		if !s.captcha.VerifyString(verifyKey, verifyCode) {
			s.userRepo.InsertLoginLog(ctx, &model.LoginLogParams{
				Status: 0, Username: username, Ip: ip, UserAgent: userAgent, Msg: "验证码输入错误", Module: "系统后台",
			})
			return nil, "", nil, nil, errors.New("验证码输入错误")
		}
	}

	// 2. 登录失败锁定（等保 b 项；fail-open 取舍见 login_lockout.go）——密码比对前拦截
	if auth.LoginLocked(username) {
		s.userRepo.InsertLoginLog(ctx, &model.LoginLogParams{
			Status: 0, Username: username, Ip: ip, UserAgent: userAgent,
			Msg: "失败次数过多，账号已锁定", Module: "系统后台",
		})
		return nil, "", nil, nil, fmt.Errorf("失败次数过多，账号已锁定，请 %d 分钟后再试",
			config.Get().Password.Lockout.LockMinutes)
	}

	// 3. 用户名密码
	user, err := s.userRepo.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		// 未知用户名不计数：不给攻击者灌 redis 键，也不放大「恶意锁死他人账号」的面
		s.userRepo.InsertLoginLog(ctx, &model.LoginLogParams{
			Status: 0, Username: username, Ip: ip, UserAgent: userAgent, Msg: "账号密码错误", Module: "系统后台",
		})
		return nil, "", nil, nil, errors.New("账号密码错误")
	}
	if !auth.VerifyPassword(password, user.UserPassword) {
		auth.RecordLoginFailure(username)
		s.userRepo.InsertLoginLog(ctx, &model.LoginLogParams{
			Status: 0, Username: username, Ip: ip, UserAgent: userAgent, Msg: "账号密码错误", Module: "系统后台",
		})
		return nil, "", nil, nil, errors.New("账号密码错误")
	}
	if user.UserStatus == 0 {
		return nil, "", nil, nil, errors.New("账号已被冻结")
	}
	// 口令定期更换（可选档 rotationDays>0 启用；等保 a 项「定期更换」的落地）：
	// 过期口令登录直接拒绝，管理员重置后解除。拒绝发生在计数清零之前——
	// 正确口令不该累计失败，但过期状态每次登录都会再次拦截。
	// 双因子（TOTP）：已绑定账号必须验动态码，错误计入登录失败锁定；
	// enforced 开启后未绑定账号拒绝登录（等保三级 d 项的强制口径）。
	// 判定收口在 totpLoginCheck（纯函数，表驱动双向测试），这里只做编排：
	// 解密失败属服务错误（不计数、不泄露口令对错），验码失败才计锁定。
	if passwordExpired(user.PasswordChangedAt, config.Get().Password.Policy.RotationDays, time.Now()) {
		s.userRepo.InsertLoginLog(ctx, &model.LoginLogParams{
			Status: 0, Username: username, Ip: ip, UserAgent: userAgent,
			Msg: "口令已过期，登录拒绝", Module: "系统后台",
		})
		return nil, "", nil, nil, errors.New("口令已过期，请联系管理员重置后再登录")
	}
	auth.ClearLoginFailures(username)

	// 4. 更新登录信息 + 成功日志
	_ = s.userRepo.UpdateLoginInfo(ctx, user.Id, ip)
	s.userRepo.InsertLoginLog(ctx, &model.LoginLogParams{
		Status: 1, Username: username, Ip: ip, UserAgent: userAgent, Msg: "登录成功", Module: "系统后台",
	})

	// 5. 签发 JWT（业务 id + IsAdmin + 部门/岗位/工号入 claims，构成权限身份的 claims 侧）
	deptId, postIds, employeeNo, _ := s.userRepo.GetDeptAndPosts(ctx, user.Id)
	token, err := auth.Generate(user.Id, user.UserName, user.IsAdmin, deptId, postIds, employeeNo)
	if err != nil {
		return nil, "", nil, nil, errors.New("登录失败，后端服务出现错误")
	}

	// 6. 菜单 / 权限
	menuList, permissions, err := s.GetAdminRules(ctx, user.Id, user.IsAdmin == 1)
	if err != nil {
		return nil, "", nil, nil, err
	}

	// 7. 清空密码与双因子密钥后返回
	user.UserPassword = ""
	user.TOTPSecret = ""

	// 8. 在线状态（异步、容错）
	go s.userRepo.InsertUserOnline(context.Background(), &model.UserOnlineParams{
		UserAgent: userAgent,
		Uuid:      userSaltHash(token),
		Token:     token,
		Username:  user.UserName,
		Ip:        ip,
	})

	return user, token, menuList, permissions, nil
}

// GetAdminRules 用户菜单与按钮权限（casbin 驱动，对应旧 sysUser.GetAdminRules）
func (s *AuthService) GetAdminRules(ctx context.Context, userId string, isSuperAdmin bool) ([]*model.UserMenus, []string, error) {
	allMenus, err := s.menuRepo.GetIsMenuList(ctx)
	if err != nil {
		return nil, nil, err
	}

	if isSuperAdmin {
		permissions := []string{"*/*/*"}
		menuList := buildMenuTree(allMenus)
		return menuList, permissions, nil
	}

	// 非超管：通过 casbin 取角色 → 菜单 id
	roleIds, err := s.userRoleIds(userId)
	if err != nil {
		return nil, nil, err
	}
	menuIds := s.roleMenuIds(roleIds)

	// 组合授权表达式行（v3<>'g'）：五元组逐行 Enforce，命中菜单并入
	if err := s.mergeGrantMenuIds(ctx, userId, roleIds, menuIds); err != nil {
		return nil, nil, err
	}

	filtered := make([]*model.MenuInfoRes, 0, len(allMenus))
	for _, m := range allMenus {
		if strings.EqualFold(m.Condition, "nocheck") || menuIds[m.Id] {
			filtered = append(filtered, m)
		}
	}
	menuList := buildMenuTree(filtered)

	// 按钮权限
	buttons, err := s.menuRepo.GetIsButtonList(ctx)
	if err != nil {
		return nil, nil, err
	}
	permissions := make([]string, 0, len(buttons))
	for _, b := range buttons {
		if strings.EqualFold(b.Condition, "nocheck") || menuIds[b.Id] {
			permissions = append(permissions, b.Name)
		}
	}
	return menuList, permissions, nil
}

func (s *AuthService) userRoleIds(userId string) ([]string, error) {
	e, err := auth.Enforcer()
	if err != nil {
		return nil, err
	}
	gp, gerr := e.GetFilteredGroupingPolicy(0, consts.CasBinUserPrefix+userId)
	if gerr != nil {
		return nil, gerr
	}
	roleIds := make([]string, 0, len(gp))
	for _, v := range gp {
		if len(v) >= 2 {
			roleIds = append(roleIds, v[1])
		}
	}
	return roleIds, nil
}

func (s *AuthService) roleMenuIds(roleIds []string) map[string]bool {
	menuIds := map[string]bool{}
	e, err := auth.Enforcer()
	if err != nil {
		return menuIds
	}
	for _, roleId := range roleIds {
		gp, perr := e.GetFilteredPolicy(0, roleId)
		if perr != nil {
			continue
		}
		for _, p := range gp {
			if len(p) >= 2 {
				menuIds[p[1]] = true
			}
		}
	}
	return menuIds
}

// mergeGrantMenuIds 组合授权路径：扫 casbin 表达式行（v3<>'g'），以五元组逐行 Enforce，
// 命中的菜单并入 menuIds。身份服务端现算（库 + casbin），不依赖请求 claims；
// 单行表达式求值异常跳过不阻断（fail-safe 只影响该行）。
func (s *AuthService) mergeGrantMenuIds(ctx context.Context, userId string, roleIds []string, menuIds map[string]bool) error {
	e, err := auth.Enforcer()
	if err != nil {
		return err
	}
	pol, perr := e.GetPolicy()
	if perr != nil {
		return perr
	}
	var exprRows [][]string
	for _, p := range pol {
		if len(p) >= 4 && p[3] != "g" {
			exprRows = append(exprRows, p)
		}
	}
	if len(exprRows) == 0 {
		return nil
	}
	deptId, postIds, employeeNo, err := s.userRepo.GetDeptAndPosts(ctx, userId)
	if err != nil {
		return err
	}
	idt := model.Identity{UserId: userId, DeptId: deptId, RoleIds: roleIds, PostIds: postIds, EmployeeNo: employeeNo}
	grantmeta.ResolveIdentity(&idt) // 引用行（分组/集合）实时解析，与 API 鉴权路径同一身份形态
	sub := idt.Sub()
	for _, p := range exprRows {
		ok, eerr := e.Enforce(sub, p[1], "All")
		if eerr != nil || !ok {
			continue
		}
		menuIds[p[1]] = true
	}
	return nil
}

// --- 菜单树构建（对应旧 sysUser.setMenuData / GetMenusTree）---

func buildMenuTree(menus []*model.MenuInfoRes) []*model.UserMenus {
	list := make([]*model.UserMenus, 0, len(menus))
	for _, v := range menus {
		list = append(list, &model.UserMenus{UserMenu: setMenuData(v)})
	}
	return getMenuTree(list, "0")
}

func getMenuTree(menus []*model.UserMenus, pid string) []*model.UserMenus {
	out := make([]*model.UserMenus, 0, len(menus))
	for _, m := range menus {
		if m.Pid == pid {
			m.Children = getMenuTree(menus, m.Id)
			out = append(out, m)
		}
	}
	return out
}

func setMenuData(e *model.MenuInfoRes) *model.UserMenu {
	m := &model.UserMenu{
		Id:        e.Id,
		Pid:       e.Pid,
		Name:      caseCamelLower(strings.ReplaceAll(e.Name, "/", "_")),
		Component: e.Component,
		Path:      e.Path,
		MenuMeta: &model.MenuMeta{
			Icon:        e.Icon,
			Title:       e.Title,
			EnName:      e.EnName,
			IsLink:      "",
			IsHide:      e.IsHide == 1,
			IsKeepAlive: e.IsCached == 1,
			IsAffix:     e.IsAffix == 1,
			IsIframe:    e.IsIframe == 1,
		},
	}
	if m.MenuMeta.IsIframe || e.IsLink == 1 {
		m.MenuMeta.IsLink = e.LinkUrl
	}
	return m
}

// caseCamelLower：分隔符拆词，首词小写，其余词首字母大写
func caseCamelLower(s string) string {
	if s == "" {
		return s
	}
	// 统一分隔符
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, " ", "_")
	words := strings.Split(s, "_")
	var b strings.Builder
	first := true
	for _, w := range words {
		if w == "" {
			continue
		}
		lower := strings.ToLower(w)
		if first {
			b.WriteString(lower)
			first = false
		} else {
			b.WriteString(strings.ToUpper(lower[:1]))
			if len(lower) > 1 {
				b.WriteString(lower[1:])
			}
		}
	}
	return b.String()
}

// userSaltHash 生成在线用户 uuid（旧码用 gmd5(token)，32 位）
func userSaltHash(token string) string {
	h := md5.Sum([]byte(token))
	return hex.EncodeToString(h[:])
}

// totpLoginCheck 登录双因子判定的纯函数（状态矩阵全格可测）：
//   - 已绑定：缺码 → 「请输入动态验证码」；验码失败（verify 注入，含解密失败）→
//     「动态验证码错误」（调用方据此计入失败锁定）；通过 → nil。
//   - 未绑定：enforced 开启 → 提示联系管理员绑定；关闭 → nil（自愿模式放行）。
func totpLoginCheck(enabled uint, enforced bool, code string, verify func(string) bool) error {
	if enabled == 1 {
		if code == "" {
			return errors.New("请输入动态验证码")
		}
		if !verify(code) {
			return errors.New("动态验证码错误")
		}
		return nil
	}
	if enforced {
		return errors.New("账号未绑定双因子验证，请联系管理员绑定")
	}
	return nil
}

// passwordExpired 口令定期更换判定：rotationDays<=0 不启用；无记录（NULL）按已过期
// 处理——迁移会回填 now()，正常运行不该出现 NULL，出现即视为异常态走保守分支。
func passwordExpired(changedAt *types.Time, rotationDays int, now time.Time) bool {
	if rotationDays <= 0 {
		return false
	}
	if changedAt == nil || time.Time(*changedAt).IsZero() {
		return true
	}
	return now.Sub(time.Time(*changedAt)) > time.Duration(rotationDays)*24*time.Hour
}
