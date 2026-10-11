package service

import (
	"context"
	"errors"

	rt "github.com/aiarsenal/runtime-go"

	"aiarsenal/domain/repository"
	"aiarsenal/infrastructure/agentruntime"
)

// agent_tty_service.go — 实例交互式终端（websocket tty）服务层。
//
// 仅 localdocker：经 agentruntime 桥接对实例容器 docker exec attach（TTY 模式），
// 流（*agentruntime.TTYStream）由 controller 的 websocket 双向泵透传。
// CVM（云 provider）不支持——容器在远端宿主，本机 docker daemon 上没有它。

// AgentTtyService 交互式终端服务。
type AgentTtyService struct {
	bridge   *agentruntime.Client
	adapter  rt.Adapter
	instRepo repository.IAgentInstanceRepository
}

// NewAgentTtyService 构造。
func NewAgentTtyService(bridge *agentruntime.Client, adapter rt.Adapter, instRepo repository.IAgentInstanceRepository) *AgentTtyService {
	return &AgentTtyService{bridge: bridge, adapter: adapter, instRepo: instRepo}
}

// Open 打开实例容器内 /bin/sh 的 TTY 流。
// width/height 为初始终端尺寸（<=0 时不下发，由首条 resize 消息补）。
// 调用方（controller）已负责 token 鉴权与归属校验，这里只做运行时与实例状态闸。
func (s *AgentTtyService) Open(ctx context.Context, instanceId string, width, height int) (*agentruntime.TTYStream, error) {
	if s.adapter == nil || s.adapter.Provider() != "localdocker" {
		return nil, errors.New("仅本机 Docker 运行时支持交互式终端")
	}
	inst, err := s.instRepo.Get(ctx, instanceId)
	if err != nil {
		return nil, err
	}
	if inst.Status != 1 {
		return nil, errors.New("实例未运行，无法打开终端")
	}
	return s.bridge.ExecTTY(ctx, agentruntime.ContainerRef(instanceId, inst.ContainerId), width, height)
}
