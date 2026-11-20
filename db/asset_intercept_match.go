package db

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// 资产拦截规则的匹配/执行层。asset_intercept.go 只负责规则存储，这里负责把
// 「目标资产」的域名/IP/URL 与启用中的规则做匹配。供 agent 工具（add_intent、
// insert_assets）在下发意图 / 插入资产前调用，命中则拒绝。

// AssetInterceptKindLabel 返回 kind 的中文标签，用于给 agent 的说明消息。
func AssetInterceptKindLabel(kind string) string {
	switch kind {
	case "exact_domain":
		return "域名(全等)"
	case "exact_ip":
		return "IP(全等)"
	case "exact_url":
		return "URL(全等)"
	case "fuzzy_domain":
		return "域名(模糊)"
	case "fuzzy_ip":
		return "IP(模糊)"
	case "fuzzy_url":
		return "URL(模糊)"
	case "cidr":
		return "CIDR 网段"
	}
	return kind
}

// Reason 返回一条可读的命中原因，形如：命中资产拦截规则 [域名(模糊): .gov.cn]（备注）。
func (r AssetInterceptRule) Reason() string {
	s := fmt.Sprintf("命中资产拦截规则 [%s: %s]", AssetInterceptKindLabel(r.Kind), r.Pattern)
	if note := strings.TrimSpace(r.Note); note != "" {
		s += "（" + note + "）"
	}
	return s
}

// normalizeDomain lowercases a candidate and drops the FQDN trailing dot so
// "Example.COM." and "example.com" compare equal.
func normalizeDomain(d string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(d), "."))
}

// domainMatchesDelimited reports whether p occurs in d on label boundaries:
// "x.gov.cn" matches ".gov" only when the suffix is the last label or is
// followed by a dot, so "cdn.govcdn.com" and "my.education.com" do not match
// ".gov"/".edu". A leading dot in p is the separator itself, so no further left
// boundary is required.
func domainMatchesDelimited(d, p string) bool {
	d = normalizeDomain(d)
	p = strings.ToLower(strings.TrimSpace(p))
	if d == "" || p == "" {
		return false
	}
	if d == strings.TrimPrefix(p, ".") {
		return true
	}
	leftFree := strings.HasPrefix(p, ".")
	for start := 0; start <= len(d); {
		i := strings.Index(d[start:], p)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(p)
		leftOK := leftFree || i == 0 || d[i-1] == '.'
		rightOK := end == len(d) || d[end] == '.'
		if leftOK && rightOK {
			return true
		}
		start = i + 1
	}
	return false
}

// ipMatchesDelimited treats the pattern as an address prefix on octet
// boundaries: "192.168.1." and "192.168.1" match 192.168.1.10, but
// "192.168.1.1" matches only 192.168.1.1 itself.
func ipMatchesDelimited(ip, p string) bool {
	ip, p = strings.TrimSpace(ip), strings.TrimSpace(p)
	if ip == "" || p == "" {
		return false
	}
	if ip == p {
		return true
	}
	if !strings.HasPrefix(ip, p) {
		return false
	}
	if strings.HasSuffix(p, ".") {
		return true
	}
	return len(ip) > len(p) && ip[len(p)] == '.'
}

// matchOne 判断单条启用规则是否命中给定的域名/IP/URL 候选串，返回命中的具体值。
func matchOne(r AssetInterceptRule, domains, ips, urls []string) (string, bool) {
	p := strings.TrimSpace(r.Pattern)
	if p == "" {
		return "", false
	}
	switch r.Kind {
	case "exact_domain":
		for _, d := range domains {
			if normalizeDomain(d) == normalizeDomain(p) {
				return d, true
			}
		}
	case "exact_ip":
		for _, ip := range ips {
			if strings.TrimSpace(ip) == p {
				return ip, true
			}
		}
	case "exact_url":
		for _, u := range urls {
			if strings.TrimSpace(u) == p {
				return u, true
			}
		}
	case "fuzzy_domain":
		for _, d := range domains {
			if domainMatchesDelimited(d, p) {
				return d, true
			}
		}
	case "fuzzy_ip":
		for _, ip := range ips {
			if ipMatchesDelimited(ip, p) {
				return ip, true
			}
		}
	case "fuzzy_url":
		lp := strings.ToLower(p)
		for _, u := range urls {
			if u != "" && strings.Contains(strings.ToLower(u), lp) {
				return u, true
			}
		}
	case "cidr":
		_, ipnet, err := net.ParseCIDR(p)
		if err != nil {
			return "", false
		}
		for _, ip := range ips {
			if pip := net.ParseIP(strings.TrimSpace(ip)); pip != nil && ipnet.Contains(pip) {
				return ip, true
			}
		}
	}
	return "", false
}

// MatchAssetInterceptRules 返回第一条命中给定 域名/IP/URL 候选串的启用规则，及命中的具体值。
// 供 insert_assets 用原始输入（尚未落库的 assetInputItem）匹配。
func MatchAssetInterceptRules(rules []AssetInterceptRule, domains, ips, urls []string) (AssetInterceptRule, string, bool) {
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if v, ok := matchOne(r, domains, ips, urls); ok {
			return r, v, true
		}
	}
	return AssetInterceptRule{}, "", false
}

// interceptCandidates 提取一个已落库资产用于拦截匹配的 域名/IP/URL 候选串。
// URL 的 host 会被拆出并归类，使「只带 URL」的服务类资产也能被 域名/IP 规则命中。
func (a *Asset) interceptCandidates() (domains, ips, urls []string) {
	add := func(dst *[]string, s string) {
		if s = strings.TrimSpace(s); s != "" {
			*dst = append(*dst, s)
		}
	}
	add(&domains, a.Domain)
	add(&domains, a.RootDomain)
	for _, d := range a.BoundDomains {
		add(&domains, d)
	}
	add(&ips, a.IP)
	add(&urls, a.URL)
	if a.URL != "" {
		if u, err := url.Parse(a.URL); err == nil {
			if h := u.Hostname(); h != "" {
				if net.ParseIP(h) != nil {
					add(&ips, h)
				} else {
					add(&domains, h)
				}
			}
		}
	}
	return domains, ips, urls
}

// InterceptLabel 返回资产的简短标识，用于给 agent 的说明消息。
func (a *Asset) InterceptLabel() string {
	var target string
	switch {
	case a.Domain != "":
		target = a.Domain
	case a.URL != "":
		target = a.URL
	case a.IP != "":
		target = a.IP
	default:
		target = fmt.Sprintf("#%d", a.ID)
	}
	return fmt.Sprintf("资产#%d[%s] %s", a.ID, a.Type, target)
}

// hasEnabledRule 判断规则集里是否存在任一启用规则。
func hasEnabledRule(rules []AssetInterceptRule) bool {
	for _, r := range rules {
		if r.Enabled {
			return true
		}
	}
	return false
}

// AssetGateDecision 是资产闸门对一组候选串的判定结果。
type AssetGateDecision struct {
	Allowed bool
	Reason  string // 被拒原因（不含资产标识）；Allowed=true 时为空
	Target  string // 命中的具体域名/IP/URL；白名单拒绝时为空
}

// EvaluateAssetGate 执行任务级闸门判定：
//  1. 命中任一启用的任务级 block → 拒绝（任务拦截最强）。
//  2. 任务存在启用的 allow(白名单)：命中则放行——即使同时命中全局规则；
//     不命中则拒绝「不在允许范围」。
//  3. 命中全局拦截 → 拒绝。
//  4. 放行。
//
// 任务级 allow 覆盖全局内置规则，让获得授权的任务可以采集黑名单内的目标
// （如客户本身就是 .edu.cn），而不必为单个任务关闭全局规则。
// allow 为空/无启用项时白名单不生效（未配置允许规则不应挡掉全部资产）。
func EvaluateAssetGate(globalRules, taskBlockRules, taskAllowRules []AssetInterceptRule, domains, ips, urls []string) AssetGateDecision {
	if rule, v, ok := MatchAssetInterceptRules(taskBlockRules, domains, ips, urls); ok {
		return AssetGateDecision{Allowed: false, Reason: rule.Reason(), Target: v}
	}
	if hasEnabledRule(taskAllowRules) {
		if _, _, ok := MatchAssetInterceptRules(taskAllowRules, domains, ips, urls); !ok {
			return AssetGateDecision{Allowed: false, Reason: "不在任务允许(白名单)范围内，不允许测试"}
		}
		return AssetGateDecision{Allowed: true}
	}
	if rule, v, ok := MatchAssetInterceptRules(globalRules, domains, ips, urls); ok {
		return AssetGateDecision{Allowed: false, Reason: rule.Reason(), Target: v}
	}
	return AssetGateDecision{Allowed: true}
}

// AssetInterceptHit 描述一个被闸门拒绝的资产（拦截命中 或 不在允许范围）。
type AssetInterceptHit struct {
	Asset  *Asset
	Reason string // 可读原因
}

// Describe 返回一条可读的说明：资产信息 + 原因。
func (h AssetInterceptHit) Describe() string {
	return fmt.Sprintf("%s → %s", h.Asset.InterceptLabel(), h.Reason)
}

// ListAssetInterceptRules 是 *DB 同名方法的透传，让只持有 AssetStore 的调用方
// （如 agent 工具）也能读取规则。
func (s *AssetStore) ListAssetInterceptRules() ([]AssetInterceptRule, error) {
	return s.db.ListAssetInterceptRules()
}

// CheckAssetsIntercept 按 id 载入资产，逐个执行资产闸门判定，返回所有被拒的资产。
// 拦截 = 任务级 block 优先，其次任务级 allow(命中即放行，可覆盖全局规则)，
// 再次全局拦截；无任务上下文时只看全局规则。
// 无 id 时快速返回。用全局 GetByIDs（不受任务范围过滤）以保证拦截不被 scope 削弱。
func (s *AssetStore) CheckAssetsIntercept(taskID int64, ids []int64) ([]AssetInterceptHit, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	globalRules, err := s.db.ListAssetInterceptRules()
	if err != nil {
		return nil, err
	}
	var taskBlock, taskAllow []AssetInterceptRule
	if taskID > 0 {
		taskBlock, taskAllow, err = s.TaskInterceptRulesSplit(taskID)
		if err != nil {
			return nil, err
		}
	}
	// 既无任何启用规则、也无启用的允许规则 → 无需判定，全部放行。
	if !hasEnabledRule(globalRules) && !hasEnabledRule(taskBlock) && !hasEnabledRule(taskAllow) {
		return nil, nil
	}
	assets, err := s.GetByIDs(ids)
	if err != nil {
		return nil, err
	}
	var hits []AssetInterceptHit
	for _, a := range assets {
		domains, ips, urls := a.interceptCandidates()
		if d := EvaluateAssetGate(globalRules, taskBlock, taskAllow, domains, ips, urls); !d.Allowed {
			hits = append(hits, AssetInterceptHit{Asset: a, Reason: d.Reason})
		}
	}
	return hits, nil
}
