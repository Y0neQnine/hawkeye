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
	}
	obs.Matches = Identify(resp.Header, resp.Cookies(), body)
	obs.Technologies = Technologies(obs.Matches)

	if iconURL := pickFaviconURL(resp.Request.URL, body); iconURL != "" {
		if raw, ok := fetchFavicon(ctx, client, iconURL); ok {
			obs.FaviconMMH3 = FaviconMMH3(raw)
		}
	}
	return obs, nil
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
