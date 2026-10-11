//! 桌面客户端应用状态：环境切换、token 持久化、对后端的命令逻辑。
//!
//! 不依赖 tauri，可被 Win/Mac 两个 Tauri 项目复用。
//! 各项目的 `#[tauri::command]` 只是对这里的方法做薄包装。

use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Mutex;

use serde::{Deserialize, Serialize};

use crate::api::{ApiError, ClientApi};
use crate::device::collect_device_info;
use crate::login::generate_device_code;
use crate::types::{Agent, EnvInfo, ExchangeRes, HeartbeatReq, HeartbeatRes, McpSelfView, ModuleReport, SkillStateReport};

// API base_url 编译期烘焙：build 时 export AA_BASE_URL（client/build.sh 按 --env 从
// api/config/client-envs.json 取 scheme+domain 拼出）→ option_env! 烤进二进制；未设则默认 test 环境。
// 用 match 而非 .unwrap_or：后者非 const fn，const 上下文编译不过（E0658）。
pub const BASE_URL: &str = match option_env!("AA_BASE_URL") {
    Some(s) => s,
    None => "http://api.aiarsenal.cc",
};
pub const ENV_NAME: &str = match option_env!("AA_ENV_NAME") {
    Some(s) => s,
    None => "test",
};
const STATE_FILE: &str = "state.json";

/// 持久化的登录态（落盘到应用数据目录）。全部字段 `#[serde(default)]`，
/// 兼容旧版本 state.json（新增字段时旧文件缺失该字段不会反序列化失败）。
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct PersistedState {
    #[serde(default)]
    pub env: String,        // "test" | "prod"
    #[serde(default)]
    pub token: String,
    #[serde(default)]
    pub user_id: String,    // 工号
    #[serde(default)]
    pub user_name: String,
    #[serde(default)]
    pub device_id: String,  // 本机持久设备 id（首次登录生成）
    #[serde(default)]
    pub name: String,       // 真实姓名（aa_user.name）
    #[serde(default)]
    pub employee_no: String, // 工号（aa_user.employee_no）
}

/// 应用共享状态。`Mutex` 保证 Send+Sync，可作 `tauri::State`。
pub struct AppState {
    data_dir: PathBuf,
    inner: Mutex<Inner>,
}

struct Inner {
    env: String,
    token: String,
    user_id: String,
    user_name: String,
    device_id: String,
    name: String,
    employee_no: String,
    api: ClientApi,
}

#[derive(thiserror::Error, Debug)]
pub enum StateError {
    #[error("{0}")]
    Api(#[from] ApiError),
    #[error("状态文件读写失败: {0}")]
    Io(String),
    #[error("状态解析失败: {0}")]
    Parse(String),
    #[error("未登录")]
    NotLoggedIn,
}

impl StateError {
    /// 心跳等请求是否因设备登录态被服务端吊销/过期（HTTP 401）而失败。
    pub fn is_unauthorized(&self) -> bool {
        matches!(self, StateError::Api(ApiError::Unauthorized(_)))
    }
}

impl AppState {
    /// 构造：从 data_dir 读取持久化状态，按 env 创建 API 客户端。
    pub fn new(data_dir: PathBuf, _default_env: &str) -> Result<Self, StateError> {
        fs::create_dir_all(&data_dir).map_err(|e| StateError::Io(e.to_string()))?;
        let persisted = Self::load_file(&data_dir)?;
        // env + base_url 编译期烘焙（option_env! AA_BASE_URL/AA_ENV_NAME），不再运行时切。
        let env = ENV_NAME.to_string();
        let base_url = BASE_URL;
        // 首次启动生成持久 device_id 并落盘
        let device_id = if persisted.device_id.is_empty() {
            let id = generate_device_code();
            let mut p = persisted.clone();
            p.device_id = id.clone();
            if let Ok(json) = serde_json::to_string_pretty(&p) {
                let _ = fs::write(data_dir.join(STATE_FILE), json);
            }
            id
        } else {
            persisted.device_id
        };
        Ok(Self {
            data_dir,
            inner: Mutex::new(Inner {
                env,
                token: persisted.token,
                user_id: persisted.user_id,
                user_name: persisted.user_name,
                device_id,
                name: persisted.name,
                employee_no: persisted.employee_no,
                api: ClientApi::new(base_url),
            }),
        })
    }

    fn load_file(data_dir: &Path) -> Result<PersistedState, StateError> {
        let path = data_dir.join(STATE_FILE);
        if !path.exists() {
            return Ok(PersistedState::default());
        }
        let body = fs::read_to_string(&path).map_err(|e| StateError::Io(e.to_string()))?;
        serde_json::from_str(&body).map_err(|e| StateError::Parse(e.to_string()))
    }

    fn persist(inner: &Inner, data_dir: &Path) {
        let p = PersistedState {
            env: inner.env.clone(),
            token: inner.token.clone(),
            user_id: inner.user_id.clone(),
            user_name: inner.user_name.clone(),
            device_id: inner.device_id.clone(),
            name: inner.name.clone(),
            employee_no: inner.employee_no.clone(),
        };
        if let Ok(json) = serde_json::to_string_pretty(&p) {
            let _ = fs::write(data_dir.join(STATE_FILE), json);
        }
    }

    /// 取本机持久 device_id（首次启动已生成）。
    pub fn device_id(&self) -> String {
        self.inner.lock().unwrap().device_id.clone()
    }

    /// 取当前 token（安装包下载等需 Bearer 鉴权的场景用）。
    pub fn token(&self) -> String {
        self.inner.lock().unwrap().token.clone()
    }

    /// 取 API base_url（供构造包下载 URL）。
    pub fn base_url(&self) -> String {
        self.inner.lock().unwrap().api.base_url().to_string()
    }

    /// 取数据目录（持久化设置文件落这里）。
    pub fn data_dir(&self) -> &std::path::Path {
        self.data_dir.as_path()
    }

    /// 生成一次性 deviceCode（App 端发起登录用）。
    pub fn start_login(&self) -> String {
        generate_device_code()
    }

    /// deviceCode 换 JWT；成功则保存 token + 用户名并落盘。
    ///
    /// 注意：`std::sync::MutexGuard` 非 `Send`，不能跨 `.await` 持有，否则 Tauri 命令的
    /// future 不满足 `Send`。这里先把 `api` clone 出来、释放锁后再 await。
    pub async fn exchange(&self, device_code: &str) -> Result<ExchangeRes, StateError> {
        let (api, device_id) = {
            let inner = self.inner.lock().unwrap();
            (inner.api.clone(), inner.device_id.clone())
        };
        let device_info = collect_device_info(device_id);
        let res = api.exchange(device_code, &device_info).await?;
        if res.status == "ok" && !res.token.is_empty() {
            let mut inner = self.inner.lock().unwrap();
            inner.token = res.token.clone();
            inner.user_id = res.user_id.clone();
            inner.user_name = res.user_name.clone();
            inner.name = res.name.clone();
            inner.employee_no = res.employee_no.clone();
            Self::persist(&inner, &self.data_dir);
        }
        Ok(res)
    }

    pub async fn get_agents(&self) -> Result<Vec<Agent>, StateError> {
        let (token, api) = {
            let inner = self.inner.lock().unwrap();
            let token = inner.token.clone();
            if token.is_empty() {
                return Err(StateError::NotLoggedIn);
            }
            (token, inner.api.clone())
        };
        let res = api.get_agents(&token).await?;
        Ok(res)
    }

    // ===== MCP 分组自助管理（桌面客户端 MCP 管理模块；不持久化，服务端是唯一事实源） =====

    /// 已绑分组 + 可绑选项（服务端已按当前用户权限过滤）。
    pub async fn mcp_list(&self) -> Result<McpSelfView, StateError> {
        let (token, api) = {
            let inner = self.inner.lock().unwrap();
            let token = inner.token.clone();
            if token.is_empty() {
                return Err(StateError::NotLoggedIn);
            }
            (token, inner.api.clone())
        };
        Ok(api.mcp_list(&token).await?)
    }

    /// 绑定分组（服务端签 key + 入队 openclaw 重启；配置重建全在服务端）。
    pub async fn mcp_bind(&self, group_id: &str) -> Result<(), StateError> {
        let (token, api) = {
            let inner = self.inner.lock().unwrap();
            let token = inner.token.clone();
            if token.is_empty() {
                return Err(StateError::NotLoggedIn);
            }
            (token, inner.api.clone())
        };
        api.mcp_bind(&token, group_id).await?;
        Ok(())
    }

    /// 解绑分组（服务端校验权限：可使用该组 ∨ 本人绑定的行）。
    pub async fn mcp_unbind(&self, group_id: &str) -> Result<(), StateError> {
        let (token, api) = {
            let inner = self.inner.lock().unwrap();
            let token = inner.token.clone();
            if token.is_empty() {
                return Err(StateError::NotLoggedIn);
            }
            (token, inner.api.clone())
        };
        api.mcp_unbind(&token, group_id).await?;
        Ok(())
    }

    /// 心跳：上行多模块状态，下行存储结果+最新命令。未登录返回空 commands（不报错）。
    pub async fn heartbeat(&self, req: &HeartbeatReq) -> Result<HeartbeatRes, StateError> {
        let (token, api) = {
            let inner = self.inner.lock().unwrap();
            let token = inner.token.clone();
            if token.is_empty() {
                return Ok(HeartbeatRes::default());
            }
            (token, inner.api.clone())
        };
        Ok(api.heartbeat(&token, req).await?)
    }

    /// 上报 App 生命周期事件（启动/退出）：以 client agent 身份发一条心跳。
    /// status = "running"（启动）| "stopped"（退出）。失败静默——生命周期上报尽力而为，
    /// 不影响退出流程。退出时调用方应 await 后再 app.exit。
    /// skills 由调用方聚合传入（shared 不知各 agent 的 skills 根目录与编译开关）。
    pub async fn report_app_lifecycle(&self, status: &str, pid: u32, skills: Vec<SkillStateReport>) {
        let mut client_report = ModuleReport::default();
        client_report.status = status.into();
        client_report.version = crate::util::APP_VERSION.into();
        client_report.pid_or_container_id = pid.to_string();
        let mut modules = std::collections::HashMap::new();
        modules.insert(crate::util::CLIENT_AGENT_KEY.into(), client_report);
        let req = HeartbeatReq {
            device_id: self.device_id(),
            modules,
            channels: Vec::new(),
            perm_state: None, // 生命周期上报不带授权状态（引擎变更时主心跳才带）
            skills, // 技能快照照常带：退出上报也保持最新
        };
        let _ = self.heartbeat(&req).await;
    }

    pub fn logout(&self) {
        let mut inner = self.inner.lock().unwrap();
        inner.token.clear();
        inner.user_name.clear();
        inner.user_id.clear();
        inner.name.clear();
        inner.employee_no.clear();
        Self::persist(&inner, &self.data_dir);
    }

    pub fn env_info(&self) -> EnvInfo {
        let inner = self.inner.lock().unwrap();
        EnvInfo {
            env: inner.env.clone(),
            base_url: inner.api.base_url().to_string(),
            logged_in: !inner.token.is_empty(),
            user_id: inner.user_id.clone(),
            user_name: inner.user_name.clone(),
            name: inner.name.clone(),
            employee_no: inner.employee_no.clone(),
        }
    }
}
