package db

import "testing"

func TestRootDomainAsset(t *testing.T) {
	cases := []struct {
		host string
		want string
	}{
		{"example.com", "example.com"},
		{"WWW.Example.COM.", "example.com"},
		{"www.example.co.uk", "example.co.uk"},
		{"localhost", "localhost"},
		{"14.152.38.231", ""},
		{"2001:db8::1", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := RootDomainAsset(c.host); got != c.want {
			t.Errorf("RootDomainAsset(%q) = %q, want %q", c.host, got, c.want)
		}
	}
}
