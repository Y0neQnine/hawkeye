package fingerprint

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DirFinding is one discovered path: a non-soft-404 response that points at a
// backend entry (admin/console), an exposed sensitive file, or another page that
// reveals something about the deployed system.
type DirFinding struct {
	Kind          string   `json:"kind"` // admin | console | sensitive | path
	Path          string   `json:"path"`
	URL           string   `json:"url"` // final URL after same-host redirects
	StatusCode    int      `json:"status_code"`
	ContentLength int64    `json:"content_length"`
	PageTitle     string   `json:"page_title,omitempty"`
	Technologies  []string `json:"technologies,omitempty"`
}

// DirScanResult is everything one directory scan learned. Backoff is the
// server-requested pause (429/503+Retry-After) that aborted the scan; the caller
// parks the host for that long.
type DirScanResult struct {
	BaseURL      string        `json:"base_url"`
	Scanned      int           `json:"scanned"`
	Findings     []DirFinding  `json:"findings"`
	Technologies []string      `json:"technologies,omitempty"` // union over findings
	Truncated    bool          `json:"truncated,omitempty"`    // hit MaxPaths
	Aborted      bool          `json:"aborted,omitempty"`      // canceled or backed off
	Backoff      time.Duration `json:"-"`
}

// DirScanOptions tunes a scan. Zero values fall back to the polite defaults.
type DirScanOptions struct {
	ExtraPaths  []string      // probed first, classified heuristically
	MaxPaths    int           // hard cap on total paths (default 150, cap 300)
	Concurrency int           // in-flight requests (default 4)
	Delay       time.Duration // minimum gap between request starts (default 120ms)
	Timeout     time.Duration // per-request timeout (default 10s)
}

const (
	dirScanMaxBody      = 256 << 10
	dirScanMaxFindings  = 60
	dirScanDefaultPaths = 150
	dirScanHardPaths    = 300
)

// Curated path list. Ordered by value: backend entries first so a capped scan
// still covers what matters. Every entry is a plain GET target.
var (
	dirAdminPaths = []string{
		"admin", "admin/", "admin/login", "admin/index.php", "admin.php",
		"admin/login.php", "admin/login.jsp", "administrator", "administrator/login",
		"manage", "manager", "management", "manager/html", "manager/status",
		"backend", "console", "dashboard", "system", "login", "login.php",
		"login.jsp", "login.html", "signin", "user/login", "webadmin", "sysadmin",
		"adminconsole", "wp-admin", "wp-login.php", "portal", "portal/login",
		"oa", "oa/login", "seeyon", "seeyon/index.jsp", "weaver", "ecology",
		"ruoyi", "ruoyi/login", "jeecg-boot", "jeecg-boot/system/login",
	}
	dirConsolePaths = []string{
		"nacos", "nacos/", "druid", "druid/index.html", "druid/login.html",
		"eureka", "eureka/", "consul/ui", "consul/ui/", "zabbix", "zabbix.php",
		"jenkins", "grafana", "grafana/login", "kibana", "kibana/app/kibana",
		"sonar", "sonarqube", "harbor", "harbor/projects", "nexus", "portainer",
		"rancher", "phpmyadmin", "phpMyAdmin", "adminer.php", "adminer", "solr",
		"solr/", "rabbitmq", "minio", "minio/login", "apollo", "xxl-job-admin",
		"xxl-job-admin/toLogin", "skywalking", "arthas", "jolokia", "tomcat",
		"securityRealm", "hawtio", "web-console", "swagger-ui.html",
		"swagger-ui/index.html", "swagger/index.html", "v2/api-docs", "v3/api-docs",
		"api-docs", "openapi.json", "swagger.json", "api/swagger.json",
		"actuator", "actuator/health", "actuator/env", "actuator/mappings",
		"actuator/beans", "actuator/configprops", "prometheus", "metrics", "graphql",
	}
	dirSensitivePaths = []string{
		".git/config", ".git/HEAD", ".git/", ".svn/entries", ".svn/wc.db",
		".env", ".env.bak", ".env.local", ".DS_Store", ".htaccess",
		".bash_history", ".idea/workspace.xml", "id_rsa", "web.config",
		"WEB-INF/web.xml", "META-INF/MANIFEST.MF", "application.properties",
		"application.yml", "application.yaml", "config/application.yml",
		"server-status", "phpinfo.php", "info.php", "test.php", "backup.zip",
		"backup.tar.gz", "backup.sql", "www.zip", "web.zip", "site.zip", "dump.sql",
		"database.sql", "config.php.bak", "config.bak",
	}
	dirOtherPaths = []string{
		"index.php", "home", "api", "api/", "rest", "services", "ws", "health",
		"healthz", "status", "version", "debug", "test", "dev", "upload", "uploads",
		"files", "download", "backup", "data", "old", "cas", "cas/login", "sso",
		"sso/login", "idp", "saml", "oauth", "oauth/authorize", "login/oauth",
	}
)

// classifyDirPath buckets a path so the model and the recorder can tell a backend
// entry from an exposed file. Extra (caller-supplied) paths use the same rules.
func classifyDirPath(p string) string {
	low := strings.ToLower(strings.TrimSpace(p))
	switch {
	case strings.Contains(low, "actuator"), strings.Contains(low, "api-docs"),
		strings.Contains(low, "swagger"), strings.Contains(low, "openapi"),
		strings.Contains(low, "graphql"), strings.Contains(low, "druid"),
		strings.Contains(low, "nacos"), strings.Contains(low, "eureka"),
		strings.Contains(low, "consul"), strings.Contains(low, "jenkins"),
		strings.Contains(low, "grafana"), strings.Contains(low, "kibana"),
		strings.Contains(low, "phpmyadmin"), strings.Contains(low, "adminer"),
		strings.Contains(low, "zabbix"), strings.Contains(low, "prometheus"),
		strings.Contains(low, "metrics"), strings.Contains(low, "solr"),
		strings.Contains(low, "rabbitmq"), strings.Contains(low, "minio"),
		strings.Contains(low, "harbor"), strings.Contains(low, "nexus"),
		strings.Contains(low, "portainer"), strings.Contains(low, "rancher"),
		strings.Contains(low, "xxl-job"), strings.Contains(low, "skywalking"),
		strings.Contains(low, "jolokia"), strings.Contains(low, "hawtio"):
		return "console"
	case strings.HasPrefix(low, "."), strings.Contains(low, "phpinfo"),
		strings.Contains(low, ".env"), strings.Contains(low, "backup"),
		strings.Contains(low, ".bak"), strings.Contains(low, ".sql"),
		strings.Contains(low, ".zip"), strings.Contains(low, "web-inf"),
		strings.Contains(low, "meta-inf"), strings.Contains(low, ".yml"),
		strings.Contains(low, ".yaml"), strings.Contains(low, "application.properties"),
		strings.Contains(low, "server-status"), strings.Contains(low, "id_rsa"),
		strings.Contains(low, "web.config"), strings.Contains(low, "ds_store"):
		return "sensitive"
	case strings.Contains(low, "admin"), strings.Contains(low, "login"),
		strings.Contains(low, "manage"), strings.Contains(low, "manager"),
		strings.Contains(low, "system"), strings.Contains(low, "console"),
		strings.Contains(low, "dashboard"), strings.Contains(low, "backend"),
		strings.Contains(low, "wp-"), strings.Contains(low, "signin"),
		strings.Contains(low, "portal"), strings.Contains(low, "seeyon"),
		strings.Contains(low, "weaver"), strings.Contains(low, "ecology"),
		strings.Contains(low, "ruoyi"), strings.Contains(low, "jeecg"):
		return "admin"
	default:
		return "path"
	}
}

func defaultDirPaths() []string {
	out := make([]string, 0, len(dirAdminPaths)+len(dirConsolePaths)+len(dirSensitivePaths)+len(dirOtherPaths))
	out = append(out, dirAdminPaths...)
	out = append(out, dirConsolePaths...)
	out = append(out, dirSensitivePaths...)
	out = append(out, dirOtherPaths...)
	return out
}

// ScanDirectories probes common backend / console / sensitive paths on one site.
// GET only, same host, paced and bounded. A random-path baseline request is made
// first so soft-404 sites (every path returns the same page) yield no false hits.
func ScanDirectories(ctx context.Context, client *http.Client, rawURL string, opts DirScanOptions) (*DirScanResult, error) {
	base, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("无效的站点 URL：%q", rawURL)
	}
	if client == nil {
		return nil, fmt.Errorf("scan client is nil")
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.Delay <= 0 {
		opts.Delay = 120 * time.Millisecond
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.MaxPaths <= 0 {
		opts.MaxPaths = dirScanDefaultPaths
	}
	if opts.MaxPaths > dirScanHardPaths {
		opts.MaxPaths = dirScanHardPaths
	}

	prefix := strings.TrimSuffix(base.Path, "/")
	basePath := prefix
	if basePath == "" {
		basePath = "/"
	}

	paths := make([]string, 0, opts.MaxPaths)
	seen := map[string]bool{}
	addPath := func(p string) {
		p = strings.TrimSpace(p)
		p = strings.TrimLeft(p, "/")
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}
	for _, p := range opts.ExtraPaths {
		addPath(p)
	}
	for _, p := range defaultDirPaths() {
		if len(paths) >= opts.MaxPaths {
			break
		}
		addPath(p)
	}
	if len(paths) > opts.MaxPaths {
		paths = paths[:opts.MaxPaths]
	}
	truncated := len(paths) >= opts.MaxPaths
	if len(paths) == 0 {
		return nil, fmt.Errorf("没有可探测的路径")
	}

	baseReq := dirRequest{client: client, base: base, prefix: prefix, timeout: opts.Timeout}
	baseline := baseReq.probe(ctx, zeroProbePath())
	if baseline.err != nil {
		return nil, fmt.Errorf("基线请求失败：%w", baseline.err)
	}

	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu       sync.Mutex
		findings []DirFinding
		byURL    = map[string]bool{}
		techSeen = map[string]bool{}
		backoff  int64 // atomic nanoseconds
	)
	jobs := make(chan string, len(paths))
	for _, p := range paths {
		jobs <- p
	}
	close(jobs)

	var tick *time.Ticker
	if opts.Delay > 0 {
		tick = time.NewTicker(opts.Delay)
		defer tick.Stop()
	}

	var wg sync.WaitGroup
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				if scanCtx.Err() != nil {
					return
				}
				if tick != nil {
					select {
					case <-scanCtx.Done():
						return
					case <-tick.C:
					}
				}
				r := baseReq.probe(scanCtx, p)
				if r.retryAfter > 0 || r.status == http.StatusTooManyRequests {
					d := r.retryAfter
					if d <= 0 {
						d = 30 * time.Minute
					}
					for {
						cur := atomic.LoadInt64(&backoff)
						if int64(d) <= cur || atomic.CompareAndSwapInt64(&backoff, cur, int64(d)) {
							break
						}
					}
					cancel()
					return
				}
				if !dirHit(baseline, r, base) {
					continue
				}
				kind := classifyDirPath(p)
				f := DirFinding{
					Kind: kind, Path: "/" + p, URL: r.finalURL,
					StatusCode: r.status, ContentLength: r.length,
					PageTitle: r.title, Technologies: r.techs,
				}
				mu.Lock()
				if !byURL[f.URL] {
					byURL[f.URL] = true
					findings = append(findings, f)
					for _, tech := range f.Technologies {
						techSeen[tech] = true
					}
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Slice(findings, func(i, j int) bool {
		pi, pj := dirKindRank(findings[i].Kind), dirKindRank(findings[j].Kind)
		if pi != pj {
			return pi < pj
		}
		return findings[i].Path < findings[j].Path
	})
	if len(findings) > dirScanMaxFindings {
		findings = findings[:dirScanMaxFindings]
	}

	res := &DirScanResult{
		BaseURL:   base.Scheme + "://" + base.Host + basePath,
		Scanned:   len(paths),
		Findings:  findings,
		Truncated: truncated,
		Aborted:   ctx.Err() != nil || scanCtx.Err() != nil,
		Backoff:   time.Duration(atomic.LoadInt64(&backoff)),
	}
	techs := make([]string, 0, len(techSeen))
	for t := range techSeen {
		techs = append(techs, t)
	}
	sort.Strings(techs)
	res.Technologies = techs
	return res, nil
}

func dirKindRank(kind string) int {
	switch kind {
	case "admin":
		return 0
	case "console":
		return 1
	case "sensitive":
		return 2
	default:
		return 3
	}
}

func zeroProbePath() string {
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "hawkeye-" + fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return "hawkeye-" + hex.EncodeToString(b[:])
}

type dirRequest struct {
	client  *http.Client
	base    *url.URL
	prefix  string
	timeout time.Duration
}

type dirResponse struct {
	status     int
	length     int64
	title      string
	finalURL   string
	techs      []string
	retryAfter time.Duration
	err        error
}

func (d dirRequest) probe(ctx context.Context, p string) dirResponse {
	target := *d.base
	target.Path = d.prefix + "/" + p
	target.RawQuery, target.Fragment = "", ""
	reqCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return dirResponse{err: err}
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := d.client.Do(req)
	if err != nil {
		return dirResponse{err: err}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, dirScanMaxBody))
	matches := Identify(resp.Header, resp.Cookies(), body)
	final := target.String()
	if resp.Request != nil && resp.Request.URL != nil {
		final = resp.Request.URL.String()
	}
	return dirResponse{
		status:     resp.StatusCode,
		length:     int64(len(body)),
		title:      extractTitle(body),
		finalURL:   final,
		techs:      Technologies(matches),
		retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
	}
}

// dirHit decides whether a response is worth reporting: an existing/protected
// path (2xx/3xx/401/403/405) that is NOT the site's soft-404 signature and did not
// bounce back to the site root.
func dirHit(baseline, r dirResponse, base *url.URL) bool {
	if r.err != nil {
		return false
	}
	switch {
	case r.status >= 200 && r.status < 400:
	case r.status == http.StatusUnauthorized, r.status == http.StatusForbidden,
		r.status == http.StatusMethodNotAllowed:
	default:
		return false
	}
	if softDir404(baseline, r) {
		return false
	}
	if f, err := url.Parse(r.finalURL); err == nil {
		if f.Host == base.Host && strings.TrimSuffix(f.Path, "/") == strings.TrimSuffix(base.Path, "/") {
			return false // redirected back to the entry page
		}
	}
	return true
}

func softDir404(baseline, r dirResponse) bool {
	if r.status != baseline.status {
		return false
	}
	if baseline.status >= 400 {
		return true // same error status on a random path → site-wide error page
	}
	if r.title != baseline.title {
		return false
	}
	diff := r.length - baseline.length
	if diff < 0 {
		diff = -diff
	}
	tol := baseline.length / 5
	if tol < 64 {
		tol = 64
	}
	return diff <= tol
}
