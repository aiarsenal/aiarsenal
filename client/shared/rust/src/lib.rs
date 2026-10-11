//! AI Arsenal 桌面客户端共享核心（Win/Mac 复用）。
//!
//! 平台无关：与后端 `/api/v1/system/client/*` 通信、device-code 登录中转、类型定义。
//! 平台差异（安装/启停 openclaw/hermes/obsidian）由各 Tauri 项目的 `commands.rs` 实现。
//!
//! 后端响应统一形如 `{ code: 0, data: ..., message: "成功" }`，code=0 视为成功，本层负责解包 `data`。

pub mod api;
pub mod device;
pub mod login;
pub mod skills;
pub mod state;
pub mod types;
pub mod util;

pub use api::ClientApi;
pub use device::collect_device_info;
pub use login::{derive_ui_url, generate_device_code, parse_dc_from_deep_link};
pub use state::{AppState, StateError, BASE_URL, ENV_NAME};
pub use types::{Agent, AgentVersion, AgentsRes, AppPerm, ChannelLoginState, ChannelStateReport, ClientCommand, ClientPackageRef, DeviceInfo, EnvInfo, ExchangeRes, HeartbeatReq, HeartbeatRes, LogEntry, ModuleReport, RunningCommandReport};
