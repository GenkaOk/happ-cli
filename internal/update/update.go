// Package update implements self-update from GitHub releases: it finds the
// asset that matches the running platform, verifies its published SHA-256
// checksum, extracts the binary, and atomically replaces the running
// executable.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultRepo is the GitHub repository releases are published to. The old
	// owner redirects, but the API/asset names are canonical here.
	DefaultRepo = "GenkaOk/happ-cli"

	// defaultAPIBase is the GitHub REST API root.
	defaultAPIBase = "https://api.github.com"

	// maxDownloadSize caps both the archive download and the extracted
	// binary. Real builds are a few tens of megabytes.
	maxDownloadSize = 200 << 20

	// downloadTimeout bounds a single HTTP transfer.
	downloadTimeout = 5 * time.Minute

	// maxChecksumSize caps the tiny `<asset>.sha256` file.
	maxChecksumSize = 1 << 20
)

// Asset is one downloadable release artifact plus its checksum sibling.
type Asset struct {
	Name        string
	URL         string
	Size        int64
	ChecksumURL string
}

// Release is the subset of a GitHub release this package needs.
type Release struct {
	Tag    string
	Assets []Asset
}

// Client talks to the GitHub releases API.
type Client struct {
	// HTTP is used for API and asset requests; nil means a default client
	// with a five-minute timeout.
	HTTP *http.Client
	// Repo is "owner/name"; empty means DefaultRepo.
	Repo string
	// APIBase overrides the GitHub API root (default https://api.github.com).
	// It exists so tests and mirrors can point at another server.
	APIBase string

	// allowHTTP permits plain-HTTP downloads. Only tests set it: production
	// updates must come over HTTPS.
	allowHTTP bool
}

func (c *Client) repo() string {
	if c == nil || strings.TrimSpace(c.Repo) == "" {
		return DefaultRepo
	}
	return strings.TrimSpace(c.Repo)
}

func (c *Client) apiBase() string {
	if c == nil || strings.TrimSpace(c.APIBase) == "" {
		return defaultAPIBase
	}
	return strings.TrimRight(strings.TrimSpace(c.APIBase), "/")
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: downloadTimeout}
}

// apiAsset / apiRelease mirror the GitHub JSON we consume.
type apiAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type apiRelease struct {
	TagName string     `json:"tag_name"`
	Assets  []apiAsset `json:"assets"`
}

// Latest fetches the newest published (non-draft, non-prerelease) release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	u := fmt.Sprintf("%s/repos/%s/releases/latest", c.apiBase(), c.repo())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("update: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "happ-cli")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: query GitHub releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusForbidden, http.StatusTooManyRequests:
			return nil, fmt.Errorf("update: GitHub rate limit exceeded (HTTP %d); try again later or set HAPP_UPDATE_REPO to a mirror", resp.StatusCode)
		case http.StatusNotFound:
			return nil, fmt.Errorf("update: no releases found for %s (HTTP 404); check the repository name with --repo", c.repo())
		default:
			return nil, fmt.Errorf("update: GitHub API returned HTTP %d for %s", resp.StatusCode, c.repo())
		}
	}

	var rel apiRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("update: decode release metadata: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("update: release metadata for %s has no tag", c.repo())
	}

	out := &Release{Tag: rel.TagName}
	for _, a := range rel.Assets {
		if a.Name == "" || a.BrowserDownloadURL == "" {
			continue
		}
		out.Assets = append(out.Assets, Asset{
			Name:        a.Name,
			URL:         a.BrowserDownloadURL,
			Size:        a.Size,
			ChecksumURL: a.BrowserDownloadURL + ".sha256",
		})
	}
	return out, nil
}

// assetName maps a Go platform to the release asset name built by the
// workflow. Windows ships a .zip, everything else a .tar.gz.
func assetName(goos, goarch, goarm string) (string, error) {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64", "arm64":
			return "happ-linux-" + goarch + ".tar.gz", nil
		case "arm":
			v := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(goarm)), "v")
			if v == "" {
				v = "7" // Go's default for GOARCH=arm
			}
			if v != "5" && v != "6" && v != "7" {
				return "", fmt.Errorf("update: unsupported platform linux/arm (GOARM=%q); supported: armv5, armv6, armv7", goarm)
			}
			return "happ-linux-armv" + v + ".tar.gz", nil
		case "mips", "mipsle":
			return "happ-linux-" + goarch + ".tar.gz", nil
		}
	case "darwin":
		if goarch == "amd64" || goarch == "arm64" {
			return "happ-darwin-" + goarch + ".tar.gz", nil
		}
	case "windows":
		if goarch == "amd64" || goarch == "386" {
			return "happ-windows-" + goarch + ".zip", nil
		}
	}
	return "", fmt.Errorf("update: unsupported platform %s/%s: no release asset is built for it", goos, goarch)
}

// FindAsset returns the release asset matching the given Go platform.
func (c *Client) FindAsset(rel *Release, goos, goarch, goarm string) (*Asset, error) {
	if rel == nil {
		return nil, errors.New("update: no release to search")
	}
	want, err := assetName(goos, goarch, goarm)
	if err != nil {
		return nil, err
	}
	for i := range rel.Assets {
		if rel.Assets[i].Name == want {
			return &rel.Assets[i], nil
		}
	}
	names := make([]string, 0, len(rel.Assets))
	for _, a := range rel.Assets {
		names = append(names, a.Name)
	}
	avail := "none"
	if len(names) > 0 {
		avail = strings.Join(names, ", ")
	}
	return nil, fmt.Errorf("update: release %s has no asset %s; available assets: %s", rel.Tag, want, avail)
}

// CompareVersions compares two dotted version strings ("v0.3.1", "0.3.2",
// "0.4.0-3-gabc123"). A missing "v" prefix and a pre-release/build suffix are
// tolerated; components are compared numerically, so "0.3" equals "0.3.0".
func CompareVersions(a, b string) (int, error) {
	av, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	bv, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	n := len(av)
	if len(bv) > n {
		n = len(bv)
	}
	for i := 0; i < n; i++ {
		x, y := 0, 0
		if i < len(av) {
			x = av[i]
		}
		if i < len(bv) {
			y = bv[i]
		}
		switch {
		case x < y:
			return -1, nil
		case x > y:
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersion(v string) ([]int, error) {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return nil, fmt.Errorf("update: invalid version %q", v)
	}
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("update: invalid version %q", v)
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("update: invalid version %q", v)
		}
		out = append(out, n)
	}
	return out, nil
}

// IsDevVersion reports whether v carries no usable release version, which is
// the case for binaries built without -X main.version.
func IsDevVersion(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "dev", "unknown":
		return true
	}
	return false
}

// CurrentPlatform returns the platform of the running binary. runtime exposes
// GOOS/GOARCH but not GOARM (which selects the armv5/armv6/armv7 asset), so
// GOARM is read from the build info; empty means "the Go default" (armv7).
func CurrentPlatform() (goos, goarch, goarm string) {
	return runtime.GOOS, runtime.GOARCH, buildSetting("GOARM")
}

func buildSetting(key string) string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range bi.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}

// CurrentExecutable returns the absolute path of the running binary with
// symlinks resolved, so updating through a launcher symlink replaces the real
// target instead of the link.
func CurrentExecutable() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("update: locate current executable: %w", err)
	}
	return resolvePath(p)
}

func resolvePath(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", errors.New("update: empty executable path")
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("update: resolve %s: %w", p, err)
	}
	abs, err := filepath.Abs(real)
	if err != nil {
		return "", fmt.Errorf("update: resolve %s: %w", p, err)
	}
	return abs, nil
}

// Install downloads asset, verifies its published checksum, extracts the
// binary, and replaces exePath (empty means the running executable). It
// returns true when the executable was replaced. Nothing is replaced unless
// the checksum and extraction both succeed; temporary files are always
// cleaned up.
func Install(ctx context.Context, c *Client, asset *Asset, exePath string) (bool, error) {
	if asset == nil {
		return false, errors.New("update: no asset to install")
	}
	if c == nil {
		c = &Client{}
	}
	if strings.TrimSpace(asset.URL) == "" {
		return false, fmt.Errorf("update: asset %s has no download URL", asset.Name)
	}

	exe, err := resolveExecutable(exePath)
	if err != nil {
		return false, err
	}
	dir := filepath.Dir(exe)

	// A temp file in the executable's directory keeps the final rename on the
	// same filesystem, so replacing a running binary is atomic.
	tmp, err := os.CreateTemp(dir, ".happ-archive-*")
	if err != nil {
		return false, fmt.Errorf("update: target directory %s is not writable (%v); run as root (sudo) to update a system-wide install", dir, err)
	}
	archivePath := tmp.Name()
	defer os.Remove(archivePath)
	_ = tmp.Chmod(0o644)

	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	if err := c.downloadTo(ctx, asset.URL, tmp); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("update: write %s: %w", archivePath, err)
	}
	if err := c.verifyChecksum(ctx, asset, archivePath); err != nil {
		return false, err
	}

	newPath := exe + ".new"
	defer os.Remove(newPath) // no-op after a successful rename
	if err := extractBinary(archivePath, asset.Name, newPath); err != nil {
		return false, err
	}
	if _, err := replaceBinary(newPath, exe); err != nil {
		return false, err
	}
	return true, nil
}

func resolveExecutable(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return CurrentExecutable()
	}
	return resolvePath(p)
}

// downloadTo downloads srcURL into an already-open temp file.
func (c *Client) downloadTo(ctx context.Context, srcURL string, f *os.File) error {
	if err := c.checkURL(srcURL); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return fmt.Errorf("update: build request for %s: %w", srcURL, err)
	}
	req.Header.Set("User-Agent", "happ-cli")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("update: download %s: %w", srcURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: download %s: HTTP %d%s", srcURL, resp.StatusCode, statusHint(resp.StatusCode))
	}

	n, err := io.Copy(f, io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		return fmt.Errorf("update: download %s: %w", srcURL, err)
	}
	if n > maxDownloadSize {
		return fmt.Errorf("update: download %s exceeds the %d MB limit", srcURL, maxDownloadSize>>20)
	}
	if n == 0 {
		return fmt.Errorf("update: download %s is empty", srcURL)
	}
	return nil
}

// downloadLimited fetches srcURL into memory, with a hard size cap.
func (c *Client) downloadLimited(ctx context.Context, srcURL string, limit int64) ([]byte, error) {
	if err := c.checkURL(srcURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return nil, fmt.Errorf("update: build request for %s: %w", srcURL, err)
	}
	req.Header.Set("User-Agent", "happ-cli")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: download %s: %w", srcURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: download %s: HTTP %d%s", srcURL, resp.StatusCode, statusHint(resp.StatusCode))
	}
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("update: download %s: %w", srcURL, err)
	}
	if n > limit {
		return nil, fmt.Errorf("update: download %s exceeds the %d byte limit", srcURL, limit)
	}
	return buf.Bytes(), nil
}

func statusHint(code int) string {
	switch code {
	case http.StatusForbidden, http.StatusTooManyRequests:
		return " (rate limit or access denied; try again later)"
	case http.StatusNotFound:
		return " (not found)"
	}
	return ""
}

func (c *Client) checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("update: invalid download URL %q", raw)
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && c != nil && c.allowHTTP {
		return nil
	}
	return fmt.Errorf("update: refusing non-HTTPS download URL %q", raw)
}

// verifyChecksum downloads `<asset>.sha256` and compares its first field with
// the archive's SHA-256. A mismatch is fatal: nothing is replaced.
func (c *Client) verifyChecksum(ctx context.Context, asset *Asset, archivePath string) error {
	if strings.TrimSpace(asset.ChecksumURL) == "" {
		return fmt.Errorf("update: no checksum published for %s; refusing to install", asset.Name)
	}
	data, err := c.downloadLimited(ctx, asset.ChecksumURL, maxChecksumSize)
	if err != nil {
		return fmt.Errorf("update: fetch checksum for %s: %w", asset.Name, err)
	}
	want, err := parseChecksum(data)
	if err != nil {
		return fmt.Errorf("update: checksum for %s: %w", asset.Name, err)
	}
	got, err := fileSHA256(archivePath)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("update: checksum mismatch for %s: expected %s, got %s; the download was discarded and nothing was replaced", asset.Name, want, got)
	}
	return nil
}

// parseChecksum extracts the hex digest from a `sha256sum`-style file, whose
// first field is the digest and whose second is the file name.
func parseChecksum(data []byte) (string, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", errors.New("checksum file is empty")
	}
	sum := strings.ToLower(fields[0])
	if len(sum) != sha256.Size*2 {
		return "", fmt.Errorf("malformed checksum %q", fields[0])
	}
	if _, err := hex.DecodeString(sum); err != nil {
		return "", fmt.Errorf("malformed checksum %q", fields[0])
	}
	return sum, nil
}

func fileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", fmt.Errorf("update: open archive: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("update: hash archive: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// isBinaryName reports whether an archive entry is the happ binary. Both
// release archives nest it one level down as happ-<name>/happ[.exe].
func isBinaryName(name string) bool {
	base := path.Base(filepath.ToSlash(name))
	return base == "happ" || base == "happ.exe"
}

// extractBinary extracts the first happ entry of archive into destPath. The
// asset name picks the format: .zip for Windows, .tar.gz otherwise.
func extractBinary(archivePath, assetName, destPath string) error {
	if strings.HasSuffix(strings.ToLower(assetName), ".zip") {
		return extractZip(archivePath, destPath)
	}
	return extractTarGz(archivePath, destPath)
}

func extractTarGz(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("update: open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("update: read archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("update: read archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		if !isBinaryName(hdr.Name) {
			continue
		}
		return writeExtracted(io.LimitReader(tr, maxDownloadSize+1), destPath)
	}
	return errors.New("update: archive does not contain a happ binary")
}

func extractZip(archivePath, destPath string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("update: open archive: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isBinaryName(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("update: read archive entry %s: %w", f.Name, err)
		}
		err = writeExtracted(io.LimitReader(rc, maxDownloadSize+1), destPath)
		rc.Close()
		return err
	}
	return errors.New("update: archive does not contain a happ binary")
}

func writeExtracted(r io.Reader, destPath string) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("update: write %s: %w", destPath, err)
	}
	n, err := io.Copy(out, r)
	closeErr := out.Close()
	if err != nil {
		return fmt.Errorf("update: extract binary: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("update: extract binary: %w", closeErr)
	}
	if n > maxDownloadSize {
		return fmt.Errorf("update: extracted binary exceeds the %d MB limit", maxDownloadSize>>20)
	}
	if n == 0 {
		return errors.New("update: extracted binary is empty")
	}
	return nil
}

// replaceBinary swaps newPath into exePath and returns the backup path.
//
// POSIX renames are atomic over an existing file, so the current binary is
// copied to <exe>.old (best effort) and the new one renamed into place. On
// Windows the running image cannot be overwritten, so the old binary is
// renamed aside first and restored if the second rename fails.
func replaceBinary(newPath, exePath string) (string, error) {
	backup := exePath + ".old"

	if runtime.GOOS == "windows" {
		if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("update: remove stale backup %s: %w", backup, err)
		}
		if err := os.Rename(exePath, backup); err != nil {
			return "", fmt.Errorf("update: cannot move the running binary aside (%v); run as administrator", err)
		}
		if err := os.Rename(newPath, exePath); err != nil {
			if restoreErr := os.Rename(backup, exePath); restoreErr != nil {
				return "", fmt.Errorf("update: install %s: %w; restoring the previous build also failed (%v) — it is still at %s", exePath, err, restoreErr, backup)
			}
			return "", fmt.Errorf("update: install %s: %w (previous build restored)", exePath, err)
		}
		return backup, nil
	}

	if err := copyFile(exePath, backup); err != nil {
		// A missing backup must not block a working update; drop any partial copy.
		_ = os.Remove(backup)
		backup = ""
	}
	if err := os.Chmod(newPath, 0o755); err != nil {
		return "", fmt.Errorf("update: chmod %s: %w", newPath, err)
	}
	if err := os.Rename(newPath, exePath); err != nil {
		return "", fmt.Errorf("update: replace %s: %w", exePath, err)
	}
	return backup, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
