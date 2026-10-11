//! 与后端 `/api/v1/system/client/*` 通信的 HTTP 客户端。
//!
//! 所有方法接收 base_url + token（由各 Tauri 项目的 AppState 注入），无全局可变状态。
//! 后端响应 `{ code:0, data, message }`，code!=0 视为错误并返回 message。

use reqwest::Client;
use serde::de::DeserializeOwned;

use crate::types::{Agent, AgentsRes, ApiEnvelope, DeviceInfo, ExchangeRes, HeartbeatReq, HeartbeatRes, McpSelfView};

const API_PREFIX: &str = "/api/v1/system/client";

#[derive(thiserror::Error, Debug)]
pub enum ApiError {
    #[error("网络请求失败: {0}")]
    Network(String),
    /// HTTP 401：设备登录态被服务端吊销/过期（管理员 revoke、token 失效）。
    /// 与 Network 区分，调用方可据此触发强制登出 + 引导重新授权。
    #[error("登录状态已失效: {0}")]
    Unauthorized(String),
    #[error("后端返回错误: {0}")]
    Business(String),
    #[error("解析响应失败: {0}")]
    Parse(String),
}

/// 桌面客户端 API 客户端（无状态）。`reqwest::Client` 内部是 Arc，clone 成本低。
#[derive(Clone)]
pub struct ClientApi {
    base_url: String,
    http: Client,
}

impl ClientApi {
    pub fn new(base_url: &str) -> Self {
        let http = Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()
            .unwrap_or_else(|_| Client::new());
        Self {
            base_url: base_url.trim_end_matches('/').to_string(),
            http,
        }
    }

    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    /// deviceCode 换 JWT。status=pending 表示尚未在浏览器端完成绑定。
    /// device_info 由 App 采集上报，后端落设备记录并把 deviceId 绑入 JWT。
    pub async fn exchange(&self, device_code: &str, device_info: &DeviceInfo) -> Result<ExchangeRes, ApiError> {
        let body = serde_json::json!({ "deviceCode": device_code, "deviceInfo": device_info });
        self.post_json("/exchange", &body, None).await
    }

    /// 取可安装 agent 列表 + 各 agent 的版本（App 首页用）。
    pub async fn get_agents(&self, token: &str) -> Result<Vec<Agent>, ApiError> {
        let res: AgentsRes = self.get_json("/agents", Some(token)).await?;
        Ok(res.list)
    }

    /// 心跳：上行多模块状态(日志/配置变更/命令进度)，下行存储结果+最新命令。
    pub async fn heartbeat(&self, token: &str, req: &HeartbeatReq) -> Result<HeartbeatRes, ApiError> {
        self.post_json("/heartbeat", req, Some(token)).await
    }

    // ===== MCP 分组自助管理（设备 token 端点，桌面客户端 MCP 管理模块） =====

    /// 已绑分组 + 可绑选项（一次往返；服务端已按当前用户权限过滤）。
    pub async fn mcp_list(&self, token: &str) -> Result<McpSelfView, ApiError> {
        self.get_json("/mcp", Some(token)).await
    }

    /// 绑定分组（服务端签 key + 入队 openclaw 重启，配置重建全在服务端）。
    /// 响应 data 为空对象，用 Value 兜住泛型反序列化。
    pub async fn mcp_bind(&self, token: &str, group_id: &str) -> Result<serde_json::Value, ApiError> {
        let body = serde_json::json!({ "groupId": group_id });
        self.post_json("/mcp/bind", &body, Some(token)).await
    }

    /// 解绑分组（规则同上；无权限的组服务端会拒）。
    pub async fn mcp_unbind(&self, token: &str, group_id: &str) -> Result<serde_json::Value, ApiError> {
        let body = serde_json::json!({ "groupId": group_id });
        self.post_json("/mcp/unbind", &body, Some(token)).await
    }

    async fn get_json<T: DeserializeOwned>(
        &self,
        path: &str,
        token: Option<&str>,
    ) -> Result<T, ApiError> {
        let url = format!("{}{}{}", self.base_url, API_PREFIX, path);
        let mut req = self.http.get(&url);
        if let Some(t) = token {
            req = req.bearer_auth(t);
        }
        let resp = req.send().await.map_err(|e| ApiError::Network(e.to_string()))?;
        let envelope: ApiEnvelope<T> = parse_envelope(resp).await?;
        unwrap_envelope(envelope)
    }

    async fn post_json<T: DeserializeOwned, B: serde::Serialize>(
        &self,
        path: &str,
        body: &B,
        token: Option<&str>,
    ) -> Result<T, ApiError> {
        let url = format!("{}{}{}", self.base_url, API_PREFIX, path);
        let mut req = self.http.post(&url).json(body);
        if let Some(t) = token {
            req = req.bearer_auth(t);
        }
        let resp = req.send().await.map_err(|e| ApiError::Network(e.to_string()))?;
        let envelope: ApiEnvelope<T> = parse_envelope(resp).await?;
        unwrap_envelope(envelope)
    }
}

async fn parse_envelope<T: DeserializeOwned>(resp: reqwest::Response) -> Result<ApiEnvelope<T>, ApiError> {
    let status = resp.status();
    let text = resp.text().await.map_err(|e| ApiError::Parse(e.to_string()))?;
    // 401 单独分流：设备被管理员 revoke 或 token 失效，调用方应强制登出而非静默重试。
    if status.as_u16() == 401 {
        return Err(ApiError::Unauthorized(format!("HTTP 401 : {}", text)));
    }
    if !status.is_success() {
        return Err(ApiError::Network(format!("HTTP {} : {}", status, text)));
    }
    serde_json::from_str::<ApiEnvelope<T>>(&text).map_err(|e| {
        // 兜底：后端有时返回裸 data 而非 envelope
        ApiError::Parse(format!("{} (body: {})", e, &text[..text.len().min(200)]))
    })
}

fn unwrap_envelope<T: DeserializeOwned>(env: ApiEnvelope<T>) -> Result<T, ApiError> {
    if env.code != 0 {
        return Err(ApiError::Business(env.message));
    }
    env.data.ok_or_else(|| ApiError::Parse("响应缺少 data 字段".into()))
}
