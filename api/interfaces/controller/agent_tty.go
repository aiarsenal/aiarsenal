package controller

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"aiarsenal/application/service"
	"aiarsenal/infrastructure/auth"
	"aiarsenal/interfaces/middleware"
)

// agent_tty.go — 实例交互式终端（websocket tty）。
//
// 两段式（与「打开桌面」DesktopAccess/DesktopProxy 同构）：
//  1. TtyAccess（JWT+Casbin 路由）：owner 校验后签 5 分钟路径 token；
//  2. TtyServeWS（公开路由，token 自证身份）：升级 WS，经 AgentTtyService 打开
//     容器内 /bin/sh 的 exec attach 流，双向泵透传：
//     - 浏览器 → 容器：二进制帧原样写 stdin；
//     - 浏览器 → api 的文本帧按 {"cols":N,"rows":N} 解析为终端 resize；
//     - 容器 → 浏览器：stdout/stderr（TTY 模式下合流）原样以二进制帧下发。
//
// 仅 localdocker 运行时支持（CVM 容器在远端宿主），服务层已闸。

// AgentTtyController 交互式终端控制器。
type AgentTtyController struct {
	svc     *service.AgentTtyService
	instSvc *service.AgentInstanceService // 实例归属/状态校验（Get）
}

// NewAgentTtyController 构造。
func NewAgentTtyController(svc *service.AgentTtyService, instSvc *service.AgentInstanceService) *AgentTtyController {
	return &AgentTtyController{svc: svc, instSvc: instSvc}
}

// ttyWSUpgrader 复用 desktop 的宽松 Origin 策略：路径 token 已自证身份。
var ttyWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// TtyAccess 签发 tty 短期路径 token（POST /agent/instance/:id/tty/access）。
// 仅实例 owner、实例 running 可签。
func (h *AgentTtyController) TtyAccess(c *gin.Context) {
	id := c.Param("id")
	uid := middleware.GetUserId(c)
	ins, err := h.instSvc.Get(c, id)
	if err != nil || ins == nil {
		middleware.Fail(c, "未找到实例")
		return
	}
	if ins.UserId != uid {
		middleware.Fail(c, "无权访问该实例")
		return
	}
	if ins.Status != 1 {
		middleware.Fail(c, "实例未运行")
		return
	}
	// 复用 GenerateDesktopAccess：同一套「userId+instanceId 5 分钟 JWT」语义，
	// tty 与桌面共用 claims（Claims.UserId/InstanceId），不另造签发函数。
	tok, err := auth.GenerateDesktopAccess(uid, id)
	if err != nil {
		middleware.Fail(c, "签发凭证失败")
		return
	}
	middleware.Success(c, gin.H{
		"url": "/api/v1/agent/instance/" + id + "/tty/" + tok,
	})
}

// TtyServeWS 公开 WS 端点（GET /agent/instance/:id/tty/:token）。
func (h *AgentTtyController) TtyServeWS(c *gin.Context) {
	id := c.Param("id")
	tok := c.Param("token")

	claims, err := auth.Parse(tok)
	if err != nil || claims.InstanceId == "" || claims.UserId == "" {
		c.String(http.StatusUnauthorized, "终端凭证无效或已过期")
		return
	}
	// 路径 id 必须 == token instanceId（constant-time，同 DesktopProxy）。
	if subtle.ConstantTimeCompare([]byte(claims.InstanceId), []byte(id)) != 1 {
		c.String(http.StatusForbidden, "实例不匹配")
		return
	}
	ins, err := h.instSvc.Get(c, id)
	if err != nil || ins == nil {
		c.String(http.StatusServiceUnavailable, "实例不可用")
		return
	}
	if ins.UserId != claims.UserId {
		c.String(http.StatusForbidden, "无权访问该实例")
		return
	}

	in, err := ttyWSUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Upgrade 已写过响应
	}
	defer in.Close()

	stream, err := h.svc.Open(c, id, 0, 0)
	if err != nil {
		_ = in.WriteMessage(websocket.TextMessage, []byte("终端打开失败: "+err.Error()))
		return
	}
	defer stream.Close()

	done := make(chan struct{}, 2)
	// 下行：容器 stdout/stderr → 浏览器（二进制帧原样透传）。
	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 32*1024)
		for {
			n, rerr := stream.Read(buf)
			if n > 0 {
				if werr := in.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if rerr != nil {
				return
			}
		}
	}()
	// 上行：浏览器 → 容器 stdin。二进制帧原样写；文本帧按 {"cols","rows"} 解析为 resize。
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			mt, msg, e := in.ReadMessage()
			if e != nil {
				return
			}
			if mt == websocket.TextMessage {
				var resize struct {
					Cols int `json:"cols"`
					Rows int `json:"rows"`
				}
				if json.Unmarshal(msg, &resize) == nil && resize.Cols > 0 && resize.Rows > 0 {
					_ = stream.Resize(uint(resize.Cols), uint(resize.Rows))
				}
				continue
			}
			if len(msg) == 0 {
				continue
			}
			if _, e := stream.Write(msg); e != nil {
				return
			}
		}
	}()
	<-done
}
