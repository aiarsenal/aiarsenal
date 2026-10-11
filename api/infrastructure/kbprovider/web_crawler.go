package kbprovider

// web_crawl 连接器：种子 URL 出发的同域广度优先抓取。
//
// config:
//
//	seed_urls      []string 种子 URL（必填，逗号分隔字符串或数组）
//	max_pages      int      最多抓取页数（默认 20，上限 200）
//	same_host_only bool     同域限制（默认 true：只跟随种子域名下的链接）
//	max_depth      int      最大 BFS 深度（默认 5）
//
// 行为：BFS 队列逐层抓取；每页抽正文文本（<title>/meta description 入标题与摘要，
// 块级元素文本行入正文）；链接只跟 http(s) 与非二进制扩展名；限速 1 req/s；
// 非 HTML/文本响应按条目跳过计数。单页抓取/解析失败计数 skipped 继续下一页。

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// init 自注册。
func init() { register("web_crawl", &WebCrawler{}) }

// WebCrawler 网页爬取连接器。
type WebCrawler struct {
	client *http.Client
}

// 每次请求至少间隔（限速 1 req/s，对目标站点友好）
const crawlMinInterval = time.Second

// crawlBinaryExts 视为二进制不跟随/不入库的扩展名（抽不出文本，跳过计数）。
var crawlBinaryExts = map[string]struct{}{
	".pdf": {}, ".doc": {}, ".docx": {}, ".xls": {}, ".xlsx": {}, ".ppt": {}, ".pptx": {},
	".zip": {}, ".rar": {}, ".7z": {}, ".gz": {}, ".tar": {}, ".png": {}, ".jpg": {},
	".jpeg": {}, ".gif": {}, ".webp": {}, ".bmp": {}, ".svg": {}, ".ico": {}, ".mp3": {},
	".mp4": {}, ".avi": {}, ".mov": {}, ".exe": {}, ".dmg": {}, ".apk": {}, ".iso": {},
	".css": {}, ".js": {}, ".woff": {}, ".woff2": {}, ".ttf": {}, ".eot": {},
}

// crawlClient 复用 HTTP 客户端（超时收紧：单页 20s）。
func (p *WebCrawler) clientHTTP() *http.Client {
	if p.client == nil {
		p.client = &http.Client{Timeout: 20 * time.Second}
	}
	return p.client
}

// Sync 实现 Provider。
func (p *WebCrawler) Sync(ctx context.Context, cfg map[string]any, ingest IngestFunc, run *RunReporter) error {
	seeds := cfgStrings(cfg, "seed_urls")
	if len(seeds) == 0 {
		return errors.New("web_crawl：缺少 seed_urls 配置")
	}
	maxPages := cfgInt(cfg, "max_pages", 20)
	if maxPages <= 0 {
		maxPages = 20
	}
	if maxPages > 200 {
		maxPages = 200
	}
	maxDepth := cfgInt(cfg, "max_depth", 5)
	if maxDepth <= 0 {
		maxDepth = 5
	}
	sameHost := cfgBool(cfg, "same_host_only", true)

	// 种子归一（去 fragment、host 小写），以首个种子的 host 为同域基准
	type item struct{ u *url.URL; depth int }
	var queue []item
	seen := map[string]bool{}
	var hostBase string
	for _, s := range seeds {
		u, err := normalizePageURL(s)
		if err != nil {
			run.Inc("skipped", 1)
			run.Note("种子 URL 非法：%s", s)
			continue
		}
		if hostBase == "" {
			hostBase = u.Host
		}
		if k := u.String(); !seen[k] {
			seen[k] = true
			queue = append(queue, item{u: u, depth: 0})
		}
	}
	if len(queue) == 0 {
		return errors.New("web_crawl：没有可用的种子 URL")
	}

	lastAt := time.Time{} // 限速基准
	throttle := func() {
		if !lastAt.IsZero() {
			if d := time.Since(lastAt); d < crawlMinInterval {
				time.Sleep(crawlMinInterval - d)
			}
		}
		lastAt = time.Now()
	}

	fetched := 0
	for len(queue) > 0 && fetched < maxPages {
		cur := queue[0]
		queue = queue[1:]
		if cur.depth > maxDepth {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		throttle()

		pageURL := cur.u.String()
		body, contentType, err := crawlFetch(ctx, p.clientHTTP(), pageURL)
		if err != nil {
			run.Inc("skipped", 1)
			run.Note("抓取失败 %s：%v", pageURL, err)
			continue
		}
		fetched++

		switch {
		case strings.Contains(contentType, "text/html"):
			title, summary, text, links := parseHTMLPage(body, pageURL)
			run.Inc("pages", 1)
			if _, err := emit(ingest, run, pageURL, title, summary, text); err != nil {
				return err
			}
			// 链接入队（同域/非二进制/未访问）
			for _, raw := range links {
				if fetched+len(queue) >= maxPages {
					break
				}
				next, err := resolvePageURL(cur.u, raw)
				if err != nil {
					continue
				}
				if sameHost && next.Host != hostBase {
					continue
				}
				if hasBinaryExt(next.Path) {
					continue
				}
				if k := next.String(); !seen[k] {
					seen[k] = true
					queue = append(queue, item{u: next, depth: cur.depth + 1})
				}
			}
		case strings.Contains(contentType, "text/plain"):
			run.Inc("pages", 1)
			title := pageURL
			if cur.u.Path != "" && cur.u.Path != "/" {
				title = cur.u.Path
			}
			if _, err := emit(ingest, run, pageURL, title, "", string(body)); err != nil {
				return err
			}
		default:
			run.Inc("skipped", 1)
			run.Note("非文本响应跳过 %s（%s）", pageURL, contentType)
		}
	}
	return nil
}

// crawlFetch 单页抓取（UA 标识 + 5MB 上限 + 只收 2xx）。
func crawlFetch(ctx context.Context, client *http.Client, pageURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "aiarsenal-kb-crawler/1.0 (+knowledge-base sync)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	b, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, ct, err
	}
	return b, ct, nil
}

// parseHTMLPage 抽正文与链接：title/meta description → 标题与摘要；块级元素文本行 → 正文。
func parseHTMLPage(data []byte, baseURL string) (title, summary, text string, links []string) {
	root, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return "", "", "", nil
	}
	var bodyLines []string
	var walk func(n *html.Node, inBody bool)
	walk = func(n *html.Node, inBody bool) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "template":
				return // 噪声子树整枝剪掉（head 不剪：title/meta 描述要从里面取，
				//         其文本节点只在 inBody 后收集，不会漏进正文）
			case "title":
				title = strings.TrimSpace(nodeText(n))
				return
			case "meta":
				name, content := attrOf(n, "name"), attrOf(n, "content")
				if name == "description" && summary == "" {
					summary = strings.TrimSpace(content)
				}
				return
			case "a":
				if href := strings.TrimSpace(attrOf(n, "href")); href != "" {
					links = append(links, href)
				}
			case "p", "div", "br", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6",
				"section", "article", "table", "blockquote", "pre":
				if inBody {
					bodyLines = append(bodyLines, "\n") // 块级元素 = 换行边界
				}
			}
			if n.Data == "body" {
				inBody = true
			}
		}
		if n.Type == html.TextNode {
			if inBody {
				if s := strings.TrimSpace(n.Data); s != "" {
					bodyLines = append(bodyLines, s)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inBody)
		}
	}
	walk(root, false)
	text = strings.Join(bodyLines, " ")
	// 压空白：块边界已是换行占位，这里把多空白压成单空格保持行语义
	text = strings.Join(strings.Fields(text), " ")
	if title == "" {
		title = baseURL
	}
	return title, summary, text, links
}

// nodeText 取子树全部文本（title/meta 用）。
func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// attrOf 取元素属性值。
func attrOf(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// normalizePageURL 归一页面 URL（scheme 补全、host 小写、去 fragment）。
func normalizePageURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("空 URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("非法 URL：%s", raw)
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return u, nil
}

// resolvePageURL 相对链接 → 绝对（并归一）。
func resolvePageURL(base *url.URL, href string) (*url.URL, error) {
	ref, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return nil, err
	}
	return normalizePageURL(base.ResolveReference(ref).String())
}

// hasBinaryExt 路径扩展名是否属于二进制（不跟随/不入库）。
func hasBinaryExt(path string) bool {
	i := strings.LastIndex(path, ".")
	if i < 0 {
		return false
	}
	_, ok := crawlBinaryExts[strings.ToLower(path[i:])]
	return ok
}
