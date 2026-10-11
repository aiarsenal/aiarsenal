package kbprovider

// kbprovider 最小单测：SigV4 签名串（AWS 官方测试向量）、HTML 抽正文、JSON 展平。

import (
	"net/http"
	"strings"
	"testing"
)

// TestSignSigV4AWSVector AWS SigV4 官方文档测试向量（GET 对象，带 Range 头）。
// 参照 https://docs.aws.amazon.com/zh_cn/AmazonS3/latest/API/sig-v4-header-based-auth.html
func TestSignSigV4AWSVector(t *testing.T) {
	payloadHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	h := http.Header{}
	h.Set("Host", "examplebucket.s3.amazonaws.com")
	h.Set("X-Amz-Date", "20130524T000000Z")
	h.Set("X-Amz-Content-Sha256", payloadHash)
	h.Set("Range", "bytes=0-9")

	got := SignSigV4(http.MethodGet, "/test.txt", "", h, payloadHash,
		"AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "s3")

	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request" +
		",SignedHeaders=host;range;x-amz-content-sha256;x-amz-date" +
		",Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got != want {
		t.Fatalf("签名串不符\n got: %s\nwant: %s", got, want)
	}
}

// TestSignSigV4SignedHeaders 无 Range 头时 SignedHeaders 只含 host/x-amz-*。
func TestSignSigV4SignedHeaders(t *testing.T) {
	payloadHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	h := http.Header{}
	h.Set("Host", "examplebucket.s3.amazonaws.com")
	h.Set("X-Amz-Date", "20130524T000000Z")
	h.Set("X-Amz-Content-Sha256", payloadHash)

	got := SignSigV4(http.MethodGet, "/", "", h, payloadHash,
		"AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "s3")
	if !strings.Contains(got, "SignedHeaders=host;x-amz-content-sha256;x-amz-date,") {
		t.Fatalf("SignedHeaders 不符：%s", got)
	}
	if !strings.Contains(got, "Signature=") {
		t.Fatalf("缺 Signature：%s", got)
	}
}

// TestParseHTMLPage 抽正文：title/meta description 入标题摘要，script/style 剪枝，
// 块级元素换行边界。
func TestParseHTMLPage(t *testing.T) {
	page := []byte(`<!doctype html><html><head>
<title>接入指南</title>
<meta name="description" content="如何接入知识库">
<script>var x = "noise";</script>
<style>.a { color: red; }</style>
</head><body>
<div>
  <p>第一步，创建数据源。</p>
  <p>第二步，配置同步计划。</p>
  <a href="/docs/api">API 文档</a>
  <span>行内文本并入相邻块</span>
</div>
<script>var y = "noise2";</script>
</body></html>`)

	title, summary, text, links := parseHTMLPage(page, "https://docs.example.com/guide")
	if title != "接入指南" {
		t.Fatalf("标题不符：%q", title)
	}
	if summary != "如何接入知识库" {
		t.Fatalf("摘要不符：%q", summary)
	}
	if !strings.Contains(text, "第一步，创建数据源。") || !strings.Contains(text, "第二步，配置同步计划。") {
		t.Fatalf("正文缺段落：%q", text)
	}
	if strings.Contains(text, "noise") || strings.Contains(text, "color: red") {
		t.Fatalf("script/style 未剪枝：%q", text)
	}
	if strings.Contains(text, "行内文本并入相邻块") == false && strings.TrimSpace(text) == "" {
		t.Fatalf("正文为空：%q", text)
	}
	if len(links) != 1 || links[0] != "/docs/api" {
		t.Fatalf("链接抽取不符：%v", links)
	}
}

// TestFlattenJSON 递归展平：对象拼点路径、数组标量连接、嵌套对象带下标。
func TestFlattenJSON(t *testing.T) {
	data := map[string]any{
		"id":    float64(42),
		"name":  "工单",
		"tags":  []any{"紧急", "线上"},
		"extra": map[string]any{"owner": "张三", "level": float64(3)},
	}
	kv := FlattenJSON(data, "")
	if kv["name"] != "工单" || kv["id"] != "42" {
		t.Fatalf("标量展平不符：%v", kv)
	}
	if kv["tags"] != "紧急, 线上" {
		t.Fatalf("数组标量展平不符：%v", kv)
	}
	if kv["extra.owner"] != "张三" || kv["extra.level"] != "3" {
		t.Fatalf("嵌套对象展平不符：%v", kv)
	}

	text := flattenJSONText(data)
	if !strings.Contains(text, "extra.owner: 张三") || !strings.Contains(text, "tags: 紧急, 线上") {
		t.Fatalf("展平文本不符：%q", text)
	}
	// 幂等：同一数据两次展平结果一致
	if text != flattenJSONText(data) {
		t.Fatal("展平结果不幂等")
	}
}

// TestRegistryTypes 连接器注册齐全。
func TestRegistryTypes(t *testing.T) {
	want := []string{"api_pull", "database", "external_kb", "object_storage", "web_crawl"}
	got := Types()
	if len(got) != len(want) {
		t.Fatalf("注册类型数不符：%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("注册类型不符：%v（期望 %v）", got, want)
		}
	}
	if _, err := For("web_crawl"); err != nil {
		t.Fatalf("web_crawl 未注册：%v", err)
	}
	if _, err := For("nope"); err == nil {
		t.Fatal("未知类型应报错")
	}
}

// TestEmitSkip 空内容跳过计数。
func TestEmitSkip(t *testing.T) {
	run := NewRunReporter()
	var got []string
	called, err := emit(func(title, content string, meta map[string]any) error {
		got = append(got, title)
		return nil
	}, run, "k1", "标题", "", "  ")
	if err != nil || called {
		t.Fatalf("空内容应跳过：called=%v err=%v", called, err)
	}
	if run.Count("skipped") != 1 {
		t.Fatalf("skipped 计数不符：%d", run.Count("skipped"))
	}
	if _, err := emit(func(title, content string, meta map[string]any) error { return nil },
		run, "k2", "", "", "正文"); err != nil {
		t.Fatalf("合法条目不应报错：%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("不应有标题被记录：%v", got)
	}
}
