package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// API 与 aiarsenal 控制面通信的 HTTP 客户端。
type API struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewAPI(baseURL, token string) *API {
	return &API{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Heartbeat POST /api/v1/agent/runtime/heartbeat
func (a *API) Heartbeat(req *HeartbeatReq) (*HeartbeatRes, error) {
	body, _ := json.Marshal(req)
	var res HeartbeatRes
	if err := a.do("POST", "/api/v1/agent/runtime/heartbeat", body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
func (a *API) do(method, path string, body []byte, out interface{}) error {
	url := a.baseURL + path
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, string(raw))
	}
	return json.Unmarshal(raw, out)
}

// GetBinary 用 instance token 鉴权 GET 拉取二进制（skill 整包 zip），返回全 body 字节。
// 单独的长超时客户端（a.http 默认 15s 偏紧）：整包可能较大、服务端现打包，给 120s。
func (a *API) GetBinary(path string) ([]byte, error) {
	url := a.baseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}
