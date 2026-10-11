package entity

import "aiarsenal/domain/types"

// SkillIdempotencyRecord 对应表 aa_skill_idempotency_record（幂等记录，PK=request_id）。
// 用于发布/上传等接口的幂等重放保护。
type SkillIdempotencyRecord struct {
	Id                 uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	RequestId          string      `gorm:"column:request_id;primaryKey" json:"requestId"`
	ResourceType       string      `gorm:"column:resource_type" json:"resourceType"`
	ResourceId         string      `gorm:"column:resource_id" json:"resourceId"`
	Status             string      `gorm:"column:status" json:"status"`
	ResponseStatusCode int         `gorm:"column:response_status_code" json:"responseStatusCode"`
	CreatedAt          *types.Time `gorm:"column:created_at" json:"createdAt"`
	ExpiresAt          *types.Time `gorm:"column:expires_at" json:"expiresAt"`
}

func (SkillIdempotencyRecord) TableName() string { return "aa_skill_idempotency_record" }
