package entity

import (
	"encoding/json"

	"aiarsenal/domain/types"
)

// 安全审计状态机：PENDING → SCANNING → PASSED | FAILED | ERROR。
// 只有 PASSED（安全）的版本可发布/安装（service 层 ensureSkillVersionSafe 门禁）。
const (
	AuditStatusPending  uint = 0 // 待扫描（审计记录已建，尚未开始）
	AuditStatusPassed   uint = 1 // 安全（扫描完成，无高危发现）
	AuditStatusFailed   uint = 2 // 不安全（有 high 及以上发现）
	AuditStatusScanning uint = 3 // 扫描中
	AuditStatusError    uint = 4 // 扫描失败（扫描器自身出错/panic，非内容结论）
)

// SkillSecurityAudit 对应表 aa_skill_security_audit（安全扫描结果）。
type SkillSecurityAudit struct {
	Id             uint            `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AuditId        string          `gorm:"column:audit_id;uniqueIndex" json:"auditId"`
	SkillVersionId string          `gorm:"column:skill_version_id" json:"skillVersionId"`
	Status         uint            `gorm:"column:status" json:"status"`     // 见 AuditStatus* 常量（0~4）
	Severity       uint            `gorm:"column:severity" json:"severity"` // 0~4
	FindingsJson   json.RawMessage `gorm:"column:findings_json;type:jsonb" json:"findingsJson,omitempty"`
	ScannerRaw     string          `gorm:"column:scanner_raw" json:"scannerRaw"`
	ScannedAt      *types.Time     `gorm:"column:scanned_at" json:"scannedAt"`
	CreatedAt      *types.Time     `gorm:"column:created_at" json:"createdAt"`
}

func (SkillSecurityAudit) TableName() string { return "aa_skill_security_audit" }
