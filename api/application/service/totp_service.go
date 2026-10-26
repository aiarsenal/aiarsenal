package service

import (
	"context"
	"errors"
	"time"

	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/infrastructure/crypto"

	"log/slog"

	"aiarsenal/config"
)

// totp_service.go — 双因子（TOTP 动态码）绑定/激活/停用/管理员重置用例编排。
//
// 生命周期：Setup 生成密钥（密文落库、enabled=0 待激活，明文仅本次返回）→
// Activate 验一次有效动态码激活 → 日常登录要求动态码 → Disable（验码后停用）；
// 丢手机走 AdminReset（仅超管，清除绑定）。密钥落库走 AES-256-GCM（与 LLM 凭据
// 同一把 jwt.encryptKey），明文不出现在任何响应的第二报里。

// TOTPService 双因子用例服务。
//
// 防爆破：Activate/Disable 的动态码错误复用登录锁定计数（键 pwd:fail:uid:<userId>，
// 与登录的用户名计数同一模块、同一阈值/窗口）——持有窃得 JWT 者无法对停用/绑定
// 接口在线爆破 6 位动态码。
type TOTPService struct {
	userRepo repository.IUserRepository
}

// NewTOTPService 构造。
func NewTOTPService(u repository.IUserRepository) *TOTPService {
	return &TOTPService{userRepo: u}
}

// totpFailKey 双因子动态码的失败计数键：与登录锁定同模块（login_lockout.go），
// 键域用 uid: 前缀与用户名计数隔离，互不污染。理论边界：若有人建出用户名恰好
// 等于某账号内部 id 的账号（用户名校验只查非空+唯一），两个键域可碰撞——需要
// 超管配合且知道 32 位内部 id，纯理论，登记备查。
func totpFailKey(userId string) string { return "uid:" + userId }

// Setup 生成绑定密钥。已激活账号须先停用；待激活状态下重复 Setup 会换新密钥
// （旧待激活密钥作废——手机上录了半截的场景直接重来）。
func (s *TOTPService) Setup(ctx context.Context, userId string) (secret, uri string, err error) {
	_, enabled, err := s.userRepo.GetTOTP(ctx, userId)
	if err != nil {
		return "", "", errors.New("用户不存在或已被删除")
	}
	if enabled == 1 {
		return "", "", errors.New("已绑定双因子，请先停用后再重新绑定")
	}
	secret, err = auth.GenerateTOTPSecret()
	if err != nil {
		return "", "", err
	}
	enc, err := crypto.Encrypt(secret, config.Get().Jwt.EncryptKey)
	if err != nil {
		return "", "", err
	}
	if err := s.userRepo.UpdateTOTP(ctx, userId, enc, 0); err != nil {
		return "", "", err
	}
	login, err := s.userRepo.GetUserById(ctx, userId)
	if err != nil || login == nil {
		return "", "", errors.New("用户不存在或已被删除")
	}
	uri = auth.TOTPProvisioningURI(secret, config.Get().Totp.Issuer, login.UserName)
	return secret, uri, nil
}

// Activate 确认绑定：验证一次有效动态码后置 1（防密钥分发途中被换走）。
func (s *TOTPService) Activate(ctx context.Context, userId, code string) error {
	if auth.LoginLocked(totpFailKey(userId)) {
		return errors.New("尝试次数过多，请稍后再试")
	}
	secretEnc, enabled, err := s.userRepo.GetTOTP(ctx, userId)
	if err != nil {
		return errors.New("用户不存在或已被删除")
	}
	if enabled == 1 {
		return errors.New("已绑定双因子，无需重复绑定")
	}
	if secretEnc == "" {
		return errors.New("请先获取绑定密钥")
	}
	secret, err := crypto.Decrypt(secretEnc, config.Get().Jwt.EncryptKey)
	if err != nil {
		slog.Error("totp 密钥解密失败", "err", err, "userId", userId)
		return errors.New("绑定失败，请重新获取密钥")
	}
	if !auth.VerifyTOTP(secret, code, time.Now()) {
		auth.RecordLoginFailure(totpFailKey(userId))
		return errors.New("动态验证码错误，绑定失败")
	}
	auth.ClearLoginFailures(totpFailKey(userId))
	return s.userRepo.UpdateTOTP(ctx, userId, secretEnc, 1)
}

// Disable 停用：需一次有效动态码，防会话被劫持后顺手关掉双因子。
func (s *TOTPService) Disable(ctx context.Context, userId, code string) error {
	if auth.LoginLocked(totpFailKey(userId)) {
		return errors.New("尝试次数过多，请稍后再试")
	}
	secretEnc, enabled, err := s.userRepo.GetTOTP(ctx, userId)
	if err != nil {
		return errors.New("用户不存在或已被删除")
	}
	if enabled != 1 {
		return errors.New("未绑定双因子")
	}
	secret, err := crypto.Decrypt(secretEnc, config.Get().Jwt.EncryptKey)
	if err != nil {
		slog.Error("totp 密钥解密失败", "err", err, "userId", userId)
		return errors.New("停用失败，后端服务出现错误")
	}
	if !auth.VerifyTOTP(secret, code, time.Now()) {
		auth.RecordLoginFailure(totpFailKey(userId))
		return errors.New("动态验证码错误")
	}
	auth.ClearLoginFailures(totpFailKey(userId))
	return s.userRepo.UpdateTOTP(ctx, userId, "", 0)
}

// AdminReset 管理员重置（丢手机 / enforced 下未绑定 / 自绑丢失的统一恢复路径）：
// 生成**新密钥**落库并置为已激活，明文与 otpauth URI 仅本次返回、由管理员下发给
// 用户录入——用户即可用「口令+新动态码」登录（enforced 下未绑定账号无法自助绑定，
// 本方法就是系统内的唯一恢复/入职通道）。仅超管可操作，操作者读库判定，零值即拒绝。
// 审计留痕由 OperLog 中间件覆盖（PUT 非 GET），不另起写通道。
func (s *TOTPService) AdminReset(ctx context.Context, operatorId, targetUserId string) (secret, uri string, err error) {
	op, err := s.userRepo.GetEntityById(ctx, operatorId)
	if err != nil || op == nil {
		slog.Error("校验双因子重置操作者失败", "operatorId", operatorId, "err", err)
		return "", "", errors.New("没有访问权限：只有超级管理员可以重置双因子")
	}
	if op.IsAdmin != 1 {
		return "", "", errors.New("没有访问权限：只有超级管理员可以重置双因子")
	}
	login, err := s.userRepo.GetUserById(ctx, targetUserId)
	if err != nil || login == nil {
		return "", "", errors.New("用户不存在或已被删除")
	}
	secret, err = auth.GenerateTOTPSecret()
	if err != nil {
		return "", "", err
	}
	enc, err := crypto.Encrypt(secret, config.Get().Jwt.EncryptKey)
	if err != nil {
		return "", "", err
	}
	// 直接置为已激活：新密钥只有管理员经手并即时下发，用户录入后立即可登录；
	// 若用户想换成只有自己知道的密钥，可 Disable（凭新码）后再自助 Setup。
	if err := s.userRepo.UpdateTOTP(ctx, targetUserId, enc, 1); err != nil {
		return "", "", err
	}
	// 重置即给目标干净的两侧计数：uid 侧（绑定/停用接口的爆破尝试）
	// 与用户名侧（登录动态码错码）都不带入新周期
	auth.ClearLoginFailures(totpFailKey(targetUserId))
	auth.ClearLoginFailures(login.UserName)
	uri = auth.TOTPProvisioningURI(secret, config.Get().Totp.Issuer, login.UserName)
	return secret, uri, nil
}
