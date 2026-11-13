package server

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	updateRepo = "Y0neQnine/hawkeye"
	// UpdateExitCode tells start.sh / start.bat to relaunch immediately. The next
	// process swaps the staged binary (see ApplyPendingUpdate) before serving.
	UpdateExitCode = 75

	maxUpdateReleaseBytes = 4 << 20
	maxUpdateArchiveBytes = 512 << 20
	updateCacheTTL  = time.Minute
)

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type releaseMeta struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt time.Time      `json:"published_at"`
	Prerelease  bool           `json:"prerelease"`
	Assets      []releaseAsset `json:"assets"`
}

var releaseCache struct {
	mu  sync.Mutex
	at  time.Time
	rel *releaseMeta
}

func updateHTTPClient(timeout time.Duration) *http.Client {
	proxy := http.ProxyFromEnvironment
	if raw := strings.TrimSpace(os.Getenv("HAWKEYE_UPDATE_PROXY")); raw != "" {
		if u, err := url.Parse(raw); err == nil {
			proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: proxy}}
}

func updateAPIBase() string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("HAWKEYE_UPDATE_API_BASE")), "/")
	if base == "" {
		return "https://api.github.com"
	}
	return base
}

func fetchLatestRelease(ctx context.Context) (*releaseMeta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		updateAPIBase()+"/repos/"+updateRepo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "hawkeye-update")
	if tok := strings.TrimSpace(os.Getenv("HAWKEYE_UPDATE_GITHUB_TOKEN")); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := updateHTTPClient(20 * time.Second).Do(req)
	if err != nil {
		return nil, fmt.Errorf("访问 GitHub 失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		switch resp.StatusCode {
		case http.StatusForbidden, http.StatusTooManyRequests:
			return nil, fmt.Errorf("GitHub API 限流（HTTP %d），请稍后重试，或配置 HAWKEYE_UPDATE_GITHUB_TOKEN", resp.StatusCode)
		case http.StatusNotFound:
			return nil, errors.New("未找到 Hawkeye 的 Release（HTTP 404）")
		}
		return nil, fmt.Errorf("GitHub API 返回 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var rel releaseMeta
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxUpdateReleaseBytes)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("解析 Release 信息失败：%w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return nil, errors.New("Release 信息缺少版本号")
	}
	return &rel, nil
}

func latestReleaseCached(ctx context.Context) (*releaseMeta, error) {
	releaseCache.mu.Lock()
	if releaseCache.rel != nil && time.Since(releaseCache.at) < updateCacheTTL {
		rel := releaseCache.rel
		releaseCache.mu.Unlock()
		return rel, nil
	}
	releaseCache.mu.Unlock()

	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, err
	}
	releaseCache.mu.Lock()
	releaseCache.rel = rel
	releaseCache.at = time.Now()
	releaseCache.mu.Unlock()
	return rel, nil
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func versionParts(v string) [3]int {
	var out [3]int
	core := strings.SplitN(normalizeVersion(v), "-", 2)[0]
	for i, seg := range strings.SplitN(core, ".", 3) {
		if i >= 3 {
			break
		}
		n, err := strconv.Atoi(seg)
		if err != nil {
			break
		}
		out[i] = n
	}
	return out
}

func versionNewer(current, latest string) bool {
	cur, lat := normalizeVersion(current), normalizeVersion(latest)
	if lat == "" {
		return false
	}
	if cur == "" || cur == "dev" {
		return true
	}
	if cur == lat {
		return false
	}
	c, l := versionParts(cur), versionParts(lat)
	for i := range c {
		if c[i] != l[i] {
			return l[i] > c[i]
		}
	}
	return strings.Contains(cur, "-") && !strings.Contains(lat, "-")
}

func releaseAssetName(version string) string {
	return fmt.Sprintf("hawkeye-%s-%s-%s.zip", normalizeVersion(version), runtime.GOOS, runtime.GOARCH)
}

func findAsset(assets []releaseAsset, name string) *releaseAsset {
	for i := range assets {
		if strings.EqualFold(assets[i].Name, name) {
			return &assets[i]
		}
	}
	return nil
}

func trimNotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 4000 {
		return s[:4000] + "\n…"
	}
	return s
}

func (s *Server) updateCheck(w http.ResponseWriter, r *http.Request) {
	rel, err := latestReleaseCached(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, "检查更新失败："+err.Error())
		return
	}
	latest := normalizeVersion(rel.TagName)
	assetName := releaseAssetName(latest)
	writeJSON(w, http.StatusOK, map[string]any{
		"current":        BuildVersion,
		"latest":         latest,
		"available":      versionNewer(BuildVersion, latest),
		"platform_asset": findAsset(rel.Assets, assetName) != nil,
		"asset":          assetName,
		"notes":          trimNotes(rel.Body),
		"html_url":       rel.HTMLURL,
		"published_at":   rel.PublishedAt,
	})
}

func assetURLAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	// Production releases always come from GitHub. A custom API base (local test
	// server) may serve archives itself; still require http(s).
	if updateAPIBase() != "https://api.github.com" {
		return true
	}
	if u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

func downloadTo(ctx context.Context, asset *releaseAsset, dst io.Writer, limit int64) (int64, error) {
	if asset == nil || strings.TrimSpace(asset.BrowserDownloadURL) == "" {
		return 0, errors.New("Release 缺少下载地址")
	}
	if !assetURLAllowed(asset.BrowserDownloadURL) {
		return 0, fmt.Errorf("拒绝下载非 GitHub 域名的安装包：%s", asset.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "hawkeye-update")
	resp, err := updateHTTPClient(10 * time.Minute).Do(req)
	if err != nil {
		return 0, fmt.Errorf("下载 %s 失败：%w", asset.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("下载 %s 失败：HTTP %d", asset.Name, resp.StatusCode)
	}
	if !assetURLAllowed(resp.Request.URL.String()) {
		return 0, fmt.Errorf("下载 %s 被重定向到非 GitHub 域名，已中止", asset.Name)
	}
	return io.Copy(dst, io.LimitReader(resp.Body, limit))
}

func checksumFor(sums, name string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.EqualFold(path.Base(strings.TrimPrefix(fields[1], "*")), name) {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS 中没有 %s", name)
}

func executableMagicOK(goos string, head []byte) bool {
	switch goos {
	case "linux":
		return bytes.HasPrefix(head, []byte("\x7fELF"))
	case "windows":
		return bytes.HasPrefix(head, []byte("MZ"))
	case "darwin":
		if len(head) < 4 {
			return false
		}
		be := uint32(head[0])<<24 | uint32(head[1])<<16 | uint32(head[2])<<8 | uint32(head[3])
		return be == 0xfeedface || be == 0xfeedfacf || be == 0xcafebabe || be == 0xcefaedfe || be == 0xcffaedfe
	}
	return true
}

func (s *Server) updateApply(w http.ResponseWriter, r *http.Request) {
	if !s.updateMu.TryLock() {
		writeErr(w, http.StatusConflict, "已有更新正在下载，请稍候")
		return
	}
	defer s.updateMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "检查更新失败："+err.Error())
		return
	}
	latest := normalizeVersion(rel.TagName)
	if !versionNewer(BuildVersion, latest) {
		writeErr(w, http.StatusBadRequest, "当前已是最新版本 v"+normalizeVersion(BuildVersion))
		return
	}
	archive := findAsset(rel.Assets, releaseAssetName(latest))
	if archive == nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("Release v%s 没有 %s/%s 的安装包", latest, runtime.GOOS, runtime.GOARCH))
		return
	}
	sums := findAsset(rel.Assets, "SHA256SUMS")
	if sums == nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("Release v%s 缺少 SHA256SUMS，无法校验，已中止更新", latest))
		return
	}

	var sumsBuf bytes.Buffer
	if _, err := downloadTo(ctx, sums, &sumsBuf, maxUpdateReleaseBytes+1); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	wantSum, err := checksumFor(sumsBuf.String(), archive.Name)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	zipFile, err := os.CreateTemp("", "hawkeye-update-*.zip")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "创建临时文件失败："+err.Error())
		return
	}
	zipPath := zipFile.Name()
	defer os.Remove(zipPath)

	hasher := sha256.New()
	n, err := downloadTo(ctx, archive, io.MultiWriter(zipFile, hasher), maxUpdateArchiveBytes+1)
	closeErr := zipFile.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if n > maxUpdateArchiveBytes {
		writeErr(w, http.StatusBadRequest, "安装包超过 512MB，已中止更新")
		return
	}
	gotSum := hex.EncodeToString(hasher.Sum(nil))
	if gotSum != wantSum {
		writeErr(w, http.StatusBadGateway, fmt.Sprintf("SHA256 校验失败（期望 %s，实际 %s），已中止更新", wantSum, gotSum))
		return
	}

	exe, err := os.Executable()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "定位当前程序失败："+err.Error())
		return
	}
	exeDir := filepath.Dir(exe)
	binName := "hawkeye"
	if runtime.GOOS == "windows" {
		binName = "hawkeye.exe"
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "打开安装包失败："+err.Error())
		return
	}
	defer zr.Close()
	var entry *zip.File
	for _, f := range zr.File {
		if f.Mode().IsRegular() && strings.EqualFold(path.Base(f.Name), binName) {
			entry = f
			break
		}
	}
	if entry == nil {
		writeErr(w, http.StatusBadGateway, "安装包内未找到 "+binName)
		return
	}
	if entry.UncompressedSize64 > uint64(maxUpdateArchiveBytes) {
		writeErr(w, http.StatusBadRequest, "安装包内程序体积异常，已中止更新")
		return
	}

	stagedTmp, err := os.CreateTemp(exeDir, ".hawkeye-update-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError,
			"程序目录不可写，无法自动更新："+err.Error()+"。可在服务器执行 ./update.sh（Docker 请拉取新镜像）")
		return
	}
	stagedTmpPath := stagedTmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(stagedTmpPath)
		}
	}()

	src, err := entry.Open()
	if err != nil {
		_ = stagedTmp.Close()
		writeErr(w, http.StatusBadGateway, "读取安装包失败："+err.Error())
		return
	}
	_, copyErr := io.Copy(stagedTmp, io.LimitReader(src, maxUpdateArchiveBytes+1))
	_ = src.Close()
	syncErr := stagedTmp.Sync()
	closeErr = stagedTmp.Close()
	if copyErr == nil {
		copyErr = syncErr
	}
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		writeErr(w, http.StatusInternalServerError, "写入新版本失败："+copyErr.Error())
		return
	}
	head, err := os.Open(stagedTmpPath)
	if err == nil {
		buf := make([]byte, 8)
		_, _ = io.ReadFull(head, buf)
		_ = head.Close()
		if !executableMagicOK(runtime.GOOS, buf) {
			writeErr(w, http.StatusBadGateway, "下载的程序格式与当前平台不符，已中止更新")
			return
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(stagedTmpPath, 0o755); err != nil {
			writeErr(w, http.StatusInternalServerError, "设置执行权限失败："+err.Error())
			return
		}
	}
	staged := exe + ".new"
	_ = os.Remove(staged)
	if err := os.Rename(stagedTmpPath, staged); err != nil {
		writeErr(w, http.StatusInternalServerError, "暂存新版本失败："+err.Error())
		return
	}
	cleanup = false

	log.Printf("[update] 已下载并校验 v%s（%s），退出以完成换装", latest, archive.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "current": BuildVersion, "latest": latest, "restarting": true,
	})
	updateRestart()
}

// updateRestart is a variable so tests can stage without exiting the test runner.
var updateRestart = func() {
	go func() {
		time.Sleep(1200 * time.Millisecond)
		os.Exit(UpdateExitCode)
	}()
}

// ApplyPendingUpdate swaps a staged binary into place. Call it at process start
// (before serving); when it returns true the caller must exit with UpdateExitCode
// so the supervisor launches the new executable.
func ApplyPendingUpdate() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	staged := exe + ".new"
	st, err := os.Stat(staged)
	if err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			log.Printf("[update] 换装失败（无法重命名当前程序）：%v", err)
			return false
		}
		if err := os.Rename(staged, exe); err != nil {
			_ = os.Rename(old, exe)
			log.Printf("[update] 换装失败：%v", err)
			return false
		}
		_ = os.Remove(old)
	} else if err := os.Rename(staged, exe); err != nil {
		log.Printf("[update] 换装失败：%v", err)
		return false
	}
	log.Printf("[update] 已换装新版本 %s，正在重启", exe)
	return true
}
