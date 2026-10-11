package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"aiarsenal/infrastructure/market"
	"aiarsenal/infrastructure/market/xfyun"
)

// SkillMarketService 第三方 skill 市场服务：聚合各市场适配器，
// 提供来源列表 / 搜索 / 一键同步（下载包 → 复用 SkillPackageService.ImportZip 落地为本地 skill）。
type SkillMarketService struct {
	pkgSvc *SkillPackageService
	reg    *market.Registry
}

func NewSkillMarketService(pkgSvc *SkillPackageService, reg *market.Registry) *SkillMarketService {
	return &SkillMarketService{pkgSvc: pkgSvc, reg: reg}
}

// MarketSyncReq 一键同步请求（controller 绑定）。
type MarketSyncReq struct {
	Source      string `json:"source"`      // 市场来源 key，如 "xfyun"
	Slug        string `json:"slug"`        // 第三方 skill 唯一标识
	Version     string `json:"version"`     // 可空，空则取 latest
	NamespaceId string `json:"namespaceId"` // 同步到本地的目标命名空间
}

// MarketSyncResult 同步结果。
type MarketSyncResult struct {
	SkillId   string `json:"skillId"`
	VersionId string `json:"versionId"`
}

// ListSources 列出可用市场源。
func (s *SkillMarketService) ListSources() []market.MarketSource {
	if s.reg == nil {
		return nil
	}
	return s.reg.List()
}

// Search 搜索某市场的 skill。page 为 1-based（前端友好），内部转 0-based 调适配器。
func (s *SkillMarketService) Search(ctx context.Context, sourceKey, q string, page, limit int) (items []market.MarketSkill, total int, err error) {
	a, ok := s.reg.Get(sourceKey)
	if !ok {
		return nil, 0, fmt.Errorf("不支持的市场来源: %s", sourceKey)
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 12
	}
	return a.Search(ctx, q, page-1, limit)
}

// Sync 一键同步：从市场下载包 → 复用 SkillPackageService.ImportZip 落地为本地 skill。
func (s *SkillMarketService) Sync(ctx context.Context, req *MarketSyncReq, userId string) (*MarketSyncResult, error) {
	if req == nil || req.Source == "" || req.Slug == "" || req.NamespaceId == "" {
		return nil, errors.New("source/slug/namespaceId 不能为空")
	}
	a, ok := s.reg.Get(req.Source)
	if !ok {
		return nil, fmt.Errorf("不支持的市场来源: %s", req.Source)
	}
	raw, _, err := a.Download(ctx, req.Slug, req.Version)
	if err != nil {
		slog.Warn("第三方 skill 下载失败", "source", req.Source, "slug", req.Slug, "version", req.Version, "err", err)
		return nil, fmt.Errorf("下载技能包失败: %w", err)
	}
	skillId, versionId, err := s.pkgSvc.ImportZip(ctx, raw, &ImportZipReq{
		NamespaceId: req.NamespaceId,
		Source:      req.Source,
		SourceURL:   a.DetailURL(req.Slug),
		Version:     req.Version,
	}, userId)
	if err != nil {
		return nil, err
	}
	slog.Info("第三方 skill 一键同步完成", "source", req.Source, "slug", req.Slug, "skillId", skillId, "versionId", versionId, "userId", userId)
	return &MarketSyncResult{SkillId: skillId, VersionId: versionId}, nil
}

// MarketImportReq 链接导入请求（controller 绑定）。
type MarketImportReq struct {
	URL            string   `json:"url"`            // zip 直链或详情页链接
	NamespaceId    string   `json:"namespaceId"`    // 导入到本地的目标命名空间
	UpstreamAuthor string   `json:"upstreamAuthor"` // 原作者（引用安装的 @作者/标识 时拆出作者；空则回落包内 SKILL.md author）
	Tags           []string `json:"tags"`           // 导入后给技能打的标签（自由文本）
}

// ImportURL 链接导入：支持三类输入，落地均复用 ImportZip（含包解析/查重/追加版本）。
//  1. 讯飞详情页 /space/{ns}/{slug}（或旧 /skill/{slug}）→ 走适配器下载（限流+包校验）；
//  2. 任意 zip 直链 → 受限抓取（SSRF 防护/限跳/限 10MB）后导入；
//  3. 其它市场详情页 → 抓页面 HTML 解析 zip 链接，逐个尝试下载导入。
func (s *SkillMarketService) ImportURL(ctx context.Context, req *MarketImportReq, userId string) (*MarketSyncResult, error) {
	if req == nil || req.URL == "" || req.NamespaceId == "" {
		return nil, errors.New("url/namespaceId 不能为空")
	}
	u, err := market.ValidatePublicURL(req.URL)
	if err != nil {
		return nil, err
	}

	// 1) 讯飞详情页
	if ns, slug, ok := xfyun.ResolveDetailPath(u); ok {
		raw, _, derr := s.downloadXfyun(ctx, ns, slug)
		if derr != nil {
			return nil, fmt.Errorf("下载讯飞技能包失败: %w", derr)
		}
		return s.importFetched(ctx, raw, "xfyun", req.URL, req, userId)
	}

	// 2) 通用抓取
	res, err := market.FetchURL(ctx, req.URL)
	if err != nil {
		return nil, err
	}
	if market.LooksLikeZip(res.Body) {
		return s.importFetched(ctx, res.Body, "url", req.URL, req, userId)
	}

	// 3) 详情页解析 zip 链接
	if market.LooksLikeHTML(res.ContentType, res.Body) {
		links := market.ScrapeZipLinks(res.Body, u)
		if len(links) == 0 {
			return nil, errors.New("详情页中未找到 zip 下载链接")
		}
		var lastErr error
		for _, link := range links {
			r2, ferr := market.FetchURL(ctx, link)
			if ferr != nil {
				lastErr = ferr
				continue
			}
			if !market.LooksLikeZip(r2.Body) {
				lastErr = fmt.Errorf("候选链接不是 zip: %s", link)
				continue
			}
			return s.importFetched(ctx, r2.Body, "url", link, req, userId)
		}
		return nil, fmt.Errorf("详情页 zip 候选链接均下载失败: %w", lastErr)
	}
	return nil, fmt.Errorf("链接既不是 zip 也不是详情页（content-type: %s）", res.ContentType)
}

// downloadXfyun 按命名空间下载讯飞 skill（非 global 命名空间走 canonical 直连）。
func (s *SkillMarketService) downloadXfyun(ctx context.Context, ns, slug string) ([]byte, string, error) {
	a, ok := s.reg.Get("xfyun")
	if !ok {
		return nil, "", errors.New("讯飞市场适配器未注册")
	}
	if xa, is := a.(*xfyun.Adapter); is {
		return xa.DownloadNS(ctx, ns, slug, "")
	}
	// 兜底：仅 global 命名空间可达
	return a.Download(ctx, slug, "")
}

// importFetched 统一落地：zip 字节 → ImportZip（SourceURL 记实际下载地址，便于回溯重下；
// UpstreamAuthor/Tags 由请求透传，作者空则回落包内 SKILL.md author）。
func (s *SkillMarketService) importFetched(ctx context.Context, raw []byte, source, sourceURL string, req *MarketImportReq, userId string) (*MarketSyncResult, error) {
	skillId, versionId, err := s.pkgSvc.ImportZip(ctx, raw, &ImportZipReq{
		NamespaceId:    req.NamespaceId,
		Source:         source,
		SourceURL:      sourceURL,
		UpstreamAuthor: req.UpstreamAuthor,
		Tags:           req.Tags,
	}, userId)
	if err != nil {
		return nil, err
	}
	slog.Info("第三方 skill 链接导入完成", "source", source, "sourceUrl", sourceURL, "skillId", skillId, "versionId", versionId, "userId", userId)
	return &MarketSyncResult{SkillId: skillId, VersionId: versionId}, nil
}
