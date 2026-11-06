package repository

import (
	"context"

	"aiarsenal/domain/entity"
)

type ICalendarSyncRepository interface {
	ListAccounts(ctx context.Context, ownerUserId string) ([]*entity.CalendarSyncAccount, error)
	ListEnabledAccounts(ctx context.Context) ([]*entity.CalendarSyncAccount, error)
	GetAccount(ctx context.Context, accountId string) (*entity.CalendarSyncAccount, error)
	AddAccount(ctx context.Context, account *entity.CalendarSyncAccount) (string, error)
	EditAccount(ctx context.Context, account *entity.CalendarSyncAccount) error
	DeleteAccount(ctx context.Context, accountId string) error
	DetachAccount(ctx context.Context, accountId string) error
	UpdateAccountStatus(ctx context.Context, accountId, status, lastError string) error

	ListLinks(ctx context.Context, accountId string) ([]*entity.CalendarSyncLink, error)
	GetLinkByEvent(ctx context.Context, accountId, eventId string) (*entity.CalendarSyncLink, error)
	GetLinkByHref(ctx context.Context, accountId, href string) (*entity.CalendarSyncLink, error)
	UpsertLink(ctx context.Context, link *entity.CalendarSyncLink) error
	DeleteLink(ctx context.Context, linkId string) error

	// ListCalendarEvents 包含软删事件，供同步判断本地墓碑是否需要推送到远端。
	ListCalendarEvents(ctx context.Context, ownerUserId, accountId string) ([]*entity.ScheduleEvent, error)
	GetEventIncludingDeleted(ctx context.Context, eventId string) (*entity.ScheduleEvent, error)
	SoftDeleteEvent(ctx context.Context, eventId string) error
}
