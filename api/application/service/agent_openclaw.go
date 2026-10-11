package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"time"

	"aiarsenal/domain/repository"
)

// agent_openclaw.go — openclaw 专属配置构造（free function）：openclaw.json。
//
// 给两个下游场景共用：
//   - 桌面设备场景：desktop_service.issueOpenclawConfig
//   - 实例/manager 场景：agent_openclaw_service.deliverOpenclawConfig
//
// 各 service 自己负责密钥签发（desktop 现签、instance 预签传入），共享点只到「拿到 key+baseURL 之后」
// 的配置构造。模型列表由通用 llm_models.go 拉取（与 agent 无关）；hermes 等其它 agent 的
// 配置构造对称放 agent_hermes.go / agent_<name>.go。

// OpenclawConfigOpts 构造 openclaw.json 的参数（两场景差异都收进这里）。
//
// Workspace: 桌面 "~/.openclaw/workspace" / 实例 "/aa/.openclaw/workspace"。
// Channels: 桌面传设备已启用渠道（非 nil 即写出 channels 对象，openclaw 要求 channels 是对象）；
//
//	实例传 nil（不下发 channels）。
//
// PluginsEntries: 桌面传 plugins.entries.<channel>.enabled；实例 nil。
// DefaultsModel: 实例设 "aiarsenal/<firstID>" 开箱即用；桌面留空。
// API: 适配器 openai-completions / anthropic-messages；空 = OpenclawProviderAPI 按部署协议族自选。
// ExecNode: 桌面传 client 注册的 node 名 → 写 tools.exec.host=node（exec 全部路由进 client 的
// system.run，TCC 权限归 AI Arsenal.app 而非网关裸 node 二进制；client 不在线时 exec fail closed）；
// 实例传空（容器内无配对 node，强制 node 会让 exec 全废）。
type OpenclawConfigOpts struct {
	LLMBaseURL     string
	APIKey         string
	API            string
	Models         []interface{}
	Workspace      string
	Channels       map[string]interface{} // nil=不写 channels key；非 nil（含空 map）=写出 channels 对象
	PluginsEntries map[string]interface{} // 仅 len>0 时写出 plugins.entries
	DefaultsModel  string                 // agents.defaults.model；空则不写
	ExecNode       string                 // 非空 → tools.exec.host=node + tools.exec.node 指向该 node
	McpServers     map[string]interface{} // 仅 len>0 时写出 mcp.servers（key=组名，value含 url/transport/headers）
}

// openaiFamilyProtocols 网关 /v1/chat/completions 可路由的 openai 兼容协议族
// （与 llm 网关 proxy.openaiCompatibleProviders 对齐——那边能路由的才能配 openai-completions 适配器）。
var openaiFamilyProtocols = map[string]bool{
	"openai": true, "deepseek": true, "moonshot": true, "groq": true, "openrouter": true, "ollama": true,
}

// openclawFallbackMaxTokens anthropic-messages 适配器下模型条目缺 maxTokens 时的兜底值。
// 16384 经 llm 网关 + glm 实测放行；openclaw 侧还会按 min(modelMax, 32k) 收敛。
const openclawFallbackMaxTokens = 16384

// positiveNumber interface{}（int/float64 等 JSON 与 Go 两种来源）是否为正数，缺字段/0/类型不符都算否。
func positiveNumber(v interface{}) bool {
	switch n := v.(type) {
	case int:
		return n > 0
	case int64:
		return n > 0
	case float64:
		return n > 0
	}
	return false
}

// OpenclawProviderAPI 按 aa_llm_model 活跃部署的协议族选 openclaw 适配器。
// 网关两个入口按协议族隔离路由：/v1/chat/completions 只路由 openai 兼容族、/v1/messages 只路由 anthropic 族；
// 部署全是 anthropic 族（如 bigmodel anthropic 端点凭据）却配 openai-completions 时，运行期每次调用必报
// "no model deployed"——而 /v1/models 不分协议族，下发前的模型校验发现不了这种错配，须在此处提前选对适配器。
// 有任一 openai 族部署 → openai-completions（多族并存时保持既有行为）；只有 anthropic 族 → anthropic-messages；
// 查询失败/无活跃部署 → 默认 openai-completions。
func OpenclawProviderAPI(ctx context.Context, llmModelRepo repository.ILLMModelRepository) string {
	if llmModelRepo == nil {
		return "openai-completions"
	}
	rows, err := llmModelRepo.All(ctx)
	if err != nil {
		return "openai-completions"
	}
	hasOpenAI, hasAnthropic := false, false
	for _, m := range rows {
		if m == nil || m.DeletedAt != nil || m.Blocked != 0 || m.Status != 1 {
			continue
		}
		if openaiFamilyProtocols[m.Provider] {
			hasOpenAI = true
		} else if m.Provider == "anthropic" {
			hasAnthropic = true
		}
	}
	if !hasOpenAI && hasAnthropic {
		return "anthropic-messages"
	}
	return "openai-completions"
}

// llmBaseURLIsPrivate 探测 llmBaseURL 的主机是否解析到私网/环回/链路本地地址。
// 主机是 IP 字面量直接判定；域名走系统解析（含 /etc/hosts，本地测试环境即靠它指向 127.0.0.1）。
// 解析失败/超时按公网处理（不写放行标记），最坏情况是本地环境首跑被 SSRF 拦——显式可排查。
func llmBaseURLIsPrivate(baseURL string) bool {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ipIsPrivate(ip)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil && ipIsPrivate(ip) {
			return true
		}
	}
	return false
}

func ipIsPrivate(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

// BuildOpenclawConfig 构造 openclaw.json 内容（gateway + models.providers.aiarsenal + agents + commands，
// 按需 channels/plugins）。gateway auth token 随机生成（每次下发新 token）。MarshalIndent 便于可读。
func BuildOpenclawConfig(opts OpenclawConfigOpts) string {
	gwToken := make([]byte, 24)
	_, _ = rand.Read(gwToken)
	agentDefaults := map[string]interface{}{"workspace": opts.Workspace}
	if opts.DefaultsModel != "" {
		agentDefaults["model"] = opts.DefaultsModel
	}
	api := opts.API
	if api == "" {
		api = "openai-completions"
	}
	// anthropic-messages 适配器强制要求正的 maxTokens：模型条目缺失该字段时传输层直接抛
	// "requires a positive maxTokens value"，请求出不了进程（openai-completions 无此要求）。
	// aa_llm_model.max_output_tokens 未配（0）时富化不写字段，agent 每轮必炸重试三次后报错——
	// 在此兜底补默认值，覆盖 FetchLLMModels 与 fallbackLLMModels 两条来源。
	if api == "anthropic-messages" {
		for _, m := range opts.Models {
			if mm, ok := m.(map[string]interface{}); ok && !positiveNumber(mm["maxTokens"]) {
				mm["maxTokens"] = openclawFallbackMaxTokens
			}
		}
	}
	// baseUrl 随适配器变化（fetchLLMModels 自己拼 /v1/models，仍用根 llmBaseURL，不受影响）：
	//   - openai-completions：须带 /v1——适配器只往 baseUrl 后拼 /chat/completions（不拼 /v1），
	//     网关仅在 /v1/chat/completions 提供路由；不带 /v1 会落到 /chat/completions → 404 page not found。
	//   - anthropic-messages：用根地址——适配器按 Anthropic SDK 约定拼 /v1/messages（baseUrl 不带 /v1）。
	providerBase := strings.TrimRight(opts.LLMBaseURL, "/")
	if api != "anthropic-messages" {
		providerBase += "/v1"
	}
	aiarsenalProvider := map[string]interface{}{
		"baseUrl": providerBase,
		"apiKey":  opts.APIKey,
		"api":     api,
		"models":  opts.Models,
	}
	// 本地测试环境 llm 网关域名经 /etc/hosts 指向 127.0.0.1，openclaw 的 SSRF 防护会拦截指向
	// 私网/环回地址的模型请求（SsrFBlockedError，请求出不了进程）。探测到 llmBaseURL 解析到
	// 私网/环回地址时显式写 request.allowPrivateNetwork 放行；公网域名不写，保持默认防护。
	if llmBaseURLIsPrivate(opts.LLMBaseURL) {
		aiarsenalProvider["request"] = map[string]interface{}{"allowPrivateNetwork": true}
	}
	cfgMap := map[string]interface{}{
		"gateway": map[string]interface{}{
			"mode": "local", "port": 18789, "bind": "loopback",
			"auth":      map[string]interface{}{"mode": "token", "token": hex.EncodeToString(gwToken)},
			"tailscale": map[string]interface{}{"mode": "off", "resetOnExit": false},
		},
		"models": map[string]interface{}{
			"mode": "merge",
			"providers": map[string]interface{}{
				"aiarsenal": aiarsenalProvider,
			},
		},
		"agents":   map[string]interface{}{"defaults": agentDefaults},
		"commands": map[string]interface{}{"restart": true},
		// messages.suppressToolErrors：工具执行失败时不向聊天渠道追加「⚠️ 🛠️ Exec failed: …」
		// 通告（openclaw 官方开关，config/types.messages.ts）。App 控制闸门的拦截（APP_NOT_ALLOWED）
		// 对 exec 也是失败，agent 汇报纪律（AGENTS.md）已要求它在回复里明说被拦原因，渠道侧再发
		// 一条英文原始命令通告就是重复噪音；静默执行失败也一并压掉——agent 回复里本就会带结果。
		"messages": map[string]interface{}{"suppressToolErrors": true},
		// 浏览器控制服务（loopback gateway.port+2=18791）是 opt-in：gateway 进程环境不设
		// OPENCLAW_EAGER_BROWSER_CONTROL_SERVER 就永远不监听（openclaw 文档 tools/browser-control.md）。
		// 桌面端 desktop_node 的 browser.proxy 是直接 HTTP 打 18791 的，不设则 agent 浏览器工具
		// 全部 UNAVAILABLE（node invoke failed）。config env.vars 在网关启动时写入进程环境
		// （进程环境已有同名键则不覆盖），随配置下发持久化。
		"env": map[string]interface{}{
			"vars": map[string]interface{}{"OPENCLAW_EAGER_BROWSER_CONTROL_SERVER": "1"},
		},
		// meta 必须带：openclaw 配置守卫用 hasMeta（meta.lastTouchedVersion/At 是 string）对比 last-good，
		// 缺 meta 判 missing-meta-vs-last-good → 自动恢复备份（下发被改回旧值）。openclaw 自写时自动设，
		// api 下发须自带（值不校验，只校验是 string）。
		"meta": map[string]interface{}{
			"lastTouchedVersion": "aiarsenal-api",
			"lastTouchedAt":      time.Now().UTC().Format(time.RFC3339),
		},
	}
	if opts.Channels != nil {
		cfgMap["channels"] = opts.Channels // openclaw 要求 channels 是对象不是数组；空对象也写出
	}
	// exec 路由收紧：默认 tools.exec.host=auto 在 sandbox 关闭时解析为 gateway（本机网关），
	// 模型可绕过 node 提示直接 exec 本地 shell，敏感操作（osascript 通讯录/screencapture）的
	// TCC 权限会授给网关裸 node 二进制。桌面场景强制 host=node，本地执行只走 client 的 system.run。
	if opts.ExecNode != "" {
		cfgMap["tools"] = map[string]interface{}{
			"exec": map[string]interface{}{"host": "node", "node": opts.ExecNode},
		}
	}
	if len(opts.PluginsEntries) > 0 {
		cfgMap["plugins"] = map[string]interface{}{"entries": opts.PluginsEntries}
	}
	// MCP 网关接入：mcp.servers.{组名} = {url, transport: streamable-http, headers.Authorization: Bearer <key>}。
	// 桌面设备走全量重建下发；docker 实例走读改写重推（upsertMcpSection），两条链路最终落到同一段。
	if len(opts.McpServers) > 0 {
		cfgMap["mcp"] = map[string]interface{}{"servers": opts.McpServers}
	}
	b, _ := json.MarshalIndent(cfgMap, "", "  ")
	return string(b)
}
