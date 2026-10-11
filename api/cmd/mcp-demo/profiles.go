package main

// 15 个演示 server 画像：`--name demo-xxx`（stdio 子进程）或 http `/mcp/demo-xxx`（路径）
// 各暴露独立实例，供分组管理页测试选成员与网关聚合。工具行为全部本地合成，无外部依赖；
// 随机量按输入（城市/符号/IP 等）哈希种子生成——同输入同输出，演示可复现。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math/big"
	mathrand "math/rand"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// demoArg 工具参数（schema 用；required=false 时可缺省）
type demoArg struct {
	name     string
	desc     string
	required bool
}

// demoTool 表驱动的演示工具：fn 收解包后的参数 map，返回文本结果或错误
type demoTool struct {
	name string
	desc string
	args []demoArg
	fn   func(a map[string]any) (string, error)
}

// demoProfile 一个逻辑演示 server（name 即注册名 / URL 段）
type demoProfile struct {
	name  string
	desc  string
	tools []demoTool
}

// buildProfileServer 按画像构造独立 mcp.Server（低阶 AddTool：schema 手写、参数不校验，handler 自取）
func buildProfileServer(p *demoProfile) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "aiarsenal-demo-" + p.name, Version: "1.0"}, nil)
	for _, t := range p.tools {
		t := t
		props := map[string]any{}
		var required []string
		for _, a := range t.args {
			props[a.name] = map[string]any{"type": "string", "description": a.desc}
			if a.required {
				required = append(required, a.name)
			}
		}
		schema := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			schema["required"] = required
		}
		srv.AddTool(&mcp.Tool{Name: t.name, Description: t.desc, InputSchema: schema},
			func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				var a map[string]any
				if len(req.Params.Arguments) > 0 {
					if err := json.Unmarshal(req.Params.Arguments, &a); err != nil {
						return nil, fmt.Errorf("参数解析失败：%w", err)
					}
				}
				out, err := t.fn(a)
				if err != nil {
					return nil, err
				}
				return textResult(out), nil
			})
	}
	return srv
}

// findProfile 按注册名取画像（未知名字返回 nil）
func findProfile(name string) *demoProfile {
	for i := range demoProfiles {
		if demoProfiles[i].name == name {
			return &demoProfiles[i]
		}
	}
	return nil
}

// ===== 参数取值助手（缺省/类型不符回落默认值，演示工具不 strict） =====

func argStr(m map[string]any, k, def string) string {
	if v, ok := m[k].(string); ok && v != "" {
		return v
	}
	return def
}

func argInt(m map[string]any, k string, def int) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case string:
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func argFloat(m map[string]any, k string, def float64) float64 {
	switch v := m[k].(type) {
	case float64:
		return v
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%g", &f); err == nil {
			return f
		}
	}
	return def
}

// seededRand 同一输入同一随机序列（演示数据可复现）
func seededRand(key string) *mathrand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return mathrand.New(mathrand.NewSource(int64(h.Sum64())))
}

// randHex n 字节随机十六进制（真实 crypto 随机，供 uuid/etag 类）
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = cryptoRand(b)
	return hex.EncodeToString(b)
}

func cryptoRand(b []byte) (int, error) { return rand.Read(b) }

// fakeUUID v4 形态演示 UUID（version/variant 位按 RFC 4122 写死）
func fakeUUID() string {
	h := randHex(16)
	return fmt.Sprintf("%s-%s-4%s-%c%s-%s", h[0:8], h[8:12], h[13:16],
		"89ab"[mathrand.Intn(4)], h[17:20], h[20:32])
}

var weatherCond = []string{"晴", "多云", "阴", "小雨", "阵雨", "雷阵雨", "小雪", "雾"}
var weekNames = []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
var exchangeRates = map[string]float64{ // 演示用固定牌价（以 CNY 为基准）
	"CNY": 1, "USD": 7.16, "EUR": 7.78, "JPY": 0.048, "HKD": 0.92, "GBP": 9.05,
}

// demoProfiles 15 个画像目录（注册名与种子 SQL 一一对应）
var demoProfiles = []demoProfile{
	{
		name: "demo-weather", desc: "天气查询演示（合成数据，同城市结果可复现）",
		tools: []demoTool{
			{name: "weather", desc: "查询城市当日天气（演示合成数据）", args: []demoArg{{"city", "城市名", true}},
				fn: func(a map[string]any) (string, error) {
					city := argStr(a, "city", "北京")
					r := seededRand("weather:" + city)
					lo := 8 + r.Intn(14)
					return fmt.Sprintf("【演示数据】%s 今天%s，%d~%d℃，湿度 %d%%，%s %d 级",
						city, weatherCond[r.Intn(len(weatherCond))], lo, lo+6+r.Intn(6),
						40+r.Intn(50), []string{"东风", "东南风", "南风", "西南风", "北风"}[r.Intn(5)], 1+r.Intn(6)), nil
				}},
			{name: "forecast", desc: "查询城市未来 N 天预报（演示合成数据）", args: []demoArg{{"city", "城市名", true}, {"days", "天数 1-7，默认 3", false}},
				fn: func(a map[string]any) (string, error) {
					city := argStr(a, "city", "北京")
					days := argInt(a, "days", 3)
					if days < 1 || days > 7 {
						return "", fmt.Errorf("days 超出范围 1-7：%d", days)
					}
					r := seededRand("forecast:" + city)
					var b strings.Builder
					fmt.Fprintf(&b, "【演示数据】%s 未来 %d 天：\n", city, days)
					for i := range days {
						lo := 8 + r.Intn(14)
						fmt.Fprintf(&b, "  D+%d %s %d~%d℃\n", i+1, weatherCond[r.Intn(len(weatherCond))], lo, lo+6+r.Intn(6))
					}
					return strings.TrimRight(b.String(), "\n"), nil
				}},
		},
	},
	{
		name: "demo-search", desc: "网页搜索演示（合成结果，无真实检索）",
		tools: []demoTool{
			{name: "search", desc: "搜索关键词，返回演示结果列表", args: []demoArg{{"query", "搜索关键词", true}, {"limit", "条数 1-10，默认 5", false}},
				fn: func(a map[string]any) (string, error) {
					q := argStr(a, "query", "")
					if q == "" {
						return "", fmt.Errorf("query 不能为空")
					}
					n := argInt(a, "limit", 5)
					if n < 1 || n > 10 {
						return "", fmt.Errorf("limit 超出范围 1-10：%d", n)
					}
					var b strings.Builder
					fmt.Fprintf(&b, "【演示数据】“%s” 的 %d 条结果：\n", q, n)
					for i := range n {
						fmt.Fprintf(&b, "  %d. %s——%s 相关的演示条目 %d（demo-search.example/%s/%d）\n",
							i+1, q, q, i+1, strings.ReplaceAll(q, " ", "-"), i+1)
					}
					return strings.TrimRight(b.String(), "\n"), nil
				}},
		},
	},
	{
		name: "demo-translate", desc: "文本翻译演示（加注目标语言标签，原文返回——无真实模型）",
		tools: []demoTool{
			{name: "translate", desc: "给文本加目标语言标签后原样返回（演示）", args: []demoArg{{"text", "待翻译文本", true}, {"target", "目标语言：en | ja | ko | fr | de，默认 en", false}},
				fn: func(a map[string]any) (string, error) {
					text := argStr(a, "text", "")
					if text == "" {
						return "", fmt.Errorf("text 不能为空")
					}
					target := argStr(a, "target", "en")
					switch target {
					case "en", "ja", "ko", "fr", "de":
					default:
						return "", fmt.Errorf("不支持的 target：%s（en | ja | ko | fr | de）", target)
					}
					return fmt.Sprintf("【演示→%s】%s（demo-translate 不含真实模型，仅演示管线）", target, text), nil
				}},
		},
	},
	{
		name: "demo-calc", desc: "数学计算演示（四则 + 区间随机数）",
		tools: []demoTool{
			{name: "calc", desc: "简单四则运算；除零返回错误（演示失败路径）", args: []demoArg{{"a", "左操作数（数字）", true}, {"op", "运算符：add | sub | mul | div", true}, {"b", "右操作数（数字）", true}},
				fn: func(a map[string]any) (string, error) {
					x, y := argFloat(a, "a", 0), argFloat(a, "b", 0)
					op := argStr(a, "op", "")
					var v float64
					switch op {
					case "add":
						v = x + y
					case "sub":
						v = x - y
					case "mul":
						v = x * y
					case "div":
						if y == 0 {
							return "", fmt.Errorf("除零：%v / %v", x, y)
						}
						v = x / y
					default:
						return "", fmt.Errorf("不支持的 op：%s（add | sub | mul | div）", op)
					}
					return fmt.Sprintf("%g %s %g = %g", x, op, y, v), nil
				}},
			{name: "random", desc: "生成区间内随机整数（crypto 随机）", args: []demoArg{{"min", "下界，默认 0", false}, {"max", "上界（含），默认 100", false}},
				fn: func(a map[string]any) (string, error) {
					lo, hi := argInt(a, "min", 0), argInt(a, "max", 100)
					if hi < lo {
						return "", fmt.Errorf("max 小于 min：%d < %d", hi, lo)
					}
					v, err := rand.Int(rand.Reader, big.NewInt(int64(hi-lo+1)))
					if err != nil {
						return "", err
					}
					return fmt.Sprintf("%d", lo+int(v.Int64())), nil
				}},
		},
	},
	{
		name: "demo-uuid", desc: "ID 生成演示（crypto 随机 v4 形态）",
		tools: []demoTool{
			{name: "uuid", desc: "批量生成 v4 形态 UUID", args: []demoArg{{"count", "个数 1-50，默认 1", false}},
				fn: func(a map[string]any) (string, error) {
					n := argInt(a, "count", 1)
					if n < 1 || n > 50 {
						return "", fmt.Errorf("count 超出范围 1-50：%d", n)
					}
					ids := make([]string, n)
					for i := range ids {
						ids[i] = fakeUUID()
					}
					return strings.Join(ids, "\n"), nil
				}},
		},
	},
	{
		name: "demo-qrcode", desc: "二维码演示（返回占位元数据，不生成真实图片）",
		tools: []demoTool{
			{name: "qrcode", desc: "生成二维码占位描述（尺寸/容错/内容摘要）", args: []demoArg{{"text", "编码内容", true}, {"size", "边长像素 64-1024，默认 256", false}},
				fn: func(a map[string]any) (string, error) {
					text := argStr(a, "text", "")
					if text == "" {
						return "", fmt.Errorf("text 不能为空")
					}
					size := argInt(a, "size", 256)
					if size < 64 || size > 1024 {
						return "", fmt.Errorf("size 超出范围 64-1024：%d", size)
					}
					return fmt.Sprintf("【演示】%d×%d 二维码占位（内容 %d 字节，容错 M，etag %s）——demo-qrcode 不输出真实图片",
						size, size, len(text), randHex(8)), nil
				}},
		},
	},
	{
		name: "demo-datetime", desc: "日期时间演示（当前时间/星期推算）",
		tools: []demoTool{
			{name: "now", desc: "返回当前时间（动态结果）", args: []demoArg{{"format", "输出格式：datetime | date | time | unix，默认 datetime", false}},
				fn: func(a map[string]any) (string, error) {
					now := time.Now()
					switch argStr(a, "format", "datetime") {
					case "", "datetime":
						return now.Format("2006-01-02 15:04:05 -0700 MST"), nil
					case "date":
						return now.Format("2006-01-02"), nil
					case "time":
						return now.Format("15:04:05"), nil
					case "unix":
						return fmt.Sprintf("%d", now.Unix()), nil
					default:
						return "", fmt.Errorf("不支持的 format（datetime | date | time | unix）")
					}
				}},
			{name: "weekday", desc: "推算日期是星期几（2006-01-02）", args: []demoArg{{"date", "日期，默认今天", false}},
				fn: func(a map[string]any) (string, error) {
					s := argStr(a, "date", time.Now().Format("2006-01-02"))
					t, err := time.Parse("2006-01-02", s)
					if err != nil {
						return "", fmt.Errorf("日期格式应为 2006-01-02：%s", s)
					}
					return fmt.Sprintf("%s 是 %s", s, weekNames[t.Weekday()]), nil
				}},
		},
	},
	{
		name: "demo-ip", desc: "IP 归属地演示（按 IP 哈希合成地区/运营商）",
		tools: []demoTool{
			{name: "lookup", desc: "查询 IP 归属（演示合成数据）", args: []demoArg{{"ip", "IPv4 地址，缺省 203.0.113.1", false}},
				fn: func(a map[string]any) (string, error) {
					ip := argStr(a, "ip", "203.0.113.1")
					r := seededRand("ip:" + ip)
					return fmt.Sprintf("【演示数据】%s → %s（运营商：%s）", ip,
						[]string{"北京", "上海", "广州", "深圳", "杭州", "成都", "新加坡", "东京"}[r.Intn(8)],
						[]string{"电信", "联通", "移动"}[r.Intn(3)]), nil
				}},
		},
	},
	{
		name: "demo-exchange", desc: "汇率换算演示（固定演示牌价，以 CNY 为基准）",
		tools: []demoTool{
			{name: "rate", desc: "查询两币种演示牌价", args: []demoArg{{"from", "源币种代码，默认 CNY", false}, {"to", "目标币种代码，默认 USD", false}},
				fn: func(a map[string]any) (string, error) {
					from, to := strings.ToUpper(argStr(a, "from", "CNY")), strings.ToUpper(argStr(a, "to", "USD"))
					f, okf := exchangeRates[from]
					t, okt := exchangeRates[to]
					if !okf || !okt {
						return "", fmt.Errorf("不支持的币种：%s/%s（支持 CNY USD EUR JPY HKD GBP）", from, to)
					}
					return fmt.Sprintf("1 %s ≈ %.4f %s（演示固定牌价）", from, t/f, to), nil
				}},
			{name: "convert", desc: "按演示牌价换算金额", args: []demoArg{{"amount", "金额（数字）", true}, {"from", "源币种，默认 CNY", false}, {"to", "目标币种，默认 USD", false}},
				fn: func(a map[string]any) (string, error) {
					from, to := strings.ToUpper(argStr(a, "from", "CNY")), strings.ToUpper(argStr(a, "to", "USD"))
					f, okf := exchangeRates[from]
					t, okt := exchangeRates[to]
					if !okf || !okt {
						return "", fmt.Errorf("不支持的币种：%s/%s（支持 CNY USD EUR JPY HKD GBP）", from, to)
					}
					return fmt.Sprintf("%g %s ≈ %.2f %s（演示固定牌价）", argFloat(a, "amount", 0), from, argFloat(a, "amount", 0)*t/f, to), nil
				}},
		},
	},
	{
		name: "demo-stock", desc: "行情演示（按代码哈希合成价格与走势）",
		tools: []demoTool{
			{name: "quote", desc: "查询标的演示行情", args: []demoArg{{"symbol", "标的代码，如 AAPL/600519", true}},
				fn: func(a map[string]any) (string, error) {
					sym := argStr(a, "symbol", "")
					if sym == "" {
						return "", fmt.Errorf("symbol 不能为空")
					}
					r := seededRand("stock:" + sym)
					base := 10 + r.Float64()*490
					return fmt.Sprintf("【演示数据】%s 现价 %.2f（%.2f%%）成交量 %d 万手", sym, base,
						(r.Float64()-0.5)*6, 100+r.Intn(900)), nil
				}},
			{name: "trend", desc: "查询标的近 N 日演示走势", args: []demoArg{{"symbol", "标的代码", true}, {"days", "天数 1-30，默认 7", false}},
				fn: func(a map[string]any) (string, error) {
					sym := argStr(a, "symbol", "")
					if sym == "" {
						return "", fmt.Errorf("symbol 不能为空")
					}
					days := argInt(a, "days", 7)
					if days < 1 || days > 30 {
						return "", fmt.Errorf("days 超出范围 1-30：%d", days)
					}
					r := seededRand("trend:" + sym)
					p := 10 + r.Float64()*490
					var b strings.Builder
					fmt.Fprintf(&b, "【演示数据】%s 近 %d 日收盘：\n", sym, days)
					day := time.Now().AddDate(0, 0, -days)
					for range days {
						p *= 1 + (r.Float64()-0.5)*0.04
						fmt.Fprintf(&b, "  %s %.2f\n", day.Format("01-02"), p)
						day = day.AddDate(0, 0, 1)
					}
					return strings.TrimRight(b.String(), "\n"), nil
				}},
		},
	},
	{
		name: "demo-mail", desc: "邮件发送演示（生成消息号，不真实投递）",
		tools: []demoTool{
			{name: "send", desc: "登记一封演示邮件（返回 message-id）", args: []demoArg{{"to", "收件地址", true}, {"subject", "主题", true}},
				fn: func(a map[string]any) (string, error) {
					to, subj := argStr(a, "to", ""), argStr(a, "subject", "")
					if to == "" || subj == "" {
						return "", fmt.Errorf("to/subject 不能为空")
					}
					return fmt.Sprintf("【演示】已登记发往 %s 的邮件“%s”，message-id <%s@demo-mail>（不会真实投递）",
						to, subj, randHex(12)), nil
				}},
		},
	},
	{
		name: "demo-storage", desc: "对象存储演示（合成 etag 与对象列表）",
		tools: []demoTool{
			{name: "put", desc: "登记一个演示对象（返回 etag）", args: []demoArg{{"key", "对象键名", true}},
				fn: func(a map[string]any) (string, error) {
					key := argStr(a, "key", "")
					if key == "" {
						return "", fmt.Errorf("key 不能为空")
					}
					return fmt.Sprintf("【演示】已写入 %s（etag \"%s\"，%d 字节）", key, randHex(16), 64+seededRand(key).Intn(4096)), nil
				}},
			{name: "list", desc: "按前缀列演示对象", args: []demoArg{{"prefix", "键名前缀，默认空", false}},
				fn: func(a map[string]any) (string, error) {
					pre := argStr(a, "prefix", "")
					r := seededRand("list:" + pre)
					var b strings.Builder
					fmt.Fprintf(&b, "【演示】前缀“%s”下 %d 个对象：\n", pre, 3)
					for i := range 3 {
						fmt.Fprintf(&b, "  %sobj-%d.bin（%d 字节）\n", pre, i+1, 128+r.Intn(8192))
					}
					return strings.TrimRight(b.String(), "\n"), nil
				}},
		},
	},
	{
		name: "demo-notify", desc: "消息通知演示（返回合成送达回执）",
		tools: []demoTool{
			{name: "send", desc: "向渠道发演示通知（返回回执 id）", args: []demoArg{{"channel", "渠道名：im | sms | webhook，默认 im", false}, {"text", "通知内容", true}},
				fn: func(a map[string]any) (string, error) {
					text := argStr(a, "text", "")
					if text == "" {
						return "", fmt.Errorf("text 不能为空")
					}
					ch := argStr(a, "channel", "im")
					switch ch {
					case "im", "sms", "webhook":
					default:
						return "", fmt.Errorf("不支持的 channel：%s（im | sms | webhook）", ch)
					}
					return fmt.Sprintf("【演示】%s 通知已入队（回执 ntf_%s，内容 %d 字）", ch, randHex(10), len([]rune(text))), nil
				}},
		},
	},
	{
		name: "demo-git", desc: "代码仓库演示（合成提交与分支）",
		tools: []demoTool{
			{name: "commits", desc: "列仓库最近演示提交", args: []demoArg{{"repo", "仓库名 owner/name", true}, {"limit", "条数 1-20，默认 5", false}},
				fn: func(a map[string]any) (string, error) {
					repo := argStr(a, "repo", "")
					if repo == "" {
						return "", fmt.Errorf("repo 不能为空")
					}
					n := argInt(a, "limit", 5)
					if n < 1 || n > 20 {
						return "", fmt.Errorf("limit 超出范围 1-20：%d", n)
					}
					msgs := []string{"fix: 修复边界条件", "feat: 新增导出能力", "refactor: 抽公共逻辑", "test: 补齐单测", "docs: 更新说明", "chore: 升级依赖"}
					r := seededRand("commits:" + repo)
					var b strings.Builder
					fmt.Fprintf(&b, "【演示数据】%s 最近 %d 条提交：\n", repo, n)
					for i := range n {
						fmt.Fprintf(&b, "  %s %s（demo-user，%d 小时前）\n", randHex(7), msgs[r.Intn(len(msgs))], i*3+r.Intn(3)+1)
					}
					return strings.TrimRight(b.String(), "\n"), nil
				}},
			{name: "branches", desc: "列仓库演示分支", args: []demoArg{{"repo", "仓库名 owner/name", true}},
				fn: func(a map[string]any) (string, error) {
					repo := argStr(a, "repo", "")
					if repo == "" {
						return "", fmt.Errorf("repo 不能为空")
					}
					return fmt.Sprintf("【演示数据】%s 分支：main（默认）、dev、release/1.0、feature/demo-%s", repo, randHex(4)), nil
				}},
		},
	},
	{
		name: "demo-text", desc: "文本处理演示（大小写/反转/slug 与统计）",
		tools: []demoTool{
			{name: "transform", desc: "文本变形：upper | lower | reverse | slug", args: []demoArg{{"text", "输入文本", true}, {"mode", "变形模式，默认 upper", false}},
				fn: func(a map[string]any) (string, error) {
					text := argStr(a, "text", "")
					if text == "" {
						return "", fmt.Errorf("text 不能为空")
					}
					switch argStr(a, "mode", "upper") {
					case "upper":
						return strings.ToUpper(text), nil
					case "lower":
						return strings.ToLower(text), nil
					case "reverse":
						runes := []rune(text)
						for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
							runes[i], runes[j] = runes[j], runes[i]
						}
						return string(runes), nil
					case "slug":
						lower := strings.ToLower(text)
						return strings.Join(strings.Fields(lower), "-"), nil
					default:
						return "", fmt.Errorf("不支持的 mode（upper | lower | reverse | slug）")
					}
				}},
			{name: "count", desc: "统计字符数/单词数/行数", args: []demoArg{{"text", "输入文本", true}},
				fn: func(a map[string]any) (string, error) {
					text := argStr(a, "text", "")
					return fmt.Sprintf("字符 %d（含空白）/ 单词 %d / 行数 %d", len(text), len(strings.Fields(text)), strings.Count(text, "\n")+1), nil
				}},
		},
	},
}
