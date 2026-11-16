package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/alert"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

// F-002 默认凭据收口：默认安装零种子——基线不再携带任何用户账号（101 个演示账号
// 移入 resource/data/demo.sql，仅显式 AIARSENAL_DEMO=true 的 rebuild-db 加载），
// 首个超管由本文件在启动时引导生成。两条互补防线：
//  1. BootstrapAdmin：库内无任何 is_admin=1 账号时创建超管，口令取
//     AIARSENAL_ADMIN_PASSWORD（deploy.sh 生成注入）或 crypto/rand 生成并一次性打印；
//  2. ScanSeedDefaultPasswords：存量部署升级后，对演示账号名形态（hi/userNNN）逐个
//     校验公开默认口令 aiarsenal123，命中即走告警面（后台一次性，不阻塞启动）。

// generateAdminPassword 生成 24 字节随机口令（base64url，32 字符）。
// 字符集满足最严档（大小写+数字+符号中 base64url 含前三类与 - _）。
func generateAdminPassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// isDemoAccountName 演示账号名形态（种子 hi + user001..user100）。
// 存量部署自建账号不受影响；匹配形态仅决定"值得校验默认口令"的候选集。
func isDemoAccountName(name string) bool {
	if name == "hi" {
		return true
	}
	matched, _ := regexp.MatchString(`^user\d{3}$`, name)
	return matched
}

// seedPasswordRisk 返回仍可用公开默认口令登录的演示账号名（纯函数，表测覆盖）。
func seedPasswordRisk(users []entity.User) []string {
	var risky []string
	for _, u := range users {
		if !isDemoAccountName(u.UserName) {
			continue
		}
		if auth.VerifyPassword("aiarsenal123", u.UserPassword) {
			risky = append(risky, u.UserName)
		}
	}
	return risky
}

// needsBootstrapAdmin 引导决策纯函数（F-002）：库内超管数为 0 才引导。
// 变异锚点：改判据（如 >=0/恒 true）→ TestNeedsBootstrapAdmin 格红。
func needsBootstrapAdmin(adminCount int) bool { return adminCount == 0 }

// shouldSkipSeedScan 种子扫描跳过决策纯函数（F-002）：显式声明演示环境才跳过。
// 变异锚点：放宽判据（如 !demo 也跳过恒 false 反转）→ TestShouldSkipSeedScan 格红。
func shouldSkipSeedScan(demoDeclared bool) bool { return demoDeclared }

// BootstrapAdmin 库内无超管时创建首个超管（F-002）。username 依次尝试 hi/admin/admin2…
// （种子 hi 已随演示数据移出基线，正常全新安装必命中 hi）；口令 env 优先、否则随机生成，
// 仅打印一次到启动日志（GitLab 初始口令同款模式）；password_changed_at 取当前时间
// （随机口令不参与首登强制改密/轮换判定）。
func BootstrapAdmin(ctx context.Context) error {
	d := db.DB()
	var admins int64
	if err := d.WithContext(ctx).Model(&entity.User{}).Where("is_admin = 1").Count(&admins).Error; err != nil {
		return fmt.Errorf("bootstrap: count admins: %w", err)
	}
	if !needsBootstrapAdmin(int(admins)) {
		return nil
	}
	pwd := config.Get().Bootstrap.AdminPassword
	if pwd == "" {
		g, err := generateAdminPassword()
		if err != nil {
			return fmt.Errorf("bootstrap: generate password: %w", err)
		}
		pwd = g
	}
	exists := func(ctx context.Context, name string) (bool, error) {
		var n int64
		if err := d.WithContext(ctx).Model(&entity.User{}).Where("user_name = ?", name).Count(&n).Error; err != nil {
			return false, err
		}
		return n > 0, nil
	}
	name, err := pickAdminName(ctx, exists)
	if err != nil {
		return err
	}
	hash, err := auth.HashPassword(pwd)
	if err != nil {
		return fmt.Errorf("bootstrap: hash password: %w", err)
	}
	now := types.Time(time.Now())
	u := entity.User{
		UserId:            idgen.New(),
		UserName:          name,
		Name:              "Administrator",
		UserPassword:      hash,
		UserStatus:        1,
		IsAdmin:           1,
		PasswordChangedAt: &now,
	}
	if err := d.WithContext(ctx).Create(&u).Error; err != nil {
		return fmt.Errorf("bootstrap: create admin: %w", err)
	}
	// 一次性口令展示：超管凭据只存在于本次启动日志，请立即登录改密。
	fmt.Printf("\n============================================================\n"+
		"  初始超管账号已创建（首次启动引导，F-002）\n"+
		"  用户名: %s\n"+
		"  口令:   %s\n"+
		"  请立即登录并修改口令；本口令不会再次显示。\n"+
		"============================================================\n\n", name, pwd)
	slog.Info("初始超管已创建（引导生成）", "username", name, "userId", u.UserId)
	return nil
}

// pickAdminName 候选用户名选择（F-002）：hi → admin → admin2 → admin3 依次试
// 占用（exists 返回该名是否已被占用），全占用报错。抽为纯函数便于表测（第二轮
// 审计缺口 2 的 DB 路径自动化）。
func pickAdminName(ctx context.Context, exists func(ctx context.Context, name string) (bool, error)) (string, error) {
	for _, candidate := range []string{"hi", "admin", "admin2", "admin3"} {
		taken, err := exists(ctx, candidate)
		if err != nil {
			return "", fmt.Errorf("bootstrap: check name: %w", err)
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("bootstrap: 候选超管用户名均已被占用")
}

// scanPipeline 种子扫描的可注入管线（F-002 第二轮审计缺口：候选查询与告警出口
// 抽为字段后，跳过决策/候选过滤/风险判定/告警内容全链可免库表测）。
type scanPipeline struct {
	fetchCandidates func(ctx context.Context) ([]entity.User, error)
	dispatch        func(eventType, severity, title, body, bizKey string)
}

// ScanSeedDefaultPasswords 存量部署的默认口令扫描（F-002）：演示账号名形态且仍可用
// aiarsenal123 登录 → 告警面提示。显式 AIARSENAL_DEMO=true 的演示环境跳过（已声明）。
// 后台 goroutine 调用（PBKDF2 校验候选集可能秒级），失败只记日志不影响启动。
func ScanSeedDefaultPasswords(ctx context.Context) {
	p := scanPipeline{
		fetchCandidates: func(ctx context.Context) ([]entity.User, error) {
			var users []entity.User
			err := db.DB().WithContext(ctx).Model(&entity.User{}).
				Where("user_name = 'hi' OR user_name ~ '^user[0-9]{3}$'").
				Find(&users).Error
			return users, err
		},
		dispatch: alert.Dispatch,
	}
	runSeedScan(ctx, p, config.Get().Bootstrap.Demo)
}

// runSeedScan 扫描管线主体（可注入形态，表测覆盖 skip 决策/候选过滤/告警内容）。
func runSeedScan(ctx context.Context, p scanPipeline, demoDeclared bool) {
	if shouldSkipSeedScan(demoDeclared) {
		slog.Info("演示模式（AIARSENAL_DEMO=true），跳过种子默认口令扫描")
		return
	}
	users, err := p.fetchCandidates(ctx)
	if err != nil {
		slog.Warn("种子默认口令扫描失败（跳过）", "err", err)
		return
	}
	if len(users) == 0 {
		return
	}
	risky := seedPasswordRisk(users)
	if len(risky) == 0 {
		return
	}
	p.dispatch("seed_default_password", "warn",
		"演示种子账号仍可使用公开默认口令登录",
		fmt.Sprintf("检测到 %d 个演示种子账号（%v…）仍可用公开默认口令 aiarsenal123 登录：任一可访问登录页的攻击者可直接接管（审计 F-002）。处置：立即修改这些账号口令，或将部署显式声明为演示环境（AIARSENAL_DEMO=true）并物理隔离公网。",
			len(risky), risky[:min(3, len(risky))]),
		"seed_default_password")
	slog.Warn("检测到演示种子账号仍可公开默认口令登录（已告警）", "count", len(risky))
}
