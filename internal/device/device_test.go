package device

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPersistsIdentity(t *testing.T) {
	dir := t.TempDir()

	d1, err := Load(dir)
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if d1.HWID == "" || d1.UUID == "" {
		t.Fatalf("empty identity: hwid=%q uuid=%q", d1.HWID, d1.UUID)
	}

	// Second Load must return the same values.
	d2, err := Load(dir)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if d2.HWID != d1.HWID {
		t.Errorf("HWID changed: %q → %q", d1.HWID, d2.HWID)
	}
	if d2.UUID != d1.UUID {
		t.Errorf("UUID changed: %q → %q", d1.UUID, d2.UUID)
	}
}

func TestLoadRecoversFromCorruption(t *testing.T) {
	dir := t.TempDir()

	// Write corrupted JSON.
	if err := os.WriteFile(filepath.Join(dir, "device.json"), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}

	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load after corruption: %v", err)
	}
	if d.HWID == "" || d.UUID == "" {
		t.Fatalf("identity not regenerated after corruption")
	}
}

func TestLoadEmptyJSON(t *testing.T) {
	dir := t.TempDir()

	// Write valid JSON with empty fields.
	data, _ := json.Marshal(ID{})
	if err := os.WriteFile(filepath.Join(dir, "device.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	d, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.HWID == "" || d.UUID == "" {
		t.Fatalf("identity not regenerated from empty JSON")
	}
}

func TestHWIDFormat(t *testing.T) {
	hwid := newHWID()
	// Must be 8-4-4-4-12 hex (36 chars).
	if len(hwid) != 36 {
		t.Errorf("HWID length = %d, want 36: %q", len(hwid), hwid)
	}
	for i, c := range hwid {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				t.Errorf("expected '-' at position %d, got %q", i, string(c))
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("invalid hex at position %d: %q", i, string(c))
			}
		}
	}
}

func TestUUIDFormat(t *testing.T) {
	uuid := newUUID()
	if len(uuid) != 36 {
		t.Errorf("UUID length = %d, want 36: %q", len(uuid), uuid)
	}
	// Version nibble must be 4.
	if uuid[14] != '4' {
		t.Errorf("UUID version nibble = %q, want '4': %q", string(uuid[14]), uuid)
	}
}

func TestHeaders(t *testing.T) {
	d := &ID{HWID: "deadbeef-1234-5678-9abc-def012345678", UUID: "00000000-0000-4000-8000-000000000000"}
	h := d.Headers()
	if h.Get("x-device-os") != "iOS" {
		t.Errorf("x-device-os = %q", h.Get("x-device-os"))
	}
	if h.Get("x-hwid") != d.HWID {
		t.Errorf("x-hwid = %q", h.Get("x-hwid"))
	}
	if h.Get("x-client") != "INCY" {
		t.Errorf("x-client = %q", h.Get("x-client"))
	}
	if h.Get("x-uuid") != d.UUID {
		t.Errorf("x-uuid = %q", h.Get("x-uuid"))
	}
}

func TestHWIDStable(t *testing.T) {
	// Same hostname must produce same HWID.
	hw1 := newHWID()
	hw2 := newHWID()
	if hw1 != hw2 {
		t.Errorf("HWID not stable: %q vs %q", hw1, hw2)
	}
}

func TestUUIDRandom(t *testing.T) {
	// Two UUIDs should differ (probabilistic — false positive rate ~2^-122).
	u1 := newUUID()
	u2 := newUUID()
	if u1 == u2 {
		t.Errorf("UUID collision: both %q", u1)
	}
}
