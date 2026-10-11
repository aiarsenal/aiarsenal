package ops

// 平台数据的 6 个只读工具（/mcp，网关注册名 platform-ops）。
// agent 专用聚合读模型：跨库计数合成、状态翻中文——UI 的分页 CRUD 没有这个形态，
// SQL 直接写在工具旁（显式列名，不 SELECT *：实例/设备表有 token 类秘密列）。

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/infrastructure/db"
)

func buildPlatformServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-ops", Version: "1.0"}, nil)

	// 平台总览（跨四库计数，合成一个 JSON）
	opsTool(srv, "platform_overview", "平台运营总览（真实数据）：实例/设备/技能/MCP 注册表与密钥数量、今日 MCP 调用量与失败数", nil,
		func(ctx context.Context, a map[string]any) (string, error) {
			out := map[string]any{}
			n, err := scalar(db.AgentDB(), "SELECT count(*) FROM aa_agent_instance")
			if err != nil {
				return "", err
			}
			out["agent_instances"] = n
			n, err = scalar(db.AgentDB(), "SELECT count(*) FROM aa_agent_instance WHERE status = 1")
			if err != nil {
				return "", err
			}
			out["agent_running"] = n
			n, err = scalar(db.ClientDB(), "SELECT count(*) FROM aa_client_device")
			if err != nil {
				return "", err
			}
			out["devices"] = n
			n, err = scalar(db.SkillDB(), "SELECT count(*) FROM aa_skill WHERE hidden = 0 AND status = 1")
			if err != nil {
				return "", err
			}
			out["skills"] = n
			for q, k := range map[string]string{
				"SELECT count(*) FROM aa_mcp_server WHERE enabled = 1":                                              "mcp_servers",
				"SELECT count(*) FROM aa_mcp_group":                                                                 "mcp_groups",
				"SELECT count(*) FROM aa_mcp_bearer_key WHERE enabled = 1":                                          "mcp_keys",
				"SELECT count(*) FROM aa_mcp_activity WHERE ts >= date_trunc('day', now())":                         "mcp_calls_today",
				"SELECT count(*) FROM aa_mcp_activity WHERE ts >= date_trunc('day', now()) AND status <> 'success'": "mcp_errors_today",
			} {
				n, err = scalar(db.McpDB(), q)
				if err != nil {
					return "", err
				}
				out[k] = n
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), nil
		})

	// agent 实例列表
	opsTool(srv, "agent_list", "查 agent 实例列表（真实数据）：容器化 openclaw/hermes 实例的名称/类型/状态/镜像/端口；instance_id 列供技能装卸类工具引用",
		[]toolArg{{"agent", "按类型过滤：openclaw | hermes（缺省全部）", false}, {"keyword", "名称模糊过滤", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			agent := argStr(a, "agent", "")
			if agent != "" && agent != "openclaw" && agent != "hermes" {
				return "", fmt.Errorf("agent 取值：openclaw | hermes（留空=全部）")
			}
			rows, err := queryRows(db.AgentDB(), `
SELECT instance_id, name, agent, version,
       CASE status WHEN 0 THEN '创建中' WHEN 1 THEN '运行中' WHEN 2 THEN '已停止' WHEN 3 THEN '错误'
                   ELSE status::text END            AS status,
       image_tag, host_port, access_url, user_id,
       to_char(created_at, 'YYYY-MM-DD HH24:MI')    AS created_at
FROM aa_agent_instance
WHERE ($1 = '' OR agent = $1) AND ($2 = '' OR name ILIKE '%' || $2 || '%')
ORDER BY id DESC LIMIT $3`, agent, argStr(a, "keyword", ""), clampInt(argInt(a, "limit", 10), 1, 50))
			return jsonOut(rows, err)
		})

	// agent 实例详情（不含任何 token 列）
	opsTool(srv, "agent_detail", "查单个 agent 实例详情（真实数据）：规格/镜像/容器/访问地址（不含密钥）",
		[]toolArg{{"name", "实例名称", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			name := argStr(a, "name", "")
			if name == "" {
				return "", fmt.Errorf("name 不能为空")
			}
			rows, err := queryRows(db.AgentDB(), `
SELECT name, agent, version, description,
       CASE status WHEN 0 THEN '创建中' WHEN 1 THEN '运行中' WHEN 2 THEN '已停止' WHEN 3 THEN '错误'
                   ELSE status::text END                              AS status,
       cpu_cores, memory_gb, disk_gb, gpu_enabled, gpu_type, gpu_count,
       os_type, os_version, image_registry, image_tag,
       container_name, host_port, access_url, storage_class, mount_path,
       to_char(created_at, 'YYYY-MM-DD HH24:MI:SS')                   AS created_at,
       to_char(updated_at, 'YYYY-MM-DD HH24:MI:SS')                   AS updated_at
FROM aa_agent_instance WHERE name = $1`, name)
			if err != nil {
				return "", err
			}
			if len(rows) == 0 {
				return fmt.Sprintf("未找到实例：%s", name), nil
			}
			b, _ := json.MarshalIndent(rows[0], "", "  ")
			return string(b), nil
		})

	// 桌面设备列表
	opsTool(srv, "device_list", "查桌面客户端设备列表（真实数据）：名称/平台/版本/最近登录",
		[]toolArg{{"platform", "按平台过滤：darwin | win", false}, {"keyword", "名称模糊过滤", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			plat := argStr(a, "platform", "")
			if plat != "" && plat != "darwin" && plat != "win" {
				return "", fmt.Errorf("platform 取值：darwin | win（留空=全部）")
			}
			rows, err := queryRows(db.ClientDB(), `
SELECT name, user_name, platform, os, arch, app_version, status, blocked,
       to_char(last_login_at, 'YYYY-MM-DD HH24:MI')  AS last_login_at,
       to_char(created_at, 'YYYY-MM-DD HH24:MI')     AS created_at
FROM aa_client_device
WHERE ($1 = '' OR platform = $1) AND ($2 = '' OR name ILIKE '%' || $2 || '%')
ORDER BY last_login_at DESC NULLS LAST LIMIT $3`, plat, argStr(a, "keyword", ""), clampInt(argInt(a, "limit", 10), 1, 50))
			return jsonOut(rows, err)
		})

	// 技能库检索
	opsTool(srv, "skill_search", "检索平台技能库（真实数据）：按名称/slug/简介模糊匹配",
		[]toolArg{{"keyword", "检索关键词", true}, {"limit", "条数 1-20，默认 5", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			kw := argStr(a, "keyword", "")
			if kw == "" {
				return "", fmt.Errorf("keyword 不能为空")
			}
			rows, err := queryRows(db.SkillDB(), `
SELECT slug, display_name, summary, download_count, star_count, rating_avg,
       to_char(updated_at, 'YYYY-MM-DD HH24:MI')     AS updated_at
FROM aa_skill
WHERE hidden = 0 AND status = 1
  AND (display_name ILIKE '%' || $1 || '%' OR slug ILIKE '%' || $1 || '%' OR summary ILIKE '%' || $1 || '%')
ORDER BY download_count DESC LIMIT $2`, kw, clampInt(argInt(a, "limit", 5), 1, 20))
			return jsonOut(rows, err)
		})

	// MCP 调用审计
	opsTool(srv, "mcp_audit", "查 MCP 网关调用审计（真实数据）：最近的工具调用、耗时、成败与调用方",
		[]toolArg{{"server", "按 server 名过滤（如 demo-weather）", false}, {"limit", "条数 1-100，默认 20", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			rows, err := queryRows(db.McpDB(), `
SELECT to_char(ts, 'YYYY-MM-DD HH24:MI:SS')          AS ts,
       server_name, tool, status, duration_ms, group_name, key_name,
       left(coalesce(error_message, ''), 200)        AS error
FROM aa_mcp_activity
WHERE $1 = '' OR server_name = $1
ORDER BY id DESC LIMIT $2`, argStr(a, "server", ""), clampInt(argInt(a, "limit", 20), 1, 100))
			return jsonOut(rows, err)
		})

	// ===== 看板/监控（Deps 装配了才注册；userId 口径=调用方本人，系统身份可显式 user_id/空=全量） =====

	// monitorUid 监控类工具的 user_id：actor 锁定本人；系统身份显式传参（空=全量）
	monitorUid := func(ctx context.Context, a map[string]any) string {
		if t := identityFromCtx(ctx); t != nil {
			return t.UserId
		}
		return argStr(a, "user_id", "")
	}

	if d.Dashboard != nil {
		opsTool(srv, "dashboard_overview", "查我的 agent 看板总览（真实数据）：实例/运行中/团队/模型/扫描任务计数（invocation24h/cost24h 两项暂恒 0）",
			[]toolArg{{"user_id", "系统身份显式指定用户（调用方身份存在时忽略此参数）", false}},
			func(ctx context.Context, a map[string]any) (string, error) {
				res, err := d.Dashboard.Overview(ctx, monitorUid(ctx, a))
				if err != nil {
					return "", err
				}
				return jsonOutAny(res)
			})
	}

	if d.Monitor != nil {
		opsTool(srv, "invocation_list", "查模型调用记录（真实数据）：token 用量/延迟/成败，定位某次调用",
			[]toolArg{{"model_id", "按模型过滤", false}, {"user_id", "系统身份显式指定用户（调用方身份存在时忽略此参数）", false}, {"page", "页码，默认 1", false}, {"limit", "条数 1-50，默认 10", false}},
			func(ctx context.Context, a map[string]any) (string, error) {
				list, total, err := d.Monitor.ListInvocations(ctx, monitorUid(ctx, a), argStr(a, "model_id", ""),
					clampInt(argInt(a, "page", 1), 1, 10000), clampInt(argInt(a, "limit", 10), 1, 50))
				if err != nil {
					return "", err
				}
				items := make([]map[string]any, 0, len(list))
				for _, e := range list {
					items = append(items, map[string]any{
						"invocationId": e.InvocationId, "instanceId": e.InstanceId,
						"modelId": e.ModelId, "requestedModel": e.RequestedModel,
						"totalTokens": e.TotalTokens, "latencyMs": e.LatencyMs,
						"status": e.Status, "createdAt": fmtTime(e.CreatedAt, "2006-01-02 15:04"),
					})
				}
				return jsonOutAny(map[string]any{"total": total, "items": items})
			})

		opsTool(srv, "audit_event_list", "查治理审计事件（真实数据）：类型/严重度/消息，追踪治理动作",
			[]toolArg{{"event_type", "按事件类型过滤", false}, {"user_id", "系统身份显式指定用户（调用方身份存在时忽略此参数）", false}, {"limit", "条数 1-50，默认 20", false}},
			func(ctx context.Context, a map[string]any) (string, error) {
				list, total, err := d.Monitor.ListAuditEvents(ctx, monitorUid(ctx, a), argStr(a, "event_type", ""),
					1, clampInt(argInt(a, "limit", 20), 1, 50))
				if err != nil {
					return "", err
				}
				items := make([]map[string]any, 0, len(list))
				for _, e := range list {
					items = append(items, map[string]any{
						"auditEventId": e.AuditEventId, "instanceId": e.InstanceId,
						"eventType": e.EventType, "severity": e.Severity, "message": e.Message,
						"createdAt": fmtTime(e.CreatedAt, "2006-01-02 15:04"),
					})
				}
				return jsonOutAny(map[string]any{"total": total, "items": items})
			})

		opsTool(srv, "cost_record_list", "查成本记录（真实数据）：按调用的 token 计费与估算成本",
			[]toolArg{{"user_id", "系统身份显式指定用户（调用方身份存在时忽略此参数）", false}, {"limit", "条数 1-50，默认 20", false}},
			func(ctx context.Context, a map[string]any) (string, error) {
				list, total, err := d.Monitor.ListCostRecords(ctx, monitorUid(ctx, a),
					1, clampInt(argInt(a, "limit", 20), 1, 50))
				if err != nil {
					return "", err
				}
				items := make([]map[string]any, 0, len(list))
				for _, e := range list {
					items = append(items, map[string]any{
						"costRecordId": e.CostRecordId, "instanceId": e.InstanceId,
						"modelName": e.ModelName, "totalTokens": e.TotalTokens,
						"estimatedCost": e.EstimatedCost, "currency": e.Currency,
						"recordedAt": fmtTime(e.RecordedAt, "2006-01-02 15:04"),
					})
				}
				return jsonOutAny(map[string]any{"total": total, "items": items})
			})

		opsTool(srv, "risk_hit_list", "查风险命中记录（真实数据）：规则命中/严重度/处置动作",
			[]toolArg{{"rule_id", "按规则 id 过滤", false}, {"limit", "条数 1-50，默认 20", false}},
			func(ctx context.Context, a map[string]any) (string, error) {
				list, total, err := d.Monitor.ListRiskHits(ctx, argStr(a, "rule_id", ""),
					1, clampInt(argInt(a, "limit", 20), 1, 50))
				if err != nil {
					return "", err
				}
				items := make([]map[string]any, 0, len(list))
				for _, e := range list {
					items = append(items, map[string]any{
						"riskHitId": e.RiskHitId, "instanceId": e.InstanceId,
						"ruleName": e.RuleName, "severity": e.Severity,
						"action": e.Action, "matchSummary": e.MatchSummary,
						"createdAt": fmtTime(e.CreatedAt, "2006-01-02 15:04"),
					})
				}
				return jsonOutAny(map[string]any{"total": total, "items": items})
			})

		opsTool(srv, "audit_log_list", "查用户操作审计日志（真实数据）：谁在何时对什么资源做了什么",
			[]toolArg{{"action", "按动作过滤", false}, {"user_id", "系统身份显式指定用户（调用方身份存在时忽略此参数）", false}, {"limit", "条数 1-50，默认 20", false}},
			func(ctx context.Context, a map[string]any) (string, error) {
				list, total, err := d.Monitor.ListAuditLogs(ctx, monitorUid(ctx, a), argStr(a, "action", ""),
					1, clampInt(argInt(a, "limit", 20), 1, 50))
				if err != nil {
					return "", err
				}
				items := make([]map[string]any, 0, len(list))
				for _, e := range list {
					items = append(items, map[string]any{
						"auditLogId": e.AuditLogId, "userId": e.UserId, "action": e.Action,
						"resourceType": e.ResourceType, "resourceId": e.ResourceId,
						"createdAt": fmtTime(e.CreatedAt, "2006-01-02 15:04"),
					})
				}
				return jsonOutAny(map[string]any{"total": total, "items": items})
			})
	}

	return srv
}
