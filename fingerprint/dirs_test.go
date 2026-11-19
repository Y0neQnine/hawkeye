package fingerprint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDetectDefaultPage(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		noMatch bool
		want    bool
		kind    string
	}{
		{"nginx welcome", 200, `<html><title>Welcome to nginx!</title><body><h1>Welcome to nginx!</h1></body></html>`, true, true, "nginx"},
		{"apache default", 200, `<html><body><h1>It works!</h1></body></html>`, true, true, "apache"},
		{"baota empty vhost", 200, `<html><title>站点创建成功</title></html>`, true, true, "baota"},
		{"listing", 200, `<html><title>Index of /</title><h1>Index of /</h1></html>`, true, true, "listing"},
		{"spring whitelabel", 200, `<html><h1>Whitelabel Error Page</h1></html>`, false, true, "spring"},
		{"nginx 404", 404, `<html><center><h1>404 Not Found</h1></center><center>nginx</center></html>`, true, true, "nginx-404"},
		{"blank", 200, ``, true, true, "blank"},
		{"real site", 200, strings.Repeat("<div>业务系统内容</div>", 100), false, false, ""},
		{"403 is not a default page", 403, `denied`, true, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, kind := detectDefaultPage(c.status, []byte(c.body), c.noMatch)
			if got != c.want || kind != c.kind {
				t.Fatalf("detectDefaultPage = (%v,%q), want (%v,%q)", got, kind, c.want, c.kind)
			}
		})
	}
}

func TestScanDirectoriesFindsBackend(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`<html><title>Welcome to nginx!</title><h1>Welcome to nginx!</h1></html>`))
	})
	mux.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "rememberMe=abc")
		w.Write([]byte(`<html><head><title>后台登录</title></head><body>login</body></html>`))
	})
	mux.HandleFunc("/actuator/env", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := ScanDirectories(context.Background(), srv.Client(), srv.URL, DirScanOptions{
		ExtraPaths:  []string{"/custom/panel"},
		Delay:       time.Millisecond,
		Concurrency: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]DirFinding{}
	for _, f := range res.Findings {
		byPath[f.Path] = f
	}
	admin, ok := byPath["/admin/login"]
	if !ok || admin.Kind != "admin" || admin.StatusCode != 200 || admin.PageTitle != "后台登录" {
		t.Fatalf("admin finding = %+v (all=%+v)", admin, res.Findings)
	}
	if !containsString(admin.Technologies, "Apache Shiro") {
		t.Fatalf("admin technologies = %v, want Apache Shiro", admin.Technologies)
	}
	act, ok := byPath["/actuator/env"]
	if !ok || act.StatusCode != 401 || act.Kind != "console" {
		t.Fatalf("actuator finding = %+v", act)
	}
	if !containsString(res.Technologies, "Apache Shiro") {
		t.Fatalf("result technologies = %v, want union including Apache Shiro", res.Technologies)
	}
	// The custom path was probed (it 404s here, so it must NOT be a finding).
	if _, reported := byPath["/custom/panel"]; reported {
		t.Fatalf("404 custom path must not be reported: %+v", res.Findings)
	}
}

func TestScanDirectoriesSoft404IsSilent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><title>same page</title><body>everything looks like this</body></html>`))
	}))
	defer srv.Close()

	res, err := ScanDirectories(context.Background(), srv.Client(), srv.URL, DirScanOptions{
		ExtraPaths: []string{"admin", "login"},
		MaxPaths:   5,
		Delay:      time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("soft-404 site must yield no findings, got %+v", res.Findings)
	}
}

func TestScanDirectoriesBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/hawkeye-") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	res, err := ScanDirectories(context.Background(), srv.Client(), srv.URL, DirScanOptions{
		MaxPaths:    10,
		Delay:       time.Millisecond,
		Concurrency: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Backoff < time.Minute {
		t.Fatalf("Backoff = %s, want >= 1m", res.Backoff)
	}
	if !res.Aborted {
		t.Fatalf("scan should abort on 429: %+v", res)
	}
}
