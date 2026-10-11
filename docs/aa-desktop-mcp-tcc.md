# aa-desktop 桌面控制 MCP 与 macOS TCC 授权排障

> hermes 不改一行源码获得桌面控制：客户端内置环回 MCP server，与 openclaw node 协议共用同一套闸门。
> 本文含 2026-09-19 上线实录，以及「面板显示已授权、系统实际仍拒绝」的 TCC 死锁完整拆解——重编译客户端前必读第四节。

---

## 一、架构

```
微信 ──▶ hermes（设备原生进程， ~/.hermes）
             │ 首次调用工具时惰性连接
             ▼
     http://127.0.0.1:43250/mcp   ← aa-desktop 环回 MCP（仅本机可达，无鉴权）
             │  client/mac …/desktop_node/mcp_server.rs（std 手写 HTTP，POST-only
             │  JSON-RPC，无 session id——mcp 2.0.0 客户端因此不建 GET SSE 流）
             ▼
     protocol::dispatch ── 与 openclaw node 协议（ws://127.0.0.1:18789）同一入口
             │
             ├─ app_gate：App 权限清单（aa_client_app_perm），空清单 = 全禁（fail-closed）；
             │   system.run 的 osascript/open 等与 computer.act 注入型 action 均过此门
             ├─ exec_approvals / act_gate：执行审批与前台 App 白名单
             └─ TCC 权限硬闸门：screen.snapshot 前校验屏幕录制、注入型 action 前校验辅助功能
```

四个工具（hermes 侧名字带 `mcp__aa_desktop__` 前缀）：

| 工具 | 落到 dispatch | 用途 |
|---|---|---|
| `desktop_run` | `system.run` | 跑命令（open/osascript/screencapture…），过 App 闸门 |
| `desktop_act` | `computer.act` | 鼠标键盘注入，过 act_gate + 辅助功能校验 |
| `desktop_snapshot` | `screen.snapshot` | 截屏，过屏幕录制校验 |
| `desktop_notify` | `system.notify` | 系统通知 |

要点：

- **后端注入**：`api/application/service/client_service.go` 的 `issueHermesConfig` 无条件写
  `mcp_servers.aa-desktop → http://127.0.0.1:43250/mcp`（仅 hermes；openclaw 走原生 node 协议不需要）。
  端口 43250 是 api `deviceDesktopMcpURL` 与客户端 `mcp_server.rs DESKTOP_MCP_PORT` 的**编译期约定**，改须两边同步。
- **闸门只有一份**：两个 agent（openclaw node / hermes aa-desktop）同一套权限边界，App 清单空时桌面命令全拒，无安全敞口。
- **600s 读写超时**：TCC 授权框会阻塞调用，短超时会误杀。

## 二、TCC 权限分裂：重编译客户端必读

客户端是 ad-hoc 签名（无 Apple Development 证书），**每次重编译 cdhash 都变**。macOS TCC 对不同权限的键控方式不同：

| 权限 | TCC 层级 | 键 | 重编译（adhoc）后 | 面板表现 |
|---|---|---|---|---|
| 屏幕录制 / 辅助功能 | 系统级 | **cdhash** | ❌ 授权作废 | 旧条目仍显示「开」——**死数据** |
| 自动化 AppleEvents | 用户级 | bundleId | ✅ 保留 | 正常 |

 最迷惑的是最后一列组合：用户看到开关是开的，实际新二进制不在授权范围内。
 日常约束：**不重编译客户端，授权一直有效**；重编译后按第四节流程重授权一次。

## 三、权限硬闸门：无声失败的教训

无权限时 macOS **不报错**，只给假结果：

- 无屏幕录制：`screencapture` / ScreenCaptureKit 拍出**纯壁纸图**（无任何窗口），exit 0。上线实录：agent 连截 4 次全是壁纸图发给用户，表面「成功」。
- 无辅助功能：CGEvent 投递**静默 no-op**，点击无反应、输入不进去。

已在 `protocol.rs` 加硬校验，明确报错优于无声假图：

- `screen.snapshot` → `ensure_screen_recording(true)`，未授权直接回错并附操作指引（interactive=true：未决定态先触发系统授权框）
- `computer.act` 注入型 action（click/type/key/scroll…）→ `ensure_accessibility(true)`，同上

## 四、「已授权仍被拒」排障实录（2026-09-19）

### 现象

- 系统设置 → 隐私与安全性 → 屏幕录制：「AI Arsenal」**开关是开的**
- 客户端日志（`~/Library/Application Support/aiarsenal-client/aiarsenal-client.log`）：`Screen=false`
- 实测：`screencapture` 报 `could not create image from display`

### 两个弯路（都踩了）

1. **假重启**：`killall "AI Arsenal"` 静默失败——进程名实际是 `aiarsenal-client-mac`（CFBundleExecutable），
   `2>/dev/null` 把报错吞了。屏幕录制授权**必须重启进程才生效**，没重启就永远看不到授权生效。
   正确姿势：`pkill -f aiarsenal-client-mac` 或 `ps` 查 pid 再 kill。
2. **死数据开关**：面板里旧条目显示「开」（绑旧 cdhash），单纯关-开循环也不可靠。最终 `tccutil reset` 全清最干净。

### 修复定式（实测有效顺序）

```bash
# 1. 清旧条目（系统级 TCC 两项都清）
tccutil reset ScreenCapture cc.aiarsenal.client
tccutil reset Accessibility cc.aiarsenal.client

# 2. 重启 App：启动时会发起全新授权请求（屏幕上弹系统授权框）
pkill -f aiarsenal-client-mac; sleep 2; open -a "AI Arsenal"

# 3. 用户操作（唯一人工步骤）：
#    系统设置 → 隐私与安全性 → 屏幕录制：「AI Arsenal」开关此时应为「关」→ 打开 →
#    弹「退出并重新打开」点确认（⚠️ 关键信号：打开开关时没弹这个框 = 没绑上，重做 1-3）
#    辅助功能面板同样打开「AI Arsenal」
```

### 验证（不用等微信 E2E）

```bash
# 屏幕录制：exitCode 0 = 已生效；"could not create image from display" = 未生效
curl -s -X POST http://127.0.0.1:43250/mcp -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"desktop_run","arguments":{"command":["screencapture","-x","/tmp/t.png"]}}}'

# 辅助功能：能返回前台窗口名 = UI 脚本可用（AppleEvents + Accessibility 都通）
curl -s -X POST http://127.0.0.1:43250/mcp -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"desktop_run","arguments":{"command":["osascript","-e","tell application \"System Events\" to get name of every window of (first process whose frontmost is true)"]}}}'

# 完整链路（hermes 实调的工具）：isError=false 且 base64 是大图
curl -s -X POST http://127.0.0.1:43250/mcp -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"desktop_snapshot","arguments":{}}}'
```

判据陷阱：`screencapture` exit 0 **不代表**拍到真图（无权限时也 exit 0 只拍壁纸）——独立排查时要肉眼确认图里有窗口；经客户端工具链路已被第二节闸门挡住。

## 五、一劳永逸（未实施）

Apple Development 证书 + 固定签名（sign-debug.sh 式重签）后 cdhash 稳定，重编译不再作废系统级 TCC 授权。
当前机器无 Xcode、钥匙串无该证书；在上证书之前，接受「重编译客户端 = 重走第四节授权流程一次」的约定。

## 六、权限边界：App 权限清单管不到 hermes 自带工具（2026-09-19 实测）

上线当天实测：微信让 hermes「打开 WPS 写简历」，agent 全程**没用 aa-desktop 工具**（客户端日志零次 MCP 调用），
走的是 hermes 自带 `terminal`/`write_file`——python 生成 docx → `open -a wpsoffice` 打开 → `screencapture` 验证。
**hermes 是跑在用户账户下的完整 agent，自带 25 个内置工具直接执行，不经客户端任何闸门**；
且它是 AI Arsenal 的子进程，TCC 责任进程是宿主 App，屏幕录制等授权直接继承（TCC 也拦不住）。

结论：App 权限清单的约束力只覆盖两条链路——openclaw node 协议 + aa-desktop MCP（`desktop_run/act/snapshot/notify`）。
对 hermes 的自带工具无约束力，产品上不再假装有：

- 管理后台设备运行详情的「App 权限」卡：设备未启用 openclaw → 整卡置灰 + 警示说明
- `AdminSetDeviceAppPerms`：未启用 openclaw 的设备拒绝保存（与前端同口径）
- 双开设备（openclaw+hermes）追加提示行：hermes 不受清单约束
- 心跳照常下发既有清单：aa-desktop 闸门对 hermes 的 MCP 工具链继续生效（零回归）

若将来要真正约束 hermes 自带工具，出路是 hermes 自身的审批系统（`approvals.mode: manual/smart/off`，
源码在 hermes-agent/tools/approval*.py，配置键 `approvals.mode` 可经 config 下发）——尚未启用。
