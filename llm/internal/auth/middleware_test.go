package auth

import (
	"encoding/json"
	"testing"

	"aiarsenal/llm/internal/model"
)

// vkeyCache 缓存序列化必须保住 token_hash：直接 Marshal VirtualKey 会因
// json:"-" 丢字段，缓存命中回读 TokenHash 恒为空（历史上因此产生过一批
// api_key 为空的 spend_log / key 维度日聚合行）。
func TestVkeyCacheRoundTripKeepsTokenHash(t *testing.T) {
	k := &model.VirtualKey{KeyAlias: "instance:abc:hermes", UserId: "u1", Spend: model.Milli(12_500)} // $1.25 → 1.25 万毫
	k.TokenHash = "deadbeef"

	b, err := json.Marshal(vkeyCache{VirtualKey: *k, TokenHash: k.TokenHash})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var c vkeyCache
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := c.VirtualKey
	got.TokenHash = c.TokenHash

	if got.TokenHash != k.TokenHash {
		t.Fatalf("token_hash 缓存回读丢失: got %q want %q", got.TokenHash, k.TokenHash)
	}
	if got.KeyAlias != k.KeyAlias || got.UserId != k.UserId || got.Spend != k.Spend {
		t.Fatalf("其余字段回读不一致: %+v", got)
	}

	// 反证：直接 Marshal VirtualKey 不含 token_hash（json:"-" 防外泄，包装结构的理由）。
	raw, _ := json.Marshal(*k)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal 本体: %v", err)
	}
	if _, ok := m["token_hash"]; ok {
		t.Fatalf("VirtualKey 本体 JSON 不应包含 token_hash（防外泄），却出现了: %s", raw)
	}
}
