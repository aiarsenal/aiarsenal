package types

import (
	"encoding/json"
	"fmt"
	"math"
)

// Milli 金额整数类型：1 Milli = 1 毫 = 万分之一主币单位（$0.0001）。
// 金额的存储与计算全部走 int64 毫（无浮点累积误差）；JSON 边界自动换算回主币
// 浮点数（对外载荷单位不变，前端零改动），换算用最近舍入吸收二进制浮点尾差。
type Milli int64

// MilliFromFloat 主币浮点 → 毫（最近舍入）。
func MilliFromFloat(v float64) Milli { return Milli(math.Round(v * 10000)) }

// Float 毫 → 主币浮点（展示单位）。
func (m Milli) Float() float64 { return float64(m) / 10000 }

// MarshalJSON 对外输出主币浮点（如 1091 毫 → 0.1091）。
func (m Milli) MarshalJSON() ([]byte, error) { return json.Marshal(m.Float()) }

// UnmarshalJSON JSON 载荷数值一律按主币解读（与 Marshal 对称：400 = $400 = 400 万毫），
// 最近舍入转毫。
func (m *Milli) UnmarshalJSON(b []byte) error {
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		*m = MilliFromFloat(f)
		return nil
	}
	return fmt.Errorf("types.Milli: 非法数值 %s", b)
}
