package entity

import "time"

// AlertRecord 对应表 aa_alert_record：安全事件告警的落库记录（等保三级 8.1.5.4f
// 「应对安全事件进行识别、报警」+ 8.1.4.3c 审计记录保护的组合要求）。
//
// 合规口径：外送（delivered 通道投递）可经 config 关闭/节流，**落库不可关闭**——
// 本表没有删除/清空端点，留存随库（≥180 天由运维备份策略保障，网安法 21 条）。
// 表结构变更只能走迁移，禁止应用层 DROP/TRUNCATE。
type AlertRecord struct {
	Id        uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	EventType string    `gorm:"column:event_type;size:50;not null" json:"eventType"` // login_lockout / insecure_defaults / log_tamper_attempt / redis_unavailable / migration_failure
	Severity  string    `gorm:"column:severity;size:20;not null" json:"severity"`    // info / warning / error
	Title     string    `gorm:"column:title;size:200;not null" json:"title"`
	Body      string    `gorm:"column:body;type:text" json:"body"`
	BizKey    string    `gorm:"column:biz_key;size:200" json:"bizKey"` // 节流键（如锁定的用户名）；空=不节流
	Delivered bool      `gorm:"column:delivered;not null" json:"delivered"`
	Retries   int       `gorm:"column:retries;not null" json:"retries"`
	LastError string    `gorm:"column:last_error;type:text" json:"lastError"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

func (AlertRecord) TableName() string { return "aa_alert_record" }
