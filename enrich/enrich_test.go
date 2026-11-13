package enrich

import (
	"testing"
	"time"
)

func TestPaceHostHonorsBlock(t *testing.T) {
	e := &Engine{
		closed:   make(chan struct{}),
		hostLast: make(map[string]time.Time),
	}
	if !e.paceHost("a.example.com") {
		t.Fatal("first probe to a host should be allowed")
	}
	e.markHostBlocked("a.example.com", time.Minute)
	if e.paceHost("a.example.com") {
		t.Fatal("a host parked for backoff must be skipped")
	}
}

func TestBackoffPark(t *testing.T) {
	cases := []struct {
		name        string
		code        int
		retryAfter  time.Duration
		wantPark    bool
		wantAtLeast time.Duration
	}{
		{"403 is path-level, never parks", 403, 0, false, 0},
		{"503 without Retry-After does not park", 503, 0, false, 0},
		{"429 parks even without Retry-After", 429, 0, true, backoffCooldown},
		{"429 honors a longer Retry-After", 429, 2 * time.Hour, true, 2 * time.Hour},
		{"503 with Retry-After parks", 503, 10 * time.Minute, true, backoffCooldown},
		{"Retry-After is capped", 429, 48 * time.Hour, true, maxBackoffCooldown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, ok := backoffPark(c.code, c.retryAfter)
			if ok != c.wantPark {
				t.Fatalf("park=%v want %v", ok, c.wantPark)
			}
			if c.wantPark && d < c.wantAtLeast {
				t.Fatalf("park duration=%s want >= %s", d, c.wantAtLeast)
			}
		})
	}
}
