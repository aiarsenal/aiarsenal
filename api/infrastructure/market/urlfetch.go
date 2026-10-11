// Package market — 链接导入的安全抓取工具。
// 用户可提交任意第三方市场链接（zip 直链 / 详情页），服务端代为抓取：
// 仅允许 http/https、拒绝私网与回环目标（SSRF 防护，重定向每一跳都重检）、
// 限制跳数与响应大小，供 ImportURL 复用。
package market

import (
	"context"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	// urlFetchMaxBytes 单次抓取上限（zip 包与详情页共用），与本地 skill 包上限一致。
	urlFetchMaxBytes = 10 * 1024 * 1024
	// urlFetchMaxRedir 重定向跟随上限（详情页 → OSS 预签名直链常见 1~2 跳）。
	urlFetchMaxRedir = 3
	// scrapeMaxCandidates 详情页解析出的 zip 候选链接最多尝试数。
	scrapeMaxCandidates = 5
)

// urlFetchClient 链接导入专用 client：每跳校验目标主机，拒绝私网。
var urlFetchClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= urlFetchMaxRedir {
			return fmt.Errorf("重定向超过 %d 跳", urlFetchMaxRedir)
		}
		return checkPublicHost(req.URL)
	},
}

// ValidatePublicURL 校验用户提交的链接：http/https 且目标主机不是私网/回环。
func ValidatePublicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("链接格式不合法: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("仅支持 http/https 链接")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("链接缺少主机名")
	}
	if err := checkPublicHost(u); err != nil {
		return nil, err
	}
	return u, nil
}

// checkPublicHost 解析主机名（含字面量 IP），任一地址落在私网/回环/链路本地即拒绝。
func checkPublicHost(u *url.URL) error {
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("链接缺少主机名")
	}
	// 字面量 IP 直接判；域名走 DNS 解析后逐 IP 判。
	ips := []net.IP{net.ParseIP(host)}
	if ips[0] == nil {
		resolved, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
		if err != nil {
			return fmt.Errorf("域名解析失败: %s", host)
		}
		ips = make([]net.IP, 0, len(resolved))
		for _, r := range resolved {
			ips = append(ips, r.IP)
		}
	}
	for _, ip := range ips {
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("不允许访问内网地址: %s", host)
		}
	}
	return nil
}

// FetchResult 一次受限抓取的结果。
type FetchResult struct {
	Body        []byte
	ContentType string
	FinalURL    string // 跟随重定向后的最终地址
}

// FetchURL 抓取用户提交的链接（限大小限跳数，每跳 SSRF 重检）。
func FetchURL(ctx context.Context, raw string) (*FetchResult, error) {
	if _, err := ValidatePublicURL(raw); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "aiarsenal-skill-import/1.0")
	resp, err := urlFetchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("抓取链接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("链接返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, urlFetchMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if len(body) > urlFetchMaxBytes {
		return nil, fmt.Errorf("响应超过上限(%d字节)", urlFetchMaxBytes)
	}
	return &FetchResult{
		Body:        body,
		ContentType: strings.ToLower(resp.Header.Get("Content-Type")),
		FinalURL:    resp.Request.URL.String(),
	}, nil
}

// LooksLikeZip 按 zip 本地文件头魔数判断（PK\x03\x04；空包 PK\x05\x06）。
func LooksLikeZip(b []byte) bool {
	if len(b) < 4 {
		return false
	}
	return (b[0] == 'P' && b[1] == 'K' && b[2] == 3 && b[3] == 4) ||
		(b[0] == 'P' && b[1] == 'K' && b[2] == 5 && b[3] == 6)
}

// LooksLikeHTML 按 content-type 或正文开头是否为标签判断。
func LooksLikeHTML(ctype string, body []byte) bool {
	if strings.Contains(ctype, "text/html") || strings.Contains(ctype, "application/xhtml") {
		return true
	}
	t := strings.TrimLeft(string(body[:min(len(body), 256)]), " \t\r\n\uFEFF")
	return strings.HasPrefix(t, "<!DOCTYPE") || strings.HasPrefix(t, "<html") ||
		strings.HasPrefix(t, "<head") || strings.HasPrefix(t, "<body")
}

var (
	// 绝对 zip 链接（含查询串）：https://…/xxx.zip?…
	reAbsZip = regexp.MustCompile(`https?://[^\s"'<>\\]+?\.zip(?:\?[^\s"'<>\\]*)?`)
	// 相对 zip 路径：href="/…/xxx.zip" / src="…/xxx.zip"
	reRelZip = regexp.MustCompile(`(?:href|src)=["']([^"'<>]+?\.zip(?:\?[^"'<>]*)?)["']`)
)

// ScrapeZipLinks 从详情页 HTML 提取 zip 下载链接（绝对链接优先，相对链接基于页面地址补全）。
func ScrapeZipLinks(body []byte, base *url.URL) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	add := func(raw string) {
		u, err := base.Parse(html.UnescapeString(raw))
		if err != nil || u.Scheme == "" || u.Host == "" || seen[u.String()] {
			return
		}
		seen[u.String()] = true
		out = append(out, u.String())
	}
	decoded := html.UnescapeString(string(body))
	for _, m := range reAbsZip.FindAllString(decoded, -1) {
		add(m)
	}
	if base != nil {
		for _, m := range reRelZip.FindAllStringSubmatch(decoded, -1) {
			add(m[1])
		}
	}
	if len(out) > scrapeMaxCandidates {
		out = out[:scrapeMaxCandidates]
	}
	return out
}
