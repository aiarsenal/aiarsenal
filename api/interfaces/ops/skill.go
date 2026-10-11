package ops

// 技能域（aa_skill 库 + agent 库实例技能表）：技能库检索/版本/市场/安全审计 + 实例装卸。
// 读端点 platform-skill（/mcp/skill）、写端点 platform-skill-write（/mcp/skill-write）。
// 市场同步/链接导入会从 ops 进程出网（service 层已有 SSRF 防护/限跳/限 10MB）。
// 安装两条路（均收口到平台安全门禁，禁止绕过平台直接从外部下载装进实例）：
//   - 搜索安装：skill_install_search 出候选 → 用户确认选哪个 → skill_install(confirmed=true)
//   - 外部引用安装：skill_install_external 导入平台库 → 安全扫描 → 「安全」才自动发布+安装，
//     「不安全」拒装并返回原因。扫描终态结论由扫描器统一发双通道通知（站内+数字人同一消息，
//     见 service/channel_notice_service.go），本工具不再重复发。
// 刻意不做的：技能包创建与版本上传（multipart 文件上传不适合 MCP 文本工具，
// 入库请走 Web UI 或市场导入）。

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"aiarsenal/application/service"
	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
)

// skillDefaultNS 市场导入目标命名空间缺省值：取第一个 GLOBAL 命名空间
func skillDefaultNS(ctx context.Context, d Deps) (string, error) {
	all, err := d.SkillNS.All(ctx)
	if err != nil {
		return "", err
	}
	for _, ns := range all {
		if ns.Type == "GLOBAL" && ns.Status == 1 {
			return ns.NamespaceId, nil
		}
	}
	return "", fmt.Errorf("平台无可用全局命名空间，请显式传 namespace_id")
}

// ===== 读 server（platform-skill，/mcp/skill） =====

func buildSkillServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-skill", Version: "1.0"}, nil)

	opsTool(srv, "skill_list", "查技能库列表（真实数据）：按命名空间/slug/名称/关键词过滤；配对端点 platform-ops 的 skill_search 是语义检索",
		[]toolArg{{"keyword", "名称/简介模糊过滤", false}, {"namespace_id", "按命名空间过滤", false}, {"status", "状态过滤：1=上架 0=下架（缺省全部）", false}, {"page", "页码，默认 1", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, total, err := d.Skill.List(ctx, argStr(a, "namespace_id", ""), "", "",
				argStr(a, "status", ""), argStr(a, "keyword", ""),
				clampInt(argInt(a, "page", 1), 1, 10000), clampInt(argInt(a, "limit", 10), 1, 50))
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, s := range list {
				items = append(items, map[string]any{
					"skillId": s.SkillId, "slug": s.Slug, "displayName": s.DisplayName,
					"summary": s.Summary, "namespaceId": s.NamespaceId,
					"source": s.Source, "downloadCount": s.DownloadCount,
					"updatedAt": fmtTime(s.UpdatedAt, "2006-01-02 15:04"),
				})
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	opsTool(srv, "skill_get", "查技能详情（真实数据）：skill_id 取全量字段（含溯源：贡献人/原作者/原链接/来源标识/标签）；要装到实例用 skill_install",
		[]toolArg{{"skill_id", "技能 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			s, err := d.Skill.Get(ctx, argStr(a, "skill_id", ""))
			if err != nil {
				return fmt.Sprintf("未找到技能：%s", argStr(a, "skill_id", "")), nil
			}
			tags := []string{}
			if ts, terr := d.Skill.ListTags(ctx, s.SkillId); terr == nil {
				for _, t := range ts {
					tags = append(tags, t.TagName)
				}
			}
			return jsonOutAny(map[string]any{
				"skillId": s.SkillId, "slug": s.Slug, "displayName": s.DisplayName,
				"summary": s.Summary, "namespaceId": s.NamespaceId,
				"ownerId": s.OwnerId, "ownerName": s.OwnerName, "source": s.Source,
				"sourceUrl": s.SourceUrl, "upstreamAuthor": s.UpstreamAuthor, "marketRef": s.MarketRef,
				"tags":       tags,
				"visibility": s.Visibility, "status": s.Status, "latestVersionId": s.LatestVersionId,
				"downloadCount": s.DownloadCount, "starCount": s.StarCount,
				"createdAt": fmtTime(s.CreatedAt, "2006-01-02 15:04"),
			})
		})

	opsTool(srv, "skill_search", "语义检索技能库（真实数据）：按意图找技能（如「PDF 抽取」「定时提醒」），向量 top-k 命中",
		[]toolArg{{"query", "检索意图", true}, {"limit", "条数 1-20，默认 5", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			q := argStr(a, "query", "")
			if q == "" {
				return "", fmt.Errorf("query 不能为空")
			}
			rows, err := d.Skill.Search(ctx, q, clampInt(argInt(a, "limit", 5), 1, 20), d.Embed)
			return jsonOut(rows, err)
		})

	opsTool(srv, "skill_version_list", "查技能版本列表（真实数据）：版本号/状态（DRAFT|PUBLISHED|YANKED）/安全审计状态（SAFE|UNSAFE|SCANNING|SCAN_ERROR|NO_AUDIT）/变更记录",
		[]toolArg{{"skill_id", "技能 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, err := d.Skill.ListVersions(ctx, argStr(a, "skill_id", ""))
			if err != nil {
				return "", err
			}
			statusName := map[uint]string{0: "DRAFT", 1: "PUBLISHED", 2: "YANKED"}
			items := make([]map[string]any, 0, len(list))
			for _, v := range list {
				items = append(items, map[string]any{
					"versionId": v.VersionId, "version": v.Version,
					"status": statusName[v.Status], "changelog": v.Changelog,
					"auditStatus": auditVerStatusEn(v),
					"fileCount":   v.FileCount, "totalSize": v.TotalSize,
					"publishedAt": fmtTime(v.PublishedAt, "2006-01-02 15:04"),
				})
			}
			return jsonOutAny(map[string]any{"items": items})
		})

	opsTool(srv, "skill_market_sources", "查可同步的第三方技能市场源（真实数据）：支持平台内搜索+一键同步的市场（目前讯飞）；仅收录站点链接的外部市场目录用 skill_market_sites 查（用户提到外部市场时先查目录推荐）",
		nil,
		func(ctx context.Context, a map[string]any) (string, error) {
			sources := d.Market.ListSources()
			items := make([]map[string]any, 0, len(sources))
			for _, src := range sources {
				items = append(items, map[string]any{
					"key": src.Key, "name": src.Name, "desc": src.Desc, "siteUrl": src.SiteURL,
				})
			}
			// 结果级引导（比描述更可靠）：这是「可同步源」不是完整目录——用户问外部市场时
			// 应推 skill_market_sites 的平台收录目录（名称+链接+介绍）。
			return jsonOutAny(map[string]any{
				"items": items,
				"next":  "以上仅为支持站内搜索/一键同步的市场源；平台收录的完整外部市场目录（含各站点链接与介绍）用 skill_market_sites 查询并推荐给用户",
			})
		})

	// 外部市场目录（aa_skill_market_site）：平台只存链接不代搜。给 agent 的引导话术固定：
	// 站点链接给用户 → 用户登录原站搜索 → 拿到安装引用（zip/详情页链接，或 @作者/标识+站点链接）
	// → 回平台走 skill_install_external（导入→安全扫描→结论双通道通知→安全才安装）。
	opsTool(srv, "skill_market_sites", "查外部技能市场目录（真实数据）：平台收录的第三方市场站点链接清单（平台不代搜这些站）。用户想找外部技能时：把站点链接推给用户，请用户登录原站搜索并拿到安装引用（技能的 zip 直链/详情页链接，或 @作者/标识 + 站点链接），再走 platform-skill-write 的 skill_install_external 安装（自动导入→安全扫描→结论通知→安装）；平台技能库里已有的技能优先用 skill_install_search 搜候选",
		nil,
		func(ctx context.Context, a map[string]any) (string, error) {
			sites, err := d.MarketSite.All(ctx)
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(sites))
			for _, st := range sites {
				items = append(items, map[string]any{
					"name": st.Name, "siteUrl": st.SiteUrl, "description": st.Description,
				})
			}
			return jsonOutAny(map[string]any{
				"items": items,
				"next":  "请把以上站点链接给用户并说明：登录原站搜索技能，拿到安装引用（zip/详情页链接，或 @作者/标识 + 站点链接）后回来，用 skill_install_external 安装（平台会安全扫描并通知结论）",
			})
		})

	opsTool(srv, "skill_market_search", "搜第三方技能市场（真实数据）：按市场源 key 搜技能（讯飞可同步，其余站内跳转）",
		[]toolArg{{"q", "搜索词", true}, {"source", "市场源 key（skill_market_sources 查得，如 xfyun）", true}, {"page", "页码，默认 1", false}, {"limit", "条数 1-20，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			items, total, err := d.Market.Search(ctx, argStr(a, "source", ""), argStr(a, "q", ""),
				clampInt(argInt(a, "page", 1), 1, 100), clampInt(argInt(a, "limit", 10), 1, 20))
			if err != nil {
				return "", err
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	opsTool(srv, "skill_sec_list", "查技能安全审计（真实数据）：版本级扫描结论（PASSED|FAILED|SCANNING|ERROR|PENDING）与风险等级",
		[]toolArg{{"version_id", "按版本 id 过滤", false}, {"status", "按结论过滤：0=PENDING 1=PASSED 2=FAILED 3=SCANNING 4=ERROR", false}, {"limit", "条数 1-50，默认 10", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, total, err := d.SkillSec.ListAudits(ctx, argStr(a, "version_id", ""), argStr(a, "status", ""),
				1, clampInt(argInt(a, "limit", 10), 1, 50))
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, au := range list {
				items = append(items, map[string]any{
					"auditId": au.AuditId, "skillVersionId": au.SkillVersionId,
					"status": auditStatusEn(au.Status), "severity": au.Severity,
					"scannedAt": fmtTime(au.ScannedAt, "2006-01-02 15:04"),
				})
			}
			return jsonOutAny(map[string]any{"total": total, "items": items})
		})

	opsTool(srv, "skill_namespace_list", "查技能命名空间（真实数据）：GLOBAL/TEAM 命名空间清单（市场导入目标）",
		nil,
		func(ctx context.Context, a map[string]any) (string, error) {
			all, err := d.SkillNS.All(ctx)
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(all))
			for _, ns := range all {
				items = append(items, map[string]any{
					"namespaceId": ns.NamespaceId, "slug": ns.Slug,
					"displayName": ns.DisplayName, "type": ns.Type,
				})
			}
			return jsonOutAny(items)
		})

	opsTool(srv, "instance_skill_list", "查实例已装技能（真实数据）：某 agent 实例装了哪些技能、装到的版本与库最新版（可判断升级）",
		[]toolArg{{"instance_id", "实例 id（platform-ops 的 agent_list instance_id 列）", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			list, err := d.Runtime.ListInstanceSkills(ctx, argStr(a, "instance_id", ""))
			if err != nil {
				return "", err
			}
			items := make([]map[string]any, 0, len(list))
			for _, it := range list {
				items = append(items, map[string]any{
					"skillId": it.SkillId, "slug": it.Slug, "displayName": it.DisplayName,
					"versionId": it.VersionId, "version": it.Version,
					"latestVersionId": it.LatestVersionId, "latestVersion": it.LatestVersion,
				})
			}
			return jsonOutAny(map[string]any{"items": items})
		})

	// 搜索安装候选：关键词+语义混合搜库，出候选+审计状态+已装剔除；
	// 安装动作在写端点 skill_install，且必须带 confirmed=true（用户确认门禁）。
	opsTool(srv, "skill_install_search", "搜可安装技能候选（真实数据）：按关键词/意图搜平台技能库，返回候选清单（含安全审计状态、是否可装、是否已装）；用户想找技能装时先用本工具，把候选展示给用户、由用户确认选哪一个后，再调 platform-skill-write 的 skill_install（带 confirmed=true）——未经用户确认不得安装",
		[]toolArg{{"query", "搜索意图/关键词（如「PDF 抽取」「投标决策」）", true}, {"instance_id", "目标实例 id（给出则剔除该实例已装技能，agent_list instance_id 列）", false}, {"limit", "条数 1-20，默认 8", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			q := argStr(a, "query", "")
			if q == "" {
				return "", fmt.Errorf("query 不能为空")
			}
			limit := clampInt(argInt(a, "limit", 8), 1, 20)
			installed := map[string]bool{}
			if instanceId := argStr(a, "instance_id", ""); instanceId != "" {
				if list, err := d.Runtime.ListInstanceSkills(ctx, instanceId); err == nil {
					for _, it := range list {
						installed[it.SkillId] = true
					}
				}
			}
			// 候选池：语义 top-k 在前 + 关键词 LIKE 补充，按 skill_id 去重
			type cand struct {
				id, title, summary, source string
				score                      any
			}
			ordered := []cand{}
			seen := map[string]bool{}
			if rows, err := d.Skill.Search(ctx, q, limit, d.Embed); err == nil {
				for _, r := range rows {
					id, _ := r["skill_id"].(string)
					if id == "" || seen[id] {
						continue
					}
					seen[id] = true
					title, _ := r["title"].(string)
					summ, _ := r["summary"].(string)
					ordered = append(ordered, cand{id, title, summ, "", r["score"]})
				}
			}
			if list, _, err := d.Skill.List(ctx, "", "", "", "1", q, 1, limit); err == nil {
				for _, sk := range list {
					if seen[sk.SkillId] {
						continue
					}
					seen[sk.SkillId] = true
					ordered = append(ordered, cand{sk.SkillId, sk.DisplayName, sk.Summary, sk.Source, nil})
				}
			}
			if len(ordered) > limit {
				ordered = ordered[:limit]
			}
			items := make([]map[string]any, 0, len(ordered))
			for _, c := range ordered {
				if installed[c.id] {
					continue // 该实例已装：不再作为候选
				}
				// 可装口径同 skill_install 的版本解析：任一非撤回且审计「安全」的版本即可装
				installable, verLabel, latestAudit := false, "", "无版本"
				if vs, err := d.Skill.ListVersions(ctx, c.id); err == nil {
					for _, v := range vs { // ListVersions 按 id desc：最新非撤回版本的状态作展示
						if v.Status == 2 {
							continue
						}
						latestAudit = auditVerStatusZh(v)
						break
					}
					for _, v := range vs {
						if v.Status == 2 {
							continue
						}
						if v.AuditStatus != nil && *v.AuditStatus == entity.AuditStatusPassed {
							installable, verLabel = true, v.Version
							break
						}
					}
				}
				item := map[string]any{
					"skillId": c.id, "name": c.title, "summary": c.summary,
					"installable": installable, "installableVersion": verLabel,
					"latestAudit": latestAudit,
				}
				if c.source != "" {
					item["source"] = c.source
				}
				if c.score != nil {
					item["matchScore"] = c.score
				}
				items = append(items, item)
			}
			if len(items) == 0 {
				return "（无候选）平台技能库中没有匹配且该实例未安装的技能；可请管理员在 Web 端导入，或用户提供外部引用后走 skill_install_external", nil
			}
			return jsonOutAny(map[string]any{
				"items": items,
				"next":  "请把以上候选展示给用户，由用户确认选择安装哪一个；确认后调 platform-skill-write 的 skill_install（skill_id=用户选定的候选，confirmed=true，version_id 缺省=最新安全版）。未经用户确认不得安装",
			})
		})

	return srv
}

// ===== 写 server（platform-skill-write，/mcp/skill-write） =====

func buildSkillWriteServer(d Deps) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-mcp-skill-write", Version: "1.0"}, nil)

	opsTool(srv, "skill_market_sync", "从第三方市场同步技能入库（写真实库）：source+slug 必填；导入后需 skill_version_publish 发布才能被安装",
		[]toolArg{{"source", "市场源 key（可同步源目前是 xfyun）", true}, {"slug", "第三方技能标识（skill_market_search 查得）", true}, {"version", "版本号（缺省=最新）", false}, {"namespace_id", "目标命名空间（缺省=全局命名空间）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			ns := argStr(a, "namespace_id", "")
			if ns == "" {
				var err error
				if ns, err = skillDefaultNS(ctx, d); err != nil {
					return "", err
				}
			}
			res, err := d.Market.Sync(ctx, &service.MarketSyncReq{
				Source: argStr(a, "source", ""), Slug: argStr(a, "slug", ""),
				Version: argStr(a, "version", ""), NamespaceId: ns,
			}, projActor(ctx))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已导入 skill_id=%s，version_id=%s（DRAFT，需 skill_version_publish 发布）", res.SkillId, res.VersionId), nil
		})

	opsTool(srv, "skill_market_import_url", "按链接导入技能（写真实库）：zip 直链或市场详情页；可一并记原作者/打标签（溯源：贡献人=导入人、原作者、原链接自动落档）；导入后需 skill_version_publish 发布",
		[]toolArg{{"url", "zip 直链或市场详情页链接", true}, {"namespace_id", "目标命名空间（缺省=全局命名空间）", false}, {"upstream_author", "原作者（外部市场作者 handle 如 @user_xxx；缺省回落包内 SKILL.md author）", false}, {"tags", "标签，逗号分隔（如「PDF,办公」）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			ns := argStr(a, "namespace_id", "")
			if ns == "" {
				var err error
				if ns, err = skillDefaultNS(ctx, d); err != nil {
					return "", err
				}
			}
			res, err := d.Market.ImportURL(ctx, &service.MarketImportReq{
				URL: argStr(a, "url", ""), NamespaceId: ns,
				UpstreamAuthor: argStr(a, "upstream_author", ""),
				Tags:           argTagsList(a, "tags"),
			}, projActor(ctx))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已导入 skill_id=%s，version_id=%s（DRAFT，需 skill_version_publish 发布）", res.SkillId, res.VersionId), nil
		})

	opsTool(srv, "skill_version_publish", "发布技能版本（写真实库）：DRAFT→PUBLISHED，发布后才能被实例安装为该版本",
		[]toolArg{{"version_id", "版本 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			if err := d.Skill.PublishVersion(ctx, argStr(a, "version_id", "")); err != nil {
				return "", err
			}
			return "已发布", nil
		})

	opsTool(srv, "skill_version_yank", "下架技能版本（写真实库）：PUBLISHED→YANKED，已装实例不受影响、新安装不再可选",
		[]toolArg{{"version_id", "版本 id", true}, {"reason", "下架原因", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			by := projActor(ctx)
			if err := d.Skill.YankVersion(ctx, argStr(a, "version_id", ""), by, argStr(a, "reason", "")); err != nil {
				return "", err
			}
			return "已下架", nil
		})

	opsTool(srv, "skill_install", "给 agent 实例装技能（下发真实命令）：须用户确认——先用 platform-skill 的 skill_install_search 出候选、用户明确选定后带 confirmed=true 调用；返回命令 id，安装异步完成，稍后用 instance_skill_list 核对装上没有",
		[]toolArg{{"instance_id", "实例 id（platform-ops 的 agent_list instance_id 列）", true}, {"skill_id", "技能 id（skill_install_search 查得、用户选定的候选）", true}, {"version_id", "版本 id（缺省=最新安全版）", false}, {"confirmed", "用户已确认安装该技能传 true（缺省 false=拒绝执行，先展示候选征得用户确认）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			if argStr(a, "confirmed", "") != "true" {
				return "", fmt.Errorf("未经用户确认：安装技能须先把候选展示给用户（platform-skill 的 skill_install_search）、由用户明确选定，再带 confirmed=true 重新调用")
			}
			cmdId, err := d.Runtime.InstallSkill(ctx, argStr(a, "instance_id", ""), argStr(a, "skill_id", ""),
				argStr(a, "version_id", ""), projActor(ctx))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已下发安装命令（cmd_id=%s），安装异步完成，稍后用 instance_skill_list 核对", cmdId), nil
		})

	// 外部引用安装：导入平台库 → 安全扫描（结论发站内通知）→「安全」自动发布+安装。
	// 用户在对话里明确点名要装的引用（如「请根据 https://…/install/xx.md，安装 @user/slug」）
	// 即视为已确认，无需再走 confirmed；安装一律经平台库过安全门禁，不给绕过平台直装外部包。
	opsTool(srv, "skill_install_external", "按外部引用安装技能（@作者/标识 或链接，写真实库）：自动导入平台技能库（落档溯源：贡献人=操作人、原作者、原链接、标签、命名空间）→安全扫描（终态结论由平台统一双通道通知技能归属人：站内+数字人转达）→扫描「安全」才自动发布并安装到实例，「不安全」拒装并返回原因；用户让装某个外部技能（如『请根据 https://…/install/xx.md，安装 @user/slug』）时用本工具，禁止绕过平台直接下载安装",
		[]toolArg{
			{"instance_id", "目标实例 id（platform-ops 的 agent_list instance_id 列）", true},
			{"handle", "外部技能引用 @作者/标识（如 @user_xxx/my-skill）；与 url 至少给一个", false},
			{"url", "市场站点链接（安装说明 md / 详情页 / zip 直链）；handle 方式必传（取站点根构造下载链接），单独给则按链接导入", false},
			{"namespace_id", "导入目标命名空间（缺省=全局命名空间）", false},
			{"tags", "导入后给技能打的标签，逗号分隔（如「PDF,办公」）", false},
			{"wait_seconds", "等待安全扫描出结论的秒数（5-180，默认 60）", false},
		},
		func(ctx context.Context, a map[string]any) (string, error) {
			instanceId := argStr(a, "instance_id", "")
			handle := argStr(a, "handle", "")
			rawURL := argStr(a, "url", "")
			if instanceId == "" {
				return "", fmt.Errorf("instance_id 不能为空")
			}
			if handle == "" && rawURL == "" {
				return "", fmt.Errorf("handle 与 url 至少给一个")
			}
			// 实例先验：导入是有副作用的长流程，别等最后才发现实例不存在
			instRows, err := queryRows(db.AgentDB(), `SELECT name FROM aa_agent_instance WHERE instance_id = $1`, instanceId)
			if err != nil {
				return "", err
			}
			if len(instRows) == 0 {
				return "", fmt.Errorf("实例不存在: %s", instanceId)
			}
			instName, _ := instRows[0]["name"].(string)
			by := projActor(ctx)

			// 引用 → 导入链接：@作者/标识 + 站点根 → 站点技能包下载协议
			// （对齐 openclaw 生态市场 GET {root}/api/v1/download?slug=&ownerHandle= 返回 zip），
			// 之后的抓取/校验/入库全复用 ImportURL（SSRF 防护/限跳/限 10MB）。
			importURL := rawURL
			handleSlug := ""
			ownerHandle := ""
			if handle != "" {
				if rawURL == "" {
					return "", fmt.Errorf("handle 方式需一并传 url（市场站点链接，用于取站点根构造下载链接）")
				}
				u, uerr := url.Parse(rawURL)
				if uerr != nil || u.Scheme == "" || u.Host == "" {
					return "", fmt.Errorf("url 非法: %s", rawURL)
				}
				owner, slug := parseSkillHandle(handle)
				if slug == "" {
					return "", fmt.Errorf("handle 非法（应为 @作者/标识）: %s", handle)
				}
				handleSlug = slug
				ownerHandle = owner
				q := url.Values{"slug": []string{slug}}
				if owner != "" {
					q.Set("ownerHandle", owner)
				}
				importURL = u.Scheme + "://" + u.Host + "/api/v1/download?" + q.Encode()
			}

			ns := argStr(a, "namespace_id", "")
			if ns == "" {
				var nerr error
				if ns, nerr = skillDefaultNS(ctx, d); nerr != nil {
					return "", nerr
				}
			}

			// 库里已有同名技能且存在「安全」版本：直接装库版本，不重复导入
			//（每次引用都重导会追加版本+重扫，制造版本垃圾；要更新内容请走管理端导入）。
			if handleSlug != "" {
				if sk := skillFindBySlug(ctx, d, ns, handleSlug); sk != nil {
					if cmdId, ierr := d.Runtime.InstallSkill(ctx, instanceId, sk.SkillId, "", by); ierr == nil {
						return fmt.Sprintf("库中已有 %s 的安全版本，未重复导入，已直接下发安装（cmd_id=%s）；安装异步完成，稍后用 instance_skill_list 核对", handleSlug, cmdId), nil
					}
					// 无安全版本等 → 落到下面的导入重扫
				}
			}

			// 溯源：@作者/标识 的作者拆出记原作者（空则回落包内 SKILL.md author）；来源链接记实际导入链接
			author := ""
			if ownerHandle != "" {
				author = "@" + ownerHandle
			}
			res, err := d.Market.ImportURL(ctx, &service.MarketImportReq{
				URL: importURL, NamespaceId: ns,
				UpstreamAuthor: author, Tags: argTagsList(a, "tags"),
			}, by)
			if err != nil {
				return "", fmt.Errorf("导入失败: %w", err)
			}
			waitSec := clampInt(argInt(a, "wait_seconds", 60), 5, 180)
			audit := waitSkillAudit(ctx, d, res.VersionId, waitSec)
			name, ver := skillVerLabel(ctx, d, res.SkillId, res.VersionId)
			switch {
			case audit != nil && audit.Status == entity.AuditStatusPassed:
				if perr := d.Skill.PublishVersion(ctx, res.VersionId); perr != nil {
					return "", fmt.Errorf("扫描已通过但发布失败: %w（version_id=%s，可稍后用 skill_version_publish 重试）", perr, res.VersionId)
				}
				cmdId, ierr := d.Runtime.InstallSkill(ctx, instanceId, res.SkillId, res.VersionId, by)
				if ierr != nil {
					return "", ierr
				}
				return fmt.Sprintf("导入→安全扫描「安全」→已发布→已下发安装到实例 %s（cmd_id=%s）；安装异步完成，稍后用 instance_skill_list 核对。扫描结论已由平台统一双通道通知技能归属人（站内+数字人）", instName, cmdId), nil
			case audit != nil && audit.Status == entity.AuditStatusFailed:
				reason := service.FindingsSummary(audit.FindingsJson)
				return fmt.Sprintf("安全扫描未通过（不安全），已拒绝安装。原因：%s（version_id=%s，结论已通知技能归属人，明细见平台安全审计页）", reason, res.VersionId), nil
			case audit != nil && audit.Status == entity.AuditStatusError:
				return fmt.Sprintf("安全扫描失败（扫描器出错，非内容结论）：%s（version_id=%s，可重扫后再试，结论已通知技能归属人）", audit.ScannerRaw, res.VersionId), nil
			default: // 无审计记录 / 仍在待扫描·扫描中（超时未出结论）
				return fmt.Sprintf("已导入「%s」%s（skill_id=%s version_id=%s），安全扫描在 %d 秒内未出结论，本次未安装；稍后用 skill_sec_list 查结论，安全后 skill_version_publish + skill_install",
					name, ver, res.SkillId, res.VersionId, waitSec), nil
			}
		})

	opsTool(srv, "skill_uninstall", "卸载 agent 实例上的技能（下发真实命令）：返回命令 id，异步完成",
		[]toolArg{{"instance_id", "实例 id", true}, {"skill_id", "技能 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			cmdId, err := d.Runtime.UninstallSkill(ctx, argStr(a, "instance_id", ""), argStr(a, "skill_id", ""), projActor(ctx))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已下发卸载命令（cmd_id=%s），异步完成", cmdId), nil
		})

	// 技能元数据更新：取现行记录改非空字段再落库（与 Web 编辑同一条链路）。
	// 注意版本级「更新」走导入链：skill_market_import_url 传原链接会追加新版本（扫完发布即可升）。
	opsTool(srv, "skill_update", "更新技能元数据（写真实库）：改名称/简介/可见性；内容更新走导入链（skill_market_import_url 传原链接=追加新版本）",
		[]toolArg{{"skill_id", "技能 id", true}, {"display_name", "新名称（缺省不改）", false}, {"summary", "新简介（缺省不改）", false}, {"visibility", "可见性：0=公开 1=私有（缺省不改）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			sk, err := d.Skill.Get(ctx, argStr(a, "skill_id", ""))
			if err != nil {
				return "", fmt.Errorf("未找到技能：%s", argStr(a, "skill_id", ""))
			}
			if v := argStr(a, "display_name", ""); v != "" {
				sk.DisplayName = v
			}
			if v := argStr(a, "summary", ""); v != "" {
				sk.Summary = v
			}
			if v := argStr(a, "visibility", ""); v == "0" || v == "1" {
				sk.Visibility = uint(v[0] - '0')
			}
			sk.UpdatedBy = projActor(ctx)
			if err := d.Skill.Edit(ctx, sk); err != nil {
				return "", err
			}
			return "已更新", nil
		})

	opsTool(srv, "skill_delete", "删除技能（写真实库，软删除）：连带版本/文件记录保留入库，列表即不可见；已装实例不受影响（如需回收先 skill_uninstall）",
		[]toolArg{{"skill_id", "技能 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			if err := d.Skill.Delete(ctx, []string{argStr(a, "skill_id", "")}); err != nil {
				return "", err
			}
			return "已删除（软删除，列表不再可见）", nil
		})

	// ===== 外部市场目录管理（aa_skill_market_site，与 Web「其他市场」页同一条链路） =====

	opsTool(srv, "skill_market_site_add", "外部市场目录新增站点（写真实库）：平台只存链接引导用户去原站搜索，收录新市场时用",
		[]toolArg{{"name", "市场名称", true}, {"site_url", "市场站点链接（http(s)）", true}, {"description", "描述", false}, {"icon", "emoji 图标（如 ☁️）", false}, {"sort", "排序（小在前，缺省 0）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			marketId, err := d.MarketSite.Add(ctx, &service.MarketSiteReq{
				Name: argStr(a, "name", ""), SiteUrl: argStr(a, "site_url", ""),
				Description: argStr(a, "description", ""), Icon: argStr(a, "icon", ""),
				Sort: argInt(a, "sort", 0),
			}, projActor(ctx))
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("已新增市场目录项 market_id=%s", marketId), nil
		})

	opsTool(srv, "skill_market_site_edit", "外部市场目录编辑站点（写真实库）：改名称/链接/描述/图标/排序/启停",
		[]toolArg{{"market_id", "目录项 id", true}, {"name", "市场名称", true}, {"site_url", "市场站点链接（http(s)）", true}, {"description", "描述", false}, {"icon", "emoji 图标（如 ☁️）", false}, {"status", "1=显示 0=隐藏（缺省 1）", false}, {"sort", "排序（小在前，缺省 0）", false}},
		func(ctx context.Context, a map[string]any) (string, error) {
			req := &service.MarketSiteReq{
				MarketId: argStr(a, "market_id", ""), Name: argStr(a, "name", ""),
				SiteUrl: argStr(a, "site_url", ""), Description: argStr(a, "description", ""),
				Icon: argStr(a, "icon", ""), Sort: argInt(a, "sort", 0),
			}
			switch argStr(a, "status", "1") {
			case "0":
				req.Status = 0
			default:
				req.Status = 1
			}
			if err := d.MarketSite.Edit(ctx, req, projActor(ctx)); err != nil {
				return "", err
			}
			return "已更新市场目录项", nil
		})

	opsTool(srv, "skill_market_site_delete", "外部市场目录删除站点（写真实库）：目录不再展示该市场，已导入技能不受影响",
		[]toolArg{{"market_id", "目录项 id", true}},
		func(ctx context.Context, a map[string]any) (string, error) {
			if err := d.MarketSite.Delete(ctx, argStr(a, "market_id", "")); err != nil {
				return "", err
			}
			return "已删除市场目录项", nil
		})

	return srv
}

// ===== 外部引用安装/候选搜索的共用助手 =====

// argTagsList 逗号分隔的标签参数 → 去空白的字符串切片（空/全空返回 nil）。
// 注意与 project.go 的 argTags（返回规整字符串）不是同一个。
func argTagsList(m map[string]any, k string) []string {
	v, _ := m[k].(string)
	if v == "" {
		return nil
	}
	out := []string{}
	for _, t := range strings.Split(v, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseSkillHandle 解析外部技能引用 @作者/标识 → (作者, 标识)；@标识 也接受（作者空）。
func parseSkillHandle(h string) (owner, slug string) {
	h = strings.TrimPrefix(strings.TrimSpace(h), "@")
	parts := strings.SplitN(h, "/", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", strings.TrimSpace(parts[0])
}

// skillFindBySlug 命名空间内按 slug 精确匹配（List 是 LIKE 匹配，需遍历比对）。
func skillFindBySlug(ctx context.Context, d Deps, namespaceId, slug string) *entity.Skill {
	list, _, err := d.Skill.List(ctx, namespaceId, slug, "", "", "", 1, 50)
	if err != nil {
		return nil
	}
	for _, sk := range list {
		if sk.Slug == slug {
			return sk
		}
	}
	return nil
}

// waitSkillAudit 轮询版本最新审计直到终态（安全/不安全/扫描失败）或超时；
// 超时/无记录时返回最后查到的行（可能 nil 或仍 PENDING/SCANNING），由调用方分支处理。
func waitSkillAudit(ctx context.Context, d Deps, versionId string, waitSec int) *entity.SkillSecurityAudit {
	deadline := time.Now().Add(time.Duration(waitSec) * time.Second)
	for {
		a, err := d.SkillSec.LatestAudit(ctx, versionId)
		if err == nil && a != nil &&
			a.Status != entity.AuditStatusPending && a.Status != entity.AuditStatusScanning {
			return a
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return a
		}
		select {
		case <-ctx.Done():
			return a
		case <-time.After(time.Second):
		}
	}
}

// skillVerLabel 技能名 + 版本号（通知/回执文案用，查不到回落 id）。
func skillVerLabel(ctx context.Context, d Deps, skillId, versionId string) (string, string) {
	name, ver := skillId, versionId
	if sk, err := d.Skill.Get(ctx, skillId); err == nil && sk != nil && sk.DisplayName != "" {
		name = sk.DisplayName
	}
	if v, err := d.Skill.GetVersion(ctx, versionId); err == nil && v != nil && v.Version != "" {
		ver = v.Version
	}
	return name, ver
}

// auditStatusEn 审计状态码 → 英文标识（工具输出，五态）。
func auditStatusEn(s uint) string {
	switch s {
	case entity.AuditStatusPassed:
		return "PASSED"
	case entity.AuditStatusFailed:
		return "FAILED"
	case entity.AuditStatusScanning:
		return "SCANNING"
	case entity.AuditStatusError:
		return "ERROR"
	default:
		return "PENDING"
	}
}

// auditVerStatusEn 版本审计状态 → 英文标识（nil=无审计记录，同门禁口径：无记录不可装）。
func auditVerStatusEn(v *entity.SkillVersion) string {
	if v.AuditStatus == nil {
		return "NO_AUDIT"
	}
	return auditStatusEn(*v.AuditStatus)
}

// auditVerStatusZh 版本审计状态 → 中文（候选列表展示用）。
func auditVerStatusZh(v *entity.SkillVersion) string {
	if v.AuditStatus == nil {
		return "无审计记录"
	}
	switch *v.AuditStatus {
	case entity.AuditStatusPassed:
		return "安全"
	case entity.AuditStatusFailed:
		return "不安全"
	case entity.AuditStatusScanning:
		return "扫描中"
	case entity.AuditStatusError:
		return "扫描失败"
	default:
		return "待扫描"
	}
}
