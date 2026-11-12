package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assetServer serves one archive at /<name> and its checksum at /<name>.sha256.
// A corrupted checksum is returned when badSum is true.
func assetServer(t *testing.T, name string, archive []byte, badSum bool) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(archive)
	digest := hex.EncodeToString(sum[:])
	if badSum {
		digest = sha256Hex([]byte("different archive"))
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + name:
			w.Write(archive)
		case "/" + name + ".sha256":
			w.Write([]byte(digest + "  " + name + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
}

// fakeInstall lays out a temp directory containing an old happ binary.
func fakeInstall(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	exe := filepath.Join(dir, "happ")
	if err := os.WriteFile(exe, []byte("old binary contents"), 0o644); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return exe
}

func tempLeftovers(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".happ-archive-*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

func TestInstallReplacesBinary(t *testing.T) {
	const assetName = "happ-linux-amd64.tar.gz"
	binary := []byte("new binary contents")
	archive := buildTarGz(t, []archiveEntry{
		{"happ-linux-amd64/README.md", []byte("decoy")},
		{"happ-linux-amd64/happ", binary},
	})

	srv := assetServer(t, assetName, archive, false)
	defer srv.Close()

	exe := fakeInstall(t)
	c := &Client{allowHTTP: true, HTTP: srv.Client()}
	asset := &Asset{
		Name:        assetName,
		URL:         srv.URL + "/" + assetName,
		Size:        int64(len(archive)),
		ChecksumURL: srv.URL + "/" + assetName + ".sha256",
	}

	replaced, err := Install(context.Background(), c, asset, exe)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !replaced {
		t.Fatal("Install reported replaced=false, want true")
	}

	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("read installed binary: %v", err)
	}
	if string(got) != string(binary) {
		t.Errorf("installed binary = %q, want %q", got, binary)
	}
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("installed binary mode = %o, want 0755", perm)
	}

	backup, err := os.ReadFile(exe + ".old")
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(backup) != "old binary contents" {
		t.Errorf("backup contents = %q, want the previous binary", backup)
	}

	if leftovers := tempLeftovers(t, filepath.Dir(exe)); len(leftovers) != 0 {
		t.Errorf("temp archives left behind: %v", leftovers)
	}
	if _, err := os.Stat(exe + ".new"); !os.IsNotExist(err) {
		t.Errorf("%s.new left behind", exe)
	}
}

func TestInstallChecksumMismatchKeepsOriginal(t *testing.T) {
	const assetName = "happ-linux-amd64.tar.gz"
	archive := buildTarGz(t, []archiveEntry{
		{"happ-linux-amd64/happ", []byte("attacker-controlled binary")},
	})
	srv := assetServer(t, assetName, archive, true)
	defer srv.Close()

	exe := fakeInstall(t)
	c := &Client{allowHTTP: true, HTTP: srv.Client()}
	asset := &Asset{
		Name:        assetName,
		URL:         srv.URL + "/" + assetName,
		ChecksumURL: srv.URL + "/" + assetName + ".sha256",
	}

	replaced, err := Install(context.Background(), c, asset, exe)
	if err == nil {
		t.Fatal("Install with a bad checksum = nil error, want checksum mismatch")
	}
	if replaced {
		t.Fatal("Install with a bad checksum reported replaced=true")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("error = %q, want it to mention a checksum mismatch", err)
	}

	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if string(got) != "old binary contents" {
		t.Errorf("original binary changed to %q", got)
	}
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Error("checksum failure created a backup, but nothing should have been replaced")
	}
	if _, err := os.Stat(exe + ".new"); !os.IsNotExist(err) {
		t.Error("checksum failure left a .new file")
	}
	if leftovers := tempLeftovers(t, filepath.Dir(exe)); len(leftovers) != 0 {
		t.Errorf("temp archives left behind: %v", leftovers)
	}
}

func TestInstallMissingChecksumKeepsOriginal(t *testing.T) {
	const assetName = "happ-linux-amd64.tar.gz"
	archive := buildTarGz(t, []archiveEntry{
		{"happ-linux-amd64/happ", []byte("new")},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/"+assetName {
			w.Write(archive)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	exe := fakeInstall(t)
	c := &Client{allowHTTP: true, HTTP: srv.Client()}
	asset := &Asset{Name: assetName, URL: srv.URL + "/" + assetName, ChecksumURL: srv.URL + "/" + assetName + ".sha256"}

	if _, err := Install(context.Background(), c, asset, exe); err == nil {
		t.Fatal("Install without a checksum file = nil error, want error")
	}
	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old binary contents" {
		t.Errorf("original binary changed to %q", got)
	}
}

func TestInstallUnwritableTarget(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission checks do not apply")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "happ")
	if err := os.WriteFile(exe, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)

	c := &Client{}
	asset := &Asset{Name: "happ-linux-amd64.tar.gz", URL: "https://example.com/x.tar.gz"}
	_, err = Install(context.Background(), c, asset, exe)
	if err == nil {
		t.Fatal("Install into a read-only directory = nil error, want error")
	}
	if !strings.Contains(err.Error(), "root") {
		t.Errorf("error = %q, want a hint about running as root", err)
	}
}
