package compression

import (
	"encoding/json"
	"strings"
)

// 工具结果压缩服务（规则式压缩，降低大结果对上下文的占用）。
// 对超长文本工具输出做截断+摘要，降低回传 AI 客户端的 token 量。
// 配置存于 mcp 库 aa_mcp_config.tool_result_compression（jsonb）。

type Config struct {
	Enabled   bool `json:"enabled"`
	MaxLength int  `json:"maxLength"` // 超过此长度（字符）则压缩，0=不限
}

type Service struct {
	cfg *Config
}

func New() *Service { return &Service{cfg: &Config{Enabled: false}} }

func (s *Service) ApplyConfig(jsonStr string) {
	if jsonStr == "" || jsonStr == "{}" {
		s.cfg = &Config{Enabled: false}
		return
	}
	var c Config
	if err := json.Unmarshal([]byte(jsonStr), &c); err == nil {
		s.cfg = &c
	}
}

func (s *Service) Compress(text string) string {
	if s.cfg == nil || !s.cfg.Enabled || s.cfg.MaxLength <= 0 || len(text) <= s.cfg.MaxLength {
		return text
	}
	half := s.cfg.MaxLength / 2
	var b strings.Builder
	b.WriteString(text[:half])
	b.WriteString("\n\n...[compressed by mcp gateway, original length=")
	b.WriteString(itoa(len(text)))
	b.WriteString("]...\n\n")
	b.WriteString(text[len(text)-min(half, 256):])
	return b.String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
