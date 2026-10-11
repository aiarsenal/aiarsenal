package service

import (
	"encoding/json"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// EmbeddingProvider 文本→向量。默认 LocalEmbedding（hashing trick，离线可跑）；
// 接入真实模型时替换为实现（如调用 OpenAI/本地 embedding API）。
type EmbeddingProvider interface {
	Embed(text string) ([]float32, error)
	Dim() int
}

// LocalEmbedding 离线嵌入：分词 → token 哈希到固定维 → TF 加权 → L2 归一化。
// 不是真语义，但能按词项重叠做相似度检索；结构与真模型一致，便于平滑替换。
type LocalEmbedding struct{ DimN int }

func NewLocalEmbedding(dim int) *LocalEmbedding { return &LocalEmbedding{DimN: dim} }
func (e *LocalEmbedding) Dim() int              { return e.DimN }

func (e *LocalEmbedding) Embed(text string) ([]float32, error) {
	vec := make([]float32, e.DimN)
	for _, tok := range tokenize(text) {
		h := fnv.New32a()
		h.Write([]byte(tok))
		idx := int(h.Sum32()) % e.DimN
		vec[idx] += 1
	}
	// L2 归一化
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range vec {
			vec[i] = float32(float64(vec[i]) / norm)
		}
	}
	return vec, nil
}

// tokenize 切词：英文按 [a-z0-9]+，中文按单字。
func tokenize(s string) []string {
	s = strings.ToLower(s)
	var toks []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			toks = append(toks, b.String())
			b.Reset()
		}
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r > 0x4e00 && r < 0x9fff: // CJK
			flush()
			toks = append(toks, string(r))
		case unicode.IsLetter(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			flush()
		}
	}
	flush()
	return toks
}

// VectorToPGText 把向量格式化成 pgvector 文本形：[0.1,0.2,...]
func VectorToPGText(v []float32) string {
	b, _ := json.Marshal(v)
	return string(b)
}
