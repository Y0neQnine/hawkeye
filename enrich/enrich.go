// Package enrich is the engine-side (non-AI) asset auto-completion layer described
// in docs/资产模型与自动关联设计.md §5: an async worker pool that resolves domains
// (dnsx) and probes web assets (HTTP, through the recording proxy) and writes the
// results back into the asset graph — creating IP/port nodes, resolves/exposes
// edges, and filling attrs.dns / attrs.http. DNS is ungated; HTTP probing is gated
// by RoE (§5.2).
package enrich

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/Autumn-27/artex/fingerprint"

	"github.com/miekg/dns"
	"github.com/projectdiscovery/dnsx/libs/dnsx"
)

type jobKind int

const (
	jobDNS  jobKind = iota // resolve a domain
	jobHTTP                // probe a web asset (site)
)

type job struct {
	kind jobKind
	id   int64  // asset id (domain for DNS, site for HTTP)
	arg  string // host (DNS) or url (HTTP)
}

// Engine owns the resolver, the proxy-routed HTTP client, and the worker pool.
type Engine struct {
	as     *db.AssetStore
	resolv *dnsx.DNSX
	client *http.Client

	jobs   chan job
	cool   sync.Map // dedup/cooldown: "kind:id" -> time.Time (last run)
	once   sync.Once
	closed chan struct{}

	// Per-host pacing keeps a batch of URLs on one site from arriving as a burst
	// that looks like scanning. WAF/CDN edges commonly rate-limit by IP when that
	// happens; probes are serialized per host and a host that explicitly asks for
	// backoff (429, or 503 with Retry-After) is parked.
	hostMu      sync.Mutex
	hostLocks   map[string]*sync.Mutex
	hostLast    map[string]time.Time
	hostBlocked sync.Map // host -> time.Time (unblock time)
}

const (
	cooldown    = 5 * time.Minute
	httpTimeout = 12 * time.Second
	queueSize   = 1024

	// hostProbeInterval is the minimum gap between HTTP probes to the same host.
	hostProbeInterval = 3 * time.Second
	// backoffCooldown is the minimum park after a host explicitly asks us to back
	// off. 403 does not park: it is usually a path-level denial (auth, directory
	// listing) rather than an IP block, and parking would silently skip sites that
	// only protect some paths.
	backoffCooldown = 30 * time.Minute
	// maxBackoffCooldown caps Retry-After so one bogus header cannot park a host
	// indefinitely.
	maxBackoffCooldown = 6 * time.Hour
)

// New builds the engine. proxy() returns the recording-proxy address to route HTTP
// probes through (so they land in the traffic store), evaluated per request so the
// runtime traffic-capture toggle takes effect live; "" = direct. Returns a usable
// engine even if the resolver fails to init (DNS becomes a no-op).
func New(as *db.AssetStore, proxy func() string, workers int) *Engine {
	if workers <= 0 {
		workers = 4
	}
	resolv, err := dnsx.New(dnsx.Options{
		BaseResolvers: dnsx.DefaultResolvers,
		MaxRetries:    3,
		QuestionTypes: []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeCNAME},
		Timeout:       4 * time.Second,
	})
	if err != nil {
		log.Printf("[enrich] dnsx 初始化失败，DNS 解析停用：%v", err)
		resolv = nil
	}
	e := &Engine{
		as:       as,
		resolv:   resolv,
		client:   buildClient(proxy),
		jobs:     make(chan job, queueSize),
		closed:   make(chan struct{}),
		hostLast: make(map[string]time.Time),
	}
	for i := 0; i < workers; i++ {
		go e.worker()
	}
	return e
}

// buildClient returns an HTTP client that dials via the recording proxy (resolved
// per-request via proxy(), so the traffic-capture toggle applies live) and skips
// TLS verification (the proxy re-signs with its MITM CA; targets are often
// self-signed — this is a pentest probe).
func buildClient(proxy func() string) *http.Client {
	tr := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives:   true,
		TLSHandshakeTimeout: httpTimeout,
	}
	if proxy != nil {
		tr.Proxy = func(*http.Request) (*url.URL, error) {
			p := proxy()
			if p == "" {
				return nil, nil // direct
			}
			return url.Parse(p)
		}
	}
	return &http.Client{
		Transport: tr,
		Timeout:   httpTimeout,
		// 跟随最多 3 跳同主机重定向（http→https、/→登录页很常见）；跨主机不跟，
		// 避免把采集带出授权范围。
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 || req.URL.Host != via[0].URL.Host {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// ResolveDomain enqueues a DNS resolution for a domain asset (ungated). host is the
// FQDN. No-op for an empty engine.
func (e *Engine) ResolveDomain(id int64, host string) { e.enqueue(job{jobDNS, id, host}) }

// ProbeSite enqueues an HTTP probe for a web-asset (site). rawURL is the site URL.
func (e *Engine) ProbeSite(id int64, rawURL string) { e.enqueue(job{jobHTTP, id, rawURL}) }

func (e *Engine) enqueue(j job) {
	if e == nil || id0(j.id) || j.arg == "" {
		return
	}
	select {
	case e.jobs <- j:
	default: // queue full → drop (best-effort enrichment)
		log.Printf("[enrich] 队列已满，丢弃任务 kind=%d id=%d", j.kind, j.id)
	}
}

func id0(id int64) bool { return id <= 0 }

// Close stops the workers (idempotent).
func (e *Engine) Close() {
	if e == nil {
		return
	}
	e.once.Do(func() { close(e.closed) })
}

func (e *Engine) worker() {
	for {
		select {
		case <-e.closed:
			return
		case j := <-e.jobs:
			if e.onCooldown(j) {
				continue
			}
			switch j.kind {
			case jobDNS:
				e.doDNS(j.id, j.arg)
			case jobHTTP:
				e.doHTTP(j.id, j.arg)
			}
		}
	}
}

// onCooldown returns true (skip) if this (kind,id) ran within the cooldown window.
func (e *Engine) onCooldown(j job) bool {
	key := string(rune(j.kind)) + ":" + itoa(j.id)
	if v, ok := e.cool.Load(key); ok {
		if t, ok := v.(time.Time); ok && time.Since(t) < cooldown {
			return true
		}
	}
	e.cool.Store(key, time.Now())
	return false
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// hostLock returns the per-host serialization mutex, creating it on demand.
func (e *Engine) hostLock(host string) *sync.Mutex {
	e.hostMu.Lock()
	defer e.hostMu.Unlock()
	if e.hostLocks == nil {
		e.hostLocks = make(map[string]*sync.Mutex)
	}
	m, ok := e.hostLocks[host]
	if !ok {
		m = &sync.Mutex{}
		e.hostLocks[host] = m
	}
	return m
}

func (e *Engine) hostBlockedNow(host string) bool {
	v, ok := e.hostBlocked.Load(host)
	if !ok {
		return false
	}
	until, _ := v.(time.Time)
	return time.Now().Before(until)
}

func (e *Engine) markHostBlocked(host string, d time.Duration) {
	if d <= 0 {
		return
	}
	e.hostBlocked.Store(host, time.Now().Add(d))
}

// backoffPark reports whether a response asks the client to stop, and for how
// long. Only explicit signals count: 429 is a rate limit, and 503 parks only when
// the server also sent Retry-After (overload with a stated recovery time). A bare
// 403 is a path-level denial and a bare 503 is usually maintenance/upstream
// failure — both are recorded, neither parks the host.
func backoffPark(code int, retryAfter time.Duration) (time.Duration, bool) {
	switch {
	case code == http.StatusTooManyRequests:
	case code == http.StatusServiceUnavailable && retryAfter > 0:
	default:
		return 0, false
	}
	d := backoffCooldown
	if retryAfter > d {
		d = retryAfter
	}
	if d > maxBackoffCooldown {
		d = maxBackoffCooldown
	}
	return d, true
}

// paceHost serializes probes to one host and enforces hostProbeInterval so a
// batch of URLs on the same site does not arrive as a scan-like burst.
func (e *Engine) paceHost(host string) bool {
	if e.hostBlockedNow(host) {
		return false
	}
	mu := e.hostLock(host)
	mu.Lock()
	defer mu.Unlock()
	if e.hostBlockedNow(host) {
		return false
	}
	e.hostMu.Lock()
	last := e.hostLast[host]
	e.hostMu.Unlock()
	if !last.IsZero() {
		if wait := hostProbeInterval - time.Since(last); wait > 0 {
			select {
			case <-time.After(wait):
			case <-e.closed:
				return false
			}
		}
	}
	e.hostMu.Lock()
	e.hostLast[host] = time.Now()
	e.hostMu.Unlock()
	return true
}

// ---- DNS ----

func (e *Engine) doDNS(id int64, host string) {
	if e.resolv == nil {
		return
	}
	data, err := e.resolv.QueryMultiple(host)
	if err != nil || data == nil {
		return
	}
	ips := uniq(append(append([]string{}, data.A...), data.AAAA...))
	// CNAME 指向 CDN/WAF 厂商域名时，解析结果属于共享边缘节点：登记 IP 并打标签，
	// 便于在资产里区分"目标自有主机"与"CDN/WAF 出口"，标签不参与企业归属。
	tags := detectNetworkTags(data.CNAME)
	// Upsert resolved IPs into the asset store.
	for _, ip := range ips {
		_, _ = e.as.UpsertIP(db.UpsertIPReq{
			IP:           ip,
			BoundDomains: []string{host},
			Technologies: tags,
		})
	}
	// Record A records as subdomains if host looks like a subdomain.
	for _, a := range data.A {
		_, _ = e.as.UpsertSubdomain(db.UpsertSubdomainReq{
			Domain:      host,
			RecordType:  "A",
			RecordValue: []string{a},
		})
	}
	for _, aaaa := range data.AAAA {
		_, _ = e.as.UpsertSubdomain(db.UpsertSubdomainReq{
			Domain:      host,
			RecordType:  "AAAA",
			RecordValue: []string{aaaa},
		})
	}
	for _, cname := range data.CNAME {
		_, _ = e.as.UpsertSubdomain(db.UpsertSubdomainReq{
			Domain:      host,
			RecordType:  "CNAME",
			RecordValue: []string{cname},
		})
	}
}

// ---- HTTP probe ----

// doHTTP performs one read-only GET (plus favicon) and writes back status/title/
// technologies/favicon hash. This is what makes "采集到站点即识别指纹" automatic:
// every inserted service URL is enriched without the agent having to re-probe it.
func (e *Engine) doHTTP(id int64, rawURL string) {
	host := hostOf(rawURL)
	if host == "" {
		return
	}
	if !e.paceHost(host) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*httpTimeout)
	defer cancel()
	obs, err := fingerprint.Probe(ctx, e.client, rawURL)
	if err != nil {
		return
	}
	if d, ok := backoffPark(obs.StatusCode, obs.RetryAfter); ok {
		e.markHostBlocked(host, d)
		log.Printf("[enrich] %s 返回 %d，按退避暂停自动探测该主机 %s", rawURL, obs.StatusCode, d)
	}
	statusCode := obs.StatusCode
	bodyLen := obs.ContentLength
	_, _ = e.as.UpsertHTTPService(db.UpsertHTTPServiceReq{
		URL:           rawURL,
		Technologies:  obs.Technologies,
		StatusCode:    &statusCode,
		ContentLength: &bodyLen,
		PageTitle:     obs.PageTitle,
		FaviconMMH3:   obs.FaviconMMH3,
	})
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func isBlank(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case float64:
		return x == 0
	case int:
		return x == 0
	}
	return false
}

func uniq(in []string) []string {
	seen := map[string]struct{}{}
	out := in[:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
