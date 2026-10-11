package types

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Time 兼容旧 gtime.Time 的 JSON 输出格式 "2006-01-02 15:04:05"，
// 同时可被 GORM 扫描/写入。null 时序列化为 null。
type Time time.Time

const layout = "2006-01-02 15:04:05"

func (t Time) MarshalJSON() ([]byte, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(tt.Format(layout))
}

func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if string(b) == "null" {
		return nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	tt, err := time.ParseInLocation(layout, s, time.Local)
	if err != nil {
		return err
	}
	*t = Time(tt)
	return nil
}

// GORM 支持
func (t Time) GormDataType() string { return "timestamp" }

// Value 供 gorm/database 写库：非零值转 time.Time，零值返回 nil（→ DB NULL）。
// 没有它，gorm 对非 nil 的 *types.Time 无法序列化写入（只能写 NULL），故 created_at 恒空。
func (t Time) Value() (driver.Value, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return nil, nil
	}
	return tt, nil
}

func (t *Time) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		return nil
	case time.Time:
		*t = Time(v)
		return nil
	case string:
		if v == "" {
			return nil
		}
		tt, err := time.ParseInLocation(layout, v, time.Local)
		if err != nil {
			return err
		}
		*t = Time(tt)
		return nil
	case []byte:
		return t.Scan(string(v))
	case sql.NullTime:
		if !v.Valid {
			return nil
		}
		*t = Time(v.Time)
		return nil
	}
	return nil
}
