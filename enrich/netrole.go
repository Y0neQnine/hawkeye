package enrich

import "strings"

// netRoleRule maps a CDN/WAF provider's CNAME suffix to the tags written onto
// the IP asset (assets.technologies). Tags are informational only: they never
// change company attribution, so a shared CDN/WAF egress stays unowned unless
// one of its bound_domains belongs to an in-scope company.
type netRoleRule struct {
	suffixes []string
	tags     []string
}

// netRoleRules is a heuristic suffix table; a CNAME chain is strong evidence
// that the resolved address belongs to that provider's shared edge, not to the
// target's own infrastructure.
var netRoleRules = []netRoleRule{
	// 云 WAF 同时承担加速/反代，CDN+WAF 一起标注
	{[]string{".saaswaf.com", ".dbappwaf.cn"}, []string{"CDN", "WAF", "saaswaf"}},
	{[]string{".yundunwaf.com", ".aliyundun.com"}, []string{"CDN", "WAF", "aliyun"}},
	{[]string{".qcloudwaf.com", ".waf.tencent-cloud.com"}, []string{"CDN", "WAF", "tencent-cloud"}},
	{[]string{".bcewaf.com"}, []string{"CDN", "WAF", "baidu"}},
	{[]string{".safedog.cn"}, []string{"WAF", "safedog"}},
	{[]string{".incapdns.net"}, []string{"CDN", "WAF", "imperva"}},
	{[]string{".sucuri.net"}, []string{"CDN", "WAF", "sucuri"}},

	// 纯 CDN / 云厂商共享边缘
	{[]string{".cloudflare.net", ".cloudflare.com", ".yunjiasu.com"}, []string{"CDN", "cloudflare"}},
	{[]string{".alicdn.com", ".kunlunaq.com", ".kunlunsl.com"}, []string{"CDN", "aliyun"}},
	{[]string{".cdn.dnsv1.com", ".tencentcdn.com", ".tlivecdn.com"}, []string{"CDN", "tencent-cloud"}},
	{[]string{".wscdns.com", ".wscloudcdn.com", ".lxdns.com", ".ourwebcdn.com", ".ccgslb.com", ".wcloudcdn.com", ".cdntip.com"}, []string{"CDN", "wangsu"}},
	{[]string{".akamai.net", ".akamaiedge.net", ".edgekey.net", ".edgesuite.net", ".akamaized.net"}, []string{"CDN", "akamai"}},
	{[]string{".fastly.net", ".fastlylb.net"}, []string{"CDN", "fastly"}},
	{[]string{".cloudfront.net"}, []string{"CDN", "aws"}},
	{[]string{".azureedge.net", ".azurefd.net"}, []string{"CDN", "azure"}},
	{[]string{".bdydns.com", ".bcecdn.com"}, []string{"CDN", "baidu"}},
	{[]string{".hwcdn.net", ".hwclouds-dns.com", ".cdnhwc1.com", ".cdnhwc2.com"}, []string{"CDN", "huawei-cloud"}},
	{[]string{".qiniudns.com", ".qbox.me"}, []string{"CDN", "qiniu"}},
	{[]string{".upyun.com"}, []string{"CDN", "upyun"}},
	{[]string{".ksyuncdn.com", ".ks-cdn.com"}, []string{"CDN", "kingsoft-cloud"}},
	{[]string{".jiashule.com"}, []string{"CDN", "jiasule"}},
	{[]string{".360wzb.com", ".qhcdn.com", ".360safedns.com"}, []string{"CDN", "qihoo360"}},
	{[]string{".cdngc.net", ".panthercdn.com"}, []string{"CDN", "cdnetworks"}},
	{[]string{".stackpathdns.com"}, []string{"CDN", "stackpath"}},
}

// detectNetworkTags returns the deduplicated CDN/WAF/provider tags implied by a
// DNS CNAME chain (order: first rule match wins per tag).
func detectNetworkTags(cnames []string) []string {
	var out []string
	seen := make(map[string]bool, 4)
	for _, cname := range cnames {
		c := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(cname)), ".")
		if c == "" {
			continue
		}
		for _, rule := range netRoleRules {
			for _, sfx := range rule.suffixes {
				if c == strings.TrimPrefix(sfx, ".") || strings.HasSuffix(c, sfx) {
					for _, tag := range rule.tags {
						if !seen[tag] {
							seen[tag] = true
							out = append(out, tag)
						}
					}
					break
				}
			}
		}
	}
	return out
}
