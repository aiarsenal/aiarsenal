# MCP 接入指南

MCP（Model Context Protocol）网关把分散的上游 MCP server（stdio 子进程 / SSE / streamable-http）聚合成一个统一端点，配 Bearer 密钥鉴权与调用审计，供 Claude Code、Cursor、Claude Desktop 等 AI 客户端接入。

## 架构

```
AI 客户端                    nginx                      mcp 网关                上游 server
(Claude Code/ ──Bearer──▶ mcp.aiarsenal.cc ──▶ :43213 /mcp[/:scope] ──▶ ┌ stdio 子进程（按需拉起）
 Cursor/桌面端)             (本机 80 / prod 443)   SSE + streamable-http   ├ SSE 远端
                                                  聚合·鉴权·审计·压缩      └ streamable-http 远端
       ▲
       └─ 管理后台 api :43211（server/分组/密钥/审计/提示词/资源/设置 7 个页面）
              └─ 与网关共享 aa_mcp PostgreSQL 库（两 Go module 零代码耦合，表结构按约定同步）
```

- 网关与 api 分离的原因：SSE 长连接 + stdio 子进程常驻 + 会话状态，与 api 的无状态 REST 模型冲突。
- 端口：网关 `:43213`，管理 api `:43211`；本机 `scripts/local-test.sh` 起 nginx（`http://mcp.aiarsenal.cc`），prod 走 https（80 → 301 443）。

## 快速开始（三步）

1. **管理台签发密钥**：`MCP → 密钥` → 「签发密钥」→ 弹窗复制明文（仅显示一次；泄露点「轮换」立即作废旧值）。
2. **客户端接入**：密钥页 / 服务器页点「接入」，弹窗给出现成配置（粘 token 即可复制），见下文三种客户端。
3. **验证**：客户端里列工具（能看到 `demo__echo` 等），或按下文 curl 直测；`MCP → 调用审计` 出现记录即全链路通。

## 端点与 scope 语义

| 端点 | 聚合范围 | 说明 |
|---|---|---|
| `/mcp` | 全部启用的 server | 推荐默认；`$smart` 等价（预留给 pgvector 智能路由） |
| `/mcp/{group}` | 指定分组的成员 | 分组在 `MCP → 分组` 维护，可对成员裁剪 tools/prompts/resources |
| `/mcp/{server}` | 单个 server | 只暴露该 server 的工具 |

- **工具名一律带 `server__` 前缀**（如 `demo__echo`、`fetch__get`），前缀与 scope 无关——即使 `/mcp/demo` 单 server 端点也带前缀，避免多源工具重名。
- 网关另暴露内置提示词/资源：prompt `builtin_prompts`、资源模板 `aa-mcp://builtin/{name}`（`MCP → 提示词/资源` 页面维护内容）。
- `DELETE /mcp/{server}` 停掉该 server 的 stdio 子进程（会话级回收，下次调用再按需拉起）。

## 密钥与授权

密钥（Bearer Key）字段：

| 字段 | 说明 |
|---|---|
| kind | `user` / `system` |
| accessType | `all`（全部）· `groups`（指定分组）· `servers`（指定 server）· `custom`（分组+服务器混合白名单） |
| enabled | 停用立即失效（401） |

请求头：`Authorization: Bearer <明文>`。密钥的授权范围决定能访问哪些 scope；无权限的 scope 返回 401/403。

## 客户端接入

端点：本机 `http://mcp.aiarsenal.cc/mcp`（hosts 已由 `local-test.sh config` 指向 127.0.0.1）；prod `https://mcp.aiarsenal.cc/mcp`。以下 `<ENDPOINT>` 按环境替换。

### Claude Code（终端）

```bash
claude mcp add --transport http aiarsenal <ENDPOINT> \
  --header "Authorization: Bearer <TOKEN>"
```

单 server scope 把 `<ENDPOINT>` 换成 `<ENDPOINT>/demo` 即可。

### Cursor（`~/.cursor/mcp.json` 或项目 `.cursor/mcp.json`）

```json
{
  "mcpServers": {
    "aiarsenal": {
      "url": "<ENDPOINT>",
      "headers": { "Authorization": "Bearer <TOKEN>" }
    }
  }
}
```

### Claude Desktop（`claude_desktop_config.json`，经 mcp-remote 桥接）

桌面端不原生支持 http MCP，用 `mcp-remote` 桥接：

```json
{
  "mcpServers": {
    "aiarsenal": {
      "command": "npx",
      "args": [
        "-y", "mcp-remote", "<ENDPOINT>",
        "--header", "Authorization:Bearer <TOKEN>"
      ]
    }
  }
}
```

## curl 直测（streamable-http JSON-RPC）

```bash
EP=http://mcp.aiarsenal.cc/mcp            # prod 用 https
H='Authorization: Bearer <TOKEN>'
A='Accept: application/json, text/event-stream'

# 1) initialize —— 响应头取 Mcp-Session-Id
curl -sD /tmp/h -X POST "$EP" -H "$H" -H "$A" -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"0"}}}'
SID=$(grep -i '^mcp-session-id:' /tmp/h | awk '{print $2}' | tr -d '\r')

# 2) initialized 通知（带会话头）
curl -s -X POST "$EP" -H "$H" -H "$A" -H 'Content-Type: application/json' -H "Mcp-Session-Id: $SID" \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# 3) 列工具（应见 demo__echo / demo__now / demo__calc …）
curl -s -X POST "$EP" -H "$H" -H "$A" -H 'Content-Type: application/json' -H "Mcp-Session-Id: $SID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

# 4) 调用工具
curl -s -X POST "$EP" -H "$H" -H "$A" -H 'Content-Type: application/json' -H "Mcp-Session-Id: $SID" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"demo__echo","arguments":{"message":"hi","repeat":2}}}'

# 5) 结束会话
curl -s -X DELETE "$EP" -H "$H" -H "Mcp-Session-Id: $SID"
```

## 内置 demo server

Go 单二进制 `api/bin/mcp-demo`（源码 `api/cmd/mcp-demo`，api 模块多 main 之一），用于跑通全链路与演示。种子已入库：`demo`（stdio，启用）、`demo-http`（streamable-http，默认停用）。

| 工具 | 参数 | 用途 |
|---|---|---|
| `echo` | `message` string，`repeat` int（≤10） | 连通性验证 |
| `now` | `format` enum：datetime/date/time/unix | 动态返回 |
| `calc` | `a`、`b` number，`op` enum：add/sub/mul/div | 结构化参数；**除零返回 isError**，演示失败审计 |
| `longtext` | `length` int（默认 2000，上限 20000） | 演示网关工具结果压缩 |
| `slow` | `delay_ms` int（≤20000） | 演示调用超时控制 |

启动命令（手工运行 http 模式时，配合启用的 `demo-http` server）：

```bash
cd api && go build -o bin/mcp-demo ./cmd/mcp-demo
./bin/mcp-demo --transport http --host 127.0.0.1 --port 43219   # stdio 模式去掉 --transport 即可（网关自动拉起，无需手工跑）
```

stdio 模式由网关按需 spawn（工作目录 = `api/`，故 command 填相对路径 `bin/mcp-demo`；prod systemd `WorkingDirectory` 同为 `api/`，两端一致）。

## 管理页面能力

- **服务器 → 「测试」**：后台直连该 server（已存配置或表单未保存配置均可），列出 tools/prompts/resources 与 inputSchema——不消耗密钥、不产生网关会话。
- **服务器 → 工具「调用」**：页面内试调用任意工具，参数 JSON 按 inputSchema 生成骨架；结果与耗时回显，工具级错误（isError）标红。
- **调用审计**：网关侧记录密钥名 + 来源 IP；页面试调用记录 `keyName=console`。字段含服务器/工具/状态/耗时/入参出参（截断 2000 字符）。

## 工具结果压缩

`MCP → 设置`（或 `aa_mcp_config.tool_result_compression`）：

```json
{ "enabled": true, "maxLength": 4000 }
```

超过 `maxLength` 字符的文本结果被压成「头一半 + `...[compressed by mcp gateway, original length=N]...` + 尾部（≤256 字符）」，降低大结果（如 `longtext`）对客户端上下文的占用。默认关闭。

## 常见问题

| 现象 | 原因 / 处理 |
|---|---|
| 401 | 密钥停用/已轮换，或 accessType 未覆盖目标 scope |
| 列工具没有某 server | 该 server 停用；或 stdio 命令拉起失败（看网关日志，路径相对 `api/` 工作目录） |
| 客户端连接成功但调用超时 | demo 的 `slow` 工具就是干这个的；真实 server 超时多为上游阻塞 |
| prod 接 80 端口 POST 被重定向 | prod 强制 https，端点用 `https://mcp.aiarsenal.cc/mcp` |
