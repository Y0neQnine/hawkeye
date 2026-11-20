package enrich

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Autumn-27/artex/db"
	"github.com/projectdiscovery/dnsx/libs/dnsx"
)

const (
	// livenessConcurrency bounds simultaneous probes in the post-collection sweep.
	livenessConcurrency = 8
	// livenessCooldown throttles repeated sweeps for one task: monitor re-runs and
	// duplicate terminal-status writes must not re-probe the whole asset list.
	livenessCooldown = 30 * time.Minute
	// livenessStartDelay lets in-flight asset writes land before the sweep reads
	// the task's asset list (the task is marked done while workers are cancelled).
	livenessStartDelay = 10 * time.Second
	// livenessBudget caps one sweep so a huge task cannot pin the goroutine forever.
	livenessBudget      = 20 * time.Minute
	livenessDialTimeout = 3 * time.Second
	livenessHTTPTimeout = 8 * time.Second
)

// livenessResult is the outcome of one target probe. skip=true means the probe
// could not run (host parked, resolver unavailable) — the asset keeps its
// previous liveness state instead of being marked dead.
type livenessResult struct {
	alive  bool
	reason string
	skip   bool
}

// ProbeTaskLiveness schedules the post-collection liveness sweep for a finished
// task: domains/subdomains resolve via DNS, IPs dial TCP, services take one GET.
// Results land in assets.extra (alive/alive_at/alive_reason). Best-effort: at
// most one sweep per task per livenessCooldown.
func (e *Engine) ProbeTaskLiveness(taskID int64) {
	if e == nil || e.as == nil || taskID <= 0 {
		return
	}
	if v, ok := e.liveSeen.Load(taskID); ok {
		if t, _ := v.(time.Time); ok && time.Since(t) < livenessCooldown {
			return
		}
	}
	e.liveSeen.Store(taskID, time.Now())
	go func() {
		select {
		case <-e.closed:
			return
		case <-time.After(livenessStartDelay):
		}
		targets, err := e.as.ListLivenessTargets(taskID)
		if err != nil {
			log.Printf("[liveness] task %d 读取存活探测目标失败：%v", taskID, err)
			return
		}
		if len(targets) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), livenessBudget)
		defer cancel()
		var (
			wg      sync.WaitGroup
			checked int32
			alive   int32
		)
		sem := make(chan struct{}, livenessConcurrency)
	sweep:
		for _, tgt := range targets {
			select {
			case <-e.closed:
				break sweep
			case <-ctx.Done():
				break sweep
			case sem <- struct{}{}:
			}
			wg.Add(1)
			go func(t db.LivenessTarget) {
				defer wg.Done()
				defer func() { <-sem }()
				res := e.probeLivenessTarget(ctx, t)
				if res.skip {
					log.Printf("[liveness] 资产 %d（%s %s）跳过探测：%s", t.ID, t.Type, t.Domain+t.IP+t.URL, res.reason)
					return
				}
				if err := e.as.SetLiveness(t.ID, res.alive, res.reason, time.Now()); err != nil {
					log.Printf("[liveness] 资产 %d 存活结果写回失败：%v", t.ID, err)
					return
				}
				atomic.AddInt32(&checked, 1)
				if res.alive {
					atomic.AddInt32(&alive, 1)
				}
			}(tgt)
		}
		wg.Wait()
		n, a := atomic.LoadInt32(&checked), atomic.LoadInt32(&alive)
		log.Printf("[liveness] task %d 存活探测完成：探测 %d，存活 %d，失活 %d", taskID, n, a, n-a)
	}()
}

func (e *Engine) probeLivenessTarget(ctx context.Context, t db.LivenessTarget) livenessResult {
	switch t.Type {
	case "root_domain", "subdomain":
		return probeDomainLiveness(e.resolv, t.Domain)
	case "ip":
		return probeTCPLiveness(ctx, t.IP, t.OpenPorts)
	case "service":
		if t.URL != "" {
			return e.probeURLLiveness(ctx, t.URL)
		}
		if t.IP != "" && t.Port > 0 {
			return probeTCPLiveness(ctx, t.IP, []int{t.Port})
		}
		return livenessResult{reason: "服务缺少 URL 与端口", skip: true}
	}
	return livenessResult{reason: "不支持的类型", skip: true}
}

// probeDomainLiveness treats a successful DNS answer (A/AAAA/CNAME) as alive.
func probeDomainLiveness(resolv *dnsx.DNSX, domain string) livenessResult {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return livenessResult{reason: "缺少域名", skip: true}
	}
	if resolv == nil {
		return livenessResult{reason: "DNS 解析不可用", skip: true}
	}
	data, err := resolv.QueryMultiple(domain)
	if err != nil {
		// 解析器本身不可达：无法据此判定资产失活，保留上次状态。
		return livenessResult{reason: "DNS 解析失败：" + shortText(err.Error()), skip: true}
	}
	n, status := 0, ""
	if data != nil {
		n = len(data.A) + len(data.AAAA) + len(data.CNAME)
		status = data.StatusCode
	}
	return classifyDNSResult(n, status)
}

// classifyDNSResult turns a DNS answer into alive/dead/skip. Only a definitive
// "no such records" (NXDOMAIN/NOERROR with no answer) means dead; SERVFAIL,
// REFUSED and empty responses are inconclusive and must not mark an asset dead.
func classifyDNSResult(records int, status string) livenessResult {
	if records > 0 {
		return livenessResult{alive: true, reason: fmt.Sprintf("DNS 解析成功（%d 条记录）", records)}
	}
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "NXDOMAIN", "NOERROR":
		return livenessResult{reason: "DNS 无解析记录"}
	case "":
		return livenessResult{reason: "DNS 无响应", skip: true}
	default:
		return livenessResult{reason: "DNS 返回 " + status + "，未判定", skip: true}
	}
}

// probeTCPLiveness treats a successful TCP connect on any candidate port as
// alive. Known ports from open_ports are tried first; empty means just 80/443.
func probeTCPLiveness(ctx context.Context, ip string, ports []int) livenessResult {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return livenessResult{reason: "缺少 IP", skip: true}
	}
	if len(ports) == 0 {
		ports = []int{80, 443}
	}
	if len(ports) > 8 {
		ports = ports[:8]
	}
	names := make([]string, 0, len(ports))
	for _, p := range ports {
		names = append(names, strconv.Itoa(p))
		dctx, cancel := context.WithTimeout(ctx, livenessDialTimeout)
		conn, err := (&net.Dialer{}).DialContext(dctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(p)))
		cancel()
		if err == nil {
			_ = conn.Close()
			return livenessResult{alive: true, reason: fmt.Sprintf("TCP %d 端口可连接", p)}
		}
		if ctx.Err() != nil {
			break
		}
	}
	return livenessResult{reason: "TCP 端口均不可连接（" + strings.Join(names, "/") + "）"}
}

// probeURLLiveness treats any HTTP response (including 401/403/404/5xx) as
// alive; only transport-level failures count as dead. It shares the enrichment
// per-host pacing and backoff, so a parked host is skipped rather than marked
// dead.
func (e *Engine) probeURLLiveness(ctx context.Context, rawURL string) livenessResult {
	host := hostOf(rawURL)
	if host == "" {
		return livenessResult{reason: "URL 无效", skip: true}
	}
	if !e.paceHost(host) {
		return livenessResult{reason: "主机处于退避期", skip: true}
	}
	hctx, cancel := context.WithTimeout(ctx, livenessHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(hctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return livenessResult{reason: "URL 无效：" + shortText(err.Error()), skip: true}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Hawkeye liveness)")
	resp, err := e.client.Do(req)
	if err != nil {
		if hctx.Err() != nil {
			return livenessResult{reason: "HTTP 请求超时"}
		}
		return livenessResult{reason: "HTTP 请求失败：" + shortText(err.Error())}
	}
	_ = resp.Body.Close()
	return livenessResult{alive: true, reason: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func shortText(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return s
}
