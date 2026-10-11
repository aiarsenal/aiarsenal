# AI Arsenal 桌面客户端（Win / Mac）

Win / Mac 两个桌面应用，三大功能：
1. 跳到系统浏览器登录后跳回 App（device-code 中转 + `aiarsenal://` 自定义 scheme）；
2. 经用户授权（系统提权）后安装 openclaw / hermes / obsidian 客户端并配置、启停；
3. 与 API 通信获取这三客户端配置并上报运行情况。

## 目录结构

```
client/
  shared/
    frontend/   Vue3 + Vite 共享前端（两平台同一套 UI）
    rust/       共享 Rust crate：API 客户端 + device-code 登录 + AppState + 类型
  win/          Tauri 项目（Windows）—— npm run dev / npm run build
  mac/          Tauri 项目（macOS）  —— npm run dev / npm run build
```

平台差异只在各自 `src-tauri/src/platform.rs`（安装/启停命令）；共享命令（登录中转、取规格、上报、环境切换）是对 `shared/rust` 中 `AppState` 的薄包装。

## 后端依赖

`api/` 已新增 4 个端点（见 `interfaces/controller/client.go`，路由 `/api/v1/system/client/*`）：
- `POST /exchange`（公开）— deviceCode 换 JWT
- `POST /bind`（JWTAuth）— 浏览器登录后绑定 deviceCode
- `GET /clients`（JWTAuth）— 取三客户端规格（数据存独立 client 库 `aa_client_client_config`）
- `POST /clients/report`（JWTAuth）— 上报运行情况到 client 库 `aa_client_client_status`

client 是独立 PG 库（与 aiarsenal/agent/llm/mcp/skill 同实例异库），建表+种子在 `api/resource/data/client.sql`，由 `db.InitClientDB()` 初始化。

`ui/` 新增公开页 `/client-login?dc=<deviceCode>`：账密+验证码登录 → 绑定 → 重定向 `aiarsenal://login?dc=...` 跳回 App。

## 首次构建步骤（每个平台项目）

1. 装好 Node.js 与 Rust（https://tauri.app/start/prerequisites/）。
2. 生成图标（首次必做，否则 `tauri build` 报缺图标；`tauri dev` 不需要）：
   ```bash
   cd client/<win|mac>
   npm install
   # 准备一张 1024x1024 的 icon.png 放到 src-tauri/icons/icon.png
   npx tauri icon src-tauri/icons/icon.png
   ```
3. 开发运行：
   ```bash
   cd client/win   # 或 client/mac
   npm run dev
   ```
4. 打包：`npm run build`（Win 出 .msi/.exe，Mac 出 .dmg/.app）。

### macOS 构建：代码签名与 TCC 权限（可选，编译本身不需要 Apple 账号）

mac 端默认 ad-hoc 签名，**装好 Rust + Xcode CLT 就能编译运行，不需要任何 Apple ID**；本机编译的产物没有 quarantine 属性，双击直接跑。唯一影响是 TCC 权限体验：

- ad-hoc 签名每次编译都变 → **屏幕录制 / 辅助功能授权每次重编译作废**，要去「系统设置 → 隐私与安全性」重新勾一次；
- 想跨编译保持授权，用真实的 **Apple Development** 证书重签（`pnpm build:debug` 会自动执行 [mac/scripts/sign-debug.sh](mac/scripts/sign-debug.sh)）：
  1. Xcode → Settings → Accounts → 登录 Apple ID（**免费账号即可**，无需 $99 开发者计划）→ Manage Certificates → ➕ Apple Development；
  2. 重新 `pnpm build:debug`，脚本自动探测钥匙串里的证书并重签。

证书探测顺序：环境变量 `AARS_SIGN_IDENTITY` > 维护者指纹 > 钥匙串里任意 Apple Development 证书 > 都没有则跳过（App 照常可用，仅 TCC 权限每次重编译需重新授权）。证书只存在各开发者自己电脑的钥匙串里，仓库不含任何证书/密钥。

> ⚠️ 自签证书（openssl 自制）不行——macOS TCC 不认不受信任的签名，开关会弹回。

## 预编译客户端（开源发布包）

GitHub Release 随开源附编译好的 client（macOS `.dmg` / Windows `.msi`）。**预编译包只允许连接 http 服务**——与本地开发环境完全一致的形态：需要自己部署服务端，并配置 hosts 把域名解析到自己的服务器；不连接任何官方线上服务。

使用步骤：

1. **自部署服务端**（二选一）：
   - macOS 本机：`bash scripts/local-test.sh deploy`（纯 HTTP + `/etc/hosts` 本机解析，见根 README「方式一」）；
   - 自有服务器（Ubuntu）：`sudo bash deploy.sh test`（纯 HTTP）。预编译客户端只走 http，若要在公网服务器安全使用，请自行按根 README「方式二」套 HTTPS 后从源码编译对应环境的 client。
2. **配置 hosts**（把四个域名指向部署了服务端的那台机器；本机部署即 `127.0.0.1`）：

   ```
   127.0.0.1 aiarsenal.cc api.aiarsenal.cc mcp.aiarsenal.cc llm.aiarsenal.cc
   ```

   （macOS/Linux：`/etc/hosts`；Windows：`C:\Windows\System32\drivers\etc\hosts`，需管理员权限。）
3. **安装并登录**：预编译包烘焙的是 test 环境（`http://api.aiarsenal.cc`），hosts 生效后启动 App，登录页会打开 `http://aiarsenal.cc/#/client-login`，账密登录后自动经 `aiarsenal://` 回跳绑定。

> 预编译包为何限定 http：发布产物指向 `aiarsenal.cc` 系域名，http(test) 环境下用户配好 hosts 即连到**自己的**服务端；官方不提供也不维护指向任何官方服务器的客户端构建。

## 三客户端说明

- **openclaw**：npm 全局包（`npm i -g openclaw`），Node CLI，配置 `~/.openclaw/openclaw.json`（Win: `%USERPROFILE%\.openclaw\`），`openclaw gateway run` 启动。前提：本机有 Node.js。
- **hermes**：Linux 容器运行时，**只能走 Docker**。前提：本机有 Docker Desktop。App 用 `docker pull/run/start/stop` 管理容器 `aiarsenal-hermes`。
- **obsidian**：官方桌面应用。Win 下载 .exe 静默安装（`/S`）；Mac 下载 .dmg → `hdiutil attach` → 拷到 `/Applications/`（提权）。

安装需要管理员权限时：Win 走 MSI/NSIS 安装器提权，Mac 用 `osascript ... with administrator privileges`（首次会弹系统授权框 = "用户授权"）。

## mac 桌面节点（替代 OpenClaw.app）

mac 客户端除管理 CLI 外，自当 openclaw 的 macOS desktop node（feature `openclaw-desktop`，默认启用；禁用后 openclaw 退回无桌面能力，可轻松卸载）。连接本地 gateway（`ws://127.0.0.1:18789`）暴露 `screen.*` / `computer.act` / `system.run` 等命令，与 openclaw CLI 完全互操作。以下三块能力都经此链路工作。

### 权限一次性申请页（mac）

mac 的 TCC 权限（屏幕录制/辅助功能/相机/通讯录/日历/提醒事项/照片/定位/通知）各命令懒弹框，换机或重编译后重走雷区。Dashboard「系统权限」按钮打开清单页：9 项状态一览（绿=已授权/橙=未申请/红=拒绝）+ 逐项「申请」或「去系统设置」深链 + 「一键申请」（串行逐项，防系统弹框叠加）；打开期间 2s 轮询，在系统设置里勾完授权回来自动变绿。定位项只读展示（主动请求有 RunLoop 存活陷阱，实弹申请走 `location.get`）。

### exec 审批（agent 执行命令的闸门）

agent 入口是微信和浏览器网页（提示注入面），`system.run` 不能裸奔。策略文件与 openclaw CLI 完全互操作：`~/.openclaw/exec-approvals.json`（`openclaw exec approvals node get/set` 读写同一文件，含 baseHash CAS 防并发覆盖）。

Dashboard「执行审批」四档切换：

| 档位 | 行为 |
|---|---|
| 全部放行（**默认**） | 与现状完全一致，零拦截 |
| 白名单外询问 | 白名单命中静默放行；未命中弹窗（允许一次 / 总是允许 / 拒绝），60s 无回应自动拒绝 |
| 白名单外拒绝 | 只跑白名单命令，未命中直接拒 |
| 全部拒绝 | 拒绝一切命令执行 |

「总是允许」会把该命令写入白名单（`source: "allow-always"`，重启后命中即静默）；首次切到白名单档会种子 19 条只读命令（ls/cat/grep/rg/find 等）。弹窗是全局的（登录页/任意时刻都可能来），错过事件不用怕——后端每 2s 重发直到超时。

### Canvas（agent 的可视化面板）

openclaw gateway 的 canvas 插件让 agent 在用户桌面上开 HTML 面板。客户端实现全部 8 条 node 命令：

- `canvas.present {url?, placement?}` / `canvas.hide` / `canvas.navigate {url}`：900×640 常驻窗口（关闭=隐藏，保页面状态）
- `canvas.eval {javaScript} → {result}`：在面板页执行 JS 取返回值（wry 吞 JS 异常时返回空串）
- `canvas.snapshot {format, maxWidth?, quality?} → {format, base64}`：截面板渲染图（WKWebView takeSnapshot）
- `canvas.a2ui.push / pushJSONL / reset`：A2UI 宿主页消息注入（宿主页地址由 gateway 握手下发）

面板文件托管在 canvas 根（`$OPENCLAW_STATE_DIR/canvas`，默认 `~/.openclaw/canvas`）经自定义 scheme `openclaw-canvas://localhost/` 提供，目录自动补 `index.html`；文件监听 300ms 去抖自动 reload——agent 写完即所见。http(s)/file URL 直接加载，绝对路径文件自动转 `file://`。


## 与本机已有 openclaw 的关系（版本对齐与用户确认）

openclaw 是**本机单实例**设计：一个 `~/.openclaw`、一个 gateway 端口（18789），客户端无法与本机自用的 openclaw 共存，只能接管。接管策略是确定性的：**探测 → 上报 → 备份 → 确认后对齐**。

1. **环境探测随心跳上报**（登录后每跳上报，探测结果 60s 缓存）：`modules.openclaw.env` 携带本机所有 openclaw 来源（客户端管理的 `~/.openclaw/bin` runtime / PATH 里的 npm 全局 / OpenClaw.app）各自的版本、配置是否存在、18789 是否被占用、以及是否需要版本对齐（`aligned=false` 时管理后台设备页版本列显示「版本待对齐」）。
2. **版本钉死**：客户端构建可经 `client/build.sh --openclaw-version` 钉死目标版本（`option_env!` 烘焙）。desktop node 协议、gateway allowCommands 都是按钉死版本对齐实现的，版本漂移即协议不匹配——所以**旧版要升、新版也要降**，一律收敛到目标版本。
3. **升/降级需本机用户确认**：install 检测到本机已有 openclaw 且版本 ≠ 目标时，App 弹确认框（从托盘拉出主窗口）列出检测到的版本与目标、说明备份行为；**确认** → 原位重装对齐（保留 `~/.openclaw` 的 identity 与渠道登录态）；**拒绝或 10 分钟超时** → 命令就地失败，机器不做任何改动。版本已一致的机器静默跳过重装，不打扰。
4. **配置覆盖前备份**：install 下发的 `configTemplate`（含 LLM key）覆盖 `openclaw.json` 前，先备份为 `openclaw.json.bak.<unix时间戳>`。
5. **卸载不销毁用户数据**：uninstall 把整个 `~/.openclaw` 挪到 `~/.openclaw.bak.<unix时间戳>`（Win 只卸 npm 包，`~/.openclaw` 原样保留），确认无需后由用户手动删。
6. **外部 gateway 占用端口**：start 时发现 18789 被版本不符/非本客户端管理的 gateway 占着，如实报错引导走 install 对齐流程，**不擅杀用户进程**。

## 登录流程

1. App 启动 → `LoginWait` 生成 `deviceCode`，调 `open_login_page` 用系统浏览器打开 `http://api.aiarsenal.cc/client-login?dc=<dc>`。
2. 用户在浏览器完成账密+验证码登录 → 前端调 `/client/bind` 绑定 → 重定向 `aiarsenal://login?dc=<dc>`。
3. OS 把 `aiarsenal://` 路由给 App（`tauri-plugin-deep-link`），App 调 `/client/exchange` 拿到 JWT 并持久化。同时 App 每 2s 轮询 exchange 作 fallback（scheme 未拉起时也能拿到 token）。
4. 进入 Dashboard：调 `/client/clients` 取三客户端规格，对每个客户端执行安装/启停/配置，并调 `/client/clients/report` 上报状态。

## 环境（编译期烘焙，运行时不可切）

**App 内没有环境切换**——API 地址在构建时确定：`client/build.sh --env test|prod` 或 `--base-url` 从 `api/config/client-envs.json` 取 scheme+domain，经 `option_env!` 烘焙进二进制（未设默认 test 环境 `http://api.aiarsenal.cc`）。Dashboard 会以标签显示当前构建环境（test=测试 / prod=生产 / 其它显示构建名）。要换环境必须重新编译，预编译包同理（见上文「预编译客户端」）。登录态（token、用户名）持久化在 `~/AppData/Roaming/aiarsenal-client/state.json`（Win）或 `~/Library/Application Support/aiarsenal-client/state.json`（Mac）。
