package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"aiarsenal/domain/entity"
	"aiarsenal/infrastructure/db"
	"aiarsenal/infrastructure/idgen"
)

type CalendarSyncRepository struct{ db *gorm.DB }

func NewCalendarSyncRepository() *CalendarSyncRepository {
	return &CalendarSyncRepository{db: db.ScheduleDB()}
}

func (r *CalendarSyncRepository) ListAccounts(ctx context.Context, ownerUserId string) ([]*entity.CalendarSyncAccount, error) {
	var out []*entity.CalendarSyncAccount
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ? and deleted_at is null", ownerUserId).
		Order("created_at desc, id desc").Find(&out).Error
	return out, err
}

func (r *CalendarSyncRepository) ListEnabledAccounts(ctx context.Context) ([]*entity.CalendarSyncAccount, error) {
	var out []*entity.CalendarSyncAccount
	err := r.db.WithContext(ctx).
		Where("enabled = 1 and deleted_at is null").
		Order("id asc").Find(&out).Error
	return out, err
}

func (r *CalendarSyncRepository) GetAccount(ctx context.Context, accountId string) (*entity.CalendarSyncAccount, error) {
	var out entity.CalendarSyncAccount
	if err := r.db.WithContext(ctx).
		Where("account_id = ? and deleted_at is null", accountId).
		First(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *CalendarSyncRepository) AddAccount(ctx context.Context, account *entity.CalendarSyncAccount) (string, error) {
	account.AccountId = idgen.New()
	now := nowPtr()
	account.CreatedAt = now
	account.UpdatedAt = now
	if account.LastSyncStatus == "" {
		account.LastSyncStatus = "idle"
	}
	if err := r.db.WithContext(ctx).Create(account).Error; err != nil {
		return "", err
	}
	return account.AccountId, nil
}

func (r *CalendarSyncRepository) EditAccount(ctx context.Context, account *entity.CalendarSyncAccount) error {
	account.UpdatedAt = nowPtr()
	updates := map[string]any{
		"name":                  account.Name,
		"server_url":            account.ServerUrl,
		"calendar_url":          account.CalendarUrl,
		"username":              account.Username,
		"enabled":               account.Enabled,
		"sync_interval_minutes": account.SyncIntervalMinutes,
		"conflict_policy":       account.ConflictPolicy,
		"updated_at":            account.UpdatedAt,
	}
	if account.PasswordEnc != "" {
		updates["password_enc"] = account.PasswordEnc
	}
	return r.db.WithContext(ctx).Model(&entity.CalendarSyncAccount{}).
		Where("account_id = ? and deleted_at is null", account.AccountId).
		Updates(updates).Error
}

func (r *CalendarSyncRepository) DeleteAccount(ctx context.Context, accountId string) error {
	return r.db.WithContext(ctx).Model(&entity.CalendarSyncAccount{}).
		Where("account_id = ? and deleted_at is null", accountId).
		Update("deleted_at", time.Now()).Error
}

func (r *CalendarSyncRepository) DetachAccount(ctx context.Context, accountId string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entity.ScheduleEvent{}).
			Where("source_account_id = ?", accountId).
			Updates(map[string]any{"source_account_id": "", "external_uid": ""}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ?", accountId).Delete(&entity.CalendarSyncLink{}).Error; err != nil {
			return err
		}
		return tx.Model(&entity.CalendarSyncAccount{}).
			Where("account_id = ? and deleted_at is null", accountId).
			Update("deleted_at", time.Now()).Error
	})
}

func (r *CalendarSyncRepository) UpdateAccountStatus(ctx context.Context, accountId, status, lastError string) error {
	return r.db.WithContext(ctx).Model(&entity.CalendarSyncAccount{}).
		Where("account_id = ? and deleted_at is null", accountId).
		Updates(map[string]any{
			"last_sync_at":     time.Now(),
			"last_sync_status": status,
			"last_sync_error":  lastError,
			"updated_at":       time.Now(),
		}).Error
}

func (r *CalendarSyncRepository) ListLinks(ctx context.Context, accountId string) ([]*entity.CalendarSyncLink, error) {
	var out []*entity.CalendarSyncLink
	err := r.db.WithContext(ctx).Where("account_id = ?", accountId).Order("id asc").Find(&out).Error
	return out, err
}

func (r *CalendarSyncRepository) GetLinkByEvent(ctx context.Context, accountId, eventId string) (*entity.CalendarSyncLink, error) {
	var out entity.CalendarSyncLink
	if err := r.db.WithContext(ctx).
		Where("account_id = ? and event_id = ?", accountId, eventId).
		First(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *CalendarSyncRepository) GetLinkByHref(ctx context.Context, accountId, href string) (*entity.CalendarSyncLink, error) {
	var out entity.CalendarSyncLink
	if err := r.db.WithContext(ctx).
		Where("account_id = ? and remote_href = ?", accountId, href).
		First(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *CalendarSyncRepository) UpsertLink(ctx context.Context, link *entity.CalendarSyncLink) error {
	var existing entity.CalendarSyncLink
	err := r.db.WithContext(ctx).
		Where("account_id = ? and (event_id = ? or remote_href = ?)", link.AccountId, link.EventId, link.RemoteHref).
		First(&existing).Error
	now := nowPtr()
	if err == nil {
		link.LinkId = existing.LinkId
		link.CreatedAt = existing.CreatedAt
		link.UpdatedAt = now
		return r.db.WithContext(ctx).Model(&entity.CalendarSyncLink{}).
			Where("link_id = ?", existing.LinkId).
			Updates(map[string]any{
				"event_id":         link.EventId,
				"remote_href":      link.RemoteHref,
				"remote_etag":      link.RemoteEtag,
				"remote_uid":       link.RemoteUid,
				"local_updated_at": link.LocalUpdatedAt,
				"exceptions_hash":  link.ExceptionsHash,
				"conflict":         link.Conflict,
				"conflict_payload": link.ConflictPayload,
				"last_synced_at":   link.LastSyncedAt,
				"updated_at":       link.UpdatedAt,
			}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	link.LinkId = idgen.New()
	link.CreatedAt = now
	link.UpdatedAt = now
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *CalendarSyncRepository) DeleteLink(ctx context.Context, linkId string) error {
	return r.db.WithContext(ctx).Where("link_id = ?", linkId).Delete(&entity.CalendarSyncLink{}).Error
}

func (r *CalendarSyncRepository) ListCalendarEvents(ctx context.Context, ownerUserId, accountId string) ([]*entity.ScheduleEvent, error) {
	var out []*entity.ScheduleEvent
	err := r.db.WithContext(ctx).
		Where("organizer_id = ? and (source_account_id = '' or source_account_id = ?)", ownerUserId, accountId).
		Order("id asc").Find(&out).Error
	return out, err
}

func (r *CalendarSyncRepository) GetEventIncludingDeleted(ctx context.Context, eventId string) (*entity.ScheduleEvent, error) {
	var out entity.ScheduleEvent
	if err := r.db.WithContext(ctx).Where("event_id = ?", eventId).First(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *CalendarSyncRepository) SoftDeleteEvent(ctx context.Context, eventId string) error {
	return r.db.WithContext(ctx).Model(&entity.ScheduleEvent{}).
		Where("event_id = ? and deleted_at is null", eventId).
		Update("deleted_at", time.Now()).Error
}
