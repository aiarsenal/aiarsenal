package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// 令牌交换（网关→ops 第二跳身份透传）的单源实现：
// 网关（interfaces/gateway）按归属用户签发，ops（interfaces/ops）验签后按该用户
// 五元组收紧数据可见性。载荷字段序即签名基准——两侧共用本文件，杜绝各写一份 HMAC
// 后字段序漂移。

// ActorClaims 令牌交换载荷（json 字段序即签名基准，勿调整字段顺序）。
type ActorClaims struct {
	KeyId string `json:"keyId"`
	Owner string `json:"owner"`
	Kind  string `json:"kind"`
	Exp   int64  `json:"exp"`
}

// actorTTL 载荷有效期：短窗防重放（网关每请求现签，60s 只需覆盖网关→ops 一跳）。
const actorTTL = 60 * time.Second

// SignActor 身份载荷 HMAC-SHA256 签名。payload 明文随请求头透传，sig 为 hex 签名。
func SignActor(signKey, keyId, owner, kind string, now time.Time) (payload, sig string, err error) {
	b, err := json.Marshal(ActorClaims{
		KeyId: keyId, Owner: owner, Kind: kind, Exp: now.Add(actorTTL).Unix(),
	})
	if err != nil {
		return "", "", err
	}
	m := hmac.New(sha256.New, []byte(signKey))
	m.Write(b)
	return string(b), hex.EncodeToString(m.Sum(nil)), nil
}

// VerifyActor 验签 + 解析：HMAC 常时比较、exp 未过、Owner 非空（无归属不构成身份）。
// 任一不满足返回 false，调用方回落系统身份判定。
func VerifyActor(signKey, payload, sig string) (ActorClaims, bool) {
	if payload == "" || sig == "" || signKey == "" {
		return ActorClaims{}, false
	}
	m := hmac.New(sha256.New, []byte(signKey))
	m.Write([]byte(payload))
	if !hmac.Equal([]byte(hex.EncodeToString(m.Sum(nil))), []byte(sig)) {
		return ActorClaims{}, false
	}
	var c ActorClaims
	if err := json.Unmarshal([]byte(payload), &c); err != nil || c.Exp < time.Now().Unix() || c.Owner == "" {
		return ActorClaims{}, false
	}
	return c, true
}
