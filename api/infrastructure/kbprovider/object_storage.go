package kbprovider

// object_storage 连接器：最小 S3 SigV4 客户端（零第三方依赖，约 200 行）。
// ListObjectsV2 翻页列举 + GetObject 拉对象，文本对象入库，非文本扩展名跳过计数。
//
// config:
//
//	endpoint    string   例 https://s3.amazonaws.com 或 https://minio.internal:9000（必填）
//	region      string   区域（默认 us-east-1；MinIO 等自建任意值即可）
//	bucket      string   存储桶（必填，path-style：https://<endpoint-host>/<bucket>/<key>）
//	prefix      string   对象 Key 前缀（可空）
//	access_key  string   AK（必填）
//	secret_key  string   SK（必填）
//	extensions  []string 入库扩展名白名单（默认 .txt,.md,.markdown,.csv,.log,.json）
//	max_objects int      最多处理对象数（默认 1000，上限 10000）
//
// 仅文本对象入库；其余对象（含 .pdf 等二进制）列举到即计 skipped——PDF 抽取属
// 文件管线（KbExtractText），走本连接器的对象内容没有 IngestData 的字节入口，不硬造。

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// init 自注册。
func init() { register("object_storage", &ObjectStorage{}) }

// ObjectStorage S3 兼容对象存储连接器（SigV4）。
type ObjectStorage struct{}

// s3TextDefaults 默认入库扩展名。
var s3TextDefaults = []string{".txt", ".md", ".markdown", ".csv", ".log", ".json"}

// Sync 实现 Provider。
func (p *ObjectStorage) Sync(ctx context.Context, cfg map[string]any, ingest IngestFunc, run *RunReporter) error {
	endpoint := cfgString(cfg, "endpoint")
	bucket := cfgString(cfg, "bucket")
	ak := cfgString(cfg, "access_key")
	sk := cfgString(cfg, "secret_key")
	if endpoint == "" || bucket == "" || ak == "" || sk == "" {
		return errors.New("object_storage：缺少 endpoint/bucket/access_key/secret_key 配置")
	}
	region := cfgString(cfg, "region")
	if region == "" {
		region = "us-east-1"
	}
	prefix := cfgString(cfg, "prefix")
	exts := cfgStrings(cfg, "extensions")
	if len(exts) == 0 {
		exts = s3TextDefaults
	}
	allowExt := map[string]bool{}
	for _, e := range exts {
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		allowExt[strings.ToLower(e)] = true
	}
	maxObjects := cfgInt(cfg, "max_objects", 1000)
	if maxObjects <= 0 {
		maxObjects = 1000
	}
	if maxObjects > 10000 {
		maxObjects = 10000
	}

	host, basePath, err := s3Endpoint(endpoint)
	if err != nil {
		return err
	}
	objectRoot := strings.TrimSuffix(basePath, "/") + "/" + bucket // path-style 根

	client := &http.Client{Timeout: 60 * time.Second}
	done := errors.New("max_objects") // 用哨兵跳出翻页（不作为失败上抛）

	listErr := func() error {
		token := ""
		for page := 0; ; page++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			q := url.Values{}
			q.Set("list-type", "2")
			if prefix != "" {
				q.Set("prefix", prefix)
			}
			q.Set("max-keys", "1000")
			if token != "" {
				q.Set("continuation-token", token)
			}
			body, err := s3Do(ctx, client, host, objectRoot+"/", q, http.MethodGet, ak, sk, region, nil)
			if err != nil {
				return fmt.Errorf("列举失败：%w", err)
			}
			var res s3ListResult
			if err := xml.Unmarshal(body, &res); err != nil {
				return fmt.Errorf("列举响应解析失败：%w", err)
			}
			for _, obj := range res.Contents {
				if err := ctx.Err(); err != nil {
					return err
				}
				if run.Count("objects") >= int64(maxObjects) {
					return done
				}
				run.Inc("objects", 1)
				ext := s3Ext(obj.Key)
				if !allowExt[ext] || obj.Size == 0 {
					run.Inc("skipped", 1)
					continue
				}
				data, err := s3Do(ctx, client, host, objectRoot+"/"+escapeS3Path(obj.Key), nil, http.MethodGet, ak, sk, region, nil)
				if err != nil {
					run.Inc("skipped", 1)
					run.Note("对象拉取失败 %s：%v", obj.Key, err)
					continue
				}
				title := s3Title(obj.Key)
				if _, err := emit(ingest, run, obj.Key, title, "", string(data)); err != nil {
					return err
				}
			}
			if !res.IsTruncated || res.NextContinuationToken == "" {
				return nil
			}
			token = res.NextContinuationToken
		}
	}()
	if listErr != nil && listErr != done {
		return listErr
	}
	return nil
}

// ---- 最小 S3 客户端（ListObjectsV2 + GetObject，SigV4 头签名） ----

// s3ListResult ListObjectsV2 响应（只取需要的字段）。
type s3ListResult struct {
	IsTruncated           bool `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
	Contents              []struct {
		Key  string `xml:"Key"`
		Size int64  `xml:"Size"`
	} `xml:"Contents"`
}

// s3Endpoint 解析 endpoint → (host, 基路径)。host 形如 s3.amazonaws.com[:port]，基路径给代理子路径。
func s3Endpoint(endpoint string) (string, string, error) {
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return "", "", fmt.Errorf("object_storage：endpoint 非法：%s", endpoint)
	}
	return strings.ToLower(u.Host), strings.TrimSuffix(u.Path, "/"), nil
}

// s3Do 发一个 SigV4 签名请求并返回 2xx 响应体。
func s3Do(ctx context.Context, client *http.Client, host, rawPath string, query url.Values, method, ak, sk, region string, body []byte) ([]byte, error) {
	u := &url.URL{Scheme: "https", Host: host, Path: rawPath, RawQuery: encodeS3Query(query)}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	payloadHash := sha256Hex(nil) // GET 无体；固定空载荷哈希
	req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	auth := SignSigV4(method, rawPath, u.RawQuery, req.Header, payloadHash, ak, sk, region, "s3")
	req.Header.Set("Authorization", auth)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d：%s", resp.StatusCode, truncateErrText(data))
	}
	return data, nil
}

// SignSigV4 计算 AWS Signature Version 4 请求签名（Authorization 头整串）。
// 纯函数（时间取 headers 里的 X-Amz-Date），对象存储实现与单测共用。
func SignSigV4(method, canonicalPath, canonicalQuery string, headers http.Header, payloadHash, accessKey, secretKey, region, service string) string {
	amzDate := headers.Get("X-Amz-Date")
	dateStamp := amzDate
	if len(dateStamp) > 8 {
		dateStamp = dateStamp[:8]
	}
	// 规范头：小写名 + trim 值，按名排序；host 必带
	signed := map[string]string{"host": headers.Get("Host")}
	for _, h := range []string{"x-amz-date", "x-amz-content-sha256", "content-type", "range", "content-md5"} {
		if v := headers.Get(h); v != "" {
			signed[h] = strings.TrimSpace(v)
		}
	}
	names := make([]string, 0, len(signed))
	for k := range signed {
		names = append(names, k)
	}
	sort.Strings(names)
	var ch strings.Builder
	for _, k := range names {
		ch.WriteString(k + ":" + signed[k] + "\n")
	}
	signedHeaders := strings.Join(names, ";")

	canonicalRequest := strings.Join([]string{
		method, canonicalPath, canonicalQuery, ch.String(), signedHeaders, payloadHash,
	}, "\n")
	scope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, scope, sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	key := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	key = hmacSHA256(key, []byte(region))
	key = hmacSHA256(key, []byte(service))
	key = hmacSHA256(key, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(key, []byte(stringToSign)))
	return "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + scope +
		",SignedHeaders=" + signedHeaders + ",Signature=" + signature
}

// encodeS3Query 规范化查询串：按名排序，值按 AWS 规则 URI 编码（空格 %20、斜杠 %2F）。
func encodeS3Query(q url.Values) string {
	if len(q) == 0 {
		return ""
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		kk := uriEscape(k, false)
		for _, v := range q[k] {
			parts = append(parts, kk+"="+uriEscape(v, false))
		}
	}
	return strings.Join(parts, "&")
}

// escapeS3Path 对象 Key 路径编码：逐段 URI 编码（保留 '/'）。
func escapeS3Path(key string) string {
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = uriEscape(s, true)
	}
	return strings.Join(segs, "/")
}

// uriEscape AWS 规则 URI 编码：仅 [A-Za-z0-9-_.~] 不转义；keepSlash 时 '/' 原样保留。
func uriEscape(s string, keepSlash bool) string {
	const hexDigits = "0123456789ABCDEF"
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
			c == '-' || c == '_' || c == '.' || c == '~':
			sb.WriteByte(c)
		case c == '/' && keepSlash:
			sb.WriteByte(c)
		default:
			sb.WriteByte('%')
			sb.WriteByte(hexDigits[c>>4])
			sb.WriteByte(hexDigits[c&0x0f])
		}
	}
	return sb.String()
}

// sha256Hex 空载荷/任意字节的 SHA-256 十六进制。
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hmacSHA256 单轮 HMAC。
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// s3Ext 对象 Key 的扩展名（小写，带点）。
func s3Ext(key string) string {
	name := key
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		return strings.ToLower(name[i:])
	}
	return ""
}

// s3Title 对象 Key → 文档标题（去前缀目录与扩展名）。
func s3Title(key string) string {
	name := key
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSuffix(name, ".")
}

// truncateErrText 错误响应体截断（防超长 XML 刷屏日志）。
func truncateErrText(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
