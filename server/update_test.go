package server

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestVersionNewer(t *testing.T) {
	cases := []struct {
		cur, latest string
		want        bool
	}{
		{"0.2.0", "0.3.0", true},
		{"v0.2.0", "v0.2.1", true},
		{"0.3.0", "0.3.0", false},
		{"0.3.1", "0.3.0", false},
		{"dev", "0.3.0", true},
		{"", "0.3.0", true},
		{"0.3.0", "", false},
		{"0.3.0-rc1", "0.3.0", true},
	}
	for _, c := range cases {
		if got := versionNewer(c.cur, c.latest); got != c.want {
			t.Errorf("versionNewer(%q, %q) = %v, want %v", c.cur, c.latest, got, c.want)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	sums := "abc123  hawkeye-0.3.0-linux-amd64.zip\ndef456 *other.zip\n"
	got, err := checksumFor(sums, "hawkeye-0.3.0-linux-amd64.zip")
	if err != nil || got != "abc123" {
		t.Fatalf("checksumFor = %q, %v", got, err)
	}
	if _, err := checksumFor(sums, "missing.zip"); err == nil {
		t.Fatal("expected error for missing entry")
	}
}

func TestExecutableMagic(t *testing.T) {
	cases := []struct {
		goos string
		head []byte
		want bool
	}{
		{"linux", []byte("\x7fELF...."), true},
		{"linux", []byte("MZ......"), false},
		{"windows", []byte("MZ......"), true},
		{"darwin", []byte{0xcf, 0xfa, 0xed, 0xfe}, true},
	}
	for _, c := range cases {
		if got := executableMagicOK(c.goos, c.head); got != c.want {
			t.Errorf("executableMagicOK(%q, %q) = %v, want %v", c.goos, c.head, got, c.want)
		}
	}
}

type fakeRelease struct {
	version  string
	binBytes []byte
	badSum   bool
}

func newFakeReleaseServer(t *testing.T, rel fakeRelease) *httptest.Server {
	t.Helper()
	binName := "hawkeye"
	if runtime.GOOS == "windows" {
		binName = "hawkeye.exe"
	}
	zipName := fmt.Sprintf("hawkeye-%s-%s-%s.zip", rel.version, runtime.GOOS, runtime.GOARCH)

	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create(fmt.Sprintf("hawkeye-%s-%s-%s/%s", rel.version, runtime.GOOS, runtime.GOARCH, binName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(rel.binBytes); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipBytes := archive.Bytes()
	sum := sha256.Sum256(zipBytes)
	if rel.badSum {
		sum[0] ^= 0xff
	}
	sums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), zipName)

	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/repos/"+updateRepo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v" + rel.version,
			"html_url": "https://example.com/release",
			"body":     "test release",
			"assets": []map[string]any{
				{"name": zipName, "browser_download_url": srv.URL + "/assets/" + zipName, "size": len(zipBytes)},
				{"name": "SHA256SUMS", "browser_download_url": srv.URL + "/assets/SHA256SUMS", "size": len(sums)},
			},
		})
	})
	mux.HandleFunc("/assets/"+zipName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	})
	mux.HandleFunc("/assets/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sums))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func fakeBinary() []byte {
	switch runtime.GOOS {
	case "linux":
		return append([]byte("\x7fELF"), bytes.Repeat([]byte{0}, 128)...)
	case "windows":
		return append([]byte("MZ"), bytes.Repeat([]byte{0}, 128)...)
	default:
		return append([]byte{0xcf, 0xfa, 0xed, 0xfe}, bytes.Repeat([]byte{0}, 128)...)
	}
}

func TestUpdateCheckAndApply(t *testing.T) {
	rel := fakeRelease{version: "9.9.9", binBytes: fakeBinary()}
	srv := newFakeReleaseServer(t, rel)

	t.Setenv("HAWKEYE_UPDATE_API_BASE", srv.URL)
	oldVersion := BuildVersion
	BuildVersion = "0.0.1"
	t.Cleanup(func() { BuildVersion = oldVersion })
	releaseCache.mu.Lock()
	releaseCache.rel = nil
	releaseCache.mu.Unlock()

	s := &Server{}
	rec := httptest.NewRecorder()
	s.updateCheck(rec, httptest.NewRequest(http.MethodGet, "/api/system/update/check", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("check status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var check struct {
		Available     bool   `json:"available"`
		Latest        string `json:"latest"`
		PlatformAsset bool   `json:"platform_asset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		t.Fatal(err)
	}
	if !check.Available || check.Latest != "9.9.9" || !check.PlatformAsset {
		t.Fatalf("unexpected check response: %+v", check)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	staged := exe + ".new"
	_ = os.Remove(staged)
	t.Cleanup(func() { _ = os.Remove(staged) })

	oldRestart := updateRestart
	updateRestart = func() {}
	t.Cleanup(func() { updateRestart = oldRestart })

	rec = httptest.NewRecorder()
	s.updateApply(rec, httptest.NewRequest(http.MethodPost, "/api/system/update/apply", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("staged binary missing: %v", err)
	}
	if !bytes.Equal(got, rel.binBytes) {
		t.Fatalf("staged binary content mismatch (len %d vs %d)", len(got), len(rel.binBytes))
	}
}

func TestUpdateApplyRejectsBadChecksum(t *testing.T) {
	rel := fakeRelease{version: "9.9.9", binBytes: fakeBinary(), badSum: true}
	srv := newFakeReleaseServer(t, rel)

	t.Setenv("HAWKEYE_UPDATE_API_BASE", srv.URL)
	oldVersion := BuildVersion
	BuildVersion = "0.0.1"
	t.Cleanup(func() { BuildVersion = oldVersion })

	oldRestart := updateRestart
	called := false
	updateRestart = func() { called = true }
	t.Cleanup(func() { updateRestart = oldRestart })
	t.Cleanup(func() { releaseCache.mu.Lock(); releaseCache.rel = nil; releaseCache.mu.Unlock() })

	s := &Server{}
	rec := httptest.NewRecorder()
	s.updateApply(rec, httptest.NewRequest(http.MethodPost, "/api/system/update/apply", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("expected failure, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SHA256") {
		t.Fatalf("expected checksum error, got %s", rec.Body.String())
	}
	if called {
		t.Fatal("restart must not be scheduled after a failed verification")
	}
	if _, err := os.Stat(mustExecutable(t) + ".new"); err == nil {
		t.Fatal("staged binary must not exist after failed verification")
	}
}

func mustExecutable(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}
