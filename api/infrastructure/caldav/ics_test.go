package caldav

import (
	"strings"
	"testing"
	"time"
)

func TestCalendarRoundTripWithExceptions(t *testing.T) {
	loc, _ := time.LoadLocation(LocalTimeZone)
	start := time.Date(2026, 10, 28, 9, 0, 0, 0, loc)
	end := start.Add(time.Hour)
	payload, err := BuildCalendar(BuildEvent{
		UID:         "event@example.test",
		Summary:     "周会, 法务",
		Description: "第一行\n第二行",
		Location:    "会议室 A",
		Start:       start,
		End:         end,
		RRule:       "FREQ=WEEKLY;BYDAY=WE;COUNT=4",
		Exceptions: []BuildException{
			{
				OriginalStart: start.AddDate(0, 0, 7),
				Start:         start.AddDate(0, 0, 7).Add(2 * time.Hour),
				End:           start.AddDate(0, 0, 7).Add(3 * time.Hour),
				Summary:       "周会改期",
			},
			{
				OriginalStart: start.AddDate(0, 0, 14),
				Cancel:        true,
			},
		},
	})
	if err != nil {
		t.Fatalf("BuildCalendar() error = %v", err)
	}
	if !strings.Contains(string(payload), "RRULE:FREQ=WEEKLY;BYDAY=WE;COUNT=4") {
		t.Fatalf("payload missing RRULE:\n%s", payload)
	}
	parsed, err := ParseCalendar(payload)
	if err != nil {
		t.Fatalf("ParseCalendar() error = %v", err)
	}
	if parsed.Master == nil || parsed.Master.UID != "event@example.test" {
		t.Fatalf("master UID mismatch: %#v", parsed.Master)
	}
	if parsed.Master.Summary != "周会, 法务" || parsed.Master.Description != "第一行\n第二行" {
		t.Fatalf("text round-trip mismatch: %#v", parsed.Master)
	}
	if len(parsed.Exceptions) != 2 {
		t.Fatalf("exceptions = %d, want 2", len(parsed.Exceptions))
	}
	if parsed.Exceptions[1].Status != "CANCELLED" {
		t.Fatalf("cancelled exception status = %q", parsed.Exceptions[1].Status)
	}
	if !parsed.Master.Start.Equal(start) {
		t.Fatalf("start = %s, want %s", parsed.Master.Start, start)
	}
}

func TestParseFoldedLineAndAllDay(t *testing.T) {
	payload := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:all-day\r\nDTSTART;VALUE=DATE:20261028\r\nDTEND;VALUE=DATE:20261029\r\nSUMMARY:Long\r\n  title\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	parsed, err := ParseCalendar([]byte(payload))
	if err != nil {
		t.Fatalf("ParseCalendar() error = %v", err)
	}
	if !parsed.Master.AllDay {
		t.Fatal("all-day event was not detected")
	}
	if parsed.Master.Summary != "Long title" {
		t.Fatalf("folded summary = %q", parsed.Master.Summary)
	}
	if got := parsed.Master.End.Sub(parsed.Master.Start); got != 24*time.Hour {
		t.Fatalf("all-day duration = %s", got)
	}
}
