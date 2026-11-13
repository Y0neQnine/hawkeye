package fingerprint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFaviconMMH3(t *testing.T) {
	seq := make([]byte, 20)
	for i := range seq {
		seq[i] = byte(i)
	}
	cases := []struct {
		data []byte
		want string
	}{
		{[]byte("hello"), "1155597304"},
		{seq, "182740613"},
		{[]byte{0, 0, 1, 0}, "-216455174"},
	}
	for _, c := range cases {
		if got := FaviconMMH3(c.data); got != c.want {
			t.Fatalf("FaviconMMH3(%q) = %s, want %s", c.data, got, c.want)
		}
	}
}

func TestIdentifyWordPressStack(t *testing.T) {
	h := http.Header{}
	h.Set("Server", "nginx/1.24.0")
	h.Set("cf-ray", "8c0ffee")
	body := []byte(`<html><head><title>Demo</title>
<meta name="generator" content="WordPress 6.4.2">
<link rel="stylesheet" href="/wp-content/themes/a/style.css"></head></html>`)
	matches := Identify(h, []*http.Cookie{{Name: "PHPSESSID"}}, body)
	techs := Technologies(matches)
	for _, want := range []string{"Nginx 1.24.0", "WordPress 6.4.2", "PHP", "Cloudflare", "CDN/WAF", "CMS"} {
		if !containsString(techs, want) {
			t.Fatalf("technologies %v missing %q", techs, want)
		}
	}
}

func TestIdentifyWeaverCookie(t *testing.T) {
	h := http.Header{}
	matches := Identify(h, []*http.Cookie{{Name: "ecology_JSessionid"}}, []byte("<title>泛微</title>"))
	if len(matches) == 0 || matches[0].Product != "泛微 e-cology" {
		t.Fatalf("matches = %+v, want 泛微 e-cology first", matches)
	}
	if matches[0].Confidence != "high" || matches[0].Evidence == "" {
		t.Fatalf("match missing confidence/evidence: %+v", matches[0])
	}
}

func TestIdentifySeeyon(t *testing.T) {
	body := []byte(`<html><title>致远OA A8</title><script src="/seeyon/common/js/a.js"></script></html>`)
	matches := Identify(http.Header{}, nil, body)
	if !hasProduct(matches, "致远 A8") {
		t.Fatalf("matches = %+v, want 致远 A8", matches)
	}
}

func TestIdentifySpringBoot(t *testing.T) {
	body := []byte(`<html><body><h1>Whitelabel Error Page</h1></body></html>`)
	matches := Identify(http.Header{}, []*http.Cookie{{Name: "JSESSIONID"}}, body)
	if !hasProduct(matches, "Spring Boot") || !hasProduct(matches, "Java") {
		t.Fatalf("matches = %+v, want Spring Boot + Java", matches)
	}
}

func TestIdentifyGeneratorFallback(t *testing.T) {
	body := []byte(`<meta content="SomeCMS 1.0" name="generator">`)
	matches := Identify(http.Header{}, nil, body)
	if !hasProduct(matches, "SomeCMS 1.0") {
		t.Fatalf("matches = %+v, want generator product", matches)
	}
}

func TestProbe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "openresty/1.21.4.2")
		w.Header().Set("Set-Cookie", "JSESSIONID=abc")
		w.Write([]byte(`<html><head><title>内部系统</title><link rel="icon" href="/static/favicon.ico"></head>
<body>ThinkPHP</body></html>`))
	})
	mux.HandleFunc("/static/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write([]byte("hello"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	obs, err := Probe(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if obs.StatusCode != 200 || obs.PageTitle != "内部系统" {
		t.Fatalf("obs = %+v", obs)
	}
	if obs.FaviconMMH3 != "1155597304" {
		t.Fatalf("favicon hash = %q", obs.FaviconMMH3)
	}
	for _, want := range []string{"OpenResty 1.21.4.2", "ThinkPHP", "Java"} {
		if !containsString(obs.Technologies, want) {
			t.Fatalf("technologies %v missing %q", obs.Technologies, want)
		}
	}
}

func hasProduct(ms []Match, p string) bool {
	for _, m := range ms {
		if m.Product == p {
			return true
		}
	}
	return false
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
