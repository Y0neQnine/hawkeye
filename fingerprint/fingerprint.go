// Package fingerprint identifies the product/system behind a collected website
// from a single read-only HTTP GET: response headers, cookies, page keywords and
// favicon hash. It is deliberately rule-based (no LLM) so the same site always
// yields the same technologies, and it never sends anything beyond GET.
package fingerprint

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// UserAgent is sent on every probe request so operators can tell collection
// traffic apart from other clients.
const UserAgent = "Hawkeye-Fingerprint/1.0"

// Match is one identified product/system with the evidence that produced it.
type Match struct {
	Product    string `json:"product"`
	Category   string `json:"category,omitempty"`
	Version    string `json:"version,omitempty"`
	Confidence string `json:"confidence"` // high | medium
	Evidence   string `json:"evidence"`
}

// Observation is everything one read-only probe learned about a site.
type Observation struct {
	URL           string   `json:"url"`
	FinalURL      string   `json:"final_url,omitempty"`
	StatusCode    int      `json:"status_code"`
	ContentLength int64    `json:"content_length"`
	PageTitle     string   `json:"page_title,omitempty"`
	Server        string   `json:"server,omitempty"`
	Technologies  []string `json:"technologies"`
	Matches       []Match  `json:"matches"`
	FaviconMMH3   string   `json:"favicon_mmh3,omitempty"`
	// DefaultPage marks a page that is only the web server's / panel's placeholder
	// (nginx/apache/IIS welcome page, 宝塔「站点创建成功」, directory listing, blank
	// page): nothing meaningful is deployed at this path yet, so the caller may run
	// directory probing to find the real backend entry or a fingerprint-revealing
	// console (swagger/actuator/druid/...). DefaultPageKind names the placeholder.
	DefaultPage     bool   `json:"default_page"`
	DefaultPageKind string `json:"default_page_kind,omitempty"`
	// RetryAfter is the server-requested backoff (429/503), zero when absent or
	// unparsable.
	RetryAfter time.Duration `json:"-"`
}

const (
	maxBody    = 1 << 20 // 1 MiB
	maxFavicon = 256 << 10
)

// Probe performs one GET (plus at most one favicon GET) and returns the observed
// page attributes and identified products. client is supplied by the caller so
// the probe can reuse its proxy/TLS/timeout policy.
func Probe(ctx context.Context, client *http.Client, rawURL string) (*Observation, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("无效的站点 URL：%q", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))

	obs := &Observation{
		URL:           rawURL,
		FinalURL:      resp.Request.URL.String(),
		StatusCode:    resp.StatusCode,
		ContentLength: int64(len(body)),
		PageTitle:     extractTitle(body),
		Server:        resp.Header.Get("Server"),
		RetryAfter:    parseRetryAfter(resp.Header.Get("Retry-After")),
	}
	obs.Matches = Identify(resp.Header, resp.Cookies(), body)
	obs.Technologies = Technologies(obs.Matches)
	obs.DefaultPage, obs.DefaultPageKind = detectDefaultPage(resp.StatusCode, body, len(obs.Matches) == 0)

	// 429 is an explicit rate limit; fetching the icon would just add traffic from
	// an address the server is already throttling. Other statuses (403/503) are
	// ambiguous and do not suppress the favicon.
	if resp.StatusCode != http.StatusTooManyRequests {
		if iconURL := pickFaviconURL(resp.Request.URL, body); iconURL != "" {
			if raw, ok := fetchFavicon(ctx, client, iconURL); ok {
				obs.FaviconMMH3 = FaviconMMH3(raw)
			}
		}
	}
	return obs, nil
}

// parseRetryAfter accepts both forms of Retry-After: delay-seconds and HTTP-date.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// defaultPageMarkers are lower-cased body substrings that identify a stock web
// server / panel placeholder page. They are deliberately specific: a real portal
// that merely mentions "nginx" must not trigger a directory scan.
var defaultPageMarkers = []struct{ kind, marker string }{
	{"nginx", "welcome to nginx"},
	{"openresty", "welcome to openresty"},
	{"tengine", "welcome to tengine"},
	{"apache", "apache2 ubuntu default page"},
	{"apache", "test page for the apache http server"},
	{"apache", "this is the default web page for this server"},
	{"apache", "it works!"},
	{"iis", "iis windows server"},
	{"iis", "internet information services"},
	{"baota", "站点创建成功"},
	{"baota", "宝塔"},
	{"tomcat", "if you're seeing this page via a web browser"},
	{"jetty", "welcome to jetty"},
	{"spring", "whitelabel error page"},
	{"centos", "welcome to centos"},
	{"centos", "http server test page"},
	{"listing", "index of /"},
	{"listing", "directory listing for /"},
}

// detectDefaultPage reports whether a response is a stock placeholder (and, when
// true, which one) so the caller can probe common backend/fingerprint paths.
// noMatch = the fingerprint rules identified nothing on the page.
func detectDefaultPage(status int, body []byte, noMatch bool) (bool, string) {
	low := bytes.ToLower(body)
	if status == http.StatusNotFound && bytes.Contains(low, []byte("<center>nginx</center>")) {
		return true, "nginx-404"
	}
	if status < 200 || status >= 400 {
		return false, ""
	}
	for _, m := range defaultPageMarkers {
		if bytes.Contains(low, []byte(m.marker)) {
			return true, m.kind
		}
	}
	// A 200 response with an empty-ish body and zero fingerprint matches is a parked
	// / not-yet-deployed vhost; scanning often still finds the real app.
	if noMatch && status == http.StatusOK && len(body) < 256 {
		return true, "blank"
	}
	return false, ""
}

var reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

func extractTitle(body []byte) string {
	m := reTitle.FindSubmatch(body)
	if m == nil {
		return ""
	}
	title := strings.TrimSpace(html.UnescapeString(string(m[1])))
	title = strings.Join(strings.Fields(title), " ")
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	return title
}

var (
	reLinkTag = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	reHref    = regexp.MustCompile(`(?is)href\s*=\s*["']([^"']+)["']`)
)

// pickFaviconURL prefers the page's declared icon, falling back to /favicon.ico.
func pickFaviconURL(base *url.URL, body []byte) string {
	for _, tag := range reLinkTag.FindAll(body, -1) {
		low := bytes.ToLower(tag)
		if !bytes.Contains(low, []byte("icon")) {
			continue
		}
		if m := reHref.FindSubmatch(tag); m != nil {
			raw := strings.TrimSpace(string(m[1]))
			ref, err := url.Parse(raw)
			if err != nil {
				continue
			}
			return base.ResolveReference(ref).String()
		}
	}
	root := *base
	root.Path, root.RawQuery, root.Fragment = "/favicon.ico", "", ""
	return root.String()
}

func fetchFavicon(ctx context.Context, client *http.Client, iconURL string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(ctx, "GET", iconURL, nil)
	if err != nil {
		return nil, false
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return nil, false // error page pretending to be an icon
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxFavicon))
	if len(data) == 0 {
		return nil, false
	}
	return data, true
}

// FaviconMMH3 reproduces the Shodan/FOFA convention: base64 (76-col lines) of the
// raw icon bytes, hashed with MurmurHash3 x86-32 seed 0 and printed as int32.
func FaviconMMH3(data []byte) string {
	enc := base64Lines(data)
	return strconv.FormatInt(int64(int32(murmur3_32(enc, 0))), 10)
}

func base64Lines(data []byte) []byte {
	enc := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(enc, data)
	var out []byte
	for len(enc) > 76 {
		out = append(out, enc[:76]...)
		out = append(out, '\n')
		enc = enc[76:]
	}
	out = append(out, enc...)
	out = append(out, '\n')
	return out
}

func murmur3_32(data []byte, seed uint32) uint32 {
	const (
		c1 = uint32(0xcc9e2d51)
		c2 = uint32(0x1b873593)
	)
	h := seed
	n := len(data) / 4
	for i := 0; i < n; i++ {
		k := binary.LittleEndian.Uint32(data[i*4:])
		k *= c1
		k = k<<15 | k>>17
		k *= c2
		h ^= k
		h = h<<13 | h>>19
		h = h*5 + 0xe6546b64
	}
	tail := data[n*4:]
	var k uint32
	switch len(tail) {
	case 3:
		k ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k ^= uint32(tail[0])
		k *= c1
		k = k<<15 | k>>17
		k *= c2
		h ^= k
	}
	h ^= uint32(len(data))
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}
