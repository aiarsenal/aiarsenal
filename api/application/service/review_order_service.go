package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/model"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/persistence"
)

// 统一审核中心：全平台申请单汇聚（主库 aa_review_order），审核系统「我的提交/我的审核」唯一数据源。
// 单表多类型：order_type 区分审核类型，各类型注册 ReviewTypeHandler 承载专属校验/执行/通知文案
// （首个类型 llm_temp_budget 见 review_type_llm.go，后续 skill 等类型照注册即可，表与页面零改动）。
//
// 审批原子性（跨库无分布式事务）：主库行锁关单（pending 哨兵防并发双批）→ handler.Execute 执行
// 跨库业务动作（如 aa_llm 库建临时预算）→ 失败 Reopen 补偿回 pending；补偿也失败则工单留
// approved 无产物（双 DB 故障，slog.Error 留痕人工处理）。
//
// 审批人 = 申请部门现任负责人（leader 缺失/"0"/=申请人时全部超管兜底）；
// 通知双通道（站内铃铛 + hermes 数字人同一消息）：实现收口在 channel_notice_service.go。

const reviewOrderPageSize = 20

// ReviewTypeHandler 审核类型扩展点：一种申请类型一个实现，在 main 装配时注册进 ReviewOrderService。
type ReviewTypeHandler interface {
	Type() string
	// Prepare 提交前校验并构建工单内容：通用列（标题/金额）+ 类型专属 payload 快照。
	Prepare(ctx context.Context, t model.Identity, params map[string]any) (title string, amount types.Milli, payload map[string]any, err error)
	// Execute 审批通过的业务动作（自管事务，如 aa_llm 库建临时预算），返回产物引用记入 result_ref。
	Execute(ctx context.Context, t model.Identity, o *entity.ReviewOrder) (resultRef string, err error)
	// PendingNotice 通知审批人（标题/内容；跳转固定 /review/todo）。
	PendingNotice(o *entity.ReviewOrder) (title, content string)
	// ApprovedNotice 通知申请人（标题/内容/跳转）。
	ApprovedNotice(o *entity.ReviewOrder) (title, content, link string)
	// RejectedNotice 通知申请人（标题/内容/跳转）。
	RejectedNotice(o *entity.ReviewOrder, note string) (title, content, link string)
}

// ReviewAutoApprover 可选实现：提交即免审（如 llm_temp_budget 申请人持最高职级岗位）。
// 命中时 Create 落库后当场走审批通过路径（不走 pending、不通知审批人），工单照常留痕。
type ReviewAutoApprover interface {
	ShouldAutoApprove(ctx context.Context, t model.Identity) bool
}

// autoApproveNote 自动通过的审批意见（写入工单 review_note，我的提交可见缘由）。
const autoApproveNote = "最高职级自动通过（免审）"

type ReviewOrderService struct {
	orderRepo    repository.IReviewOrderRepository
	userRepo     repository.IUserRepository
	deptRepo     repository.IDeptRepository
	noticeSvc    *NoticeService
	runtimeSvc   *AgentRuntimeService
	instanceRepo repository.IAgentInstanceRepository
	handlers     map[string]ReviewTypeHandler
}

func NewReviewOrderService(r repository.IReviewOrderRepository, u repository.IUserRepository,
	d repository.IDeptRepository, n *NoticeService, rt *AgentRuntimeService,
	i repository.IAgentInstanceRepository, handlers ...ReviewTypeHandler) *ReviewOrderService {
	s := &ReviewOrderService{orderRepo: r, userRepo: u, deptRepo: d,
		noticeSvc: n, runtimeSvc: rt, instanceRepo: i, handlers: map[string]ReviewTypeHandler{}}
	for _, h := range handlers {
		s.handlers[h.Type()] = h
	}
	return s
}

// Create 提交审核工单：类型校验/内容构建在 handler.Prepare，快照姓名/部门落库。
// handler 实现 ReviewAutoApprover 且规则命中（如最高职级免审）时当场通过并通知申请人，
// 否则通知审批人（部门负责人；leader 缺失/"0"/=申请人本人时通知全部超管）。
func (s *ReviewOrderService) Create(ctx context.Context, t model.Identity, orderType string, params map[string]any) (*entity.ReviewOrder, error) {
	if t.UserId == "" {
		return nil, errors.New("缺少申请人身份")
	}
	h, ok := s.handlers[orderType]
	if !ok {
		return nil, fmt.Errorf("不支持的审核类型：%s", orderType)
	}
	title, amount, payload, err := h.Prepare(ctx, t, params)
	if err != nil {
		return nil, err
	}
	name, deptId := t.UserId, t.DeptId
	if u, uerr := s.userRepo.GetEntityById(ctx, t.UserId); uerr == nil && u != nil {
		if u.Name != "" {
			name = u.Name
		}
		deptId = u.DeptId
	}
	pj, _ := json.Marshal(payload)
	e := &entity.ReviewOrder{
		OrderType:     orderType,
		Title:         title,
		ApplicantId:   t.UserId,
		ApplicantName: name,
		DeptId:        deptId,
		Amount:        amount,
		Payload:       string(pj),
		Status:        "pending",
		CreatedBy:     t.UserId,
		UpdatedBy:     t.UserId,
	}
	orderId, err := s.orderRepo.Add(ctx, e)
	if err != nil {
		return nil, err
	}
	e.OrderId = orderId
	// 免审类型规则命中（如最高职级）：当场走通过路径，审批人记申请人本人；
	// Execute 失败则 Reopen 回 pending 并继续走人工审批通知（宁可多审不可漏发）。
	if aa, ok := h.(ReviewAutoApprover); ok && aa.ShouldAutoApprove(ctx, t) {
		if closed, aerr := s.approveInternal(ctx, t, e, h, autoApproveNote); aerr == nil {
			return closed, nil
		} else {
			slog.Error("审核工单自动通过失败，已回退人工审批", "orderId", orderId, "type", orderType, "err", aerr)
		}
	}
	// 通知审批人（best-effort，不阻断申请）。
	pt, pc := h.PendingNotice(e)
	for _, approver := range s.approverIds(ctx, deptId, t.UserId) {
		s.notifyAndPush(ctx, approver, pt, pc, "/review/todo", "warning")
	}
	return e, nil
}

// ListMine 我的提交（全类型；status 空=全部）。
func (s *ReviewOrderService) ListMine(ctx context.Context, t model.Identity, status string, pn, ps int) ([]*entity.ReviewOrder, int64, error) {
	if pn <= 0 {
		pn = 1
	}
	if ps <= 0 {
		ps = reviewOrderPageSize
	}
	return s.orderRepo.ListMine(ctx, t.UserId, status, pn, ps)
}

// ListReview 待我审批（超管=全部；否则=本部门快照）。审批视角 status 空时默认 pending。
func (s *ReviewOrderService) ListReview(ctx context.Context, t model.Identity, status string, pn, ps int) ([]*entity.ReviewOrder, int64, error) {
	if pn <= 0 {
		pn = 1
	}
	if ps <= 0 {
		ps = reviewOrderPageSize
	}
	if status != "approved" && status != "rejected" {
		status = "pending"
	}
	return s.orderRepo.ListReview(ctx, t.UserId, t.DeptId, t.IsAdmin, status, pn, ps)
}

// Approve 审批通过：先关单（行锁+pending 哨兵，防并发双批重复执行）→ handler.Execute 跨库业务
// 动作 → SetResultRef；Execute 失败 Reopen 补偿回 pending 让双方可重试。成功后通知申请人。
func (s *ReviewOrderService) Approve(ctx context.Context, t model.Identity, orderId, note string) (*entity.ReviewOrder, error) {
	o, h, err := s.reviewTarget(ctx, t, orderId)
	if err != nil {
		return nil, err
	}
	if len([]rune(note)) > 500 {
		return nil, errors.New("审批意见过长（≤500 字）")
	}
	return s.approveInternal(ctx, t, o, h, note)
}

// approveInternal 通过路径本体（人工审批与 Create 免审共用）：关单（行锁+pending 哨兵）
// → handler.Execute 跨库业务动作 → SetResultRef → 通知申请人；Execute 失败 Reopen 回 pending。
func (s *ReviewOrderService) approveInternal(ctx context.Context, t model.Identity,
	o *entity.ReviewOrder, h ReviewTypeHandler, note string) (*entity.ReviewOrder, error) {
	closed, err := s.orderRepo.Close(ctx, o.OrderId, "approved", t.UserId, s.displayName(ctx, t.UserId), note)
	if err != nil {
		return nil, err
	}
	ref, execErr := h.Execute(ctx, t, closed)
	if execErr != nil {
		if rerr := s.orderRepo.Reopen(ctx, o.OrderId); rerr != nil {
			slog.Error("审核工单补偿回滚失败（工单已关单但业务动作未执行，需人工核对）",
				"orderId", o.OrderId, "type", o.OrderType, "execErr", execErr, "reopenErr", rerr)
		}
		return nil, execErr
	}
	closed.ResultRef = ref
	if ref != "" {
		if serr := s.orderRepo.SetResultRef(ctx, o.OrderId, ref); serr != nil {
			slog.Warn("审核工单结果引用回写失败（不影响流转）", "orderId", o.OrderId, "err", serr)
		}
	}
	at, ac, al := h.ApprovedNotice(closed)
	s.notifyAndPush(ctx, closed.ApplicantId, at, ac, al, "success")
	return closed, nil
}

// Reject 驳回并通知申请人。
func (s *ReviewOrderService) Reject(ctx context.Context, t model.Identity, orderId, note string) (*entity.ReviewOrder, error) {
	_, h, err := s.reviewTarget(ctx, t, orderId)
	if err != nil {
		return nil, err
	}
	if len([]rune(note)) > 500 {
		return nil, errors.New("审批意见过长（≤500 字）")
	}
	closed, err := s.orderRepo.Close(ctx, orderId, "rejected", t.UserId, s.displayName(ctx, t.UserId), note)
	if err != nil {
		return nil, err
	}
	rt, rc, rl := h.RejectedNotice(closed, note)
	s.notifyAndPush(ctx, closed.ApplicantId, rt, rc, rl, "error")
	return closed, nil
}

// reviewTarget 取工单并校验当前身份可审（超管，或申请部门的现任负责人）。
// leader 在提交与审批之间变更时以现任为准（以部门当前配置为准的取舍）。
func (s *ReviewOrderService) reviewTarget(ctx context.Context, t model.Identity, orderId string) (*entity.ReviewOrder, ReviewTypeHandler, error) {
	o, err := s.orderRepo.Get(ctx, orderId)
	if err != nil {
		return nil, nil, errors.New("申请不存在")
	}
	if o.Status != "pending" {
		return nil, nil, persistence.ErrOrderClosed
	}
	h := s.handlers[o.OrderType]
	if h == nil {
		return nil, nil, fmt.Errorf("不支持的审核类型：%s", o.OrderType)
	}
	if !t.IsAdmin {
		allowed := false
		if o.DeptId != "" {
			if d, derr := s.deptRepo.Get(ctx, o.DeptId); derr == nil && d != nil && d.LeaderId == t.UserId {
				allowed = true
			}
		}
		if !allowed {
			return nil, nil, errors.New("无权审批该申请（需部门负责人或管理员）")
		}
	}
	return o, h, nil
}

// approverIds 申请的审批人集合：部门负责人；leader 缺失/"0"/=申请人本人时退化为全部超管。
func (s *ReviewOrderService) approverIds(ctx context.Context, deptId, applicantId string) []string {
	if deptId != "" {
		if d, err := s.deptRepo.Get(ctx, deptId); err == nil && d != nil &&
			d.LeaderId != "" && d.LeaderId != "0" && d.LeaderId != applicantId {
			return []string{d.LeaderId}
		}
	}
	ids, err := s.userRepo.ListAdminIds(ctx)
	if err != nil {
		slog.Warn("审核工单审批人兜底（超管列表）读取失败", "err", err)
		return nil
	}
	return ids
}

// notifyAndPush 双通道通知（同一消息两条投递）：委托 channel_notice_service 单源实现。
func (s *ReviewOrderService) notifyAndPush(ctx context.Context, userId, title, content, link, ntype string) {
	pushChannelNotice(ctx, s.noticeSvc, s.runtimeSvc, s.instanceRepo, userId, title, content, link, ntype, channelNotifyUser)
}

// displayName 取用户展示名（姓名优先，回退 userId）；读取失败不阻断流程。
func (s *ReviewOrderService) displayName(ctx context.Context, userId string) string {
	if u, err := s.userRepo.GetEntityById(ctx, userId); err == nil && u != nil && u.Name != "" {
		return u.Name
	}
	return userId
}

// paramStr/paramFloat 从前端/MCP 的 JSON 参数 map 取值（数字统一 float64，兼容字符串数字）。
func paramStr(m map[string]any, k, def string) string {
	if v, ok := m[k]; ok {
		switch s := v.(type) {
		case string:
			return s
		case float64:
			return strconv.FormatFloat(s, 'f', -1, 64)
		case json.Number:
			return s.String()
		case bool:
			return strconv.FormatBool(s)
		}
	}
	return def
}

func paramFloat(m map[string]any, k string, def float64) float64 {
	if v, ok := m[k]; ok {
		switch f := v.(type) {
		case float64:
			return f
		case json.Number:
			if n, err := f.Float64(); err == nil {
				return n
			}
		case string:
			if n, err := strconv.ParseFloat(strings.TrimSpace(f), 64); err == nil {
				return n
			}
		}
	}
	return def
}
