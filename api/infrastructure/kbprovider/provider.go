// Package kbprovider 知识库数据源连接器。
//
// 每种外部内容来源一个 Provider 实现（web_crawl/database/api_pull/object_storage/
// external_kb），统一走 Sync 把远端内容以「标题+正文」逐条交给 ingest 回调；
// 入库管线（清洗/分块/嵌入/去重/upsert）在 service 层（kb_datasource_service）统一装配，
// 本包只负责「取数」与「进度计数」。
//
// 约定：
//   - cfg 为该类型私有配置（aa_kb_datasource.config jsonb 解码后的 map）；
//   - ingest 回调由 service 层提供（title/content/meta → aa_kb 文档，upsert 语义）；
//   - Provider 内不得吞错：结构性失败（连不上/配置错）返回 error 使整次 run 落 failed，
//     单条内容问题（非文本/空内容/解析失败）计数 skipped 后继续。
package kbprovider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// IngestFunc 单条条目入库回调（service 层装配）。
// meta 带定位键与摘要：key（外部键，URL/行主键/对象 Key…，upsert 依据）、summary（摘要，可空）。
type IngestFunc func(title, content string, meta map[string]any) error

// Provider 数据源连接器统一接口。
type Provider interface {
	// Sync 拉取远端内容并逐条 ingest。返回 error = 整次 run 失败。
	Sync(ctx context.Context, cfg map[string]any, ingest IngestFunc, run *RunReporter) error
}

// RunReporter run 进度计数（并发安全）。计数名开放（pages/rows/objects 由各连接器
// 按来源口径自定；ingested/updated/unchanged 由 service 层 ingest 回调统计）；
// notes 记关键进度备注（非文本跳过、翻页位置等），随 stats jsonb 一并落 run 记录。
type RunReporter struct {
	mu    sync.Mutex
	count map[string]int64
	notes []string
}

// NewRunReporter 新建计数器。
func NewRunReporter() *RunReporter {
	return &RunReporter{count: map[string]int64{}}
}

// Inc 累加计数。
func (r *RunReporter) Inc(name string, n int64) {
	r.mu.Lock()
	r.count[name] += n
	r.mu.Unlock()
}

// Count 读计数。
func (r *RunReporter) Count(name string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count[name]
}

// Note 追加一条进度备注（时间线顺序保留）。
func (r *RunReporter) Note(format string, args ...any) {
	r.mu.Lock()
	r.notes = append(r.notes, fmt.Sprintf(format, args...))
	r.mu.Unlock()
}

// Snapshot 计数快照（落库用；返回副本）。
func (r *RunReporter) Snapshot() map[string]int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int64, len(r.count))
	for k, v := range r.count {
		out[k] = v
	}
	return out
}

// Notes 备注快照（返回副本）。
func (r *RunReporter) Notes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.notes...)
}

// ---- 连接器注册表 ----

var registry = map[string]Provider{}

// register 连接器自注册（各实现文件 init 里调）。
func register(dsType string, p Provider) {
	registry[dsType] = p
}

// For 取对应类型的连接器。
func For(dsType string) (Provider, error) {
	p, ok := registry[dsType]
	if !ok {
		return nil, fmt.Errorf("未知数据源类型：%s", dsType)
	}
	return p, nil
}

// Types 已注册类型名（排序稳定，供校验/展示）。
func Types() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- 配置读取助手（各连接器共用；jsonb 解码后的 map 值类型不齐，统一在此收敛） ----

// cfgString 取字符串配置（缺省 ''）。
func cfgString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	switch v := cfg[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64: // 数字容错转字符串（jsonb 里写裸数字的配置项）
		s := strings.TrimSuffix(fmt.Sprintf("%v", v), ".0")
		return s
	default:
		return ""
	}
}

// cfgStrings 取字符串数组配置：接受 JSON 数组或逗号/换行分隔的字符串。
func cfgStrings(cfg map[string]any, key string) []string {
	if cfg == nil {
		return nil
	}
	var raw []string
	switch v := cfg[key].(type) {
	case []any:
		for _, it := range v {
			if s, ok := it.(string); ok {
				raw = append(raw, s)
			}
		}
	case []string:
		raw = v
	case string:
		raw = strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '\n' || r == ';' })
	}
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// cfgInt 取整数配置（缺省/非法回默认值）。
func cfgInt(cfg map[string]any, key string, def int) int {
	if cfg == nil {
		return def
	}
	switch v := cfg[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// cfgBool 取布尔配置（缺省回默认值；接受 bool/字符串）。
func cfgBool(cfg map[string]any, key string, def bool) bool {
	if cfg == nil {
		return def
	}
	switch v := cfg[key].(type) {
	case bool:
		return v
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "true" || s == "1" || s == "yes" || s == "on" {
			return true
		}
		if s == "false" || s == "0" || s == "no" || s == "off" {
			return false
		}
	}
	return def
}

// emit 统一发布单条内容：空内容跳过计数（connectors 侧口径），title 缺省回落 key。
// 返回是否真的调用了 ingest（false = 已按 skipped 计）。
func emit(ingest IngestFunc, run *RunReporter, key, title, summary, content string) (bool, error) {
	if strings.TrimSpace(content) == "" {
		run.Inc("skipped", 1)
		run.Note("空内容跳过：%s", key)
		return false, nil
	}
	if strings.TrimSpace(title) == "" {
		title = key
	}
	meta := map[string]any{"key": key}
	if summary != "" {
		meta["summary"] = summary
	}
	if err := ingest(title, content, meta); err != nil {
		return false, err
	}
	return true, nil
}
