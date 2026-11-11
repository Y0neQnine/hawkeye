package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// 未配置任何测绘 Key 时，每个 provider 都必须返回明确的「未配置」提示，
// 而不是发起网络请求或暴露内部错误。
func TestReconSearchMissingCredentials(t *testing.T) {
	s := &Server{m: &Manager{}}
	for _, provider := range []string{"fofa", "quake", "hunter", "shodan"} {
		in, _ := json.Marshal(map[string]string{"provider": provider, "query": `domain="example.com"`})
		res, err := s.runReconSearch(context.Background(), in)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", provider, err)
		}
		if !res.IsError {
			t.Fatalf("%s: want error result for missing credentials", provider)
		}
		if !strings.Contains(res.Flatten(), "未配置") {
			t.Fatalf("%s: want 未配置 hint, got %q", provider, res.Flatten())
		}
	}
}

func TestReconSearchRejectsUnknownProvider(t *testing.T) {
	s := &Server{m: &Manager{}}
	in, _ := json.Marshal(map[string]string{"provider": "google", "query": "example.com"})
	res, err := s.runReconSearch(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Flatten(), "provider 需为") {
		t.Fatalf("want provider validation error, got %q", res.Flatten())
	}
}

func TestReconSearchRequiresQuery(t *testing.T) {
	s := &Server{m: &Manager{}}
	in, _ := json.Marshal(map[string]string{"provider": "fofa"})
	res, err := s.runReconSearch(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError || !strings.Contains(res.Flatten(), "query 不能为空") {
		t.Fatalf("want query validation error, got %q", res.Flatten())
	}
}
