package model

import (
	"encoding/json"
	"fmt"
	"math"
)

// Milli 金额整数类型：1 Milli = 1 毫 = 万分之一主币单位（$0.0001）。
// 网关与 api 是两个独立 Go module，api 侧同构实现在 api/domain/types/milli.go——
// 两侧语义必须保持一致（改任一侧须同步）。JSON 边界自动换算回主币（redis 缓存
// VirtualKey/Budget 走 JSON 序列化，Marshal/Unmarshal 对称保证缓存往返无损）。
type Milli int64

// MilliFromFloat 主币浮点 → 毫（最近舍入）。
func MilliFromFloat(v float64) Milli { return Milli(math.Round(v * 10000)) }

// Float 毫 → 主币浮点（展示单位）。
func (m Milli) Float() float64 { return float64(m) / 10000 }

// MarshalJSON 对外/缓存输出主币浮点（如 1091 毫 → 0.1091）。
func (m Milli) MarshalJSON() ([]byte, error) { return json.Marshal(m.Float()) }

// UnmarshalJSON JSON 数值一律按主币解读（与 Marshal 对称：400 = $400），
// 最近舍入转毫（兼容旧缓存里的浮点金额）。
func (m *Milli) UnmarshalJSON(b []byte) error {
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		*m = MilliFromFloat(f)
		return nil
	}
	return fmt.Errorf("model.Milli: invalid number %s", b)
}

// CostMilli 整数成本：输入/输出 tokens × 单价（毫/百万 tokens），
// +500_000 再除 1_000_000 即四舍五入到毫——全程整数运算，无浮点误差。
// 单价毫值足够大也不会溢出：tokens(1e7) × 单价(1e9 毫=$10 万/M) = 1e16 < int64 上限。
func CostMilli(promptTokens, completionTokens int, inputPerM, outputPerM Milli) Milli {
	return Milli((int64(promptTokens)*int64(inputPerM) +
		int64(completionTokens)*int64(outputPerM) + 500_000) / 1_000_000)
}
