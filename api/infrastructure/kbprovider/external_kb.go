package kbprovider

// external_kb 连接器：远端知识库服务代理。
//
// 约定的远端 HTTP 契约（GET + JSON）：
//
//	文档清单：GET {endpoint}?action=list&offset=0&limit=100
//	          → {"list":[{"key","title","summary","content"},...],"total":N}
//	          （裸数组 [ ... ] 亦可）
//	搜索：    GET {endpoint}?{query_param}=<q>&k=<n>
//	          → {"list":[{"key","title","summary","content","score"},...]}
//	          （裸数组亦可）
//
// config:
//
//	endpoint    string 远端服务地址（必填，如 https://kb.internal/api/v1/docs）
//	auth_header string 认证头名（如 Authorization；可空）
//	auth_value  string 认证头值（如 Bearer xxx；或 "Name: value" 整头一行写法）
//	query_param string 搜索参数名（默认 q）
//	page_size   int    清单翻页页大小（默认 100，上限 500）
//	max_items   int    最多入库条数（默认 1000，上限 10000）

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// init 自注册。
func init() { register("external_kb", &ExternalKB{}) }

// ExternalKB 远端知识库服务代理连接器。
type ExternalKB struct{}

// ExternalHit 外部知识库搜索命中（service 层透传合并用）。
type ExternalHit struct {
	Key     string  `json:"key"`
	Title   string  `json:"title"`
	Summary string  `json:"summary"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// Sync 实现 Provider：翻页拉远端文档清单逐条 ingest（upsert 依据 key）。
func (p *ExternalKB) Sync(ctx context.Context, cfg map[string]any, ingest IngestFunc, run *RunReporter) error {
	endpoint := cfgString(cfg, "endpoint")
	if endpoint == "" {
		return errors.New("external_kb：缺少 endpoint 配置")
	}
	pageSize := cfgInt(cfg, "page_size", 100)
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}
	maxItems := cfgInt(cfg, "max_items", 1000)
	if maxItems <= 0 {
		maxItems = 1000
	}
	if maxItems > 10000 {
		maxItems = 10000
	}

	offset := 0
	total := -1 // 未知总数（远端可能不给），按空页终止
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if offset >= maxItems {
			run.Note("达到 max_items=%d 截断", maxItems)
			break
		}
		q := url.Values{}
		q.Set("action", "list")
		q.Set("offset", strconv.Itoa(offset))
		q.Set("limit", strconv.Itoa(pageSize))
		data, err := extGet(ctx, endpoint, cfg, q)
		if err != nil {
			return fmt.Errorf("external_kb：拉取清单失败：%w", err)
		}
		var page struct {
			List  []externalDoc `json:"list"`
			Total int64         `json:"total"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return fmt.Errorf("external_kb：清单响应不是合法 JSON：%w", err)
		}
		if total < 0 && page.Total > 0 {
			total = int(page.Total)
		}
		if len(page.List) == 0 {
			break
		}
		for _, doc := range page.List {
			if err := ctx.Err(); err != nil {
				return err
			}
			run.Inc("rows", 1)
			key := doc.Key
			if key == "" {
				key = "doc-" + strconv.Itoa(offset+len(page.List))
			}
			title := doc.Title
			if title == "" {
				title = key
			}
			if _, err := emit(ingest, run, key, title, doc.Summary, doc.Content); err != nil {
				return err
			}
		}
		offset += len(page.List)
		if total >= 0 && offset >= total { // 远端给了 total 且已取满
			break
		}
		if len(page.List) < pageSize { // 末页
			break
		}
	}
	return nil
}

// ProxySearch 远端搜索透传（service 层搜索合并用）。query_param 缺省 q。
func (p *ExternalKB) ProxySearch(ctx context.Context, cfg map[string]any, query string, k int) ([]ExternalHit, error) {
	endpoint := cfgString(cfg, "endpoint")
	if endpoint == "" || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if k <= 0 {
		k = 10
	}
	q := url.Values{}
	if name := cfgString(cfg, "query_param"); name != "" {
		q.Set(name, query)
	} else {
		q.Set("q", query) // 缺省参数名
	}
	q.Set("k", strconv.Itoa(k))
	data, err := extGet(ctx, endpoint, cfg, q)
	if err != nil {
		return nil, fmt.Errorf("external_kb：搜索失败：%w", err)
	}
	var res struct {
		List []ExternalHit `json:"list"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		// 兼容裸数组响应
		var arr []ExternalHit
		if err2 := json.Unmarshal(data, &arr); err2 != nil {
			return nil, fmt.Errorf("external_kb：搜索响应不是合法 JSON：%w", err)
		}
		res.List = arr
	}
	if len(res.List) > k {
		res.List = res.List[:k]
	}
	return res.List, nil
}

// externalDoc 远端清单条目。
type externalDoc struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Content string `json:"content"`
}

// extGet 带认证头的外部请求（auth_value 支持 "Name: value" 整头一行写法）。
func extGet(ctx context.Context, endpoint string, cfg map[string]any, query url.Values) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("endpoint 非法：%s", endpoint)
	}
	vs := u.Query()
	for k, arr := range query {
		if k != "" && len(arr) > 0 {
			vs.Set(k, arr[0])
		}
	}
	u.RawQuery = vs.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if hv := cfgString(cfg, "auth_value"); hv != "" {
		// "Name: value" 整头写法：拆开；否则配合 auth_header
		if i := strings.Index(hv, ":"); i > 0 && !strings.Contains(hv[:i], " ") {
			req.Header.Set(strings.TrimSpace(hv[:i]), strings.TrimSpace(hv[i+1:]))
		} else if h := cfgString(cfg, "auth_header"); h != "" {
			req.Header.Set(h, hv)
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d（%s）", resp.StatusCode, u.Host)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 20<<20))
}
