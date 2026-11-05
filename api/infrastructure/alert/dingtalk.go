package alert

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// dingtalk.go — 钉钉机器人自实现 sender（PLAN §三拍板：算法冻结、消费面小、零依赖优于引库，
// 与 TOTP/SM3 前例同口径）。加签安全设置：HMAC-SHA256(secret, "<毫秒时间戳>\n<secret>")
// 取 base64，urlencode 后附在 query；钉钉服务端允许 ±1 小时时间戳偏移，本实现取当前时刻。

// dingtalkEndpoint 包级变量仅为测试可替换（httptest 回显）；生产固定官方端点。
var dingtalkEndpoint = "https://oapi.dingtalk.com/robot/send"

// SignDingTalk 计算钉钉加签参数。毫秒时间戳 + secret → urlencode(base64(hmac-sha256))。
// 独立导出以便国标式向量锁单测（固定 secret+时间戳断言签名结果）。
func SignDingTalk(secret string, timestampMs int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestampMs, secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

// dingtalkPayload 钉钉 text 消息体。
type dingtalkPayload struct {
	MsgType string       `json:"msgtype"`
	Text    dingtalkText `json:"text"`
	At      *dingtalkAt  `json:"at,omitempty"`
}

type dingtalkText struct {
	Content string `json:"content"`
}

type dingtalkAt struct {
	IsAtAll bool `json:"isAtAll"`
}

// sendDingTalk 向钉钉机器人投递一条 text 消息。非 2xx 或 errcode!=0 视为失败（触发重试）。
func sendDingTalk(token, secret, title, body string, client *http.Client) error {
	if token == "" {
		return fmt.Errorf("dingtalk: access_token 为空")
	}
	ts := time.Now().UnixMilli()
	endpoint := fmt.Sprintf("%s?access_token=%s&timestamp=%d&sign=%s",
		dingtalkEndpoint, url.QueryEscape(token), ts, SignDingTalk(secret, ts))
	payload, err := json.Marshal(dingtalkPayload{
		MsgType: "text",
		Text:    dingtalkText{Content: title + "\n" + body},
	})
	if err != nil {
		return fmt.Errorf("dingtalk: marshal payload: %w", err)
	}
	cl := client
	if cl == nil {
		cl = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := cl.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		// 凭据防泄漏（审计 B1）：*url.Error 的 Error() 带完整请求 URL——access_token 与
		// sign 就在 query 里。剥掉外壳只留底层原因，错误串才能进日志与 last_error。
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("dingtalk: post: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("dingtalk: decode response (http %d): %w", resp.StatusCode, err)
	}
	if out.ErrCode != 0 {
		return fmt.Errorf("dingtalk: errcode=%d errmsg=%s", out.ErrCode, out.ErrMsg)
	}
	return nil
}

// realSend 按配置构造真实外送函数：钉钉（自实现）+ shoutrrr 通道列表逐一投递。
// 任一通道失败即返回合并错误触发重试；全部为空时返回 nil（无通道=无外送，不算失败）。
func realSend(cfg Config) sendFunc {
	return func(title, body string) error {
		var errs []error
		if cfg.DingTalkToken != "" {
			if err := sendDingTalk(cfg.DingTalkToken, cfg.DingTalkSecret, title, body, nil); err != nil {
				errs = append(errs, err)
			}
		}
		for _, rawURL := range cfg.Channels {
			if err := sendShoutrrr(rawURL, title+"\n"+body); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) == 0 {
			return nil
		}
		for _, err := range errs {
			slog.Warn("告警通道失败", "err", err)
		}
		return errs[0]
	}
}
