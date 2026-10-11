package persistence

import (
	"time"

	"aiarsenal/domain/types"
)

const layout = "2006-01-02 15:04:05"

// timeToStr 把 types.Time 格式化为 "2006-01-02 15:04:05"（与旧 gtime JSON 一致）
func timeToStr(t types.Time) string {
	tt := time.Time(t)
	if tt.IsZero() {
		return ""
	}
	return tt.Format(layout)
}
