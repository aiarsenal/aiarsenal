package service

import (
	"context"
	"errors"
	"io"
	"log"

	rt "github.com/aiarsenal/runtime-go"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/agentruntime"
)

// agent_workspace_service.go — 实例工作区文件管理（/aa，localdocker 直连桥接）。
//
// 列目录/下载/上传/建目录/删除/改名全部经 agentruntime 桥接落到实例容器，
// 变更类操作（upload/download/mkdir/delete/rename）逐笔写
// aa_agent_workspace_file_audit 审计（list 只读不审）。CVM 运行时不支持。

// AgentWorkspaceService 工作区文件服务。
type AgentWorkspaceService struct {
	bridge    *agentruntime.Client
	adapter   rt.Adapter
	instRepo  repository.IAgentInstanceRepository
	auditRepo repository.IAgentWorkspaceFileAuditRepository
}

// NewAgentWorkspaceService 构造。
func NewAgentWorkspaceService(bridge *agentruntime.Client, adapter rt.Adapter, instRepo repository.IAgentInstanceRepository, auditRepo repository.IAgentWorkspaceFileAuditRepository) *AgentWorkspaceService {
	return &AgentWorkspaceService{bridge: bridge, adapter: adapter, instRepo: instRepo, auditRepo: auditRepo}
}

// isLocal 仅本机 Docker 运行时支持文件操作。
func (s *AgentWorkspaceService) isLocal() bool {
	return s.adapter != nil && s.adapter.Provider() == "localdocker"
}

// resolve 校验实例存在 + 运行时支持，返回容器引用（CopyTo/CopyFrom 目标）。
func (s *AgentWorkspaceService) resolve(ctx context.Context, instanceId string) (string, error) {
	if !s.isLocal() {
		return "", errors.New("仅本机 Docker 运行时支持工作区文件管理")
	}
	inst, err := s.instRepo.Get(ctx, instanceId)
	if err != nil {
		return "", err
	}
	return agentruntime.ContainerRef(instanceId, inst.ContainerId), nil
}

// audit 变更操作落审计（best-effort：审计失败只记日志，不影响主操作结果）。
func (s *AgentWorkspaceService) audit(ctx context.Context, instanceId, userId, action, relPath string, bytes int64) {
	if _, err := s.auditRepo.Add(ctx, &entity.AgentWorkspaceFileAudit{
		InstanceId: instanceId, UserId: userId, Action: action,
		RelativePath: relPath, Bytes: bytes,
	}); err != nil {
		// 审计缺列/缺表不应让文件操作失败，留日志排障。
		log.Printf("[workspace:%s] 审计写入失败: %v", instanceId, err)
	}
}

// List 列目录（dir 为工作区相对路径，空=根 /aa）。
func (s *AgentWorkspaceService) List(ctx context.Context, instanceId, dir string) ([]agentruntime.FileEntry, error) {
	ref, err := s.resolve(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	return s.bridge.ListFiles(ctx, ref, dir)
}

// Upload 上传/覆盖文件（relPath 为工作区相对路径）。
func (s *AgentWorkspaceService) Upload(ctx context.Context, instanceId, userId, relPath string, content []byte) error {
	ref, err := s.resolve(ctx, instanceId)
	if err != nil {
		return err
	}
	if err := s.bridge.UploadFile(ctx, ref, relPath, content); err != nil {
		return err
	}
	s.audit(ctx, instanceId, userId, "upload", relPath, int64(len(content)))
	return nil
}

// Download 下载文件：返回流（调用方负责 Close）与大小。
func (s *AgentWorkspaceService) Download(ctx context.Context, instanceId, userId, relPath string) (io.ReadCloser, int64, error) {
	ref, err := s.resolve(ctx, instanceId)
	if err != nil {
		return nil, 0, err
	}
	rc, size, err := s.bridge.DownloadFile(ctx, ref, relPath)
	if err != nil {
		return nil, 0, err
	}
	s.audit(ctx, instanceId, userId, "download", relPath, size)
	return rc, size, nil
}

// Mkdir 建目录（递归）。
func (s *AgentWorkspaceService) Mkdir(ctx context.Context, instanceId, userId, relPath string) error {
	ref, err := s.resolve(ctx, instanceId)
	if err != nil {
		return err
	}
	if err := s.bridge.Mkdir(ctx, ref, relPath); err != nil {
		return err
	}
	s.audit(ctx, instanceId, userId, "mkdir", relPath, 0)
	return nil
}

// Delete 删除文件/目录（根目录拒绝）。
func (s *AgentWorkspaceService) Delete(ctx context.Context, instanceId, userId, relPath string) error {
	ref, err := s.resolve(ctx, instanceId)
	if err != nil {
		return err
	}
	if err := s.bridge.DeleteFile(ctx, ref, relPath); err != nil {
		return err
	}
	s.audit(ctx, instanceId, userId, "delete", relPath, 0)
	return nil
}

// Rename 改名/移动。
func (s *AgentWorkspaceService) Rename(ctx context.Context, instanceId, userId, from, to string) error {
	ref, err := s.resolve(ctx, instanceId)
	if err != nil {
		return err
	}
	if err := s.bridge.RenameFile(ctx, ref, from, to); err != nil {
		return err
	}
	s.audit(ctx, instanceId, userId, "rename", from+" -> "+to, 0)
	return nil
}

// AuditList 操作审计分页。
func (s *AgentWorkspaceService) AuditList(ctx context.Context, instanceId string, pageNum, pageSize int) ([]*entity.AgentWorkspaceFileAudit, int64, error) {
	return s.auditRepo.List(ctx, instanceId, pageNum, pageSize)
}
