// Package device provides a per-machine device identity (HWID + UUID) that is
// generated once and persisted in the config directory. It also exposes the
// INCY client headers required by subscription APIs.
package device

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// ID holds the persistent device identity.
type ID struct {
	HWID string `json:"hwid"`
	UUID string `json:"uuid"`
}

// Headers returns the INCY client HTTP headers for this device.
func (d *ID) Headers() http.Header {
	h := http.Header{}
	h.Set("x-device-os", "iOS")
	h.Set("x-hwid", d.HWID)
	h.Set("x-client", "INCY")
	h.Set("x-uuid", d.UUID)
	return h
}

// Load reads the device identity from dir/device.json, or generates a new one
// and persists it. dir is typically the happ-cli config directory.
func Load(dir string) (*ID, error) {
	path := filepath.Join(dir, "device.json")
	data, err := os.ReadFile(path)
	if err == nil {
		var d ID
		if err := json.Unmarshal(data, &d); err == nil && d.HWID != "" && d.UUID != "" {
			return &d, nil
		}
		// Corrupted — regenerate.
	}

	d := &ID{
		HWID: newHWID(),
		UUID: newUUID(),
	}
	if err := d.save(dir); err != nil {
		return d, err // return identity anyway; save failure is non-fatal
	}
	return d, nil
}

func (d *ID) save(dir string) error {
	path := filepath.Join(dir, "device.json")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("device: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("device: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("device: write: %w", err)
	}
	return os.Rename(tmp, path)
}

// newHWID generates a device-bound hardware ID (8-4-4-4-12 hex).
// It uses SHA-256 of the hostname to produce a stable per-machine value.
func newHWID() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown"
	}
	h := sha256.Sum256([]byte(host + "\x00happ-hwid"))
	raw := hex.EncodeToString(h[:16]) // 32 hex chars
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		raw[0:8], raw[8:12], raw[12:16], raw[16:20], raw[20:32])
}

// newUUID generates a random UUID v4.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
