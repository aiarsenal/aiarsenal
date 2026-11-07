package entity

import "aiarsenal/domain/types"

// CalendarSyncAccount 一个用户绑定一个远端 CalDAV 日历。
// PasswordEnc 使用 aiarsenal 的 AES-256-GCM 密钥加密，任何列表接口都不返回密文。
type CalendarSyncAccount struct {
	Id                  uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	AccountId           string      `gorm:"column:account_id;uniqueIndex" json:"accountId"`
	Name                string      `gorm:"column:name" json:"name"`
	Provider            string      `gorm:"column:provider" json:"provider"` // caldav
	ServerUrl           string      `gorm:"column:server_url" json:"serverUrl"`
	CalendarUrl         string      `gorm:"column:calendar_url" json:"calendarUrl"`
	Username            string      `gorm:"column:username" json:"username"`
	PasswordEnc         string      `gorm:"column:password_enc" json:"-"`
	Enabled             uint        `gorm:"column:enabled" json:"enabled"`
	SyncIntervalMinutes int         `gorm:"column:sync_interval_minutes" json:"syncIntervalMinutes"`
	ConflictPolicy      string      `gorm:"column:conflict_policy" json:"conflictPolicy"` // manual/local/remote
	OwnerUserId         string      `gorm:"column:owner_user_id" json:"ownerUserId"`
	LastSyncAt          *types.Time `gorm:"column:last_sync_at" json:"lastSyncAt"`
	LastSyncStatus      string      `gorm:"column:last_sync_status" json:"lastSyncStatus"`
	LastSyncError       string      `gorm:"column:last_sync_error" json:"lastSyncError"`
	CreatedAt           *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           *types.Time `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt           *types.Time `gorm:"column:deleted_at" json:"deletedAt"`
}

func (CalendarSyncAccount) TableName() string { return "aa_schedule_sync_account" }

// CalendarSyncLink 本地 event ↔ 远端 .ics 资源映射。
// LocalUpdatedAt / ExceptionsHash 是上次成功同步时的本地版本，用于判断同步后是否又被本地修改。
type CalendarSyncLink struct {
	Id              uint        `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	LinkId          string      `gorm:"column:link_id;uniqueIndex" json:"linkId"`
	AccountId       string      `gorm:"column:account_id" json:"accountId"`
	EventId         string      `gorm:"column:event_id" json:"eventId"`
	RemoteHref      string      `gorm:"column:remote_href" json:"remoteHref"`
	RemoteEtag      string      `gorm:"column:remote_etag" json:"remoteEtag"`
	RemoteUid       string      `gorm:"column:remote_uid" json:"remoteUid"`
	LocalUpdatedAt  *types.Time `gorm:"column:local_updated_at" json:"localUpdatedAt"`
	ExceptionsHash  string      `gorm:"column:exceptions_hash" json:"exceptionsHash"`
	Conflict        uint        `gorm:"column:conflict" json:"conflict"`
	ConflictPayload string      `gorm:"column:conflict_payload;type:text" json:"-"`
	LastSyncedAt    *types.Time `gorm:"column:last_synced_at" json:"lastSyncedAt"`
	CreatedAt       *types.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       *types.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (CalendarSyncLink) TableName() string { return "aa_schedule_sync_link" }
