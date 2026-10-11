package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	rt "github.com/aiarsenal/runtime-go"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// agent_rollout_service.go — 运行时滚动发布（aa_agent_runtime_rollout / aa_agent_runtime_pod 接线）。
//
// localdocker 场景约定：pod 行与实例一一对应——pod_name = "aiarsenal-<instanceId>"
// （与 provision 落库的 container_name 一致）。发布单创建时按目标实例清单生成 pod 行，
// 逐批 restart 实例（restart-only：镜像升级需 recreate 容器，localdocker 下
// runtime-go Adapter 只暴露 Create/Restart， recreation 流程后续版本再补）。
//
// 批次推进选「惰性推进」（请求驱动，非后台轮询协程）：
//   - 每次进度查询/pause/resume 入口先 Advance 一次；
//   - 理由：单机 docker restart + 心跳回传本就以「秒~分钟」为节奏，后台协程常驻
//     轮询 DB 换不来更快的收敛，反而引入并发推进的锁竞争与进程重启后的协程恢复问题；
//   - 代价：无人打开进度页时批次停在「等确认」——与惰性语义一致（页面一开即推进）。
//
// 版本判定：心跳链路（agent_runtime_service）把 agent 进程版本落在
// aa_agent_instance_runtime_status.agent_proc_version；与 targetImageRef 的 tag 段
// 比对相等即该 pod 完成。超时（10 分钟无心跳变化）标 failed。
//
// 状态：rollout 0=pending 1=running 2=paused 3=completed 4=failed 5=cancelled；
// pod    0=pending 1=updating 2=done 3=failed。
const (
	rolloutStatusRunning   = 1
	rolloutStatusCompleted = 3
	rolloutStatusFailed    = 4

	podStatePending  = 0
	podStateUpdating = 1
	podStateDone     = 2
	podStateFailed   = 3

	batchTimeout = 10 * time.Minute // 批内单 pod 等待心跳版本更新的超时
)

// AgentRolloutService 滚动发布服务。
type AgentRolloutService struct {
	rolloutRepo repository.IAgentRuntimeRolloutRepository
	podRepo     repository.IAgentRuntimePodRepository
	instRepo    repository.IAgentInstanceRepository
	statusRepo  repository.IAgentInstanceRuntimeStatusRepository
	adapter     rt.Adapter
	instSvc     *AgentInstanceService // main.go 装配后回指：批次重启复用 Restart（容器与状态同步）
	mu          sync.Mutex            // Advance 串行化：并发进度查询不重复重启同一批
}

// NewAgentRolloutService 构造。
func NewAgentRolloutService(
	rolloutRepo repository.IAgentRuntimeRolloutRepository,
	podRepo repository.IAgentRuntimePodRepository,
	instRepo repository.IAgentInstanceRepository,
	statusRepo repository.IAgentInstanceRuntimeStatusRepository,
	adapter rt.Adapter,
) *AgentRolloutService {
	return &AgentRolloutService{rolloutRepo: rolloutRepo, podRepo: podRepo, instRepo: instRepo, statusRepo: statusRepo, adapter: adapter}
}

// SetInstanceService main.go 装配完 AgentInstanceService 后回指（批次重启走 Restart）。
func (s *AgentRolloutService) SetInstanceService(v *AgentInstanceService) { s.instSvc = v }

// Create 建发布单：rollout 行 + 每个目标实例一条 pending pod 行，随即置 running。
// 首批实例在第一次进度查询（Advance）时才真正重启——与惰性推进语义一致。
func (s *AgentRolloutService) Create(ctx context.Context, e *entity.AgentRuntimeRollout, instanceIds []string) (string, error) {
	if len(instanceIds) == 0 {
		return "", errors.New("请至少选择一个实例")
	}
	if e.TargetImageRef == "" {
		return "", errors.New("目标镜像不能为空")
	}
	if e.BatchSize < 1 {
		e.BatchSize = 1
	}
	if e.MaxUnavailable < 0 {
		e.MaxUnavailable = 0
	}
	if e.RuntimeType == "" {
		// 与 adapter provider 对齐（localdocker），避免两处口径漂移。
		if s.adapter != nil {
			e.RuntimeType = s.adapter.Provider()
		} else {
			e.RuntimeType = "localdocker"
		}
	}
	// 逐个校验实例存在（不校验归属：滚动发布是运维操作，管理员视角）。
	for _, id := range instanceIds {
		if _, err := s.instRepo.Get(ctx, id); err != nil {
			return "", fmt.Errorf("实例 %s 不存在", id)
		}
	}
	e.Status = 0 // pending，落库后立即置 running
	rolloutId, err := s.rolloutRepo.Add(ctx, e)
	if err != nil {
		return "", err
	}
	now := types.Time(time.Now())
	for _, id := range instanceIds {
		if _, err := s.podRepo.Add(ctx, &entity.AgentRuntimePod{
			RolloutId:   rolloutId,
			RuntimeType: e.RuntimeType,
			PodName:     "aiarsenal-" + id, // localdocker 约定：pod 行 ↔ 实例
			ImageRef:    e.TargetImageRef,  // 该 pod 应达的目标镜像
			State:       podStatePending,
			Capacity:    1,
			UsedSlots:   1,
			MetricsJson: "{}",
			LastSeenAt:  &now,
			CreatedAt:   &now,
			UpdatedAt:   &now,
		}); err != nil {
			return "", fmt.Errorf("pod 行创建失败（%s）: %w", id, err)
		}
	}
	// 置 running：started_at 落库。
	e.Status = rolloutStatusRunning
	e.StartedAt = &now
	if err := s.rolloutRepo.Update(ctx, e); err != nil {
		return rolloutId, err
	}
	log.Printf("[rollout:%s] 创建: instances=%d batch=%d image=%s by=%s",
		rolloutId, len(instanceIds), e.BatchSize, e.TargetImageRef, e.StartedBy)
	return rolloutId, nil
}

// Progress 进度查询（入口先惰性推进一次），返回发布单 + pod 批次。
func (s *AgentRolloutService) Progress(ctx context.Context, rolloutId string) (*entity.AgentRuntimeRollout, []*entity.AgentRuntimePod, error) {
	if _, err := s.Advance(ctx, rolloutId); err != nil {
		return nil, nil, err
	}
	ro, err := s.rolloutRepo.Get(ctx, rolloutId)
	if err != nil {
		return nil, nil, err
	}
	pods, _, err := s.podRepo.ListByRollout(ctx, rolloutId, 1, 1000)
	if err != nil {
		return nil, nil, err
	}
	return ro, pods, nil
}

// List 发布单分页（status<0=全部）。
func (s *AgentRolloutService) List(ctx context.Context, status int, pageNum, pageSize int) ([]*entity.AgentRuntimeRollout, int64, error) {
	return s.rolloutRepo.List(ctx, status, pageNum, pageSize)
}

// Advance 惰性推进：核对在途批次（版本达标→done，超时→failed），空档时投放下一批，
// 全部落定则收口发布单。请求驱动（见文件头注释），不依赖后台协程。
func (s *AgentRolloutService) Advance(ctx context.Context, rolloutId string) (*entity.AgentRuntimeRollout, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ro, err := s.rolloutRepo.Get(ctx, rolloutId)
	if err != nil {
		return nil, err
	}
	if ro.Status != rolloutStatusRunning {
		return ro, nil // pending/paused/completed/failed/cancelled 不推进
	}
	pods, _, err := s.podRepo.ListByRollout(ctx, rolloutId, 1, 1000)
	if err != nil {
		return nil, err
	}
	targetTag := imageTagOf(ro.TargetImageRef)

	// 1) 核对在途 pod（state=updating）：心跳版本达标→done；超时→failed。
	now := time.Now()
	inFlight := 0
	for _, p := range pods {
		if p.State != podStateUpdating {
			continue
		}
		if s.podTimedOut(p, now) {
			p.State = podStateFailed
			p.MetricsJson = `{"reason":"timeout"}`
			p.UpdatedAt = typesPtr(now)
			if err := s.podRepo.Update(ctx, p); err != nil {
				return ro, err
			}
			log.Printf("[rollout:%s] pod %s 超时标失败", rolloutId, p.PodName)
			continue
		}
		if v, ok := s.reportedVersion(ctx, instanceIdOfPod(p.PodName)); ok && v == targetTag {
			p.State = podStateDone
			p.UpdatedAt = typesPtr(now)
			if err := s.podRepo.Update(ctx, p); err != nil {
				return ro, err
			}
			log.Printf("[rollout:%s] pod %s 版本达标（%s）", rolloutId, p.PodName, v)
			continue
		}
		inFlight++
	}

	// 2) 无在途且还有 pending：投放下一批（batchSize 个）。
	if inFlight == 0 {
		batch := make([]*entity.AgentRuntimePod, 0, ro.BatchSize)
		for _, p := range pods {
			if p.State == podStatePending {
				batch = append(batch, p)
				if len(batch) >= ro.BatchSize {
					break
				}
			}
		}
		for _, p := range batch {
			if err := s.restartInstance(ctx, instanceIdOfPod(p.PodName)); err != nil {
				p.State = podStateFailed
				p.MetricsJson = `{"reason":"` + err.Error() + `"}`
				p.UpdatedAt = typesPtr(now)
				_ = s.podRepo.Update(ctx, p)
				log.Printf("[rollout:%s] pod %s 重启失败: %v", rolloutId, p.PodName, err)
				continue
			}
			p.State = podStateUpdating
			p.UpdatedAt = typesPtr(now)
			if err := s.podRepo.Update(ctx, p); err != nil {
				return ro, err
			}
			inFlight++
		}
	}

	// 3) 收口：pending 与 updating 都空 → completed / failed（任一 pod 失败即单失败）。
	remaining := 0
	failed := 0
	for _, p := range pods {
		switch p.State {
		case podStatePending, podStateUpdating:
			remaining++
		case podStateFailed:
			failed++
		}
	}
	if remaining == 0 {
		nowTypes := types.Time(now)
		if failed > 0 {
			ro.Status = rolloutStatusFailed
			ro.ErrorMessage = fmt.Sprintf("%d 个实例升级失败", failed)
		} else {
			ro.Status = rolloutStatusCompleted
		}
		ro.FinishedAt = &nowTypes
		if err := s.rolloutRepo.Update(ctx, ro); err != nil {
			return ro, err
		}
		log.Printf("[rollout:%s] 收口: status=%d failed=%d", rolloutId, ro.Status, failed)
	}
	return ro, nil
}

// podTimedOut 批内超时判定：pod.UpdatedAt（投放批次的时间）距今超 batchTimeout。
func (s *AgentRolloutService) podTimedOut(p *entity.AgentRuntimePod, now time.Time) bool {
	base := p.UpdatedAt
	if base == nil || time.Time(*base).IsZero() {
		base = p.CreatedAt
	}
	if base == nil {
		return false // 无时间基准不判超时（宁慢勿误杀）
	}
	return now.Sub(time.Time(*base)) > batchTimeout
}

// reportedVersion 读心跳链路落的 agent 进程版本（aa_agent_instance_runtime_status）。
// 无记录/读失败返回 ok=false（等待下一轮，不视作完成也不直接判失败——超时兜底）。
func (s *AgentRolloutService) reportedVersion(ctx context.Context, instanceId string) (string, bool) {
	if instanceId == "" {
		return "", false
	}
	st, err := s.statusRepo.GetByInstance(ctx, instanceId)
	if err != nil || st == nil {
		return "", false
	}
	if st.AgentProcVersion == "" {
		return "", false
	}
	return st.AgentProcVersion, true
}

// restartInstance 批次重启单个实例（优先走 AgentInstanceService.Restart：容器与库状态同步；
// 未装配时退回裸 adapter.Restart + 状态置 running）。
func (s *AgentRolloutService) restartInstance(ctx context.Context, instanceId string) error {
	if s.instSvc != nil {
		return s.instSvc.Restart(ctx, instanceId)
	}
	if s.adapter == nil {
		return errors.New("runtime adapter 未装配")
	}
	if err := s.adapter.Restart(ctx, instanceId); err != nil {
		return err
	}
	return s.instRepo.UpdateStateOnly(ctx, instanceId, 1)
}

// Pause 暂停：running→paused。在途批次照旧等心跳（已重启的实例不回滚）。
func (s *AgentRolloutService) Pause(ctx context.Context, id string) error {
	ro, err := s.rolloutRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if ro.Status != rolloutStatusRunning {
		return errors.New("仅运行中的发布单可暂停")
	}
	ro.Status = 2
	return s.rolloutRepo.Update(ctx, ro)
}

// Resume 恢复：paused→running，并立即惰性推进一次。
func (s *AgentRolloutService) Resume(ctx context.Context, id string) error {
	ro, err := s.rolloutRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if ro.Status != 2 {
		return errors.New("仅暂停中的发布单可恢复")
	}
	ro.Status = rolloutStatusRunning
	if err := s.rolloutRepo.Update(ctx, ro); err != nil {
		return err
	}
	_, err = s.Advance(ctx, id)
	return err
}

// Cancel 取消：running/paused→cancelled，未完成的 pod 行全部标 failed（reason=cancelled）。
// 已重启的实例不回滚（版本已达标者保留）。
func (s *AgentRolloutService) Cancel(ctx context.Context, id string) error {
	ro, err := s.rolloutRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if ro.Status != rolloutStatusRunning && ro.Status != 2 {
		return errors.New("仅运行中/暂停中的发布单可取消")
	}
	pods, _, err := s.podRepo.ListByRollout(ctx, id, 1, 1000)
	if err != nil {
		return err
	}
	now := types.Time(time.Now())
	for _, p := range pods {
		if p.State == podStatePending || p.State == podStateUpdating {
			p.State = podStateFailed
			p.MetricsJson = `{"reason":"cancelled"}`
			p.UpdatedAt = &now
			_ = s.podRepo.Update(ctx, p)
		}
	}
	ro.Status = 5
	ro.FinishedAt = &now
	if err := s.rolloutRepo.Update(ctx, ro); err != nil {
		return err
	}
	log.Printf("[rollout:%s] 已取消", id)
	return nil
}

// instanceIdOfPod pod_name → instanceId（localdocker 约定 "aiarsenal-<instanceId>"）。
func instanceIdOfPod(podName string) string {
	return strings.TrimPrefix(podName, "aiarsenal-")
}

// imageTagOf 取镜像引用的 tag 段（与心跳上报的 agent_proc_version 比对口径一致）：
// 先剥 registry/仓库路径（最后一个 / 之前），再取最后一个 : 之后；无 tag 视为 "latest"
//（docker 隐式 latest 语义）。
func imageTagOf(imageRef string) string {
	ref := imageRef
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		return ref[i+1:]
	}
	return "latest"
}

// typesPtr time.Time → *types.Time（本地小工具）。
func typesPtr(t time.Time) *types.Time {
	v := types.Time(t)
	return &v
}
