// Package xfyun 讯飞技能市场（https://skill.xfyun.cn）适配器。
// 经实测：其 ClawHub 兼容层 /api/v1/search、/api/v1/skills、/api/v1/download/{slug} 均
// 匿名可用（源码标注 anonymous=20/分钟），下载到的 skill 包为 ZIP（含 SKILL.md，
// frontmatter name+description 与本地 parseSkillMD 兼容）。
package xfyun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"aiarsenal/infrastructure/market"
)

const (
	baseURL = "https://skill.xfyun.cn"

	// 匿名限流上限 20/min，这里保守取更低值并叠加缓存，避免多用户并发触发对方 429。
	httpTimeout     = 20 * time.Second
	searchCacheTTL  = 60 * time.Second
	maxPackageBytes = 10 * 1024 * 1024 // 与本地 skill 包上限一致（maxTotalSize）
)

// Adapter 讯飞技能市场适配器。
type Adapter struct {
	http *http.Client

	// 限流：search / download 各自独立令牌桶（低于对方匿名上限）。
	searchLimiter   *rate.Limiter
	downloadLimiter *rate.Limiter

	// search 结果短缓存（相同 q+page+limit 60s 内命中缓存，不重复打对方接口）。
	cacheMu sync.Mutex
	cache   map[string]cacheEntry
}

type cacheEntry struct {
	items  []market.MarketSkill
	total  int
	expire time.Time
}

// New 构造讯飞适配器。
func New() *Adapter {
	return &Adapter{
		http:            &http.Client{Timeout: httpTimeout},
		searchLimiter:   rate.NewLimiter(rate.Every(4*time.Second), 5), // ≈15/min, burst 5
		downloadLimiter: rate.NewLimiter(rate.Every(6*time.Second), 3), // ≈10/min, burst 3
		cache:           make(map[string]cacheEntry),
	}
}

func (a *Adapter) Info() market.MarketSource {
	return market.MarketSource{
		Key:        "xfyun",
		Name:       "讯飞技能市场",
		Icon:       "📡",
		Logo:       baseURL + "/favicon.svg",
		ThemeColor: "#2b6cff",
		SiteURL:    baseURL,
		Desc:       "科大讯飞第三方技能市场",
		Syncable:   true,
	}
}

func (a *Adapter) DetailURL(slug string) string {
	// 讯飞 skill 详情页是 SPA 路由 /space/$namespace/$slug（其前端路由表实测，无 /skill/{slug}）；
	// 匿名搜索命中的技能均在 global 命名空间（/api/v1/skills/global/{slug} 返回 namespace=global）。
	return fmt.Sprintf("%s/space/global/%s", baseURL, url.PathEscape(slug))
}

// xfyunSearchResp 讯飞 /api/v1/search 响应。
type xfyunSearchResp struct {
	Results []struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"displayName"`
		Summary     string `json:"summary"`
		Version     string `json:"version"`
		Score       any    `json:"score"`
		UpdatedAt   int64  `json:"updatedAt"`
	} `json:"results"`
}

func (a *Adapter) Search(ctx context.Context, q string, page, limit int) ([]market.MarketSkill, int, error) {
	if limit <= 0 {
		limit = 12
	}
	if page < 0 {
		page = 0
	}

	// 命中缓存直接返回
	cacheKey := fmt.Sprintf("%s|%d|%d", q, page, limit)
	if items, total, ok := a.readCache(cacheKey); ok {
		return items, total, nil
	}

	// 限流（等待令牌，超时则失败）
	if err := a.searchLimiter.Wait(ctx); err != nil {
		return nil, 0, fmt.Errorf("搜索限流等待失败: %w", err)
	}

	u := fmt.Sprintf("%s/api/v1/search?q=%s&page=%d&limit=%d",
		baseURL, url.QueryEscape(q), page, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("请求讯飞搜索失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("讯飞搜索返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 搜索响应限 1MB
	if err != nil {
		return nil, 0, err
	}
	var sr xfyunSearchResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, 0, fmt.Errorf("解析讯飞搜索响应失败: %w", err)
	}

	items := make([]market.MarketSkill, 0, len(sr.Results))
	for _, r := range sr.Results {
		items = append(items, market.MarketSkill{
			Slug:        r.Slug,
			DisplayName: r.DisplayName,
			Summary:     r.Summary,
			Version:     r.Version,
			UpdatedAt:   r.UpdatedAt,
			DetailURL:   a.DetailURL(r.Slug),
		})
	}

	// 讯飞搜索响应无 total 字段：满页说明可能还有下一页，据此估算 total 让前端分页器可用。
	var total int
	if len(items) >= limit {
		total = (page+1)*limit + 1
	} else {
		total = page*limit + len(items)
	}

	a.writeCache(cacheKey, items, total)
	return items, total, nil
}

// errRedirectNoLocation 对方返回 3xx 但缺 Location 头，http.Client 无法跟随。
// 中文 slug 时讯飞兼容层 /api/v1/download/{slug} 会触发该上游 bug
// （2026-08-25 实测：Location 头整个缺失，客户端只能拿到裸 302）。
var errRedirectNoLocation = errors.New("重定向缺少 Location")

// canonicalDownloadURL 讯飞站内 canonical 下载路径，兼容层 302 缺 Location 时的直连回退。
func canonicalDownloadURL(slug, version string) string {
	return canonicalNSDownloadURL("global", slug, version)
}

// canonicalNSDownloadURL 指定命名空间的 canonical 下载路径（详情页链接导入用）。
func canonicalNSDownloadURL(ns, slug, version string) string {
	if version == "" || version == "latest" {
		return fmt.Sprintf("%s/api/v1/skills/%s/%s/download", baseURL, url.PathEscape(ns), url.PathEscape(slug))
	}
	return fmt.Sprintf("%s/api/v1/skills/%s/%s/versions/%s/download",
		baseURL, url.PathEscape(ns), url.PathEscape(slug), url.PathEscape(version))
}

// ResolveDetailPath 解析讯飞站内详情页 URL → (namespace, slug)。
// 现行 SPA 路由 /space/{ns}/{slug}；旧 /skill/{slug} 视为 global 命名空间。
func ResolveDetailPath(u *url.URL) (ns, slug string, ok bool) {
	if u == nil || u.Host != "skill.xfyun.cn" {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "space" && parts[1] != "" && parts[2] != "" {
		return parts[1], parts[2], true
	}
	if len(parts) >= 2 && parts[0] == "skill" && parts[1] != "" {
		return "global", parts[1], true
	}
	return "", "", false
}

// DownloadNS 从指定命名空间下载（详情页链接导入用；global 复用 Download 兼容层路径）。
func (a *Adapter) DownloadNS(ctx context.Context, ns, slug, version string) ([]byte, string, error) {
	if slug == "" {
		return nil, "", fmt.Errorf("slug 不能为空")
	}
	if ns == "" || ns == "global" {
		return a.Download(ctx, slug, version)
	}
	if version == "" {
		version = "latest"
	}
	if err := a.downloadLimiter.Wait(ctx); err != nil {
		return nil, "", fmt.Errorf("下载限流等待失败: %w", err)
	}
	// 非 global 命名空间无兼容层入口，直接走 canonical 路径。
	return a.fetchZip(ctx, canonicalNSDownloadURL(ns, slug, version), slug)
}

func (a *Adapter) Download(ctx context.Context, slug, version string) ([]byte, string, error) {
	if slug == "" {
		return nil, "", fmt.Errorf("slug 不能为空")
	}
	if version == "" {
		version = "latest"
	}
	if err := a.downloadLimiter.Wait(ctx); err != nil {
		return nil, "", fmt.Errorf("下载限流等待失败: %w", err)
	}

	// 常规走 ClawHub 兼容层（ASCII slug 会正常 302，http.Client 自动跟随）；
	// 中文 slug 时对方 302 不带 Location → 回退直连 canonical 路径再试一次。
	u := fmt.Sprintf("%s/api/v1/download/%s?version=%s", baseURL, url.PathEscape(slug), url.QueryEscape(version))
	raw, filename, err := a.fetchZip(ctx, u, slug)
	if errors.Is(err, errRedirectNoLocation) {
		return a.fetchZip(ctx, canonicalDownloadURL(slug, version), slug)
	}
	return raw, filename, err
}

// fetchZip GET 指定 URL，跟随重定向后按包上限读取，返回 zip 内容与文件名。
func (a *Adapter) fetchZip(ctx context.Context, u, slug string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	// http.Client 默认跟随 302（讯飞会跳到站内 canonical 路径 / OSS 预签名 URL）。
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("请求讯飞下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 3xx 且缺 Location 时 http.Client 原样返回该响应（无从跟随），打标让上层走直连回退。
		if resp.StatusCode >= 300 && resp.StatusCode < 400 && resp.Header.Get("Location") == "" {
			return nil, "", fmt.Errorf("讯飞下载返回 %d: %w", resp.StatusCode, errRedirectNoLocation)
		}
		return nil, "", fmt.Errorf("讯飞下载返回 %d", resp.StatusCode)
	}
	// 限大小读取：超出上限直接报错，避免大包耗尽内存。
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxPackageBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("读取讯飞包失败: %w", err)
	}
	if len(raw) > maxPackageBytes {
		return nil, "", fmt.Errorf("skill 包超过上限(%d字节)", maxPackageBytes)
	}
	filename := parseFilename(resp.Header.Get("Content-Disposition"))
	if filename == "" {
		filename = slug + ".zip"
	}
	return raw, filename, nil
}

// parseFilename 从 Content-Disposition: attachment; filename*=UTF-8”xxx.zip 提取文件名。
func parseFilename(cd string) string {
	if cd == "" {
		return ""
	}
	// 优先 filename*=UTF-8''xxx
	if i := strings.Index(cd, "filename*="); i >= 0 {
		v := cd[i+len("filename*="):]
		v = strings.Trim(v, "\"")
		if k := strings.Index(v, "''"); k >= 0 {
			v = v[k+2:]
		}
		// 去掉可能的后续参数
		for _, sep := range []string{";", ","} {
			if j := strings.Index(v, sep); j >= 0 {
				v = v[:j]
			}
		}
		if dec, err := url.QueryUnescape(strings.TrimSpace(v)); err == nil {
			return dec
		}
		return strings.TrimSpace(v)
	}
	if i := strings.Index(cd, "filename="); i >= 0 {
		v := strings.Trim(cd[i+len("filename="):], "\"")
		if j := strings.Index(v, ";"); j >= 0 {
			v = v[:j]
		}
		return strings.TrimSpace(v)
	}
	return ""
}

func (a *Adapter) readCache(key string) ([]market.MarketSkill, int, bool) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	e, ok := a.cache[key]
	if !ok || time.Now().After(e.expire) {
		return nil, 0, false
	}
	return e.items, e.total, true
}

func (a *Adapter) writeCache(key string, items []market.MarketSkill, total int) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	// 顺手清理已过期项，避免无界增长。
	now := time.Now()
	for k, v := range a.cache {
		if now.After(v.expire) {
			delete(a.cache, k)
		}
	}
	a.cache[key] = cacheEntry{items: items, total: total, expire: now.Add(searchCacheTTL)}
}
