package caldav

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
)

var ErrPreconditionFailed = errors.New("CalDAV 远端版本已变化")

type Calendar struct {
	URL         string `json:"url"`
	DisplayName string `json:"displayName"`
}

type Resource struct {
	Href string
	ETag string
	Data []byte
}

type Client struct {
	HTTP     *http.Client
	Username string
	Password string
}

func NewClient(username, password string) *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Username: username,
		Password: password,
	}
}

func (c *Client) DiscoverCalendars(ctx context.Context, serverURL string) ([]Calendar, error) {
	base, err := normalizeBaseURL(serverURL)
	if err != nil {
		return nil, err
	}
	wellKnown := base.Scheme + "://" + base.Host + "/.well-known/caldav"
	principal, err := c.currentPrincipal(ctx, wellKnown)
	if err != nil {
		principal, err = c.currentPrincipal(ctx, base.String())
	}
	if err != nil {
		return nil, err
	}
	home, err := c.calendarHome(ctx, principal)
	if err != nil {
		return nil, err
	}
	body := `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:displayname/></d:prop></d:propfind>`
	status, data, _, err := c.dav(ctx, "PROPFIND", home, body, map[string]string{"Depth": "1", "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return nil, err
	}
	if status != http.StatusMultiStatus {
		return nil, davStatusError("列出 CalDAV 日历时", status, data)
	}
	ms, err := parseMultistatus(data)
	if err != nil {
		return nil, err
	}
	var out []Calendar
	for _, response := range ms.Responses {
		href := resolveHref(home, strings.TrimSpace(response.Href))
		for _, ps := range response.Propstats {
			if ps.Prop.ResourceType.Calendar == nil {
				continue
			}
			name := strings.TrimSpace(ps.Prop.DisplayName)
			if name == "" {
				name = path.Base(strings.TrimRight(href, "/"))
			}
			out = append(out, Calendar{URL: href, DisplayName: name})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	return out, nil
}

func (c *Client) ProbeCalendar(ctx context.Context, calendarURL string) error {
	body := `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:displayname/></d:prop></d:propfind>`
	status, data, _, err := c.dav(ctx, "PROPFIND", calendarURL, body, map[string]string{"Depth": "0", "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return err
	}
	if status != http.StatusMultiStatus {
		return davStatusError("验证 CalDAV 日历失败", status, data)
	}
	return nil
}

func (c *Client) ListEvents(ctx context.Context, calendarURL string) ([]Resource, error) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">
  <d:prop><d:getetag/><c:calendar-data/></d:prop>
  <c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"/></c:comp-filter></c:filter>
</c:calendar-query>`
	status, data, _, err := c.dav(ctx, "REPORT", calendarURL, body, map[string]string{"Depth": "1", "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return nil, err
	}
	if status != http.StatusMultiStatus {
		return nil, davStatusError("读取 CalDAV 事件失败", status, data)
	}
	ms, err := parseMultistatus(data)
	if err != nil {
		return nil, err
	}
	var out []Resource
	for _, response := range ms.Responses {
		href := resolveHref(calendarURL, strings.TrimSpace(response.Href))
		for _, ps := range response.Propstats {
			if strings.Contains(ps.Status, " 404 ") || strings.TrimSpace(ps.Prop.CalendarData) == "" {
				continue
			}
			out = append(out, Resource{
				Href: href,
				ETag: strings.TrimSpace(ps.Prop.ETag),
				Data: []byte(ps.Prop.CalendarData),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Href < out[j].Href })
	return out, nil
}

func (c *Client) PutEvent(ctx context.Context, href string, data []byte, etag string) (string, error) {
	headers := map[string]string{"Content-Type": "text/calendar; charset=utf-8"}
	if etag == "" {
		headers["If-None-Match"] = "*"
	} else {
		headers["If-Match"] = etag
	}
	status, body, respHeaders, err := c.dav(ctx, "PUT", href, string(data), headers)
	if err != nil {
		return "", err
	}
	if status == http.StatusPreconditionFailed {
		return "", ErrPreconditionFailed
	}
	if status < 200 || status >= 300 {
		return "", davStatusError("写入 CalDAV 事件失败", status, []byte(body))
	}
	if newTag := strings.TrimSpace(respHeaders.Get("ETag")); newTag != "" {
		return newTag, nil
	}
	return c.getETag(ctx, href)
}

func (c *Client) DeleteEvent(ctx context.Context, href, etag string) error {
	headers := map[string]string{}
	if etag != "" {
		headers["If-Match"] = etag
	}
	status, body, _, err := c.dav(ctx, "DELETE", href, "", headers)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return nil
	}
	if status == http.StatusPreconditionFailed {
		return ErrPreconditionFailed
	}
	if status < 200 || status >= 300 {
		return davStatusError("删除 CalDAV 事件失败", status, []byte(body))
	}
	return nil
}

func (c *Client) currentPrincipal(ctx context.Context, target string) (string, error) {
	body := `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:current-user-principal/></d:prop></d:propfind>`
	status, data, _, err := c.dav(ctx, "PROPFIND", target, body, map[string]string{"Depth": "0", "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return "", err
	}
	if status != http.StatusMultiStatus {
		return "", davStatusError("CalDAV 当前用户发现失败", status, data)
	}
	ms, err := parseMultistatus(data)
	if err != nil {
		return "", err
	}
	for _, r := range ms.Responses {
		for _, ps := range r.Propstats {
			if href := strings.TrimSpace(ps.Prop.CurrentUserPrincipal.Href); href != "" {
				return resolveHref(target, href), nil
			}
		}
	}
	return "", errors.New("CalDAV 服务未返回 current-user-principal")
}

func (c *Client) calendarHome(ctx context.Context, principal string) (string, error) {
	body := `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><c:calendar-home-set/></d:prop></d:propfind>`
	status, data, _, err := c.dav(ctx, "PROPFIND", principal, body, map[string]string{"Depth": "0", "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return "", err
	}
	if status != http.StatusMultiStatus {
		return "", davStatusError("CalDAV calendar-home 发现失败", status, data)
	}
	ms, err := parseMultistatus(data)
	if err != nil {
		return "", err
	}
	for _, r := range ms.Responses {
		for _, ps := range r.Propstats {
			if href := strings.TrimSpace(ps.Prop.CalendarHomeSet.Href); href != "" {
				return resolveHref(principal, href), nil
			}
		}
	}
	return "", errors.New("CalDAV 服务未返回 calendar-home-set")
}

func (c *Client) getETag(ctx context.Context, href string) (string, error) {
	body := `<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:getetag/></d:prop></d:propfind>`
	status, data, _, err := c.dav(ctx, "PROPFIND", href, body, map[string]string{"Depth": "0", "Content-Type": "application/xml; charset=utf-8"})
	if err != nil {
		return "", err
	}
	if status != http.StatusMultiStatus {
		return "", davStatusError("读取 CalDAV ETag 失败", status, data)
	}
	ms, err := parseMultistatus(data)
	if err != nil {
		return "", err
	}
	for _, r := range ms.Responses {
		for _, ps := range r.Propstats {
			if tag := strings.TrimSpace(ps.Prop.ETag); tag != "" {
				return tag, nil
			}
		}
	}
	return "", nil
}

func (c *Client) dav(ctx context.Context, method, target, body string, headers map[string]string) (int, []byte, http.Header, error) {
	if _, err := url.ParseRequestURI(target); err != nil {
		return 0, nil, nil, fmt.Errorf("非法 CalDAV URL: %w", err)
	}
	current := target
	originalHost := ""
	var resp *http.Response
	for redirects := 0; ; redirects++ {
		if redirects > 6 {
			return 0, nil, nil, errors.New("CalDAV 重定向次数过多")
		}
		req, err := http.NewRequestWithContext(ctx, method, current, bytes.NewBufferString(body))
		if err != nil {
			return 0, nil, nil, err
		}
		parsed, _ := url.Parse(current)
		if originalHost == "" {
			originalHost = parsed.Host
		}
		// 只在初始主机内发送 Basic Auth，避免重定向把凭据带到第三方主机。
		if c.Username != "" && parsed.Host == originalHost {
			req.SetBasicAuth(c.Username, c.Password)
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err = c.HTTP.Do(req)
		if err != nil {
			return 0, nil, nil, err
		}
		if resp.StatusCode < 300 || resp.StatusCode > 399 {
			break
		}
		location := strings.TrimSpace(resp.Header.Get("Location"))
		if location == "" {
			break
		}
		_ = resp.Body.Close()
		current = resolveHref(current, location)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return 0, nil, nil, err
	}
	return resp.StatusCode, data, resp.Header, nil
}

type davMultistatus struct {
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href      string        `xml:"href"`
	Propstats []davPropstat `xml:"propstat"`
}

type davPropstat struct {
	Status string  `xml:"status"`
	Prop   davProp `xml:"prop"`
}

type davProp struct {
	CurrentUserPrincipal davHref         `xml:"current-user-principal"`
	CalendarHomeSet      davHref         `xml:"calendar-home-set"`
	ResourceType         davResourcetype `xml:"resourcetype"`
	DisplayName          string          `xml:"displayname"`
	ETag                 string          `xml:"getetag"`
	CalendarData         string          `xml:"calendar-data"`
}

type davHref struct {
	Href string `xml:"href"`
}

type davResourcetype struct {
	Calendar *struct{} `xml:"calendar"`
}

func parseMultistatus(data []byte) (*davMultistatus, error) {
	var out davMultistatus
	if err := xml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("解析 CalDAV XML 失败: %w", err)
	}
	return &out, nil
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("CalDAV 地址必须以 http:// 或 https:// 开头")
	}
	if u.Host == "" {
		return nil, errors.New("CalDAV 地址缺少主机名")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

func resolveHref(base, href string) string {
	bu, err := url.Parse(base)
	if err != nil {
		return href
	}
	hu, err := url.Parse(href)
	if err != nil {
		return href
	}
	return bu.ResolveReference(hu).String()
}

func davStatusError(action string, status int, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 300 {
		snippet = snippet[:300]
	}
	return fmt.Errorf("%s：HTTP %d %s", action, status, snippet)
}
