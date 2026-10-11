package controller

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// LLM 虚拟密钥控制器。签发返回明文一次；轮换返回新明文。

type LLMKeyController struct {
	svc              *service.LLMKeyService
	agentInstanceSvc *service.AgentInstanceService
	noticeSvc        *service.NoticeService
}

func NewLLMKeyController(s *service.LLMKeyService, ais *service.AgentInstanceService, ns *service.NoticeService) *LLMKeyController {
	return &LLMKeyController{svc: s, agentInstanceSvc: ais, noticeSvc: ns}
}

func (h *LLMKeyController) List(c *gin.Context) {
	var req dto.LLMKeySearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.List(c, req.Alias, req.UserId, req.TeamId, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.LLMKeySearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *LLMKeyController) Get(c *gin.Context) {
	hash := c.Query("tokenHash")
	if hash == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.Get(c, hash)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *LLMKeyController) Add(c *gin.Context) {
	var req dto.LLMKeyAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMKeyReq
	e := keyFromReq(r, middleware.GetUserId(c), "")
	// 密钥归属当前登录用户及其所在部门（团队=组织=部门），前端不传、后端从登录态取。
	e.UserId = middleware.GetUserId(c)
	if dept := middleware.GetDeptId(c); dept != "" {
		e.TeamId = dept
		e.OrgId = dept
	}
	hash, plain, err := h.svc.Create(c, e)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, dto.LLMKeyCreateRes{TokenHash: hash, Key: plain})
}

func (h *LLMKeyController) Edit(c *gin.Context) {
	var req dto.LLMKeyEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	r := req.LLMKeyReq
	e := keyFromReq(r, "", middleware.GetUserId(c))
	e.TokenHash = req.TokenHash
	if err := h.svc.Edit(c, e); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	// 虚拟 key 编辑（allowed_models/预算等）→ 若绑定 agent 实例，重推其配置（best-effort）。
	if h.agentInstanceSvc != nil {
		h.agentInstanceSvc.RefreshByLLMKeyHashIfBound(c, req.TokenHash, deriveLLMBaseURL(c), middleware.GetUserId(c))
	}
	middleware.Success(c, struct{}{})
}

func (h *LLMKeyController) Delete(c *gin.Context) {
	var req dto.LLMIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	// 删除前先记录哪些 key 绑了实例（删后给 owner 推通知）。instance.llm_key_hash 不随 key 删除而变。
	operator := middleware.GetUserId(c)
	type bound struct{ userId, name, instanceId string }
	var bounds []bound
	if h.agentInstanceSvc != nil {
		for _, hash := range req.Ids {
			if inst, err := h.agentInstanceSvc.GetBoundInstanceByLLMKeyHash(c, hash); err == nil && inst != nil {
				bounds = append(bounds, bound{inst.UserId, inst.Name, inst.InstanceId})
			}
		}
	}
	if err := h.svc.Delete(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	// 删后给每个受影响实例的 owner 推通知（best-effort）。
	if h.noticeSvc != nil {
		for _, b := range bounds {
			name := b.name
			if name == "" {
				name = b.instanceId
			}
			content := fmt.Sprintf("您的实例「%s」绑定的 LLM 密钥已被删除，该实例的 AI 功能将不可用，请重新绑定密钥。", name)
			if err := h.noticeSvc.Push(c, b.userId, "实例 LLM 密钥已被删除", content, "/agent/instance/list", "warning", operator); err != nil {
				log.Printf("[llm-key] 删除通知推送失败 instance=%s: %v", b.instanceId, err)
			}
		}
	}
	middleware.Success(c, struct{}{})
}

// CheckUse GET /llm/key/check-use?tokenHash=X — 删除前查该 key 是否被实例使用（前端据此决定是否二次确认）。
func (h *LLMKeyController) CheckUse(c *gin.Context) {
	hash := c.Query("tokenHash")
	if hash == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	used := false
	name := ""
	if h.agentInstanceSvc != nil {
		if inst, err := h.agentInstanceSvc.GetBoundInstanceByLLMKeyHash(c, hash); err == nil && inst != nil {
			used = true
			name = inst.Name
			if name == "" {
				name = inst.InstanceId
			}
		}
	}
	middleware.Success(c, gin.H{"usedByInstance": used, "instanceName": name})
}

func (h *LLMKeyController) Block(c *gin.Context) {
	hash := c.Query("tokenHash")
	if hash == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	blocked := c.Query("blocked") == "1"
	if err := h.svc.Block(c, hash, blocked, middleware.GetUserId(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	// 封禁/解封：若该 key 绑定了实例，给实例 owner 推一条通知（best-effort）。
	h.notifyBoundInstance(c, hash, blocked, middleware.GetUserId(c))
	middleware.Success(c, struct{}{})
}

// notifyBoundInstance 按 tokenHash 反查绑定的实例，命中则给其 owner 推一条封禁/解封通知（best-effort，失败仅记日志）。
func (h *LLMKeyController) notifyBoundInstance(c *gin.Context, tokenHash string, blocked bool, operator string) {
	if h.agentInstanceSvc == nil || h.noticeSvc == nil || tokenHash == "" {
		return
	}
	inst, err := h.agentInstanceSvc.GetBoundInstanceByLLMKeyHash(c, tokenHash)
	if err != nil || inst == nil {
		return // 该 tokenHash 未绑定实例（非 instance key），无需通知
	}
	name := inst.Name
	if name == "" {
		name = inst.InstanceId
	}
	action, ntype, effect := "已解封", "success", "已恢复可用"
	if blocked {
		action, ntype, effect = "已被封禁", "warning", "相关 AI 功能将不可用，请联系管理员"
	}
	title := "实例 LLM 密钥" + action
	content := fmt.Sprintf("您的实例「%s」绑定的 LLM 密钥%s，%s。", name, action, effect)
	if err := h.noticeSvc.Push(c, inst.UserId, title, content, "/agent/instance/list", ntype, operator); err != nil {
		log.Printf("[llm-key] 封禁/解封通知推送失败 instance=%s: %v", inst.InstanceId, err)
	}
}

func (h *LLMKeyController) Rotate(c *gin.Context) {
	hash := c.Query("tokenHash")
	if hash == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	plain, err := h.svc.Rotate(c, hash, middleware.GetUserId(c))
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	// 轮换后旧明文 5 分钟宽限失效 → 绑定的实例须重推拿新明文（best-effort）。
	if h.agentInstanceSvc != nil {
		h.agentInstanceSvc.RefreshByLLMKeyHashIfBound(c, hash, deriveLLMBaseURL(c), middleware.GetUserId(c))
	}
	middleware.Success(c, dto.LLMKeyCreateRes{TokenHash: "", Key: plain, GraceMinutes: 5})
}

func keyFromReq(r *dto.LLMKeyReq, createdBy, updatedBy string) *entity.LLMVirtualKey {
	e := &entity.LLMVirtualKey{
		KeyAlias: r.KeyAlias, UserId: r.UserId, TeamId: r.TeamId, ProjectId: r.ProjectId, OrgId: r.OrgId,
		Models: r.Models, MaxBudget: nil, SoftBudget: nil, BudgetId: r.BudgetId, TpmLimit: nil, RpmLimit: nil,
		MaxParallelRequests: nil, BudgetDuration: r.BudgetDuration, AllowedModels: r.AllowedModels,
		AllowedRoutes: r.AllowedRoutes, Blocked: r.Blocked, CreatedBy: createdBy, UpdatedBy: updatedBy,
	}
	if r.MaxBudget > 0 {
		v := r.MaxBudget
		e.MaxBudget = &v
	}
	if r.SoftBudget > 0 {
		v := r.SoftBudget
		e.SoftBudget = &v
	}
	if r.TpmLimit > 0 {
		v := r.TpmLimit
		e.TpmLimit = &v
	}
	if r.RpmLimit > 0 {
		v := r.RpmLimit
		e.RpmLimit = &v
	}
	if r.MaxParallelRequests > 0 {
		v := r.MaxParallelRequests
		e.MaxParallelRequests = &v
	}
	if r.Expires != "" {
		if t, err := time.Parse(time.RFC3339, r.Expires); err == nil {
			tt := types.Time(t)
			e.Expires = &tt
		}
	}
	return e
}
