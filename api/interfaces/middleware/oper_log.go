package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
)

// OperLog 写操作审计：非 GET 请求在执行后落一条 aa_oper_log。
//
// 挂载位置在 v1 根分组（CORS 之后、JWTAuth 之前）。中间件本身不发起鉴权，
// 只在 c.Next() 返回后读取 JWTAuth 写入的用户身份，因此对「未登录请求」不记录
// （登录、验证码、deviceCode 交换等，由各自的登录日志负责）。
//
// 三条设计约束：
//  1. 记录失败不能影响业务：Insert 出错只写 slog，不改响应。
//  2. 请求体要还原：读完请求体必须放回 c.Request.Body，否则处理器读到空 body。
//  3. 敏感字段不落库：密码、token、密钥在写库前掩码，multipart 只记占位符。
const (
	auditMaxParamBytes = 2048 // 落库的请求参数上限
	auditMaxRespBytes  = 4096 // 解析业务响应码时缓存的上限
	auditBodyReadLimit = 64 << 10
	auditWriteTimeout  = 3 * time.Second
)

// auditSkipPaths 不记审计的写接口：机器心跳与运行时计量是周期性上报，
// 按设备或实例每几秒一次，入库会把审计表刷成噪音，真正的事件反而被淹没。
var auditSkipPaths = map[string]struct{}{
	"/api/v1/client/heartbeat":        {},
	"/api/v1/agent/runtime/heartbeat": {},
	"/api/v1/agent/runtime/report":    {},
}

// 若依约定的业务类型，前端按这个值渲染标签。
const (
	businessTypeAdd    = 1
	businessTypeEdit   = 2
	businessTypeDelete = 3
)

func OperLog(repo repository.IOperLogRepository) gin.HandlerFunc {
	if repo == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if !auditRequired(c) {
			c.Next()
			return
		}

		param := auditRequestParam(c)
		w := &auditResponseWriter{ResponseWriter: c.Writer, limit: auditMaxRespBytes}
		c.Writer = w
		c.Next()

		entry := buildOperLog(c, w, param)
		if entry == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), auditWriteTimeout)
		defer cancel()
		if err := repo.Insert(ctx, entry); err != nil {
			slog.Error("写审计日志失败",
				"err", err,
				"method", entry.RequestMethod,
				"url", entry.OperUrl,
				"operName", entry.OperName,
			)
		}
	}
}

// auditRequired 判定一次请求是否需要落审计。方法先行：GET 一律不算写操作。
//
// 这里不能判断「当前有没有登录用户」：中间件挂在 JWTAuth 之前，入口处用户还没被写入
// context，判断会恒为 false。是否已登录由 buildOperLog 在 c.Next() 之后判定。
func auditRequired(c *gin.Context) bool {
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	_, skip := auditSkipPaths[c.FullPath()]
	return !skip
}

// auditResponseWriter 旁路缓存响应体，用来解析 {code,message} 判断业务成败。
// 只缓存前 limit 字节，其余透传；不改变任何响应行为。
type auditResponseWriter struct {
	gin.ResponseWriter
	buf   bytes.Buffer
	limit int
}

func (w *auditResponseWriter) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *auditResponseWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *auditResponseWriter) capture(b []byte) {
	if w.buf.Len() >= w.limit {
		return
	}
	remain := w.limit - w.buf.Len()
	if len(b) > remain {
		b = b[:remain]
	}
	w.buf.Write(b)
}

func buildOperLog(c *gin.Context, w *auditResponseWriter, param string) *entity.OperLog {
	u := GetUser(c)
	if u == nil {
		return nil
	}
	now := types.Time(time.Now())
	operatorType := 1 // 后台用户
	if u.DeviceId != "" {
		operatorType = 2 // 桌面客户端设备
	}
	return &entity.OperLog{
		OperId:        newAuditID(),
		Title:         auditTitle(c.FullPath()),
		BusinessType:  businessType(c.Request.Method),
		Method:        c.HandlerName(),
		RequestMethod: c.Request.Method,
		OperatorType:  operatorType,
		OperName:      u.UserName,
		OperUrl:       c.Request.URL.Path,
		OperIp:        c.ClientIP(),
		OperParam:     param,
		ErrorMsg:      auditErrorMsg(w),
		OperTime:      &now,
	}
}

func businessType(method string) int {
	switch method {
	case http.MethodPost:
		return businessTypeAdd
	case http.MethodPut, http.MethodPatch:
		return businessTypeEdit
	case http.MethodDelete:
		return businessTypeDelete
	default:
		return 0
	}
}

// auditTitle 取路由模式串去掉 /api/v1 前缀，例如 system/user/edit。
// 不查菜单表：审计写入在请求链路上，不为一个展示字段多打一次库。
func auditTitle(fullPath string) string {
	return strings.TrimPrefix(fullPath, "/api/v1/")
}

// auditErrorMsg 业务失败时返回 message。本项目的失败走 HTTP 200 + code=-1，
// 只看状态码会漏掉全部业务失败，所以必须解析响应体。
func auditErrorMsg(w *auditResponseWriter) string {
	if w == nil {
		return ""
	}
	status := w.ResponseWriter.Status()
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	parsed := json.Unmarshal(w.buf.Bytes(), &body) == nil
	if parsed && body.Code != 0 {
		return body.Message
	}
	if status >= http.StatusBadRequest {
		if parsed && body.Message != "" {
			return body.Message
		}
		return http.StatusText(status)
	}
	return ""
}

// auditRequestParam 读取并还原请求体，返回掩码后的文本。
func auditRequestParam(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		return "[multipart/form-data omitted]"
	}
	if c.Request.ContentLength > auditBodyReadLimit {
		return "[body too large: " + strconv.FormatInt(c.Request.ContentLength, 10) + " bytes]"
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	// 必须还原：中间件读过之后，处理器拿到的应当是原始内容。
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	return truncate(maskSensitive(raw, contentType), auditMaxParamBytes)
}

// 敏感字段：命中即整值替换为 ***，避免密码、令牌、密钥进审计表。
var sensitiveKeyRe = regexp.MustCompile(`(?i)(password|passwd|pwd|token|secret|api[_-]?key|apikey|authorization|private[_-]?key|credential)`)

var formSensitiveRe = regexp.MustCompile(`(?i)((?:password|passwd|pwd|token|secret|api[_-]?key|apikey|authorization|private[_-]?key|credential)=)[^&]*`)

func maskSensitive(raw []byte, contentType string) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var v interface{}
		if err := json.Unmarshal(trimmed, &v); err == nil {
			masked := maskValue(v)
			if b, err := json.Marshal(masked); err == nil {
				return string(b)
			}
		}
	}
	if strings.Contains(contentType, "application/x-www-form-urlencoded") || bytes.Contains(trimmed, []byte("=")) {
		return formSensitiveRe.ReplaceAllString(string(trimmed), "${1}***")
	}
	return string(trimmed)
}

func maskValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			if sensitiveKeyRe.MatchString(k) {
				t[k] = "***"
				continue
			}
			t[k] = maskValue(val)
		}
		return t
	case []interface{}:
		for i, item := range t {
			t[i] = maskValue(item)
		}
		return t
	default:
		return v
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…(truncated)"
}

// newAuditID 生成 32 位十六进制业务 id，与库中其他 oper_id 形态一致。
func newAuditID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 熵源异常时退化为纳秒时间戳（仍保持 32 位十六进制，不破坏列宽）
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
