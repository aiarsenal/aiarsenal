package main

import (
	"encoding/json"
	"os"
)

// aa.json 路径：manager 配置，声明本镜像支持的 agent + 版本（随 PVC 持久化，镜像构建时 COPY seed）。
const aaConfigPath = "/aa/.aa/aa.json"

// AAConfig /aa/.aa/aa.json 结构。support 每条是 [agent, version] 二元数组：
//
//	openclaw 镜像: {"support":[["openclaw","2026.7.1-2"]]}
//	hermes  镜像: {"support":[["hermes","0.19.0"]]}
//	common  镜像（未来，支持任意 agent）: support 列多个 或 留空（按 AIARSENAL_RUNTIME 选）。
type AAConfig struct {
	Support [][2]string `json:"support"`
}

// loadAAConfig 读 aa.json。文件缺失/解析失败返回错误（调用方按 env 兜底）。
func loadAAConfig(path string) (*AAConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c AAConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
