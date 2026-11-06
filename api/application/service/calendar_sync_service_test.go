package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aiarsenal/config"
	"aiarsenal/domain/entity"
	"aiarsenal/domain/types"
	"aiarsenal/infrastructure/crypto"
)

type syncRepoStub struct {
	events map[string]*entity.ScheduleEvent
	links  map[string]*entity.CalendarSyncLink
}

func (r *syncRepoStub) ListAccounts(context.Context, string) ([]*entity.CalendarSyncAccount, error) {
	return nil, nil
}
func (r *syncRepoStub) ListEnabledAccounts(context.Context) ([]*entity.CalendarSyncAccount, error) {
	return nil, nil
}
func (r *syncRepoStub) GetAccount(context.Context, string) (*entity.CalendarSyncAccount, error) {
	return nil, fmt.Errorf("not found")
}
func (r *syncRepoStub) AddAccount(context.Context, *entity.CalendarSyncAccount) (string, error) {
	return "account-1", nil
}
func (r *syncRepoStub) EditAccount(context.Context, *entity.CalendarSyncAccount) error {
	return nil
}
func (r *syncRepoStub) DeleteAccount(context.Context, string) error { return nil }
func (r *syncRepoStub) DetachAccount(context.Context, string) error { return nil }
func (r *syncRepoStub) UpdateAccountStatus(context.Context, string, string, string) error {
	return nil
}
func (r *syncRepoStub) ListLinks(_ context.Context, accountId string) ([]*entity.CalendarSyncLink, error) {
	var out []*entity.CalendarSyncLink
	for _, link := range r.links {
		if link.AccountId == accountId {
			out = append(out, link)
		}
	}
	return out, nil
}
func (r *syncRepoStub) GetLinkByEvent(_ context.Context, accountId, eventId string) (*entity.CalendarSyncLink, error) {
	for _, link := range r.links {
		if link.AccountId == accountId && link.EventId == eventId {
			return link, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (r *syncRepoStub) GetLinkByHref(_ context.Context, accountId, href string) (*entity.CalendarSyncLink, error) {
	for _, link := range r.links {
		if link.AccountId == accountId && link.RemoteHref == href {
			return link, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (r *syncRepoStub) UpsertLink(_ context.Context, link *entity.CalendarSyncLink) error {
	if link.LinkId == "" {
		link.LinkId = fmt.Sprintf("link-%d", len(r.links)+1)
	}
	copy := *link
	r.links[link.LinkId] = &copy
	return nil
}
func (r *syncRepoStub) DeleteLink(_ context.Context, linkId string) error {
	delete(r.links, linkId)
	return nil
}
func (r *syncRepoStub) ListCalendarEvents(context.Context, string, string) ([]*entity.ScheduleEvent, error) {
	out := make([]*entity.ScheduleEvent, 0, len(r.events))
	for _, event := range r.events {
		out = append(out, event)
	}
	return out, nil
}
func (r *syncRepoStub) GetEventIncludingDeleted(_ context.Context, eventId string) (*entity.ScheduleEvent, error) {
	event := r.events[eventId]
	if event == nil {
		return nil, fmt.Errorf("not found")
	}
	copy := *event
	return &copy, nil
}
func (r *syncRepoStub) SoftDeleteEvent(context.Context, string) error { return nil }

type eventRepoStub struct {
	store *syncRepoStub
	next  int
}

func (r *eventRepoStub) List(context.Context, string, int, int) ([]*entity.ScheduleEvent, int64, error) {
	return nil, 0, nil
}
func (r *eventRepoStub) ListByIds(context.Context, []string) ([]*entity.ScheduleEvent, error) {
	return nil, nil
}
func (r *eventRepoStub) ListOverlapping(context.Context, time.Time, time.Time) ([]*entity.ScheduleEvent, error) {
	return nil, nil
}
func (r *eventRepoStub) ListRoomEvents(context.Context, string, time.Time, time.Time, bool) ([]*entity.ScheduleEvent, error) {
	return nil, nil
}
func (r *eventRepoStub) ListByUser(context.Context, string, time.Time, time.Time) ([]*entity.ScheduleEvent, error) {
	return nil, nil
}
func (r *eventRepoStub) ListByUsers(context.Context, []string, time.Time, time.Time) ([]*entity.ScheduleEvent, error) {
	return nil, nil
}
func (r *eventRepoStub) Get(_ context.Context, id string) (*entity.ScheduleEvent, error) {
	event := r.store.events[id]
	if event == nil {
		return nil, fmt.Errorf("not found")
	}
	copy := *event
	return &copy, nil
}
func (r *eventRepoStub) Add(_ context.Context, event *entity.ScheduleEvent) (string, error) {
	r.next++
	event.EventId = fmt.Sprintf("event-%d", r.next)
	now := types.Time(time.Now())
	event.CreatedAt = &now
	event.UpdatedAt = &now
	copy := *event
	r.store.events[event.EventId] = &copy
	return event.EventId, nil
}
func (r *eventRepoStub) Edit(_ context.Context, event *entity.ScheduleEvent) error {
	now := types.Time(time.Now().Add(time.Duration(r.next+1) * time.Second))
	if event.UpdatedAt != nil && time.Time(*event.UpdatedAt).After(time.Now()) {
		now = *event.UpdatedAt
	}
	event.UpdatedAt = &now
	copy := *event
	r.store.events[event.EventId] = &copy
	return nil
}
func (r *eventRepoStub) Delete(_ context.Context, ids []string) error {
	for _, id := range ids {
		if event := r.store.events[id]; event != nil {
			now := types.Time(time.Now())
			event.DeletedAt = &now
		}
	}
	return nil
}

type exceptionRepoStub struct {
	items []*entity.ScheduleException
}

func (r *exceptionRepoStub) ListByEvent(_ context.Context, eventId string) ([]*entity.ScheduleException, error) {
	var out []*entity.ScheduleException
	for _, item := range r.items {
		if item.EventId == eventId {
			out = append(out, item)
		}
	}
	return out, nil
}
func (r *exceptionRepoStub) ListByEvents(context.Context, []string) ([]*entity.ScheduleException, error) {
	return r.items, nil
}
func (r *exceptionRepoStub) GetByEventDate(context.Context, string, string) (*entity.ScheduleException, error) {
	return nil, fmt.Errorf("not found")
}
func (r *exceptionRepoStub) Upsert(_ context.Context, item *entity.ScheduleException) (string, error) {
	r.items = append(r.items, item)
	return fmt.Sprintf("exception-%d", len(r.items)), nil
}
func (r *exceptionRepoStub) DeleteByEvent(_ context.Context, eventId string) error {
	kept := r.items[:0]
	for _, item := range r.items {
		if item.EventId != eventId {
			kept = append(kept, item)
		}
	}
	r.items = kept
	return nil
}
func (r *exceptionRepoStub) DeleteByEventDate(context.Context, string, string) error { return nil }

func TestCalendarSyncPullLocalPushAndConflict(t *testing.T) {
	const hrefPath = "/calendars/demo/main/remote-1.ics"
	var mu sync.Mutex
	remoteTitle := "Remote event"
	remoteETag := `"v1"`
	putCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, pass, ok := r.BasicAuth(); !ok || user != "demo" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == "REPORT" && r.URL.Path == "/calendars/demo/main/":
			mu.Lock()
			title, tag := remoteTitle, remoteETag
			mu.Unlock()
			ics := fmt.Sprintf("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:remote-1\r\nDTSTART:20261028T010000Z\r\nDTEND:20261028T020000Z\r\nSUMMARY:%s\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", title)
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			fmt.Fprintf(w, `<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><response><href>%s</href><propstat><prop><getetag>%s</getetag><C:calendar-data>%s</C:calendar-data></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, hrefPath, tag, ics)
		case r.Method == "PUT":
			_, _ = io.Copy(io.Discard, r.Body)
			mu.Lock()
			putCount++
			remoteTitle = "Pushed from local"
			remoteETag = `"v2"`
			mu.Unlock()
			w.Header().Set("ETag", `"v2"`)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := &syncRepoStub{events: map[string]*entity.ScheduleEvent{}, links: map[string]*entity.CalendarSyncLink{}}
	events := &eventRepoStub{store: store}
	exceptions := &exceptionRepoStub{}
	svc := NewCalendarSyncService(store, events, exceptions)
	enc, err := crypto.Encrypt("secret", config.Get().Jwt.EncryptKey)
	if err != nil {
		t.Fatal(err)
	}
	account := &entity.CalendarSyncAccount{
		AccountId: "account-1", CalendarUrl: server.URL + "/calendars/demo/main/",
		Username: "demo", PasswordEnc: enc, OwnerUserId: "user-1",
		ConflictPolicy: "manual",
	}

	result, err := svc.syncAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("initial sync error = %v", err)
	}
	if result.Pulled != 1 {
		t.Fatalf("initial pulled = %d", result.Pulled)
	}
	if len(store.events) != 1 {
		t.Fatalf("imported events = %d", len(store.events))
	}

	var local *entity.ScheduleEvent
	for _, event := range store.events {
		local = event
	}
	local.Title = "Local edit"
	future := types.Time(time.Now().Add(time.Hour))
	local.UpdatedAt = &future
	if err := events.Edit(context.Background(), local); err != nil {
		t.Fatal(err)
	}
	result, err = svc.syncAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("push sync error = %v", err)
	}
	if result.Pushed != 1 || putCount != 1 {
		t.Fatalf("push result=%#v putCount=%d", result, putCount)
	}

	mu.Lock()
	remoteTitle = "Remote concurrent edit"
	remoteETag = `"v3"`
	mu.Unlock()
	local = store.events[local.EventId]
	local.Title = "Local concurrent edit"
	future = types.Time(time.Now().Add(2 * time.Hour))
	local.UpdatedAt = &future
	_ = events.Edit(context.Background(), local)

	result, err = svc.syncAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("conflict sync error = %v", err)
	}
	if result.Conflicts != 1 {
		t.Fatalf("conflicts = %d", result.Conflicts)
	}
	if !strings.Contains(local.Title, "Local") {
		t.Fatalf("manual conflict overwrote local title: %s", local.Title)
	}
}
