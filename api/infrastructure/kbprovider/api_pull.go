package kbprovider

// api_pull 连接器：HTTP GET JSON → 定位数组 → 逐条展平为文本入库。
//
// config:
//
//	url           string          接口地址（必填，GET）
//	headers       map[string]any  附加请求头（token 等）
//	list_path     string          数组定位（点路径，如 "data.items"；空=顶层为数组，
//	                                  顶层单对象则按单条处理）
//	title_field   string          标题字段（点路径；缺省「条目 N」）
//	content_field string          正文字段（点路径；缺省整条递归展平为「键: 值」行）
//	summary_field string          摘要字段（可空）
//	key_field     string          外部键字段（点路径；缺省 item-<n>，upsert 依据）
//	max_items     int             最多入库条数（默认 1000，上限 10000）

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// init 自注册。
func init() { register("api_pull", &APIPull{}) }

// APIPull HTTP JSON 接口拉取连接器。
type APIPull struct{}

// Sync 实现 Provider。
func (p *APIPull) Sync(ctx context.Context, cfg map[string]any, ingest IngestFunc, run *RunReporter) error {
	endpoint := cfgString(cfg, "url")
	if endpoint == "" {
		return errors.New("api_pull：缺少 url 配置")
	}
	titleField := cfgString(cfg, "title_field")
	contentField := cfgString(cfg, "content_field")
	summaryField := cfgString(cfg, "summary_field")
	keyField := cfgString(cfg, "key_field")
	listPath := cfgString(cfg, "list_path")
	maxItems := cfgInt(cfg, "max_items", 1000)
	if maxItems <= 0 {
		maxItems = 1000
	}
	if maxItems > 10000 {
		maxItems = 10000
	}

	data, err := apiGet(ctx, endpoint, cfg, 20<<20)
	if err != nil {
		return fmt.Errorf("api_pull：%w", err)
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("api_pull：响应不是合法 JSON：%w", err)
	}

	items := []any{}
	if listPath != "" {
		node := jsonPath(root, listPath)
		arr, ok := node.([]any)
		if !ok {
			return fmt.Errorf("api_pull：list_path %q 未定位到数组", listPath)
		}
		items = arr
	} else {
		switch v := root.(type) {
		case []any:
			items = v
		default:
			items = []any{root} // 单对象按单条处理
		}
	}

	n := 0
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		if n >= maxItems {
			run.Note("达到 max_items=%d 截断", maxItems)
			break
		}
		obj, ok := it.(map[string]any)
		if !ok {
			run.Inc("skipped", 1)
			continue
		}
		n++
		run.Inc("rows", 1)
		key := fmt.Sprintf("item-%06d", n)
		if keyField != "" {
			if s := jsonStr(obj, keyField); s != "" {
				key = s
			}
		}
		title := jsonStr(obj, titleField)
		if title == "" {
			title = "条目 " + fmt.Sprintf("%06d", n)
		}
		summary := jsonStr(obj, summaryField)
		content := ""
		if contentField != "" {
			content = jsonStr(obj, contentField)
		}
		if strings.TrimSpace(content) == "" { // 无正文字段/字段为空：整条递归展平
			content = flattenJSONText(obj)
		}
		if _, err := emit(ingest, run, key, title, summary, content); err != nil {
			return err
		}
	}
	return nil
}

// apiGet 带 headers 的 GET（响应上限 maxBytes）。
func apiGet(ctx context.Context, url string, cfg map[string]any, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if headers, ok := cfg["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok && k != "" {
				req.Header.Set(k, s)
			}
		}
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d（%s）", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

// jsonPath 点路径取值（段为 map 键或数组下标）。
func jsonPath(v any, path string) any {
	cur := v
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[seg]
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(node) {
				return nil
			}
			cur = node[i]
		default:
			return nil
		}
	}
	return cur
}

// jsonStr 点路径取标量转字符串（数值/布尔容错）。
func jsonStr(v any, path string) string {
	if path == "" {
		return ""
	}
	switch x := jsonPath(v, path).(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	default:
		if b, err := json.Marshal(x); err == nil {
			return string(b)
		}
		return ""
	}
}

// flattenJSONText 递归展平 JSON 对象为「点路径: 标量值」行（数组内标量逗号连接，
// 数组内对象带下标展开）。api_pull 缺正文字段时的兜底内容形态；
// map 无序，按路径字典序输出保证同一数据两次拉取内容幂等一致（哈希去重不抖动）。
func flattenJSONText(v any) string {
	kv := FlattenJSON(v, "")
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		if kv[k] != "" {
			lines = append(lines, k+": "+kv[k])
		}
	}
	return strings.Join(lines, "\n")
}

// FlattenJSON 递归展平：返回 点路径 → 标量文本。
// 对象递归拼路径；标量转字符串；数组：标量逗号连接为该路径一个值，对象按下标展开。
func FlattenJSON(v any, prefix string) map[string]string {
	out := map[string]string{}
	flattenInto(v, prefix, out)
	return out
}

func flattenInto(v any, prefix string, out map[string]string) {
	key := func(seg string) string {
		if prefix == "" {
			return seg
		}
		return prefix + "." + seg
	}
	switch node := v.(type) {
	case map[string]any:
		for k, sub := range node {
			flattenInto(sub, key(k), out)
		}
	case []any:
		var scalars []string
		for i, sub := range node {
			switch sub.(type) {
			case map[string]any, []any:
				flattenInto(sub, key(strconv.Itoa(i)), out)
			default:
				if s := scalarText(sub); s != "" {
					scalars = append(scalars, s)
				}
			}
		}
		if len(scalars) > 0 && prefix != "" {
			out[prefix] = strings.Join(scalars, ", ")
		}
	default:
		if s := scalarText(node); s != "" && prefix != "" {
			out[prefix] = s
		}
	}
}

// scalarText 标量转文本。
func scalarText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		if b, err := json.Marshal(x); err == nil {
			return string(b)
		}
		return ""
	}
}
