package service

import (
	"context"
	"encoding/json"
	"fmt"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// MCP 系统配置（单例）+ 用户级配置服务。
// 单例 config id='default'；user_config 按 user_id UPSERT。

type McpConfigService struct {
	cfgRepo  repository.IMcpConfigRepository
	userRepo repository.IMcpUserConfigRepository
}

func NewMcpConfigService(c repository.IMcpConfigRepository, u repository.IMcpUserConfigRepository) *McpConfigService {
	return &McpConfigService{cfgRepo: c, userRepo: u}
}

// GetSystemConfig 取系统配置；不存在则返回默认空配置。
func (s *McpConfigService) GetSystemConfig(ctx context.Context) (*entity.McpConfig, error) {
	c, err := s.cfgRepo.Get(ctx)
	if err != nil {
		// 未初始化时返回默认
		return defaultConfig(), nil
	}
	return c, nil
}

func (s *McpConfigService) SaveSystemConfig(ctx context.Context, e *entity.McpConfig) error {
	jsonFields := []struct {
		name string
		val  *string
	}{
		{"routing", &e.Routing},
		{"install", &e.Install},
		{"toolResultCompression", &e.ToolResultCompression},
		{"mcpRouter", &e.McpRouter},
		{"auth", &e.Auth},
		{"discovery", &e.Discovery},
		{"activityLog", &e.ActivityLog},
	}
	for _, f := range jsonFields {
		if *f.val == "" {
			*f.val = "{}"
		}
		if !json.Valid([]byte(*f.val)) {
			return fmt.Errorf("%s 不是合法的 JSON", f.name)
		}
	}
	if e.NameSeparator == "" {
		e.NameSeparator = "/"
	}
	return s.cfgRepo.Save(ctx, e)
}

func (s *McpConfigService) GetUserConfig(ctx context.Context, userId string) (*entity.McpUserConfig, error) {
	c, err := s.userRepo.Get(ctx, userId)
	if err != nil {
		return &entity.McpUserConfig{UserId: userId, Routing: "{}", AdditionalConfig: "{}"}, nil
	}
	return c, nil
}

func (s *McpConfigService) SaveUserConfig(ctx context.Context, e *entity.McpUserConfig) error {
	// 列是 jsonb，非法 JSON 会一路透传成 PG 22P02 原始报错，这里提前拦下
	for _, f := range []struct {
		name string
		val  *string
	}{
		{"routing", &e.Routing},
		{"additionalConfig", &e.AdditionalConfig},
	} {
		if *f.val == "" {
			*f.val = "{}"
		}
		if !json.Valid([]byte(*f.val)) {
			return fmt.Errorf("%s 不是合法的 JSON", f.name)
		}
	}
	return s.userRepo.Save(ctx, e)
}

func defaultConfig() *entity.McpConfig {
	return &entity.McpConfig{
		Id:                    "default",
		Routing:               "{}",
		Install:               "{}",
		ToolResultCompression: "{}",
		McpRouter:             "{}",
		NameSeparator:         "/",
		Auth:                  "{}",
		Discovery:             "{}",
		ActivityLog:           "{}",
	}
}
