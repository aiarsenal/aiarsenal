package service

import (
	"log/slog"
	"path/filepath"

	storagestg "github.com/aiarsenal/runtime-go/storage"

	"aiarsenal/config"
)

// NewSkillStorageFromConfig 按 config.yaml 的 storage 段选择技能包对象存储 provider。
//
//	type=local → 本机磁盘（localRoot 空→${system.dataDir}/skill，兼容已传包）
//	type=s3    → S3 兼容（阿里云 OSS / 腾讯云 COS / MinIO / AWS S3）
//
// api 主入口与 mcp-ops 入口共用（原来是 cmd/main.go 的私有 buildStorage）。
func NewSkillStorageFromConfig() (storagestg.Storage, error) {
	cfg := config.Get().Storage
	typ := cfg.Type
	if typ == "" {
		typ = "local"
	}
	params := map[string]any{
		"endpoint":       cfg.Endpoint,
		"region":         cfg.Region,
		"accessKey":      cfg.AccessKey,
		"secretKey":      cfg.SecretKey,
		"bucket":         cfg.Bucket,
		"basePath":       cfg.BasePath,
		"useSSL":         cfg.UseSSL,
		"forcePathStyle": cfg.ForcePathStyle,
		"localRoot":      cfg.LocalRoot,
	}
	// local 模式 localRoot 兜底为 dataDir/skill，兼容旧 LocalStorage 磁盘路径
	if typ == "local" && cfg.LocalRoot == "" {
		dir := config.Get().System.DataDir
		if dir == "" {
			dir = "./resource/data"
		}
		params["localRoot"] = filepath.Join(dir, "skill")
	}
	s, err := storagestg.New(storagestg.Config{Type: typ, Params: params})
	if err != nil {
		return nil, err
	}
	slog.Info("skill storage 初始化", "provider", s.Provider())
	return s, nil
}
