package db

import "testing"

func rule(kind, pattern string, enabled bool) AssetInterceptRule {
	return AssetInterceptRule{Kind: kind, Pattern: pattern, Enabled: enabled}
}

func TestMatchAssetInterceptRules(t *testing.T) {
	cases := []struct {
		name    string
		rules   []AssetInterceptRule
		domains []string
		ips     []string
		urls    []string
		want    bool
		wantVal string
	}{
		{"内置模糊政府域名命中", []AssetInterceptRule{rule("fuzzy_domain", ".gov.cn", true)},
			[]string{"www.beijing.gov.cn"}, nil, nil, true, "www.beijing.gov.cn"},
		{"模糊教育域名命中", []AssetInterceptRule{rule("fuzzy_domain", ".edu", true)},
			[]string{"mit.edu"}, nil, nil, true, "mit.edu"},
		{"全等域名命中大小写不敏感", []AssetInterceptRule{rule("exact_domain", "Example.com", true)},
			[]string{"example.com"}, nil, nil, true, "example.com"},
		{"全等域名不命中子域", []AssetInterceptRule{rule("exact_domain", "example.com", true)},
			[]string{"a.example.com"}, nil, nil, false, ""},
		{"全等IP命中", []AssetInterceptRule{rule("exact_ip", "203.0.113.5", true)},
			nil, []string{"203.0.113.5"}, nil, true, "203.0.113.5"},
		{"模糊IP前缀命中", []AssetInterceptRule{rule("fuzzy_ip", "203.0.113.", true)},
			nil, []string{"203.0.113.99"}, nil, true, "203.0.113.99"},
		{"模糊IP按八位组边界不误伤", []AssetInterceptRule{rule("fuzzy_ip", "192.168.1.1", true)},
			nil, []string{"192.168.1.10"}, nil, false, ""},
		{"模糊IP全等命中", []AssetInterceptRule{rule("fuzzy_ip", "192.168.1.1", true)},
			nil, []string{"192.168.1.1"}, nil, true, "192.168.1.1"},
		{"模糊政府域名不误伤相似前缀", []AssetInterceptRule{rule("fuzzy_domain", ".gov", true)},
			[]string{"cdn.govcdn.com"}, nil, nil, false, ""},
		{"模糊教育域名不误伤相似前缀", []AssetInterceptRule{rule("fuzzy_domain", ".edu", true)},
			[]string{"my.education.com"}, nil, nil, false, ""},
		{"模糊域名按标签边界命中", []AssetInterceptRule{rule("fuzzy_domain", ".gov", true)},
			[]string{"x.gov.hk"}, nil, nil, true, "x.gov.hk"},
		{"全等域名忽略末尾点", []AssetInterceptRule{rule("exact_domain", "example.com", true)},
			[]string{"example.com."}, nil, nil, true, "example.com."},
		{"CIDR 命中", []AssetInterceptRule{rule("cidr", "192.168.0.0/16", true)},
			nil, []string{"192.168.5.20"}, nil, true, "192.168.5.20"},
		{"CIDR 不命中", []AssetInterceptRule{rule("cidr", "192.168.0.0/16", true)},
			nil, []string{"10.0.0.1"}, nil, false, ""},
		{"全等URL命中", []AssetInterceptRule{rule("exact_url", "https://a.gov.cn/login", true)},
			nil, nil, []string{"https://a.gov.cn/login"}, true, "https://a.gov.cn/login"},
		{"模糊URL命中路径", []AssetInterceptRule{rule("fuzzy_url", "/admin", true)},
			nil, nil, []string{"https://x.com/admin/panel"}, true, "https://x.com/admin/panel"},
		{"禁用规则不命中", []AssetInterceptRule{rule("fuzzy_domain", ".gov.cn", false)},
			[]string{"www.gov.cn"}, nil, nil, false, ""},
		{"无规则不命中", nil, []string{"www.gov.cn"}, nil, nil, false, ""},
		{"空pattern不命中", []AssetInterceptRule{rule("fuzzy_domain", "  ", true)},
			[]string{"www.gov.cn"}, nil, nil, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, val, ok := MatchAssetInterceptRules(c.rules, c.domains, c.ips, c.urls)
			if ok != c.want {
				t.Fatalf("命中 = %v, 期望 %v (rule=%+v)", ok, c.want, r)
			}
			if ok && val != c.wantVal {
				t.Fatalf("命中值 = %q, 期望 %q", val, c.wantVal)
			}
		})
	}
}

func TestEvaluateAssetGate(t *testing.T) {
	global := []AssetInterceptRule{rule("fuzzy_domain", ".gov.cn", true)}
	allow := []AssetInterceptRule{rule("fuzzy_domain", "example.com", true)}

	// 1. 命中全局拦截 → 拒绝（拦截原因优先）。
	if d := EvaluateAssetGate(global, nil, allow, []string{"www.gov.cn"}, nil, nil); d.Allowed {
		t.Fatal("命中全局拦截规则应被拒绝")
	}

	// 2. 未命中拦截、有允许规则但不命中 → 拒绝（不允许）。
	d := EvaluateAssetGate(global, nil, allow, []string{"foo.other.com"}, nil, nil)
	if d.Allowed {
		t.Fatal("有白名单且不命中应被拒绝")
	}
	if d.Reason == "" {
		t.Fatal("拒绝应带原因")
	}

	// 3. 未命中拦截、命中允许规则 → 放行。
	if d := EvaluateAssetGate(global, nil, allow, []string{"api.example.com"}, nil, nil); !d.Allowed {
		t.Fatal("命中白名单应放行")
	}

	// 4. 无允许规则（白名单未启用）→ 未命中全局拦截即放行。
	if d := EvaluateAssetGate(global, nil, nil, []string{"foo.other.com"}, nil, nil); !d.Allowed {
		t.Fatal("无白名单时未命中全局拦截应放行")
	}

	// 5. 允许规则全部禁用 → 视为白名单未启用，放行。
	disabledAllow := []AssetInterceptRule{rule("fuzzy_domain", "example.com", false)}
	if d := EvaluateAssetGate(nil, nil, disabledAllow, []string{"foo.other.com"}, nil, nil); !d.Allowed {
		t.Fatal("白名单全禁用时应放行")
	}

	// 6. 任务级允许覆盖全局拦截：客户本身就是 .gov.cn 时，任务白名单可放行。
	if d := EvaluateAssetGate(
		[]AssetInterceptRule{rule("fuzzy_domain", ".gov.cn", true)},
		nil,
		[]AssetInterceptRule{rule("exact_domain", "www.gov.cn", true)},
		[]string{"www.gov.cn"}, nil, nil,
	); !d.Allowed {
		t.Fatal("任务级允许应覆盖全局拦截")
	}

	// 7. 任务级拦截最强：同时命中任务拦截与任务允许 → 拒绝。
	if d := EvaluateAssetGate(
		nil,
		[]AssetInterceptRule{rule("fuzzy_domain", ".gov.cn", true)},
		[]AssetInterceptRule{rule("fuzzy_domain", ".gov.cn", true)},
		[]string{"www.gov.cn"}, nil, nil,
	); d.Allowed {
		t.Fatal("任务级拦截应优先于任务级允许")
	}

	// 8. 拒绝结果带上命中的具体目标，供请求侧提示。
	d = EvaluateAssetGate(global, nil, nil, []string{"www.gov.cn"}, nil, nil)
	if d.Allowed || d.Target != "www.gov.cn" {
		t.Fatalf("拒绝应返回命中的目标，得到 %+v", d)
	}
}

func TestAssetInterceptCandidates(t *testing.T) {
	// 只带 URL 的服务资产：host 应被拆出并归入域名候选，从而被 fuzzy_domain 命中。
	a := &Asset{Type: "service", URL: "https://portal.beijing.gov.cn:8443/app"}
	domains, _, urls := a.interceptCandidates()
	if len(urls) != 1 || urls[0] != a.URL {
		t.Fatalf("urls = %v", urls)
	}
	found := false
	for _, d := range domains {
		if d == "portal.beijing.gov.cn" {
			found = true
		}
	}
	if !found {
		t.Fatalf("URL host 未拆入域名候选: %v", domains)
	}
	r, _, ok := MatchAssetInterceptRules([]AssetInterceptRule{rule("fuzzy_domain", ".gov.cn", true)}, domains, nil, urls)
	if !ok {
		t.Fatalf("仅带 URL 的政府服务资产应被 fuzzy_domain 命中, rule=%+v", r)
	}

	// URL host 是 IP 时应归入 IP 候选，可被 CIDR 命中。
	b := &Asset{Type: "service", URL: "http://10.1.2.3/x"}
	_, ips, _ := b.interceptCandidates()
	if r, _, ok := MatchAssetInterceptRules([]AssetInterceptRule{rule("cidr", "10.0.0.0/8", true)}, nil, ips, nil); !ok {
		t.Fatalf("URL 中的 IP 应被 CIDR 命中, ips=%v rule=%+v", ips, r)
	}
}
