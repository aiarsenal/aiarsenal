package caldav

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	LocalTimeZone = "Asia/Shanghai"
	icsTimeLayout = "20060102T150405Z"
)

type CalendarData struct {
	Master     *Event
	Exceptions []Exception
}

type Event struct {
	UID         string
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	AllDay      bool
	RRule       string
	Status      string
}

type Exception struct {
	RecurrenceDate time.Time
	Start          time.Time
	End            time.Time
	Summary        string
	Description    string
	Location       string
	Status         string
}

type BuildEvent struct {
	UID         string
	Summary     string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	RRule       string
	Sequence    int
	Exceptions  []BuildException
}

type BuildException struct {
	OriginalStart time.Time
	Start         time.Time
	End           time.Time
	Cancel        bool
	Summary       string
	Description   string
	Location      string
}

// ParseCalendar 解析一个 CalDAV 资源中的 VEVENT 主事件和 RECURRENCE-ID 例外。
func ParseCalendar(data []byte) (*CalendarData, error) {
	lines := unfoldICSLines(data)
	var events []map[string]icsProperty
	var current map[string]icsProperty
	inEvent := false
	for _, line := range lines {
		key, prop, err := parseICSProperty(line)
		if err != nil {
			return nil, err
		}
		if key == "BEGIN" && strings.EqualFold(prop.Value, "VEVENT") {
			current = map[string]icsProperty{}
			inEvent = true
			continue
		}
		if key == "END" && strings.EqualFold(prop.Value, "VEVENT") {
			if inEvent && current != nil {
				events = append(events, current)
			}
			current = nil
			inEvent = false
			continue
		}
		if inEvent && current != nil && key != "" {
			current[key] = prop
		}
	}
	if len(events) == 0 {
		return nil, errors.New("CalDAV 资源中没有 VEVENT")
	}

	out := &CalendarData{}
	for _, props := range events {
		if _, ok := props["RECURRENCE-ID"]; ok {
			x, err := parseICSException(props)
			if err != nil {
				return nil, err
			}
			out.Exceptions = append(out.Exceptions, x)
			continue
		}
		ev, err := parseICSEvent(props)
		if err != nil {
			return nil, err
		}
		if out.Master != nil {
			return nil, errors.New("单个 CalDAV 资源包含多个主事件")
		}
		out.Master = ev
	}
	if out.Master == nil {
		return nil, errors.New("CalDAV 资源缺少主事件")
	}
	sort.SliceStable(out.Exceptions, func(i, j int) bool {
		return out.Exceptions[i].RecurrenceDate.Before(out.Exceptions[j].RecurrenceDate)
	})
	return out, nil
}

func parseICSEvent(props map[string]icsProperty) (*Event, error) {
	uid := unescapeICSText(props["UID"].Value)
	if uid == "" {
		return nil, errors.New("VEVENT 缺少 UID")
	}
	start, allDay, err := parseICSDate(props["DTSTART"], time.Local)
	if err != nil {
		return nil, fmt.Errorf("DTSTART: %w", err)
	}
	end, _, err := parseICSDate(props["DTEND"], start.Location())
	if err != nil {
		if duration, derr := parseICSDuration(props["DURATION"].Value); derr == nil {
			end = start.Add(duration)
		} else {
			end = start.Add(time.Hour)
		}
	}
	if !start.Before(end) {
		end = start.Add(time.Hour)
	}
	return &Event{
		UID:         uid,
		Summary:     unescapeICSText(props["SUMMARY"].Value),
		Description: unescapeICSText(props["DESCRIPTION"].Value),
		Location:    unescapeICSText(props["LOCATION"].Value),
		Start:       start,
		End:         end,
		AllDay:      allDay,
		RRule:       strings.TrimSpace(props["RRULE"].Value),
		Status:      strings.ToUpper(strings.TrimSpace(props["STATUS"].Value)),
	}, nil
}

func parseICSException(props map[string]icsProperty) (Exception, error) {
	original, _, err := parseICSDate(props["RECURRENCE-ID"], time.Local)
	if err != nil {
		return Exception{}, fmt.Errorf("RECURRENCE-ID: %w", err)
	}
	x := Exception{
		RecurrenceDate: original,
		Summary:        unescapeICSText(props["SUMMARY"].Value),
		Description:    unescapeICSText(props["DESCRIPTION"].Value),
		Location:       unescapeICSText(props["LOCATION"].Value),
		Status:         strings.ToUpper(strings.TrimSpace(props["STATUS"].Value)),
	}
	if startProp, ok := props["DTSTART"]; ok {
		start, _, serr := parseICSDate(startProp, original.Location())
		if serr != nil {
			return Exception{}, serr
		}
		x.Start = start
	}
	if endProp, ok := props["DTEND"]; ok {
		end, _, serr := parseICSDate(endProp, original.Location())
		if serr == nil {
			x.End = end
		}
	}
	return x, nil
}

// BuildCalendar 生成一个完整 CalDAV 资源（主事件 + 周期单场例外）。
func BuildCalendar(in BuildEvent) ([]byte, error) {
	if strings.TrimSpace(in.UID) == "" {
		return nil, errors.New("UID 不能为空")
	}
	if !in.Start.Before(in.End) {
		return nil, errors.New("开始时间须早于结束时间")
	}
	var b strings.Builder
	writeICSLine(&b, "BEGIN:VCALENDAR")
	writeICSLine(&b, "VERSION:2.0")
	writeICSLine(&b, "PRODID:-//AI Arsenal//Calendar Sync//CN")
	writeICSLine(&b, "CALSCALE:GREGORIAN")
	writeICSEvent(&b, in.UID, in.Summary, in.Description, in.Location, in.Start, in.End, in.RRule, in.Sequence, false)

	duration := in.End.Sub(in.Start)
	for _, x := range in.Exceptions {
		if x.OriginalStart.IsZero() {
			continue
		}
		start, end := x.Start, x.End
		status := ""
		if x.Cancel {
			start = x.OriginalStart
			end = x.OriginalStart.Add(duration)
			status = "CANCELLED"
		} else if end.IsZero() || !start.Before(end) {
			end = start.Add(duration)
		}
		writeICSException(&b, in.UID, x.OriginalStart, start, end, x.Summary, x.Description, x.Location, status, in.Sequence)
	}
	writeICSLine(&b, "END:VCALENDAR")
	return []byte(b.String()), nil
}

func writeICSEvent(b *strings.Builder, uid, summary, description, location string, start, end time.Time, rrule string, sequence int, _ bool) {
	writeICSLine(b, "BEGIN:VEVENT")
	writeICSLine(b, "UID:"+escapeICSText(uid))
	writeICSLine(b, "DTSTAMP:"+time.Now().UTC().Format(icsTimeLayout))
	writeICSLine(b, "DTSTART:"+start.UTC().Format(icsTimeLayout))
	writeICSLine(b, "DTEND:"+end.UTC().Format(icsTimeLayout))
	writeICSLine(b, "SUMMARY:"+escapeICSText(summary))
	if description != "" {
		writeICSLine(b, "DESCRIPTION:"+escapeICSText(description))
	}
	if location != "" {
		writeICSLine(b, "LOCATION:"+escapeICSText(location))
	}
	if rrule != "" {
		writeICSLine(b, "RRULE:"+rrule)
	}
	writeICSLine(b, "SEQUENCE:"+strconv.Itoa(sequence))
	writeICSLine(b, "END:VEVENT")
}

func writeICSException(b *strings.Builder, uid string, originalStart, start, end time.Time, summary, description, location, status string, sequence int) {
	writeICSLine(b, "BEGIN:VEVENT")
	writeICSLine(b, "UID:"+escapeICSText(uid))
	writeICSLine(b, "RECURRENCE-ID:"+originalStart.UTC().Format(icsTimeLayout))
	writeICSLine(b, "DTSTAMP:"+time.Now().UTC().Format(icsTimeLayout))
	writeICSLine(b, "DTSTART:"+start.UTC().Format(icsTimeLayout))
	writeICSLine(b, "DTEND:"+end.UTC().Format(icsTimeLayout))
	if summary != "" {
		writeICSLine(b, "SUMMARY:"+escapeICSText(summary))
	}
	if description != "" {
		writeICSLine(b, "DESCRIPTION:"+escapeICSText(description))
	}
	if location != "" {
		writeICSLine(b, "LOCATION:"+escapeICSText(location))
	}
	if status != "" {
		writeICSLine(b, "STATUS:"+status)
	}
	writeICSLine(b, "SEQUENCE:"+strconv.Itoa(sequence))
	writeICSLine(b, "END:VEVENT")
}

type icsProperty struct {
	Params map[string]string
	Value  string
}

func parseICSProperty(line string) (string, icsProperty, error) {
	if strings.TrimSpace(line) == "" {
		return "", icsProperty{}, nil
	}
	split := -1
	quoted := false
	for i, r := range line {
		if r == '"' {
			quoted = !quoted
		}
		if r == ':' && !quoted {
			split = i
			break
		}
	}
	if split < 0 {
		return "", icsProperty{}, fmt.Errorf("非法 iCalendar 行: %s", line)
	}
	left, value := line[:split], line[split+1:]
	segments := splitICSParamSegments(left)
	name := strings.ToUpper(segments[0])
	params := map[string]string{}
	for _, segment := range segments[1:] {
		if kv := strings.SplitN(segment, "=", 2); len(kv) == 2 {
			params[strings.ToUpper(kv[0])] = strings.Trim(kv[1], `"`)
		}
	}
	return name, icsProperty{Params: params, Value: value}, nil
}

func splitICSParamSegments(s string) []string {
	var out []string
	start := 0
	quoted := false
	for i, r := range s {
		if r == '"' {
			quoted = !quoted
		}
		if r == ';' && !quoted {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func parseICSDate(p icsProperty, fallback *time.Location) (time.Time, bool, error) {
	if strings.TrimSpace(p.Value) == "" {
		return time.Time{}, false, errors.New("空日期")
	}
	value := strings.TrimSpace(p.Value)
	if strings.EqualFold(p.Params["VALUE"], "DATE") || len(value) == 8 {
		t, err := time.ParseInLocation("20060102", value, fallback)
		return t, true, err
	}
	if strings.HasSuffix(value, "Z") {
		t, err := time.Parse(icsTimeLayout, value)
		if err != nil {
			return time.Time{}, false, err
		}
		return t.In(fallback), false, nil
	}
	loc := fallback
	if tz := p.Params["TZID"]; tz != "" {
		if loaded, err := time.LoadLocation(tz); err == nil {
			loc = loaded
		}
	}
	t, err := time.ParseInLocation("20060102T150405", value, loc)
	return t, false, err
}

func parseICSDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("空 DURATION")
	}
	sign := 1
	if strings.HasPrefix(value, "-") {
		sign = -1
		value = value[1:]
	}
	value = strings.TrimPrefix(value, "P")
	var duration time.Duration
	for _, token := range splitDurationTokens(value) {
		if len(token) < 2 {
			continue
		}
		n, err := strconv.Atoi(token[:len(token)-1])
		if err != nil {
			return 0, err
		}
		switch token[len(token)-1] {
		case 'W':
			duration += time.Duration(n) * 7 * 24 * time.Hour
		case 'D':
			duration += time.Duration(n) * 24 * time.Hour
		case 'H':
			duration += time.Duration(n) * time.Hour
		case 'M':
			duration += time.Duration(n) * time.Minute
		case 'S':
			duration += time.Duration(n) * time.Second
		}
	}
	return time.Duration(sign) * duration, nil
}

func splitDurationTokens(value string) []string {
	var out []string
	start := 0
	for i, r := range value {
		if r == 'T' {
			value = value[i+1:]
			break
		}
	}
	for i, r := range value {
		if (r < '0' || r > '9') && i > start {
			out = append(out, value[start:i+1])
			start = i + 1
		}
	}
	return out
}

func unfoldICSLines(data []byte) []string {
	var out []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1024), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(out) > 0 {
			out[len(out)-1] += line[1:]
			continue
		}
		out = append(out, line)
	}
	return out
}

func writeICSLine(b *strings.Builder, line string) {
	const width = 73
	raw := []byte(line)
	for len(raw) > width {
		cut := width
		for cut > 0 && raw[cut]&0xC0 == 0x80 {
			cut--
		}
		b.Write(raw[:cut])
		b.WriteString("\r\n ")
		raw = raw[cut:]
	}
	b.Write(raw)
	b.WriteString("\r\n")
}

func escapeICSText(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		`;`, `\;`,
		`,`, `\,`,
		"\r\n", `\n`,
		"\n", `\n`,
		"\r", `\n`,
	)
	return replacer.Replace(value)
}

func unescapeICSText(value string) string {
	replacer := strings.NewReplacer(
		`\\`, `\`,
		`\;`, `;`,
		`\,`, `,`,
		`\n`, "\n",
		`\N`, "\n",
	)
	return replacer.Replace(strings.TrimSpace(value))
}
