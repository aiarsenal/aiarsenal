// Package runtimenop 提供 runtime.Adapter 的空实现：
// AIARSENAL_RUNTIME=disabled 时 buildRuntimeAdapter 改用它（如无 Docker 的 Windows 测试机），
// api 可正常启动、其余模块不受影响，仅 Agent 实例相关操作一律报错。
// 不设置该环境变量时永远不会被选中，unix 部署行为不变。
package runtimenop

import (
	"context"
	"errors"
	"io"

	rt "github.com/aiarsenal/runtime-go"
)

// Adapter 空实现（无状态，零值可用）。
type Adapter struct{}

var errDisabled = errors.New("agent runtime disabled (AIARSENAL_RUNTIME=disabled)：本部署未启用容器运行时，Agent 实例操作不可用")

// Provider 实现 runtime.Adapter。
func (Adapter) Provider() string { return "disabled" }

// ---- 生命周期 ----

func (Adapter) Create(_ context.Context, _ rt.InstanceSpec) (string, error) { return "", errDisabled }
func (Adapter) Start(_ context.Context, _ string) error                     { return errDisabled }
func (Adapter) Stop(_ context.Context, _ string) error                      { return errDisabled }
func (Adapter) Restart(_ context.Context, _ string) error                   { return errDisabled }
func (Adapter) Remove(_ context.Context, _ string) error                    { return errDisabled }
func (Adapter) Status(_ context.Context, _ string) (*rt.Status, error)      { return nil, errDisabled }

// ---- 访问 ----

func (Adapter) Exec(_ context.Context, _ string, _ rt.ExecOpts) ([]byte, error) {
	return nil, errDisabled
}
func (Adapter) Logs(_ context.Context, _ string, _ rt.LogOpts) (io.ReadCloser, error) {
	return nil, errDisabled
}
func (Adapter) Shell(_ context.Context, _ string, _ rt.ShellOpts) (rt.ReadWriteCloser, error) {
	return nil, errDisabled
}

// ---- 文件（工作区） ----

func (Adapter) ListFiles(_ context.Context, _, _ string) ([]rt.FileEntry, error) {
	return nil, errDisabled
}
func (Adapter) UploadFile(_ context.Context, _, _ string, _ io.Reader, _ int64) error {
	return errDisabled
}
func (Adapter) DownloadFile(_ context.Context, _, _ string) (io.ReadCloser, error) {
	return nil, errDisabled
}
func (Adapter) Mkdir(_ context.Context, _, _ string) error        { return errDisabled }
func (Adapter) RenameFile(_ context.Context, _, _, _ string) error { return errDisabled }
func (Adapter) DeleteFile(_ context.Context, _, _ string) error    { return errDisabled }

// ---- 存储 ----

func (Adapter) CreateVolume(_ context.Context, _, _ string, _ int) (string, error) {
	return "", errDisabled
}
func (Adapter) RemoveVolume(_ context.Context, _ string) error { return errDisabled }
func (Adapter) Backup(_ context.Context, _, _ string) (*rt.Backup, error) {
	return nil, errDisabled
}

// ---- 健康 ----

func (Adapter) Inspect(_ context.Context, _ string) (*rt.InstanceDetail, error) {
	return nil, errDisabled
}
func (Adapter) Metrics(_ context.Context, _ string) (*rt.Metrics, error) {
	return nil, errDisabled
}

// ---- 管理 ----

func (Adapter) Close() error { return nil }
