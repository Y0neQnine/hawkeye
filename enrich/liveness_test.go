package enrich

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/projectdiscovery/dnsx/libs/dnsx"
	"golang.org/x/net/dns/dnsmessage"
)

func TestProbeTCPLivenessAlive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	res := probeTCPLiveness(context.Background(), "127.0.0.1", []int{port})
	if res.skip || !res.alive {
		t.Fatalf("res = %+v, want alive", res)
	}
}

func TestProbeTCPLivenessDead(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	res := probeTCPLiveness(context.Background(), "127.0.0.1", []int{port})
	if res.skip || res.alive {
		t.Fatalf("res = %+v, want dead", res)
	}
}

func TestProbeURLLivenessAliveOnAuthChallenge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	e := &Engine{client: srv.Client(), hostLast: map[string]time.Time{}, closed: make(chan struct{})}
	res := e.probeURLLiveness(context.Background(), srv.URL+"/")
	if res.skip || !res.alive || res.reason != "HTTP 401" {
		t.Fatalf("res = %+v, want alive HTTP 401", res)
	}
}

func TestProbeURLLivenessDeadOnClosedPort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	e := &Engine{client: &http.Client{}, hostLast: map[string]time.Time{}, closed: make(chan struct{})}
	res := e.probeURLLiveness(context.Background(), url+"/")
	if res.skip || res.alive {
		t.Fatalf("res = %+v, want dead", res)
	}
}

func TestClassifyDNSResult(t *testing.T) {
	cases := []struct {
		records int
		status  string
		alive   bool
		skip    bool
	}{
		{2, "NOERROR", true, false},
		{0, "NXDOMAIN", false, false},
		{0, "noerror", false, false},
		{0, "SERVFAIL", false, true},
		{0, "REFUSED", false, true},
		{0, "", false, true},
	}
	for _, c := range cases {
		res := classifyDNSResult(c.records, c.status)
		if res.alive != c.alive || res.skip != c.skip {
			t.Fatalf("classifyDNSResult(%d,%q) = %+v, want alive=%v skip=%v", c.records, c.status, res, c.alive, c.skip)
		}
	}
}

// startMockDNS serves A=1.2.3.4 for names ending in alive.test, NXDOMAIN otherwise.
func startMockDNS(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 1500)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			var req dnsmessage.Message
			if req.Unpack(buf[:n]) != nil {
				continue
			}
			resp := dnsmessage.Message{Header: dnsmessage.Header{
				ID: req.ID, Response: true, Authoritative: true, RecursionAvailable: true,
			}}
			resp.Questions = req.Questions
			for _, q := range req.Questions {
				if q.Type == dnsmessage.TypeA && strings.HasSuffix(q.Name.String(), "alive.test.") {
					resp.Answers = append(resp.Answers, dnsmessage.Resource{
						Header: dnsmessage.ResourceHeader{
							Name: q.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 60,
						},
						Body: &dnsmessage.AResource{A: [4]byte{1, 2, 3, 4}},
					})
				}
			}
			if len(resp.Answers) == 0 {
				resp.RCode = dnsmessage.RCodeNameError
			}
			out, err := resp.Pack()
			if err != nil {
				continue
			}
			_, _ = pc.WriteTo(out, addr)
		}
	}()
	return pc.LocalAddr().String()
}

func TestProbeDomainLivenessAgainstMockDNS(t *testing.T) {
	resolv, err := dnsx.New(dnsx.Options{
		BaseResolvers: []string{startMockDNS(t)},
		MaxRetries:    1,
		QuestionTypes: []uint16{dns.TypeA},
		Timeout:       2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res := probeDomainLiveness(resolv, "host.alive.test"); res.skip || !res.alive {
		t.Fatalf("alive host res = %+v, want alive", res)
	}
	if res := probeDomainLiveness(resolv, "host.dead.test"); res.skip || res.alive {
		t.Fatalf("dead host res = %+v, want dead", res)
	}
}

func TestProbeDomainLivenessSkipsWithoutResolver(t *testing.T) {
	if res := probeDomainLiveness(nil, "example.com"); !res.skip {
		t.Fatalf("res = %+v, want skip when resolver unavailable", res)
	}
	if res := probeDomainLiveness(nil, ""); !res.skip {
		t.Fatalf("res = %+v, want skip for empty domain", res)
	}
}

func TestProbeTaskLivenessNoop(t *testing.T) {
	var nilEngine *Engine
	nilEngine.ProbeTaskLiveness(1) // must not panic

	e := &Engine{closed: make(chan struct{})}
	e.ProbeTaskLiveness(0) // no asset store / invalid id: no goroutine
	if _, ok := e.liveSeen.Load(int64(0)); ok {
		t.Fatal("invalid task id should not mark liveSeen")
	}
}
