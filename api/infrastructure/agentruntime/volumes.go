package agentruntime

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

// volumes.go — 持久卷（localdocker 直连桥接）。
//
// docker 卷名与 runtime-go localdocker 的命名约定对齐（aiarsenal-vol-<bizId>），
// 这样卷既可由实例 provision 时的 spec.Volumes 挂载，也可由这里独立创建后补绑。
// storage_size_gb 仅落库做展示/配额参考：local driver 无容量上限（需卷插件），
// Create 时不下发 size（下发了会因 local driver 不支持而报错）。

// volumeNamePrefix docker 卷名前缀，与 runtime-go localdocker volPrefix 一致。
const volumeNamePrefix = "aiarsenal-vol-"

// VolumeName 由业务卷 id 推 docker 卷名。
func VolumeName(volumeBizId string) string { return volumeNamePrefix + volumeBizId }

// VolumeInfo 卷的桥接侧视图（service 落库/出参用）。
type VolumeInfo struct {
	Name       string // docker 卷名（aiarsenal-vol-<bizId>）
	Driver     string
	Mountpoint string
	Size       int64  // 真实占用字节（DiskUsage；未知为 -1）
	Labels     map[string]string
}

// toVolumeInfo 把 SDK volume.Volume 转 VolumeInfo（size 从 DiskUsage 映射取）。
func toVolumeInfo(v *volume.Volume, sizes map[string]int64) VolumeInfo {
	info := VolumeInfo{Size: -1}
	if v != nil {
		info.Name, info.Driver, info.Mountpoint, info.Labels = v.Name, v.Driver, v.Mountpoint, v.Labels
		if v.UsageData != nil {
			info.Size = v.UsageData.Size
		}
	}
	if info.Size < 0 {
		if s, ok := sizes[info.Name]; ok {
			info.Size = s
		}
	}
	return info
}

// CreateVolume 创建持久卷（docker volume create，driver=local，
// labels 标记归属，便于列卷时识别本平台管理的卷）。
func (c *Client) CreateVolume(ctx context.Context, volumeBizId string) (VolumeInfo, error) {
	v, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name:   VolumeName(volumeBizId),
		Driver: "local",
		Labels: map[string]string{
			"managed-by":          "aiarsenal",
			"aiarsenal.volume-id": volumeBizId,
		},
	})
	if err != nil {
		return VolumeInfo{}, fmt.Errorf("agentruntime: volume create %s: %w", VolumeName(volumeBizId), err)
	}
	return toVolumeInfo(&v, nil), nil
}

// RemoveVolume 删除持久卷。force=false：被容器占用时 docker 报错返回，
// 由 service 转成用户可读提示（比静默强删安全）。
func (c *Client) RemoveVolume(ctx context.Context, volumeBizId string) error {
	if err := c.cli.VolumeRemove(ctx, VolumeName(volumeBizId), false); err != nil {
		return fmt.Errorf("agentruntime: volume remove %s: %w", VolumeName(volumeBizId), err)
	}
	return nil
}

// InspectVolume 单卷详情（含真实占用）。
func (c *Client) InspectVolume(ctx context.Context, volumeBizId string) (VolumeInfo, error) {
	v, err := c.cli.VolumeInspect(ctx, VolumeName(volumeBizId))
	if err != nil {
		return VolumeInfo{}, fmt.Errorf("agentruntime: volume inspect %s: %w", VolumeName(volumeBizId), err)
	}
	sizes, _ := c.DiskUsage(ctx)
	return toVolumeInfo(&v, sizes), nil
}

// ListVolumes 列本平台管理的全部卷（labels managed-by=aiarsenal 过滤），
// size 一次 DiskUsage 带全，避免逐卷往返。
func (c *Client) ListVolumes(ctx context.Context) ([]VolumeInfo, error) {
	sizes, err := c.DiskUsage(ctx)
	if err != nil {
		// 卷列表不因磁盘统计失败而失败：size 降级为未知。
		sizes = nil
	}
	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "managed-by=aiarsenal")),
	})
	if err != nil {
		return nil, fmt.Errorf("agentruntime: volume list: %w", err)
	}
	out := make([]VolumeInfo, 0, len(resp.Volumes))
	for _, v := range resp.Volumes {
		out = append(out, toVolumeInfo(v, sizes))
	}
	return out, nil
}
