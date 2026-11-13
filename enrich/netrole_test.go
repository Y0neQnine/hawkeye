package enrich

import (
	"reflect"
	"testing"
)

func TestDetectNetworkTags(t *testing.T) {
	cases := []struct {
		name   string
		cnames []string
		want   []string
	}{
		{"direct host", []string{"www.example.gov.cn."}, nil},
		{"saaswaf chain", []string{"lw-gov-cn.cname.saaswaf.com."}, []string{"CDN", "WAF", "saaswaf"}},
		{"cloudflare", []string{"site.example.com.cdn.cloudflare.net."}, []string{"CDN", "cloudflare"}},
		{"dedupe across rules", []string{"a.saaswaf.com.", "b.saaswaf.com."}, []string{"CDN", "WAF", "saaswaf"}},
		{"mixed providers", []string{"a.saaswaf.com.", "b.cdn.dnsv1.com."}, []string{"CDN", "WAF", "saaswaf", "tencent-cloud"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectNetworkTags(tc.cnames)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("detectNetworkTags(%v) = %v, want %v", tc.cnames, got, tc.want)
			}
		})
	}
}
