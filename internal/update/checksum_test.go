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

func TestParseChecksum(t *testing.T) {
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	got, err := parseChecksum([]byte(digest + "  happ-linux-amd64.tar.gz\n"))
	if err != nil {
		t.Fatalf("parseChecksum: %v", err)
	}
	if got != digest {
		t.Errorf("parseChecksum = %q, want %q", got, digest)
	}

	// Uppercase digits are normalized.
	if got, err := parseChecksum([]byte(strings.ToUpper(digest) + " file")); err != nil || got != digest {
		t.Errorf("parseChecksum(uppercase) = %q, %v; want %q, nil", got, err, digest)
	}

	for _, bad := range []string{"", "   \n", "deadbeef  file", digest + "xx  file", "zz" + digest[2:] + "  file"} {
		if _, err := parseChecksum([]byte(bad)); err == nil {
			t.Errorf("parseChecksum(%q) = nil error, want error", bad)
		}
	}
}

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write temp archive: %v", err)
	}
	return path
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestVerifyChecksum(t *testing.T) {
	archive := []byte("archive payload")
	archivePath := writeTempFile(t, archive)
	good := sha256Hex(archive)

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/good.sha256":
			w.Write([]byte(good + "  happ-linux-amd64.tar.gz\n"))
		case "/bad.sha256":
			w.Write([]byte(sha256Hex([]byte("something else")) + "  happ-linux-amd64.tar.gz\n"))
		case "/empty.sha256":
			w.Write([]byte("\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &Client{allowHTTP: true, HTTP: srv.Client()}

	if err := c.verifyChecksum(context.Background(), &Asset{Name: "a.tar.gz", ChecksumURL: srv.URL + "/good.sha256"}, archivePath); err != nil {
		t.Errorf("verifyChecksum(good) = %v, want nil", err)
	}
	if err := c.verifyChecksum(context.Background(), &Asset{Name: "a.tar.gz", ChecksumURL: srv.URL + "/bad.sha256"}, archivePath); err == nil {
		t.Error("verifyChecksum(bad) = nil error, want checksum mismatch")
	} else if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("verifyChecksum(bad) error = %q, want it to mention a mismatch", err)
	}

	// A missing checksum file (404) is a hard error too.
	if err := c.verifyChecksum(context.Background(), &Asset{Name: "a.tar.gz", ChecksumURL: srv.URL + "/missing.sha256"}, archivePath); err == nil {
		t.Error("verifyChecksum(missing file) = nil error, want error")
	}
	if err := c.verifyChecksum(context.Background(), &Asset{Name: "a.tar.gz", ChecksumURL: srv.URL + "/empty.sha256"}, archivePath); err == nil {
		t.Error("verifyChecksum(empty checksum) = nil error, want error")
	}
	if err := c.verifyChecksum(context.Background(), &Asset{Name: "a.tar.gz"}, archivePath); err == nil {
		t.Error("verifyChecksum(no checksum URL) = nil error, want error")
	}

	if hits == 0 {
		t.Error("checksum server was never queried")
	}
}

func TestDownloadRejectsPlainHTTP(t *testing.T) {
	c := &Client{}
	if err := c.checkURL("http://example.com/happ.tar.gz"); err == nil {
		t.Error("checkURL(http) = nil, want refusal without the test override")
	}
	if err := c.checkURL("https://example.com/happ.tar.gz"); err != nil {
		t.Errorf("checkURL(https) = %v, want nil", err)
	}
	if err := c.checkURL("ftp://example.com/happ.tar.gz"); err == nil {
		t.Error("checkURL(ftp) = nil, want refusal")
	}
}
