package caldav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCalDAVClientLifecycle(t *testing.T) {
	const eventICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:remote-1\r\nDTSTART:20261028T010000Z\r\nDTEND:20261028T020000Z\r\nSUMMARY:Remote Event\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	var mu sync.Mutex
	eventBody := eventICS
	eventETag := `"v1"`
	putCount := 0
	deleteCount := 0

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "demo" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == "PROPFIND" && r.URL.Path == "/.well-known/caldav":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			fmt.Fprintf(w, `<multistatus xmlns="DAV:"><response><href>%s/principals/demo/</href><propstat><prop><current-user-principal><href>/principals/demo/</href></current-user-principal></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, server.URL)
		case r.Method == "PROPFIND" && r.URL.Path == "/principals/demo/":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			fmt.Fprintf(w, `<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><response><href>/principals/demo/</href><propstat><prop><C:calendar-home-set><href>/calendars/demo/</href></C:calendar-home-set></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`)
		case r.Method == "PROPFIND" && r.URL.Path == "/calendars/demo/":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			fmt.Fprintf(w, `<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><response><href>/calendars/demo/main/</href><propstat><prop><resourcetype><collection/><C:calendar/></resourcetype><displayname>Main Calendar</displayname></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`)
		case r.Method == "REPORT" && r.URL.Path == "/calendars/demo/main/":
			mu.Lock()
			body := eventBody
			etag := eventETag
			mu.Unlock()
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			fmt.Fprintf(w, `<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><response><href>/calendars/demo/main/remote-1.ics</href><propstat><prop><getetag>%s</getetag><C:calendar-data>%s</C:calendar-data></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, etag, body)
		case r.Method == "PUT" && r.URL.Path == "/calendars/demo/main/new.ics":
			mu.Lock()
			putCount++
			eventBody = "PUT-BODY"
			eventETag = `"v2"`
			mu.Unlock()
			w.Header().Set("ETag", `"v2"`)
			w.WriteHeader(http.StatusCreated)
		case r.Method == "DELETE" && r.URL.Path == "/calendars/demo/main/remote-1.ics":
			mu.Lock()
			deleteCount++
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("demo", "secret")
	calendars, err := client.DiscoverCalendars(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("DiscoverCalendars() error = %v", err)
	}
	if len(calendars) != 1 || calendars[0].DisplayName != "Main Calendar" {
		t.Fatalf("calendars = %#v", calendars)
	}

	resources, err := client.ListEvents(context.Background(), calendars[0].URL)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(resources) != 1 || !strings.Contains(string(resources[0].Data), "Remote Event") {
		t.Fatalf("resources = %#v", resources)
	}

	newTag, err := client.PutEvent(context.Background(), server.URL+"/calendars/demo/main/new.ics", []byte(eventICS), "")
	if err != nil {
		t.Fatalf("PutEvent() error = %v", err)
	}
	if newTag != `"v2"` || putCount != 1 {
		t.Fatalf("PutEvent() etag=%q count=%d", newTag, putCount)
	}
	if err := client.DeleteEvent(context.Background(), server.URL+"/calendars/demo/main/remote-1.ics", `"v1"`); err != nil {
		t.Fatalf("DeleteEvent() error = %v", err)
	}
	if deleteCount != 1 {
		t.Fatalf("deleteCount = %d", deleteCount)
	}
}
