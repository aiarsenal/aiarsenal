package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/auth"
)

// agent_user_service.go — Agent 平台用户管理（v2 占位菜单：aa_agent_user 新表）。
//
// 与主库 aa_user 解耦的软引用模型：username 对应 aa_user.username（唯一索引防重复
// 授权），本表自有 role（admin/operator/member）、口令哈希（md5(md5(pw)+md5(salt))，
// 与 aa_user 同口径）与实例授权（instance_ids jsonb 数组，元素为 aa_agent_instance
// .instance_id）。明文口令只在创建/重置时生成并一次性返回（前端弹窗展示），落库仅存
// 哈希。增删改查仅本表，不触碰 aa_user。

// AgentUserUpsert 用户新建/编辑入参（controller 绑定后转 entity）。
type AgentUserUpsert struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Role        string   `json:"role"`
	InstanceIds []string `json:"instanceIds"`
	IsEnabled   uint     `json:"isEnabled"`
	Remark      string   `json:"remark"`
}

// AgentUserService Agent 平台用户服务。
type AgentUserService struct {
	repo repository.IAgentUserRepository
}

// NewAgentUserService 构造。
func NewAgentUserService(repo repository.IAgentUserRepository) *AgentUserService {
	return &AgentUserService{repo: repo}
}

// agentUserRoles 合法角色集合（前端管理页三档）。
var agentUserRoles = map[string]bool{"admin": true, "operator": true, "member": true}

// normalize 校验并归一（role/isEnabled 兜底、username 去空白）。
func (s *AgentUserService) normalize(u *AgentUserUpsert) error {
	u.Username = strings.TrimSpace(u.Username)
	if u.Username == "" {
		return errors.New("用户名不能为空")
	}
	if u.Role == "" {
		u.Role = "member"
	}
	if !agentUserRoles[u.Role] {
		return errors.New("role 仅支持 admin / operator / member")
	}
	if u.IsEnabled > 1 {
		u.IsEnabled = 1
	}
	return nil
}

// List 用户分页（keyword 模糊匹配 username/display_name；status -1=全部 0/1=启停）。
func (s *AgentUserService) List(ctx context.Context, keyword string, status int, pageNum, pageSize int) ([]*entity.AgentUser, int64, error) {
	return s.repo.List(ctx, keyword, status, pageNum, pageSize)
}

// Get 用户详情。
func (s *AgentUserService) Get(ctx context.Context, id string) (*entity.AgentUser, error) {
	return s.repo.Get(ctx, id)
}

// Add 新建用户：生成随机口令（明文一次性返回，落库存盐+哈希）。username 全局唯一（本表），重复直接报错。
func (s *AgentUserService) Add(ctx context.Context, u AgentUserUpsert, createdBy string) (string, string, error) {
	if err := s.normalize(&u); err != nil {
		return "", "", err
	}
	if _, err := s.repo.GetByUsername(ctx, u.Username); err == nil {
		return "", "", errors.New("用户名已存在")
	}
	salt, err := randToken(8)
	if err != nil {
		return "", "", err
	}
	password, err := randToken(16)
	if err != nil {
		return "", "", err
	}
	id, err := s.repo.Add(ctx, &entity.AgentUser{
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		Role:         u.Role,
		InstanceIds:  mustJSON(u.InstanceIds),
		IsEnabled:    u.IsEnabled,
		PasswordSalt: salt,
		PasswordHash: auth.EncryptPassword(password, salt),
		Remark:       u.Remark,
		CreatedBy:    createdBy,
	})
	if err != nil {
		return "", "", err
	}
	return id, password, nil
}

// Edit 编辑用户。username 不可改（软引用锚点），仅改展示名/角色/授权/启停/备注。
// gorm 结构体 Updates 跳零值（isEnabled=0 无法落列），走显式 map 更新；
// remark 前端整包覆盖语义（清空传空串）。
func (s *AgentUserService) Edit(ctx context.Context, id string, u AgentUserUpsert) error {
	old, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if u.Role != "" && !agentUserRoles[u.Role] {
		return errors.New("role 仅支持 admin / operator / member")
	}
	displayName := old.DisplayName
	if u.DisplayName != "" {
		displayName = u.DisplayName
	}
	role := old.Role
	if u.Role != "" {
		role = u.Role
	}
	instanceIds := old.InstanceIds
	if u.InstanceIds != nil {
		instanceIds = mustJSON(u.InstanceIds)
	}
	return s.repo.EditFields(ctx, old.AgentUserId, map[string]interface{}{
		"display_name": displayName,
		"role":         role,
		"instance_ids": instanceIds,
		"is_enabled":   u.IsEnabled, // 显式启停：0 也生效
		"remark":       u.Remark,
		"updated_at":   time.Now(),
	})
}

// ResetPassword 重置口令：生成新随机口令（明文一次性返回），换盐重哈希。
func (s *AgentUserService) ResetPassword(ctx context.Context, id string) (string, error) {
	old, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	salt, err := randToken(8)
	if err != nil {
		return "", err
	}
	password, err := randToken(16)
	if err != nil {
		return "", err
	}
	if err := s.repo.EditFields(ctx, old.AgentUserId, map[string]interface{}{
		"password_salt": salt,
		"password_hash": auth.EncryptPassword(password, salt),
		"updated_at":    time.Now(),
	}); err != nil {
		return "", err
	}
	return password, nil
}

// Delete 删除用户（硬删，本表无软删列）。
func (s *AgentUserService) Delete(ctx context.Context, ids []string) error {
	return s.repo.Delete(ctx, ids)
}

// randToken 生成 n 字节随机源映射的字母数字口令（≈6.6 bit/字符，16 字符 105 bit）。
func randToken(n int) (string, error) {
	const charset = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 去易混字符
	out := make([]byte, n)
	max := big.NewInt(int64(len(charset)))
	for i := range out {
		v, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = charset[v.Int64()]
	}
	return string(out), nil
}

// mustJSON []string → jsonb 字符串（nil → "[]"，空数组语义而非 NULL）。
func mustJSON(arr []string) string {
	if arr == nil {
		return "[]"
	}
	b, err := json.Marshal(arr)
	if err != nil {
		return "[]"
	}
	return string(b)
}
