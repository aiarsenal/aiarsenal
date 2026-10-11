package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	rt "github.com/aiarsenal/runtime-go"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/agentruntime"
	"aiarsenal/infrastructure/idgen"
)

// agent_volume_service.go — 持久卷管理（aa_agent_persistent_volume 接线）。
//
// 卷生命周期：本服务 CreateVolume 先经 agentruntime 桥接真建 docker 卷
// （卷名 aiarsenal-vol-<bizId>，与 runtime-go localdocker 命名约定一致），
// 再落库；实例创建请求带 volume_ids 时经 MountsForInstance 绑定并把
// []rt.VolumeMount 追加进 InstanceSpec.Volumes（见 agent_instance_service.provision）。
// 仅 localdocker 运行时有卷语义：CVM（云 provider）一律拒绝。
//
// status：0=available（未绑定）1=in-use（已挂到某实例）。

// AgentVolumeService 持久卷服务。
type AgentVolumeService struct {
	repo     repository.IAgentPersistentVolumeRepository
	instRepo repository.IAgentInstanceRepository
	adapter  rt.Adapter
	bridge   *agentruntime.Client // localdocker 直连桥接（docker volume API + DiskUsage）
}

// NewAgentVolumeService 构造。bridge 由 main.go 统一创建（agentruntime.New()，惰性连接）。
func NewAgentVolumeService(repo repository.IAgentPersistentVolumeRepository, instRepo repository.IAgentInstanceRepository, adapter rt.Adapter, bridge *agentruntime.Client) *AgentVolumeService {
	return &AgentVolumeService{repo: repo, instRepo: instRepo, adapter: adapter, bridge: bridge}
}

// isLocal 仅本机 Docker 运行时支持卷操作。
func (s *AgentVolumeService) isLocal() bool {
	return s.adapter != nil && s.adapter.Provider() == "localdocker"
}

// VolumeView 卷列表/详情出参：库行 + docker 实时元数据（仅 localdocker 有值）。
type VolumeView struct {
	*entity.AgentPersistentVolume
	RuntimeSize  int64  `json:"runtimeSize"`  // docker 实测占用字节（未知 -1；CVM 恒 -1）
	Mountpoint   string `json:"mountpoint"`   // docker 卷挂载点（宿主路径）
	InstanceName string `json:"instanceName"` // 绑定实例名富化（未绑定为空）
}

// List 全量卷（卷量级小不分页），localdocker 时合并 inspect 实时数据并富化实例名。
func (s *AgentVolumeService) List(ctx context.Context) ([]*VolumeView, error) {
	rows, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	var infos map[string]agentruntime.VolumeInfo
	if s.isLocal() {
		if list, lerr := s.bridge.ListVolumes(ctx); lerr == nil {
			infos = make(map[string]agentruntime.VolumeInfo, len(list))
			for _, info := range list {
				infos[info.Name] = info
			}
		} // 列实时数据失败不阻断：降级只出库行
	}
	// 实例名富化：绑定的实例少，逐个 Get 成本可接受。
	names := make(map[string]string)
	for _, row := range rows {
		if row.InstanceId == "" || row.InstanceId == "0" {
			continue
		}
		if inst, ierr := s.instRepo.Get(ctx, row.InstanceId); ierr == nil {
			names[row.InstanceId] = inst.Name
		}
	}
	out := make([]*VolumeView, 0, len(rows))
	for _, row := range rows {
		v := &VolumeView{AgentPersistentVolume: row, RuntimeSize: -1}
		if info, ok := infos[row.VolumeName]; ok {
			v.RuntimeSize = info.Size
			v.Mountpoint = info.Mountpoint
		}
		v.InstanceName = names[row.InstanceId]
		out = append(out, v)
	}
	return out, nil
}

// Get 卷详情（带实时元数据）。
func (s *AgentVolumeService) Get(ctx context.Context, id string) (*VolumeView, error) {
	row, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	v := &VolumeView{AgentPersistentVolume: row, RuntimeSize: -1}
	if s.isLocal() {
		if info, ierr := s.bridge.InspectVolume(ctx, agentruntime.VolumeName(row.PersistentVolumeId)); ierr == nil {
			v.RuntimeSize = info.Size
			v.Mountpoint = info.Mountpoint
		}
	}
	if row.InstanceId != "" && row.InstanceId != "0" {
		if inst, ierr := s.instRepo.Get(ctx, row.InstanceId); ierr == nil {
			v.InstanceName = inst.Name
		}
	}
	return v, nil
}

// Create 真建 docker 卷（localdocker）+ 落库。
// storage_size_gb 仅落库展示/配额参考：local driver 无容量上限，不下发 size。
func (s *AgentVolumeService) Create(ctx context.Context, e *entity.AgentPersistentVolume) (string, error) {
	if !s.isLocal() {
		return "", errors.New("仅本机 Docker 运行时支持持久卷")
	}
	// 先预生成业务 id（推 docker 卷名），repo.Add 对已置 id 不覆盖（见 repo 注释）。
	e.PersistentVolumeId = idgen.New()
	if _, err := s.bridge.CreateVolume(ctx, e.PersistentVolumeId); err != nil {
		return "", fmt.Errorf("创建 docker 卷失败: %w", err)
	}
	e.VolumeName = agentruntime.VolumeName(e.PersistentVolumeId)
	e.Status = 0 // available
	if e.MountPath == "" {
		e.MountPath = "/data/" + e.PersistentVolumeId
	}
	return s.repo.Add(ctx, e)
}

// Delete 删除卷：绑定的卷先解绑/拒绝（这里拒绝，防误删生产数据），再删 docker 卷与库行。
// docker 卷不存在（如异机历史残留）按已删处理，仍清库行。
func (s *AgentVolumeService) Delete(ctx context.Context, ids []string) error {
	if !s.isLocal() {
		return errors.New("仅本机 Docker 运行时支持持久卷")
	}
	for _, id := range ids {
		row, err := s.repo.Get(ctx, id)
		if err != nil {
			return err
		}
		if row.InstanceId != "" && row.InstanceId != "0" {
			return fmt.Errorf("卷 %s 已绑定实例 %s，请先解绑再删除", row.PersistentVolumeId, row.InstanceId)
		}
		if err := s.bridge.RemoveVolume(ctx, row.PersistentVolumeId); err != nil &&
			!strings.Contains(err.Error(), "no such volume") {
			return fmt.Errorf("删除 docker 卷失败: %w", err)
		}
		if err := s.repo.Delete(ctx, []string{row.PersistentVolumeId}); err != nil {
			return err
		}
	}
	return nil
}

// Unbind 解绑卷（恢复 available，实例侧数据不动，下次创建实例可再绑）。
// 显式字段更新：instance_id 置空 / status 归 0 都是零值，gorm 结构体 Updates 会跳列。
func (s *AgentVolumeService) Unbind(ctx context.Context, id string) error {
	row, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.repo.UpdateFields(ctx, row.PersistentVolumeId, map[string]interface{}{
		"instance_id": "", "status": 0, "updated_at": time.Now(),
	})
}

// MountsForInstance 把卷列表绑定到实例并返回追加挂载表（实例创建流程调用，
// 见 agent_instance_service.provision）。任一卷不可用即整体报错——实例还没起，
// 此时失败让用户改请求重提，避免起一半少卷。
// 返回的 VolumeID 用业务卷 id（runtime-go localdocker 据此推 aiarsenal-vol-<id>）。
func (s *AgentVolumeService) MountsForInstance(ctx context.Context, volumeIds []string, instanceId string) ([]rt.VolumeMount, error) {
	if len(volumeIds) == 0 {
		return nil, nil
	}
	if !s.isLocal() {
		return nil, errors.New("仅本机 Docker 运行时支持持久卷")
	}
	mounts := make([]rt.VolumeMount, 0, len(volumeIds))
	for _, id := range volumeIds {
		row, err := s.repo.Get(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("持久卷 %s 不存在", id)
		}
		if bound := row.InstanceId; bound != "" && bound != "0" && bound != instanceId {
			return nil, fmt.Errorf("持久卷 %s 已绑定实例 %s", id, bound)
		}
		row.InstanceId = instanceId
		row.Status = 1 // in-use
		if err := s.repo.Update(ctx, row); err != nil {
			return nil, err
		}
		mp := row.MountPath
		if mp == "" {
			mp = "/data/" + row.PersistentVolumeId
		}
		mounts = append(mounts, rt.VolumeMount{VolumeID: row.PersistentVolumeId, MountPath: mp})
	}
	return mounts, nil
}
