package types

import (
	"encoding/json"
	"testing"
)

func TestMilliRoundtrip(t *testing.T) {
	cases := []struct {
		f    float64 // 主币浮点（JSON 载荷单位）
		want Milli
	}{
		{0, 0},
		{0.1091, 1091},   // 0.109136 类历史 spend 精度足够
		{2.5, 25000},     // $2.5/M 单价
		{400, 4_000_000}, // $400 预算
		{0.0001, 1},      // 最小单位 1 毫
		{123.456789, 1_234_568}, // 最近舍入（.789 → 8）
	}
	for _, c := range cases {
		m := MilliFromFloat(c.f)
		if m != c.want {
			t.Fatalf("MilliFromFloat(%v) = %d, want %d", c.f, m, c.want)
		}
		// Marshal → 主币浮点 → Unmarshal → 原值（JSON 边界无损往返）。
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("Marshal(%d): %v", m, err)
		}
		var back Milli
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("Unmarshal(%s): %v", b, err)
		}
		if back != m {
			t.Fatalf("roundtrip %d → %s → %d", m, b, back)
		}
	}
}

func TestMilliUnmarshalForms(t *testing.T) {
	var m Milli
	// 整数主币输入：JSON 数值一律按主币解读（1091 = $1091）
	if err := json.Unmarshal([]byte("1091"), &m); err != nil || m != 10_910_000 {
		t.Fatalf("int form: %v %d", err, m)
	}
	// 浮点主币输入
	if err := json.Unmarshal([]byte("0.1091"), &m); err != nil || m != 1091 {
		t.Fatalf("float form: %v %d", err, m)
	}
	// 非数值报错
	if err := json.Unmarshal([]byte(`"abc"`), &m); err == nil {
		t.Fatal("string form should error")
	}
}

// 指针字段（*Milli，对应可空金额列）同样走值 receiver 的 Marshal。
func TestMilliPointerMarshal(t *testing.T) {
	v := Milli(25000)
	b, err := json.Marshal(&v)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "2.5" {
		t.Fatalf("got %s, want 2.5", b)
	}
}
