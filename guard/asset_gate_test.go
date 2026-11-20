package guard

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestAssetCandidates(t *testing.T) {
	cases := []struct {
		name    string
		tool    string
		input   any
		domains []string
		ips     []string
		urls    []string
	}{
		{
			name:    "WebFetch URL host",
			tool:    "WebFetch",
			input:   map[string]any{"url": "https://portal.beijing.gov.cn:8443/app"},
			domains: []string{"portal.beijing.gov.cn"},
			urls:    []string{"https://portal.beijing.gov.cn:8443/app"},
		},
		{
			name:    "Bash curl URL",
			tool:    "Bash",
			input:   map[string]any{"command": "curl -s 'https://www.gov.cn/robots.txt'"},
			domains: []string{"www.gov.cn"},
			urls:    []string{"https://www.gov.cn/robots.txt"},
		},
		{
			name:    "Bash bare domain and IP",
			tool:    "Bash",
			input:   map[string]any{"command": "nmap -Pn -p80 10.1.2.3 && dnsx -d example.edu.cn"},
			domains: []string{"example.edu.cn"},
			ips:     []string{"10.1.2.3"},
		},
		{
			name:  "query string value is not a target",
			tool:  "Bash",
			input: map[string]any{"command": "curl 'https://www.google.com/search?q=gov.cn'"},
			// host 只认 www.google.com；q=gov.cn 是查询值，不应被当成目标。
			domains: []string{"www.google.com"},
			urls:    []string{"https://www.google.com/search?q=gov.cn"},
		},
		{
			name:  "path segment is not a target",
			tool:  "Bash",
			input: map[string]any{"command": "curl 'https://x.com/a.b.gov.cn'"},
			// 路径里的 a.b.gov.cn 前导 '/'，不算独立目标。
			domains: []string{"x.com"},
			urls:    []string{"https://x.com/a.b.gov.cn"},
		},
		{
			name:    "probe_directories url",
			tool:    "probe_directories",
			input:   map[string]any{"url": "http://10.0.0.5:8080"},
			ips:     []string{"10.0.0.5"},
			urls:    []string{"http://10.0.0.5:8080"},
		},
		{
			name:  "tool without asset surface",
			tool:  "insert_assets",
			input: map[string]any{"assets": []any{"gov.cn"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, _ := json.Marshal(c.input)
			domains, ips, urls := assetCandidates(c.tool, in)
			if !reflect.DeepEqual(sortedCopy(domains), sortedCopy(c.domains)) {
				t.Fatalf("domains = %v, want %v", domains, c.domains)
			}
			if !reflect.DeepEqual(sortedCopy(ips), sortedCopy(c.ips)) {
				t.Fatalf("ips = %v, want %v", ips, c.ips)
			}
			if !reflect.DeepEqual(sortedCopy(urls), sortedCopy(c.urls)) {
				t.Fatalf("urls = %v, want %v", urls, c.urls)
			}
		})
	}
}
