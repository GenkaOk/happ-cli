package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type archiveEntry struct {
	name string
	data []byte
}

// buildTarGz creates an in-memory .tar.gz with the given entries.
func buildTarGz(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name:     e.name,
			Mode:     0o755,
			Size:     int64(len(e.data)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("tar header %s: %v", e.name, err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatalf("tar write %s: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// buildZip creates an in-memory .zip with the given entries.
func buildZip(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("zip create %s: %v", e.name, err)
		}
		if _, err := w.Write(e.data); err != nil {
			t.Fatalf("zip write %s: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestExtractTarGz(t *testing.T) {
	binary := []byte("\x7fELF-new-happ-binary")
	archive := buildTarGz(t, []archiveEntry{
		{"happ-linux-amd64/README.md", []byte("decoy that must be ignored")},
		{"happ-linux-amd64/LICENSE", []byte("another decoy")},
		{"happ-linux-amd64/happ", binary},
		{"happ-linux-amd64/extra", []byte("after the binary")},
	})
	archivePath := filepath.Join(t.TempDir(), "asset.tar.gz")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "happ.new")
	if err := extractBinary(archivePath, "happ-linux-amd64.tar.gz", dest); err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(got, binary) {
		t.Errorf("extracted %q, want %q", got, binary)
	}
}

func TestExtractZip(t *testing.T) {
	binary := []byte("MZ-windows-happ-binary")
	archive := buildZip(t, []archiveEntry{
		{"happ-windows-amd64/README.md", []byte("decoy")},
		{"happ-windows-amd64/happ.exe", binary},
	})
	archivePath := filepath.Join(t.TempDir(), "asset.zip")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(t.TempDir(), "happ.exe.new")
	if err := extractBinary(archivePath, "happ-windows-amd64.zip", dest); err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(got, binary) {
		t.Errorf("extracted %q, want %q", got, binary)
	}
}

func TestExtractMissingBinary(t *testing.T) {
	archive := buildTarGz(t, []archiveEntry{
		{"happ-test/README.md", []byte("no binary here")},
	})
	archivePath := filepath.Join(t.TempDir(), "asset.tar.gz")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "happ.new")
	if err := extractBinary(archivePath, "happ-test.tar.gz", dest); err == nil {
		t.Fatal("extractBinary without a happ entry = nil error, want error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("extraction failure left %s behind", dest)
	}
}
