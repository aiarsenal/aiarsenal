// Package ops 平台内部能力 MCP server（真实数据）：把 api 自身的业务数据以 MCP 工具
// 暴露给 agent，经网关（mcp/，:43213）分组聚合后调用。与 mcp/cmd/demo 的合成数据演示相对：
// 这里每一条都来自真实库查询。与 Web UI 同一套业务逻辑（api 的 service 层），只是入口不同
// ——本包是 MCP 出入口的薄适配层，由 cmd/ops 第二 main 拉起（http :43220）。
//
// 端点（读写分开，网关侧分别注册、分组授权即可只给读不给写）：
//
//	/mcp              platform-ops           平台数据只读（实例/设备/技能/MCP 审计）
//	/mcp/doc          platform-doc           文档库只读（检索/阅读/列表）
//	/mcp/doc-write    platform-doc-write     文档库写入（新建/更新，自动重嵌）
//	/mcp/kb           platform-kb            知识库只读（检索/阅读/列表）
//	/mcp/kb-write     platform-kb-write      知识库写入（新建/更新，自动重嵌）
//	/mcp/notice       platform-notice        通知只读（我的通知/未读数/收件人解析）
//	/mcp/notice-write platform-notice-write   通知写入（发通知/标记已读）
//	/mcp/project      platform-project       项目只读（项目/任务/里程碑/看板/动态/成员）
//	/mcp/project-write platform-project-write 项目写入（建改项目任务/看板移动/评论）
//	/mcp/skill        platform-skill         技能只读（检索/版本/市场/审计/已装列表）
//	/mcp/skill-write  platform-skill-write   技能写入（市场导入/版本发布/实例装卸）
//	/mcp/user         platform-user          员工信息查询（花名册/部门/档案，只读）
//	/mcp/budget       platform-budget        预算只读（我的预算/临时预算申请列表）
//	/mcp/budget-write platform-budget-write  临时预算申请写入（申请/通过/驳回）
//	/mcp/schedule     platform-schedule      日程只读（我的日程/共同空闲/会议室）
//	/mcp/schedule-write platform-schedule-write 日程写入（创建/修改/取消，含周期与单场例外；confirmed 预检确认模式）
//
// 平台数据工具是 agent 专用聚合读模型（状态翻中文、跨库计数），UI 没有对应形态，
// SQL 直接写在工具旁、不入 persistence；列一律显式写出（不 SELECT *）——实例/设备表
// 有 token 类秘密列，禁止带出。文档工具则完全薄壳到 DocService（与 UI 上传同库同算法）。
//
// 加新工具三步：
//  1. 写一个 func(ctx, a map[string]any) (string, error)——读模型 queryRows(dbXxx, ...) 或调 service
//  2. 在对应 build*Server 里 opsTool(...) 注册名字/描述/参数
//  3. go build 后网关侧无需改动——网关按注册表自动发现工具（tools/list 实时反映）
package ops

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"

	"aiarsenal/application/service"
	"aiarsenal/config"
	"aiarsenal/infrastructure/crypto"
)

// Deps ops 入口的业务装配（cmd/ops/main.go 组装，全部与 Web UI 同一套 service）。
// 域渐次扩充：先 doc/kb，后技能/项目/通知/看板；未装配的域为零值 nil，
// 对应 server 也就不建（Run 里按 nil 判定跳过挂载）。
type Deps struct {
	Host string
	Port int

	Doc *service.DocService
	Kb  *service.KbService

	// 技能域
	Skill      *service.SkillService
	SkillNS    *service.SkillNamespaceService
	SkillSec   *service.SkillSecurityService
	Market     *service.SkillMarketService
	MarketSite *service.SkillMarketSiteService
	Runtime    *service.AgentRuntimeService
	Embed      service.EmbeddingProvider

	// 项目域
	Project   *service.ProjectService
	Task      *service.ProjectTaskService
	Milestone *service.ProjectMilestoneService
	Board     *service.ProjectBoardService
	Activity  *service.ProjectActivityService
	Report    *service.ProjectReportService
	Timeline  *service.ProjectTimelineService
	Member    *service.ProjectMemberService

	// 通知域
	Notice *service.NoticeService

	// 预算域（llm 库，跨库软引用 aa_user/aa_dept；临时预算申请=统一审核工单，主库 aa_review_order）
	LLMCfg    *service.LLMConfigService
	BudgetReq *service.ReviewOrderService

	// 日程域（aa_schedule 库：事件/参会人/单场例外/会议室）
	Schedule *service.ScheduleService

	// 看板/监控
	Dashboard *service.AgentDashboardService
	Monitor   *service.AgentMonitorService
}

// serverMount 一个端点一个 server 的挂载对（runHTTP 消费）
type serverMount struct {
	path string
	srv  *mcp.Server
}

// Run 启动 ops 入口：http 模式常驻各域端点（+ /health），stdio 模式只跑平台数据 server（本地冒烟用）
func Run(transport string, d Deps) error {
	mounts := []serverMount{
		{"/mcp", buildPlatformServer(d)},
		{"/mcp/doc", buildDocServer(d.Doc)},
		{"/mcp/doc-write", buildDocWriteServer(d.Doc)},
		{"/mcp/kb", buildKbServer(d.Kb)},
		{"/mcp/kb-write", buildKbWriteServer(d.Kb)},
	}
	if d.Notice != nil {
		mounts = append(mounts,
			serverMount{"/mcp/notice", buildNoticeServer(d)},
			serverMount{"/mcp/notice-write", buildNoticeWriteServer(d)})
	}
	if d.Project != nil {
		mounts = append(mounts,
			serverMount{"/mcp/project", buildProjectServer(d)},
			serverMount{"/mcp/project-write", buildProjectWriteServer(d)})
	}
	if d.Skill != nil {
		mounts = append(mounts,
			serverMount{"/mcp/skill", buildSkillServer(d)},
			serverMount{"/mcp/skill-write", buildSkillWriteServer(d)})
	}
	if d.LLMCfg != nil && d.BudgetReq != nil {
		mounts = append(mounts,
			serverMount{"/mcp/budget", buildBudgetServer(d)},
			serverMount{"/mcp/budget-write", buildBudgetWriteServer(d)})
	}
	if d.Schedule != nil {
		mounts = append(mounts,
			serverMount{"/mcp/schedule", buildScheduleServer(d)},
			serverMount{"/mcp/schedule-write", buildScheduleWriteServer(d)})
	}
	// 员工信息查询：纯主库读模型，不依赖 service 装配（主库必 init），直接挂载
	mounts = append(mounts, serverMount{"/mcp/user", buildUserServer()})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch transport {
	case "stdio":
		err := mounts[0].srv.Run(ctx, &mcp.StdioTransport{})
		// 管道关闭（网关回收/上游断开）属正常退出，不刷错误
		if err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "closing") {
			return err
		}
		return nil
	case "http":
		return runHTTP(ctx, mounts, d.Host, d.Port)
	default:
		return fmt.Errorf("不支持的 transport：%s（http | stdio）", transport)
	}
}

// runHTTP 以 streamable-http 暴露各域 server（与网关 /mcp 同款 handler），另带 /health
func runHTTP(ctx context.Context, mounts []serverMount, host string, port int) error {
	mux := http.NewServeMux()
	h := func(s *mcp.Server) func(*http.Request) *mcp.Server {
		return func(*http.Request) *mcp.Server { return s }
	}
	for _, m := range mounts {
		mux.Handle(m.path, mcp.NewStreamableHTTPHandler(h(m.srv), nil))
	}
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"aiarsenal-mcp-ops"}`))
	})

	s := &http.Server{Addr: fmt.Sprintf("%s:%d", host, port), Handler: withActor(mux)}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
	}()
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// ===== 令牌交换验签（网关→ops 第二跳身份透传） =====

// ctxActorKey 请求 ctx 里的已验签调用方身份
type ctxActorKey struct{}

// actorInfo 网关签名透传的调用方（任意有归属人的 bearer key：user 自建 / agent 挂
// 实例 / client 挂设备；Owner=归属用户，权限按该用户五元组判定，跟载体无关）
type actorInfo struct {
	KeyId string
	Owner string
	Kind  string
}

// withActor HTTP 包装：X-AA-Actor/X-AA-Signature 验签（HMAC-SHA256 + exp 短窗防重放）
// 通过则 actor 入 ctx；缺失/无效 = 系统身份（不设值，判定层全放行，loopback 信任边界）。
// 注意：go-sdk 有状态会话的工具调用 ctx 继承自「建连那次请求」（streamable.go Connect
// 传 req.Context()），网关→ops 的上游连接在建连时不带身份头——所以每请求身份主要靠
// opsTool 里从 req.Extra.Header 逐调用提取（SDK 把当次 POST 的 header 挂在 RequestExtra），
// 这里只覆盖建连请求自身的 ctx（直连 ops 且握手带头时仍生效）。
func withActor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a, ok := verifyActorHeader(r.Header.Get("X-AA-Actor"), r.Header.Get("X-AA-Signature")); ok {
			r = r.WithContext(context.WithValue(r.Context(), ctxActorKey{}, a))
		}
		next.ServeHTTP(w, r)
	})
}

// verifyActorHeader 验签 + 解析（签名/载荷/验签单源在 infrastructure/crypto/actor.go，
// 网关 interfaces/gateway 同一函数签发——两侧字段序天然一致）
func verifyActorHeader(payload, sig string) (actorInfo, bool) {
	c, ok := crypto.VerifyActor(config.Get().Actor.SignKey, payload, sig)
	if !ok {
		return actorInfo{}, false
	}
	return actorInfo{KeyId: c.KeyId, Owner: c.Owner, Kind: c.Kind}, true
}

// actorCtxFromRequest 每调用提取身份：SDK 会话的工具 ctx 继承自建连请求，per-POST 的
// header 只挂在 req.Extra（RequestExtra.Header）——从当次 tools/call 的头验签注入 actor，
// 会话期间换调用方（网关侧换 key 重连前的过渡）也能正确归属。无头/验签失败保持原 ctx。
func actorCtxFromRequest(ctx context.Context, req *mcp.CallToolRequest) context.Context {
	if req == nil || req.Extra == nil || req.Extra.Header == nil {
		return ctx
	}
	if a, ok := verifyActorHeader(req.Extra.Header.Get("X-AA-Actor"), req.Extra.Header.Get("X-AA-Signature")); ok {
		return context.WithValue(ctx, ctxActorKey{}, a)
	}
	return ctx
}

// toolArg 工具参数（schema 用；required=false 时可缺省）
type toolArg struct {
	name     string
	desc     string
	required bool
}

// opsTool 低阶注册：schema 手写（参数统一 string），handler 里自解参数——与网关 demo 同款风格
func opsTool(srv *mcp.Server, name, desc string, args []toolArg, fn func(ctx context.Context, a map[string]any) (string, error)) {
	props := map[string]any{}
	var required []string
	for _, t := range args {
		props[t.name] = map[string]any{"type": "string", "description": t.desc}
		if t.required {
			required = append(required, t.name)
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	srv.AddTool(&mcp.Tool{Name: name, Description: desc, InputSchema: schema},
		func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ctx = actorCtxFromRequest(ctx, req)
			var a map[string]any
			if len(req.Params.Arguments) > 0 {
				if err := json.Unmarshal(req.Params.Arguments, &a); err != nil {
					return nil, fmt.Errorf("参数解析失败：%w", err)
				}
			}
			out, err := fn(ctx, a)
			if err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: out}}}, nil
		})
}

func jsonOut(rows []map[string]any, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "（无数据）", nil
	}
	b, _ := json.MarshalIndent(rows, "", "  ")
	return string(b), nil
}

// jsonOutAny 任意结构（total+items / 单对象 map 等）的规整 JSON 输出；错误由调用方先行判断
func jsonOutAny(v any) (string, error) {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b), nil
}

// ===== 参数取值助手（缺省/类型不符回落默认值；与网关 demo 同款） =====

func argStr(m map[string]any, k, def string) string {
	if v, ok := m[k].(string); ok && v != "" {
		return v
	}
	return def
}

func argInt(m map[string]any, k string, def int) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case string:
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// clampInt 把上限参数收进合法区间（SQL LIMIT 前）
func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

// ===== 只读查询助手（挂在 infrastructure/db 的 gorm 连接上） =====

// queryRows 只读查询 → 列名到规整值的行数组（time→字符串、nil→""、数值保原样）
func queryRows(g *gorm.DB, q string, args ...any) ([]map[string]any, error) {
	rows, err := g.Raw(q, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[c] = plainVal(vals[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// scalar 单行单列计数查询（count 类）→ int64
func scalar(g *gorm.DB, q string, args ...any) (int64, error) {
	var n sql.NullInt64
	if err := g.Raw(q, args...).Row().Scan(&n); err != nil {
		return 0, err
	}
	return n.Int64, nil
}

func plainVal(v any) any {
	switch t := v.(type) {
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case []byte:
		return string(t)
	case nil:
		return ""
	default:
		return v
	}
}
