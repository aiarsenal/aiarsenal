package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/repository"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/caldav"
	"aiarsenal/infrastructure/crypto"
	"aiarsenal/infrastructure/scheduler"
)

type CalendarSyncAccountInput struct {
	Name                string `json:"name"`
	ServerUrl           string `json:"serverUrl"`
	CalendarUrl         string `json:"calendarUrl"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	Enabled             uint   `json:"enabled"`
	SyncIntervalMinutes int    `json:"syncIntervalMinutes"`
	ConflictPolicy      string `json:"conflictPolicy"`
}

type CalendarSyncAccountView struct {
	*entity.CalendarSyncAccount
	ConflictCount int `json:"conflictCount"`
	EventCount    int `json:"eventCount"`
}

type CalendarSyncResult struct {
	Pulled        int      `json:"pulled"`
	Pushed        int      `json:"pushed"`
	Skipped       int      `json:"skipped"`
	DeletedLocal  int      `json:"deletedLocal"`
	DeletedRemote int      `json:"deletedRemote"`
	Conflicts     int      `json:"conflicts"`
	Message       string   `json:"message"`
	Warnings      []string `json:"warnings,omitempty"`
}

var errUnsupportedCalendarSyncItem = errors.New("不支持的 CalDAV 事件")

type CalendarSyncService struct {
	repo          repository.ICalendarSyncRepository
	eventRepo     repository.IScheduleEventRepository
	exceptionRepo repository.IScheduleExceptionRepository
	sch           *scheduler.Scheduler
	mu            sync.Mutex
	running       map[string]bool
}

func NewCalendarSyncService(repo repository.ICalendarSyncRepository, eventRepo repository.IScheduleEventRepository, exceptionRepo repository.IScheduleExceptionRepository) *CalendarSyncService {
	return &CalendarSyncService{
		repo: repo, eventRepo: eventRepo, exceptionRepo: exceptionRepo,
		running: map[string]bool{},
	}
}

func (s *CalendarSyncService) SetScheduler(sch *scheduler.Scheduler) { s.sch = sch }

func (s *CalendarSyncService) Discover(ctx context.Context, serverURL, username, password string) ([]caldav.Calendar, error) {
	if strings.TrimSpace(serverURL) == "" || strings.TrimSpace(username) == "" || password == "" {
		return nil, errors.New("CalDAV 地址、用户名和密码不能为空")
	}
	return caldav.NewClient(username, password).DiscoverCalendars(ctx, serverURL)
}

func (s *CalendarSyncService) ListAccounts(ctx context.Context, ownerUserId string) ([]*CalendarSyncAccountView, error) {
	accounts, err := s.repo.ListAccounts(ctx, ownerUserId)
	if err != nil {
		return nil, err
	}
	out := make([]*CalendarSyncAccountView, 0, len(accounts))
	for _, account := range accounts {
		links, _ := s.repo.ListLinks(ctx, account.AccountId)
		conflicts := 0
		for _, link := range links {
			if link.Conflict == 1 {
				conflicts++
			}
		}
		out = append(out, &CalendarSyncAccountView{
			CalendarSyncAccount: account,
			ConflictCount:       conflicts,
			EventCount:          len(links),
		})
	}
	return out, nil
}

func (s *CalendarSyncService) AddAccount(ctx context.Context, ownerUserId string, in CalendarSyncAccountInput) (string, error) {
	account, err := s.accountFromInput(in, ownerUserId, "")
	if err != nil {
		return "", err
	}
	if in.Password == "" {
		return "", errors.New("新增 CalDAV 账号时密码不能为空；iCloud 请使用 App 专用密码")
	}
	if err := caldav.NewClient(account.Username, in.Password).ProbeCalendar(ctx, account.CalendarUrl); err != nil {
		return "", err
	}
	account.PasswordEnc, err = crypto.Encrypt(in.Password, config.Get().DataEncryptKey())
	if err != nil {
		return "", err
	}
	accountId, err := s.repo.AddAccount(ctx, account)
	if err != nil {
		return "", err
	}
	s.registerAccount(account)
	return accountId, nil
}

func (s *CalendarSyncService) EditAccount(ctx context.Context, ownerUserId, accountId string, in CalendarSyncAccountInput) error {
	existing, err := s.ownedAccount(ctx, ownerUserId, accountId)
	if err != nil {
		return err
	}
	account, err := s.accountFromInput(in, ownerUserId, accountId)
	if err != nil {
		return err
	}
	account.PasswordEnc = existing.PasswordEnc
	password := in.Password
	if password == "" {
		password, err = decryptCalendarPassword(existing.PasswordEnc)
		if err != nil {
			return errors.New("原凭据解密失败，请重新输入密码")
		}
	} else {
		account.PasswordEnc, err = crypto.Encrypt(password, config.Get().DataEncryptKey())
		if err != nil {
			return err
		}
	}
	if err := caldav.NewClient(account.Username, password).ProbeCalendar(ctx, account.CalendarUrl); err != nil {
		return err
	}
	if err := s.repo.EditAccount(ctx, account); err != nil {
		return err
	}
	s.registerAccount(account)
	return nil
}

func (s *CalendarSyncService) DeleteAccount(ctx context.Context, ownerUserId, accountId string) error {
	if _, err := s.ownedAccount(ctx, ownerUserId, accountId); err != nil {
		return err
	}
	if s.sch != nil {
		s.sch.Unregister("calendar-sync-" + accountId)
	}
	return s.repo.DetachAccount(ctx, accountId)
}

func (s *CalendarSyncService) SyncAccount(ctx context.Context, ownerUserId, accountId string) (*CalendarSyncResult, error) {
	account, err := s.ownedAccount(ctx, ownerUserId, accountId)
	if err != nil {
		return nil, err
	}
	return s.syncAccount(ctx, account)
}

func (s *CalendarSyncService) RegisterSchedules() {
	if s.sch == nil {
		return
	}
	accounts, err := s.repo.ListEnabledAccounts(context.Background())
	if err != nil {
		slog.Warn("日历同步：读取定时账号失败", "err", err)
		return
	}
	for _, account := range accounts {
		s.registerAccount(account)
	}
}

func (s *CalendarSyncService) SyncAllEnabled(ctx context.Context) {
	accounts, err := s.repo.ListEnabledAccounts(ctx)
	if err != nil {
		slog.Warn("日历同步：读取启用账号失败", "err", err)
		return
	}
	for _, account := range accounts {
		if _, err := s.syncAccount(ctx, account); err != nil {
			slog.Warn("日历同步失败", "accountId", account.AccountId, "err", err)
		}
	}
}

func (s *CalendarSyncService) registerAccount(account *entity.CalendarSyncAccount) {
	if s.sch == nil || account.Enabled == 0 {
		if s.sch != nil {
			s.sch.Unregister("calendar-sync-" + account.AccountId)
		}
		return
	}
	spec := syncCron(account.SyncIntervalMinutes)
	if err := s.sch.Register("calendar-sync-"+account.AccountId, spec, func(ctx context.Context) {
		if _, err := s.syncAccount(ctx, account); err != nil {
			slog.Warn("日历定时同步失败", "accountId", account.AccountId, "err", err)
		}
	}); err != nil {
		slog.Warn("日历同步定时注册失败", "accountId", account.AccountId, "err", err)
	}
}

func syncCron(minutes int) string {
	if minutes <= 0 {
		minutes = 15
	}
	if minutes > 59 {
		return "0 */1 * * *"
	}
	return fmt.Sprintf("*/%d * * * *", minutes)
}

func (s *CalendarSyncService) ownedAccount(ctx context.Context, ownerUserId, accountId string) (*entity.CalendarSyncAccount, error) {
	account, err := s.repo.GetAccount(ctx, accountId)
	if err != nil || account.OwnerUserId != ownerUserId {
		return nil, errors.New("日历同步账号不存在")
	}
	return account, nil
}

func (s *CalendarSyncService) accountFromInput(in CalendarSyncAccountInput, ownerUserId, accountId string) (*entity.CalendarSyncAccount, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.ServerUrl = strings.TrimSpace(in.ServerUrl)
	in.CalendarUrl = strings.TrimSpace(in.CalendarUrl)
	in.Username = strings.TrimSpace(in.Username)
	if in.Name == "" || in.ServerUrl == "" || in.CalendarUrl == "" || in.Username == "" {
		return nil, errors.New("账号名称、CalDAV 地址、日历地址和用户名不能为空")
	}
	if _, err := url.ParseRequestURI(in.ServerUrl); err != nil {
		return nil, errors.New("CalDAV 地址格式错误")
	}
	if _, err := url.ParseRequestURI(in.CalendarUrl); err != nil {
		return nil, errors.New("日历地址格式错误")
	}
	if in.SyncIntervalMinutes == 0 {
		in.SyncIntervalMinutes = 15
	}
	if in.SyncIntervalMinutes < 5 || in.SyncIntervalMinutes > 1440 {
		return nil, errors.New("同步间隔须为 5 ~ 1440 分钟")
	}
	if in.ConflictPolicy == "" {
		in.ConflictPolicy = "manual"
	}
	if in.ConflictPolicy != "manual" && in.ConflictPolicy != "local" && in.ConflictPolicy != "remote" {
		return nil, errors.New("冲突策略须为 manual/local/remote")
	}
	if in.Enabled > 1 {
		in.Enabled = 1
	}
	return &entity.CalendarSyncAccount{
		AccountId: accountId, Name: in.Name, Provider: "caldav",
		ServerUrl: in.ServerUrl, CalendarUrl: in.CalendarUrl,
		Username: in.Username, Enabled: in.Enabled,
		SyncIntervalMinutes: in.SyncIntervalMinutes, ConflictPolicy: in.ConflictPolicy,
		OwnerUserId: ownerUserId, LastSyncStatus: "idle",
	}, nil
}

func decryptCalendarPassword(enc string) (string, error) {
	return crypto.Decrypt(enc, config.Get().DataEncryptKey())
}

func (s *CalendarSyncService) syncAccount(ctx context.Context, account *entity.CalendarSyncAccount) (*CalendarSyncResult, error) {
	s.mu.Lock()
	if s.running[account.AccountId] {
		s.mu.Unlock()
		return nil, errors.New("该日历正在同步，请稍后")
	}
	s.running[account.AccountId] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.running, account.AccountId)
		s.mu.Unlock()
	}()

	_ = s.repo.UpdateAccountStatus(ctx, account.AccountId, "running", "")
	result, err := s.syncAccountBody(ctx, account)
	if err != nil {
		_ = s.repo.UpdateAccountStatus(ctx, account.AccountId, "error", truncateCalendarSyncError(err.Error(), 1000))
		return nil, err
	}
	status := "success"
	if result.Conflicts > 0 {
		status = "conflict"
	}
	_ = s.repo.UpdateAccountStatus(ctx, account.AccountId, status, "")
	return result, nil
}

func (s *CalendarSyncService) syncAccountBody(ctx context.Context, account *entity.CalendarSyncAccount) (*CalendarSyncResult, error) {
	password, err := decryptCalendarPassword(account.PasswordEnc)
	if err != nil {
		return nil, errors.New("日历密码解密失败")
	}
	client := caldav.NewClient(account.Username, password)
	resources, err := client.ListEvents(ctx, account.CalendarUrl)
	if err != nil {
		return nil, err
	}
	links, err := s.repo.ListLinks(ctx, account.AccountId)
	if err != nil {
		return nil, err
	}
	linkByHref := map[string]*entity.CalendarSyncLink{}
	linkByEvent := map[string]*entity.CalendarSyncLink{}
	for _, link := range links {
		linkByHref[link.RemoteHref] = link
		linkByEvent[link.EventId] = link
	}
	events, err := s.repo.ListCalendarEvents(ctx, account.OwnerUserId, account.AccountId)
	if err != nil {
		return nil, err
	}
	eventByID := map[string]*entity.ScheduleEvent{}
	eventByExternalUID := map[string]*entity.ScheduleEvent{}
	for _, event := range events {
		eventByID[event.EventId] = event
		if event.SourceAccountId == account.AccountId && event.ExternalUid != "" {
			eventByExternalUID[event.ExternalUid] = event
		}
	}

	result := &CalendarSyncResult{Message: "同步完成"}
	remoteHrefs := map[string]bool{}
	parsedRemote := map[string]*caldav.CalendarData{}
	for _, resource := range resources {
		remoteHrefs[resource.Href] = true
		data, parseErr := caldav.ParseCalendar(resource.Data)
		if parseErr != nil {
			result.Skipped++
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", resource.Href, parseErr))
			continue
		}
		parsedRemote[resource.Href] = data
		link := linkByHref[resource.Href]
		if link == nil {
			if data.Master != nil {
				if existing := eventByExternalUID[data.Master.UID]; existing != nil {
					link = &entity.CalendarSyncLink{
						AccountId: account.AccountId, EventId: existing.EventId,
						RemoteEtag: "", RemoteUid: data.Master.UID,
					}
				}
			}
		}
		if link == nil {
			if err := s.applyRemote(ctx, account, resource, data, nil); err != nil {
				if errors.Is(err, errUnsupportedCalendarSyncItem) {
					result.Skipped++
					result.Warnings = append(result.Warnings, err.Error())
					continue
				}
				return nil, err
			}
			result.Pulled++
			continue
		}
		local := eventByID[link.EventId]
		if local == nil {
			local, _ = s.repo.GetEventIncludingDeleted(ctx, link.EventId)
		}
		localChanged := s.localChanged(ctx, local, link)
		remoteChanged := strings.TrimSpace(link.RemoteEtag) != strings.TrimSpace(resource.ETag)
		if link.Conflict == 1 {
			switch account.ConflictPolicy {
			case "remote":
				if err := s.applyRemote(ctx, account, resource, data, link); err != nil {
					if errors.Is(err, errUnsupportedCalendarSyncItem) {
						result.Skipped++
						result.Warnings = append(result.Warnings, err.Error())
						continue
					}
					return nil, err
				}
				result.Pulled++
				continue
			case "local":
				// 保留本地版本，进入下方推送阶段。
			default:
				result.Conflicts++
				continue
			}
		}
		if remoteChanged && localChanged {
			switch account.ConflictPolicy {
			case "remote":
				if err := s.applyRemote(ctx, account, resource, data, link); err != nil {
					if errors.Is(err, errUnsupportedCalendarSyncItem) {
						result.Skipped++
						result.Warnings = append(result.Warnings, err.Error())
						continue
					}
					return nil, err
				}
				result.Pulled++
			case "local":
				// 保留本地，稍后推送。
			default:
				link.Conflict = 1
				link.ConflictPayload = string(resource.Data)
				_ = s.repo.UpsertLink(ctx, link)
				result.Conflicts++
			}
			continue
		}
		if remoteChanged {
			if err := s.applyRemote(ctx, account, resource, data, link); err != nil {
				if errors.Is(err, errUnsupportedCalendarSyncItem) {
					result.Skipped++
					result.Warnings = append(result.Warnings, err.Error())
					continue
				}
				return nil, err
			}
			result.Pulled++
		}
	}

	for _, link := range links {
		if remoteHrefs[link.RemoteHref] || link.Conflict == 1 {
			continue
		}
		local, _ := s.repo.GetEventIncludingDeleted(ctx, link.EventId)
		if local == nil {
			_ = s.repo.DeleteLink(ctx, link.LinkId)
			continue
		}
		if s.localChanged(ctx, local, link) {
			// 远端删除、本地修改：按本地版本重新创建。
			if err := s.pushLocal(ctx, account, client, local, link, true); err != nil {
				return nil, err
			}
			result.Pushed++
			continue
		}
		if local.DeletedAt == nil {
			if err := s.repo.SoftDeleteEvent(ctx, local.EventId); err != nil {
				return nil, err
			}
			result.DeletedLocal++
		}
		_ = s.repo.DeleteLink(ctx, link.LinkId)
	}

	for _, event := range events {
		link := linkByEvent[event.EventId]
		if event.DeletedAt != nil {
			if link != nil {
				if err := client.DeleteEvent(ctx, link.RemoteHref, link.RemoteEtag); err != nil && !errors.Is(err, caldav.ErrPreconditionFailed) {
					return nil, err
				}
				_ = s.repo.DeleteLink(ctx, link.LinkId)
				result.DeletedRemote++
			}
			continue
		}
		localChanged := link == nil || s.localChanged(ctx, event, link)
		if !localChanged || event.SourceAccountId == account.AccountId && link == nil {
			continue
		}
		if link != nil && link.Conflict == 1 {
			if account.ConflictPolicy == "manual" || account.ConflictPolicy == "remote" {
				continue
			}
		}
		remoteMissing := link != nil && !remoteHrefs[link.RemoteHref]
		if err := s.pushLocal(ctx, account, client, event, link, remoteMissing); err != nil {
			if errors.Is(err, caldav.ErrPreconditionFailed) && link != nil {
				link.Conflict = 1
				_ = s.repo.UpsertLink(ctx, link)
				result.Conflicts++
				continue
			}
			return nil, err
		}
		result.Pushed++
	}
	return result, nil
}

func (s *CalendarSyncService) localChanged(ctx context.Context, event *entity.ScheduleEvent, link *entity.CalendarSyncLink) bool {
	if event == nil {
		return false
	}
	if event.DeletedAt != nil {
		return true
	}
	if link.LocalUpdatedAt == nil || event.UpdatedAt == nil || time.Time(*event.UpdatedAt).After(time.Time(*link.LocalUpdatedAt)) {
		return true
	}
	exceptions, _ := s.exceptionRepo.ListByEvent(ctx, event.EventId)
	return exceptionsHash(exceptions) != link.ExceptionsHash
}

func (s *CalendarSyncService) applyRemote(ctx context.Context, account *entity.CalendarSyncAccount, resource caldav.Resource, data *caldav.CalendarData, link *entity.CalendarSyncLink) error {
	if data.Master == nil {
		return nil
	}
	if data.Master.Status == "CANCELLED" {
		if link != nil {
			_ = s.repo.SoftDeleteEvent(ctx, link.EventId)
			_ = s.repo.DeleteLink(ctx, link.LinkId)
		}
		return nil
	}
	ruleJSON, err := rruleJSONFromICS(data.Master.RRule)
	if err != nil {
		return err
	}
	loc, _ := time.LoadLocation(caldav.LocalTimeZone)
	start := localScheduleTime(data.Master.Start, loc)
	end := localScheduleTime(data.Master.End, loc)
	title := data.Master.Summary
	if strings.TrimSpace(title) == "" {
		title = "（无标题日历事件）"
	}
	var event *entity.ScheduleEvent
	if link != nil {
		event, _ = s.repo.GetEventIncludingDeleted(ctx, link.EventId)
	}
	if event == nil {
		event = &entity.ScheduleEvent{
			Title: title, EventType: 0, StartAt: tp(start), EndAt: tp(end),
			IsRecurring: boolInt(ruleJSON != ""), RRule: ruleJSON, Tz: caldav.LocalTimeZone,
			Location: data.Master.Location, OrganizerId: account.OwnerUserId,
			Remark: data.Master.Description, SourceAccountId: account.AccountId,
			ExternalUid: data.Master.UID, CreatedBy: account.OwnerUserId,
		}
		eventId, addErr := s.eventRepo.Add(ctx, event)
		if addErr != nil {
			return addErr
		}
		event.EventId = eventId
	} else {
		event.Title = title
		event.EventType = 0
		event.StartAt = tp(start)
		event.EndAt = tp(end)
		event.IsRecurring = boolInt(ruleJSON != "")
		event.RRule = ruleJSON
		event.Tz = caldav.LocalTimeZone
		event.Location = data.Master.Location
		event.Remark = data.Master.Description
		event.SourceAccountId = account.AccountId
		event.ExternalUid = data.Master.UID
		event.OrganizerId = account.OwnerUserId
		event.DeletedAt = nil
		if err := s.eventRepo.Edit(ctx, event); err != nil {
			return err
		}
	}
	_ = s.exceptionRepo.DeleteByEvent(ctx, event.EventId)
	var localExceptions []*entity.ScheduleException
	for _, remoteException := range data.Exceptions {
		original := localScheduleTime(remoteException.RecurrenceDate, loc)
		x := &entity.ScheduleException{
			EventId: event.EventId, OccurrenceDate: original.Format(scheduleDateLayout),
			Status: ScheduleExceptionMove,
		}
		if remoteException.Status == "CANCELLED" {
			x.Status = ScheduleExceptionCancel
		} else if remoteException.Start.IsZero() {
			continue
		} else {
			x.NewStartAt = tp(localScheduleTime(remoteException.Start, loc))
			end := remoteException.End
			if end.IsZero() {
				end = remoteException.Start.Add(time.Hour)
			}
			x.NewEndAt = tp(localScheduleTime(end, loc))
		}
		_, _ = s.exceptionRepo.Upsert(ctx, x)
		localExceptions = append(localExceptions, x)
	}
	link = &entity.CalendarSyncLink{
		LinkId: linkIdOrEmpty(link), AccountId: account.AccountId, EventId: event.EventId,
		RemoteHref: resource.Href, RemoteEtag: resource.ETag, RemoteUid: data.Master.UID,
		LocalUpdatedAt: event.UpdatedAt, ExceptionsHash: exceptionsHash(localExceptions),
		Conflict: 0, LastSyncedAt: scheduleSyncNow(),
	}
	return s.repo.UpsertLink(ctx, link)
}

func (s *CalendarSyncService) pushLocal(ctx context.Context, account *entity.CalendarSyncAccount, client *caldav.Client, event *entity.ScheduleEvent, link *entity.CalendarSyncLink, forceCreate bool) error {
	rule, err := parseRRule(event.RRule)
	if err != nil {
		return err
	}
	icsRule, err := icsRRuleFromJSON(rule)
	if err != nil {
		return err
	}
	exceptions, _ := s.exceptionRepo.ListByEvent(ctx, event.EventId)
	build := caldav.BuildEvent{
		UID: publicEventUID(event), Summary: event.Title, Description: event.Remark,
		Location: event.Location, Start: time.Time(*event.StartAt), End: time.Time(*event.EndAt),
		RRule: icsRule, Exceptions: buildExceptions(event, exceptions),
	}
	payload, err := caldav.BuildCalendar(build)
	if err != nil {
		return err
	}
	href := ""
	etag := ""
	if link != nil {
		href = link.RemoteHref
		if !forceCreate {
			etag = link.RemoteEtag
		}
	}
	if href == "" {
		href = joinCalendarHref(account.CalendarUrl, event.EventId+".ics")
	}
	newETag, err := client.PutEvent(ctx, href, payload, etag)
	if err != nil {
		return err
	}
	return s.repo.UpsertLink(ctx, &entity.CalendarSyncLink{
		LinkId: linkIdOrEmpty(link), AccountId: account.AccountId, EventId: event.EventId,
		RemoteHref: href, RemoteEtag: newETag, RemoteUid: build.UID,
		LocalUpdatedAt: event.UpdatedAt, ExceptionsHash: exceptionsHash(exceptions),
		Conflict: 0, LastSyncedAt: scheduleSyncNow(),
	})
}

func buildExceptions(event *entity.ScheduleEvent, exceptions []*entity.ScheduleException) []caldav.BuildException {
	if event.StartAt == nil || len(exceptions) == 0 {
		return nil
	}
	anchor := time.Time(*event.StartAt)
	loc := eventLoc(event.Tz)
	out := make([]caldav.BuildException, 0, len(exceptions))
	for _, x := range exceptions {
		day, err := time.ParseInLocation(scheduleDateLayout, x.OccurrenceDate, loc)
		if err != nil {
			continue
		}
		original := time.Date(day.Year(), day.Month(), day.Day(), anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), loc)
		item := caldav.BuildException{
			OriginalStart: original, Summary: event.Title, Description: event.Remark,
			Location: event.Location, Cancel: x.Status == ScheduleExceptionCancel,
		}
		if x.NewStartAt != nil {
			item.Start = time.Time(*x.NewStartAt)
		}
		if x.NewEndAt != nil {
			item.End = time.Time(*x.NewEndAt)
		}
		out = append(out, item)
	}
	return out
}

func publicEventUID(event *entity.ScheduleEvent) string {
	if event.ExternalUid != "" {
		return event.ExternalUid
	}
	return event.EventId + "@aiarsenal.local"
}

func joinCalendarHref(calendarURL, name string) string {
	if !strings.HasSuffix(calendarURL, "/") {
		calendarURL += "/"
	}
	return calendarURL + url.PathEscape(name)
}

func localScheduleTime(value time.Time, loc *time.Location) time.Time {
	return value.In(loc)
}

func exceptionsHash(exceptions []*entity.ScheduleException) string {
	rows := make([]map[string]any, 0, len(exceptions))
	for _, x := range exceptions {
		rows = append(rows, map[string]any{
			"date": x.OccurrenceDate, "status": x.Status,
			"start": timeValueString(x.NewStartAt), "end": timeValueString(x.NewEndAt),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i]["date"].(string) < rows[j]["date"].(string) })
	b, _ := json.Marshal(rows)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func timeValueString(v *types.Time) string {
	if v == nil {
		return ""
	}
	return time.Time(*v).Format(scheduleTimeLayout)
}

func linkIdOrEmpty(link *entity.CalendarSyncLink) string {
	if link == nil {
		return ""
	}
	return link.LinkId
}

func rruleJSONFromICS(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	rule := RRule{}
	for _, part := range strings.Split(raw, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key, value := strings.ToUpper(kv[0]), kv[1]
		switch key {
		case "FREQ":
			rule.Freq = strings.ToUpper(value)
		case "INTERVAL":
			rule.Interval, _ = strconv.Atoi(value)
		case "BYDAY":
			for _, day := range strings.Split(value, ",") {
				day = strings.TrimSpace(day)
				if len(day) > 2 {
					day = day[len(day)-2:]
				}
				rule.ByDay = append(rule.ByDay, strings.ToUpper(day))
			}
		case "COUNT":
			rule.Count, _ = strconv.Atoi(value)
		case "UNTIL":
			if len(value) >= 8 {
				rule.Until = value[:4] + "-" + value[4:6] + "-" + value[6:8]
			}
		}
	}
	if err := validateRRule(rule); err != nil {
		return "", fmt.Errorf("%w：周期规则 %q: %v", errUnsupportedCalendarSyncItem, raw, err)
	}
	return canonicalRRule(rule), nil
}

func icsRRuleFromJSON(rule RRule) (string, error) {
	rule = rule.normalized()
	if rule.Freq == "" {
		return "", nil
	}
	if err := validateRRule(rule); err != nil {
		return "", err
	}
	parts := []string{"FREQ=" + rule.Freq}
	if rule.Interval > 1 {
		parts = append(parts, "INTERVAL="+strconv.Itoa(rule.Interval))
	}
	if len(rule.ByDay) > 0 {
		parts = append(parts, "BYDAY="+strings.Join(rule.ByDay, ","))
	}
	if rule.Count > 0 {
		parts = append(parts, "COUNT="+strconv.Itoa(rule.Count))
	} else if rule.Until != "" {
		parts = append(parts, "UNTIL="+strings.ReplaceAll(rule.Until, "-", "")+"T235959Z")
	}
	return strings.Join(parts, ";"), nil
}

func truncateCalendarSyncError(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func scheduleSyncNow() *types.Time {
	now := types.Time(time.Now())
	return &now
}
