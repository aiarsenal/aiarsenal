package service

import (
	"context"
	"encoding/json"
	"time"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
)

// agent_usage_service.go — 运行时用量上报与查询（aa_agent_instance_usage 接线）。
//
// 上报：POST /agent/runtime/report（InstanceAuth，容器内 agent 控制面）批量推
// {usage:[{model,promptTokens,completionTokens,cost}], logs:[...]}，每个 usage 项落
// 一行（model/prompt_tokens/completion_tokens/cost），logs 整批 JSON 序列化挂到
// 首行 logs_json（日志是批级附件，逐行冗余无意义）。资源水位（cpu/mem/disk）仍走
// 心跳链路（aa_agent_instance_runtime_status），本表只收计量。
//
// 心跳服务不动（agent_runtime_service.go）；本服务只接计量与查询。

// UsageReportItem 单条模型计量（上报请求 usage 数组元素）。
type UsageReportItem struct {
	Model            string  `json:"model"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	Cost             float64 `json:"cost"`
}

// UsageReport 一次上报的计量批次。
type UsageReport struct {
	Usage []UsageReportItem `json:"usage"`
	Logs  []string          `json:"logs"` // 批级日志行（挂首行 logs_json）
}

// AgentUsageService 用量上报与查询服务。
type AgentUsageService struct {
	repo repository.IAgentInstanceUsageRepository
}

// NewAgentUsageService 构造。
func NewAgentUsageService(repo repository.IAgentInstanceUsageRepository) *AgentUsageService {
	return &AgentUsageService{repo: repo}
}

// Report 落一批计量。返回落库行数。逐行独立插入：单行失败记日志继续（计量丢一行
// 可容忍，整批回滚反而放大故障面）。
func (s *AgentUsageService) Report(ctx context.Context, instanceId string, r UsageReport) (int, error) {
	logsJson := "{}"
	if len(r.Logs) > 0 {
		if b, err := json.Marshal(r.Logs); err == nil {
			logsJson = string(b)
		}
	}
	inserted := 0
	for i, item := range r.Usage {
		row := &entity.AgentInstanceUsage{
			InstanceId:       instanceId,
			Model:            item.Model,
			PromptTokens:     item.PromptTokens,
			CompletionTokens: item.CompletionTokens,
			Cost:             item.Cost,
			LogsJson:         "{}",
		}
		if i == 0 {
			row.LogsJson = logsJson // 批级日志只挂首行
		}
		if _, err := s.repo.Add(ctx, row); err != nil {
			return inserted, err
		}
		inserted++
	}
	return inserted, nil
}

// UsageSummary 时间窗聚合（GET /agent/usage 顶部统计卡）。
type UsageSummary struct {
	TotalPromptTokens     int64   `json:"totalPromptTokens"`
	TotalCompletionTokens int64   `json:"totalCompletionTokens"`
	TotalCost             float64 `json:"totalCost"`
	Records               int64   `json:"records"`
}

// UsagePage 查询出参：分页行 + 全窗聚合。
type UsagePage struct {
	List    []*entity.AgentInstanceUsage `json:"list"`
	Total   int64                        `json:"total"`
	Summary UsageSummary                 `json:"summary"`
}

// List 查询（instanceId 空=全部实例；start/end 零值=不限）。
// 聚合口径：对同一时间窗做一次大页（10000 行）扫描求和——量级超限时聚合为近似值，
// 精确口径后续走 SQL SUM（避免为一条 SQL 在 repo 加只在摘要用的方法）。
func (s *AgentUsageService) List(ctx context.Context, instanceId string, start, end time.Time, pageNum, pageSize int) (*UsagePage, error) {
	list, total, err := s.repo.ListRange(ctx, instanceId, start, end, pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	page := &UsagePage{List: list, Total: total}
	sumRows, _, err := s.repo.ListRange(ctx, instanceId, start, end, 1, 10000)
	if err != nil {
		return page, nil // 聚合失败不阻断列表
	}
	for _, r := range sumRows {
		page.Summary.TotalPromptTokens += r.PromptTokens
		page.Summary.TotalCompletionTokens += r.CompletionTokens
		page.Summary.TotalCost += r.Cost
		page.Summary.Records++
	}
	return page, nil
}
