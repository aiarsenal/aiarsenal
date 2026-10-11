package controller

import (
	"crypto/subtle"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/gorilla/websocket"

	rt "github.com/aiarsenal/runtime-go"

	"aiarsenal/application/service"
	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/interfaces/dto"
	"aiarsenal/interfaces/middleware"
)

// AgentInstanceController Agent 运行实例：通过 localdocker 真起/停容器。
type AgentInstanceController struct{ svc *service.AgentInstanceService }

func NewAgentInstanceController(s *service.AgentInstanceService) *AgentInstanceController {
	return &AgentInstanceController{svc: s}
}

// deriveLLMBaseURL 从请求 Host 推导 LLM 网关地址（api.aiarsenal.cc→<scheme>://llm.aiarsenal.cc），
// 写进 agent 运行配置的 LLM provider baseUrl（openclaw.json/hermes config.yaml）。非 api.* 域返回空（跳过密钥相关下发）。
// scheme 跟随入口协议 X-Forwarded-Proto：测试环境纯 HTTP，生产 HTTPS；缺省 https。
// env(AIARSENAL_LLM_BASE_URL) 可整体覆盖——本机测试环境容器回连须走 host.docker.internal 直连端口
// （hosts 的 *.aiarsenal.cc→127.0.0.1 被 colima host-resolver 透传进容器 DNS，域名在容器内指向容器自身）。
func deriveLLMBaseURL(c *gin.Context) string {
	if u := os.Getenv("AIARSENAL_LLM_BASE_URL"); u != "" {
		return u
	}
	host := c.Request.Host
	if fh := c.GetHeader("X-Forwarded-Host"); fh != "" {
		host = fh
	}
	if !strings.HasPrefix(host, "api.") {
		return ""
	}
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "https"
	}
	return scheme + "://llm." + strings.TrimPrefix(host, "api.")
}

func (h *AgentInstanceController) List(c *gin.Context) {
	var req dto.AgentInstanceSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	uid := middleware.GetUserId(c)
	list, total, err := h.svc.List(c, uid, req.Name, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentInstanceSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentInstanceController) ListAll(c *gin.Context) {
	var req dto.AgentInstanceSearchReq
	if err := c.ShouldBindQuery(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	list, total, err := h.svc.ListAll(c, req.Name, req.Status, req.PageNum, req.PageSize)
	if err != nil {
		middleware.Fail(c, "获取数据失败")
		return
	}
	pn := req.PageNum
	if pn == 0 {
		pn = 1
	}
	middleware.Success(c, dto.AgentInstanceSearchRes{List: list, ListRes: dto.ListRes{CurrentPage: pn, Total: total}})
}

func (h *AgentInstanceController) Get(c *gin.Context) {
	id := c.Query("instanceId")
	if id == "" {
		middleware.Fail(c, "参数错误")
		return
	}
	v, err := h.svc.Get(c, id)
	if err != nil {
		middleware.Fail(c, "未找到")
		return
	}
	middleware.Success(c, v)
}

func (h *AgentInstanceController) Create(c *gin.Context) {
	var req dto.AgentInstanceReq
	// ShouldBindBodyWith 缓存请求体：dto.AgentInstanceReq 不归本任务改（禁止动 interfaces/dto/*），
	// volumeIds 用第二个小结构体从同一份 JSON 里补解出来（走既有 ShouldBindBodyWith 缓存）。
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	var volumeReq struct {
		VolumeIds []string `json:"volumeIds"` // 可选：创建时一并挂载的持久卷（aa_agent_persistent_volume）
	}
	_ = c.ShouldBindBodyWith(&volumeReq, binding.JSON)
	uid := middleware.GetUserId(c)
	ports := make([]rt.PortMap, 0, len(req.Ports))
	for _, p := range req.Ports {
		ports = append(ports, rt.PortMap{HostPort: p.HostPort, ContainerPort: p.ContainerPort})
	}
	// 桌面实例走 noVNC(3001)。adapter 只发布请求里列出的端口，
	// 故这里兜底确保 3001 被发布（HostPort=0 → docker 分配随机宿主端口，provision 回读实际值）。
	{
		hasDesk := false
		for _, p := range ports {
			if p.ContainerPort == 3001 {
				hasDesk = true
				break
			}
		}
		if !hasDesk {
			ports = append(ports, rt.PortMap{HostPort: 0, ContainerPort: 3001})
		}
	}
	ins := &entity.AgentInstance{
		UserId: uid, Name: req.Name, Description: req.Description,
		Agent: req.Agent, Version: req.Version, Status: 0,
		CpuCores: req.CpuCores, MemoryGb: req.MemoryGb, DiskGb: req.DiskGb,
		ImageTag: req.ImageTag, EnvironmentOverridesJson: req.EnvironmentOverridesJson,
		MountPath: req.MountPath, CreatedBy: uid,
	}
	if ins.MountPath == "" {
		ins.MountPath = "/data"
	}
	// LLM 网关地址随实例下发（api.aiarsenal.cc→<scheme>://llm.aiarsenal.cc）：写进 agent 运行配置
	// 的 models.providers.aiarsenal.baseUrl，agent 据此接 aiarsenal LLM 网关。
	id, err := h.svc.Create(c, ins, ports, deriveLLMBaseURL(c), volumeReq.VolumeIds)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"instanceId": id})
}

// RefreshLLMKey 轮换实例专属 LLM 密钥并重发 agent 运行配置（零停机，5 分钟宽限内切换）。
func (h *AgentInstanceController) RefreshLLMKey(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.RefreshLLMKey(c, id, deriveLLMBaseURL(c)); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentInstanceController) Start(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Start(c, id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentInstanceController) Stop(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Stop(c, id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentInstanceController) Restart(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Restart(c, id); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentInstanceController) Delete(c *gin.Context) {
	var req dto.AgentIdsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	if err := h.svc.Remove(c, req.Ids); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, struct{}{})
}

func (h *AgentInstanceController) Status(c *gin.Context) {
	id := c.Param("id")
	st, err := h.svc.Status(c, id)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, st)
}

func (h *AgentInstanceController) Exec(c *gin.Context) {
	id := c.Param("id")
	var req dto.AgentInstanceExecReq
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	out, err := h.svc.Exec(c, id, req.Cmd)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	middleware.Success(c, gin.H{"output": string(out)})
}

func (h *AgentInstanceController) Logs(c *gin.Context) {
	id := c.Param("id")
	follow := c.Query("follow") == "1"
	tail := c.Query("tail")
	rc, err := h.svc.Logs(c, id, follow, tail)
	if err != nil {
		middleware.Fail(c, err.Error())
		return
	}
	defer rc.Close()
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Status(200)
	_, _ = io.Copy(c.Writer, rc)
}

// DesktopAccess 签发「打开桌面」的短期路径 token，返回同源代理 URL 给前端拼 iframe.src。
// 走 agInst（JWTAuth+CasbinAuth）：仅实例 owner、且实例 running + 桌面端口就绪可签。
func (h *AgentInstanceController) DesktopAccess(c *gin.Context) {
	id := c.Param("id")
	uid := middleware.GetUserId(c)
	ins, err := h.svc.Get(c, id)
	if err != nil || ins == nil {
		middleware.Fail(c, "未找到实例")
		return
	}
	if ins.UserId != uid {
		middleware.Fail(c, "无权访问该实例")
		return
	}
	if ins.Status != 1 || ins.HostPort <= 0 {
		middleware.Fail(c, "实例未运行或桌面端口未就绪")
		return
	}
	tok, err := auth.GenerateDesktopAccess(uid, id)
	if err != nil {
		middleware.Fail(c, "签发凭证失败")
		return
	}
	// 末尾 / 必须有：桌面 SPA 用相对路径解析资产/WS，缺斜杠会被 gin 重定向丢状态。
	middleware.Success(c, gin.H{
		"url": strings.TrimRight(config.Get().DesktopProxyBaseURL(), "/") +
			"/api/v1/agent/instance/" + id + "/desktop/" + tok + "/",
	})
}

// DesktopProxy 公开反向代理（无 JWTAuth，路径里的短期 JWT 自证身份）：
// 校验 token → 反代到容器 127.0.0.1:<HostPort>，同时处理 HTTP（资产/HTML）与 WebSocket Upgrade（桌面流）。
// 剥 X-Frame-Options / CSP frame-ancestors 以允许 iframe 嵌入。
func (h *AgentInstanceController) DesktopProxy(c *gin.Context) {
	id := c.Param("id")
	tok := c.Param("token")
	sub := c.Param("any") // 含前导 '/'
	if sub == "" {
		sub = "/"
	}

	claims, err := auth.Parse(tok)
	if err != nil {
		c.String(http.StatusUnauthorized, "桌面凭证无效或已过期")
		return
	}
	if claims.InstanceId == "" || claims.UserId == "" {
		c.String(http.StatusUnauthorized, "桌面凭证无效")
		return
	}
	// 路径里的 id 必须等于 token 里的 instanceId（constant-time）
	if subtle.ConstantTimeCompare([]byte(claims.InstanceId), []byte(id)) != 1 {
		c.String(http.StatusForbidden, "实例不匹配")
		return
	}
	ins, err := h.svc.Get(c, id)
	if err != nil || ins == nil || ins.HostPort <= 0 || ins.Status != 1 {
		c.String(http.StatusServiceUnavailable, "桌面实例不可用")
		return
	}
	if ins.UserId != claims.UserId {
		c.String(http.StatusForbidden, "无权访问该实例")
		return
	}

	// 桌面走 noVNC(3001)：上游是自签 HTTPS。浏览器只与本代理（平台有效证书）通信，
	// 代理→容器这段自签证书由 Transport/Dialer 的 InsecureSkipVerify 兜住，iframe 看不到证书问题。
	upstream := "https://127.0.0.1:" + strconv.Itoa(ins.HostPort)
	if isWebSocketUpgrade(c.Request) {
		h.serveDesktopWS(c, upstream, sub)
		return
	}
	h.serveDesktopHTTP(c, upstream, sub)
}

// serveDesktopHTTP httputil.ReverseProxy 转发到容器桌面 HTTPS(3001, 自签)；剥 iframe 限制头。
func (h *AgentInstanceController) serveDesktopHTTP(c *gin.Context, upstream, sub string) {
	target, err := url.Parse(upstream)
	if err != nil {
		c.String(http.StatusInternalServerError, "上游地址错误")
		return
	}
	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = sub // 已剥离 /agent/instance/<id>/desktop/<token>
			req.Host = target.Host
			req.Header.Del("Authorization")
			req.Header.Set("X-Forwarded-Host", c.Request.Host)
		},
		// 容器 3001 是自签证书，忽略校验（浏览器看不到这段，只看平台证书）。
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("X-Frame-Options")
			if csp := resp.Header.Get("Content-Security-Policy"); csp != "" {
				resp.Header.Set("Content-Security-Policy", stripFrameAncestors(csp))
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, e error) {
			http.Error(w, "桌面实例不可达: "+e.Error(), http.StatusBadGateway)
		},
	}
	rp.ServeHTTP(c.Writer, c.Request)
}

var desktopWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 路径 token 已自证身份
}

// serveDesktopWS WebSocket 双向代理：浏览器 ↔ api ↔ 容器桌面 WS（signaling/帧流）。
// 每方向单 goroutine 读写对端（gorilla 禁止并发写，本模式安全）。
func (h *AgentInstanceController) serveDesktopWS(c *gin.Context, upstream, sub string) {
	in, err := desktopWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Upgrade 已写过响应
	}
	defer in.Close()

	u, _ := url.Parse(upstream)
	u.Scheme = "wss" // 3001 自签 HTTPS → wss
	u.Path = sub
	u.RawQuery = c.Request.URL.RawQuery

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
		Subprotocols:     websocket.Subprotocols(c.Request),
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true}, // 容器 3001 自签证书
	}
	out, _, err := dialer.DialContext(c, u.String(), nil)
	if err != nil {
		return
	}
	defer out.Close()

	done := make(chan struct{}, 2)
	pipe := func(dst, src *websocket.Conn) {
		defer func() { done <- struct{}{} }()
		for {
			mt, msg, e := src.ReadMessage()
			if e != nil {
				return
			}
			if e := dst.WriteMessage(mt, msg); e != nil {
				return
			}
		}
	}
	go pipe(out, in) // 上行：浏览器 → 容器
	go pipe(in, out) // 下行：容器 → 浏览器
	<-done
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// stripFrameAncestors 从 CSP 里删掉 frame-ancestors 指令（允许 iframe 嵌入），保留其余。
func stripFrameAncestors(csp string) string {
	parts := strings.Split(csp, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.Contains(strings.ToLower(strings.TrimSpace(p)), "frame-ancestors") {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, ";")
}
