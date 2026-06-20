package profile

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseBodyBase64List(t *testing.T) {
	links := strings.Join([]string{
		"vless://uuid-1@a.example.com:443?type=tcp&security=reality&pbk=k#A",
		"trojan://" + "pw" + "@b.example.com:443#B",
		"", // blank line should be skipped
		"ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:p")) + "@c.example.com:8388#C",
	}, "\n")
	body := []byte(base64.StdEncoding.EncodeToString([]byte(links)))

	servers, err := ParseBody(body)
	if err != nil {
		t.Fatalf("ParseBody error: %v", err)
	}
	if len(servers) != 3 {
		t.Fatalf("got %d servers, want 3: %+v", len(servers), servers)
	}
	if servers[0].Protocol != "vless" || servers[1].Protocol != "trojan" || servers[2].Protocol != "shadowsocks" {
		t.Errorf("protocols = %q %q %q", servers[0].Protocol, servers[1].Protocol, servers[2].Protocol)
	}
}

func TestParseBodyPlainList(t *testing.T) {
	// Not base64 — already contains scheme markers.
	body := []byte("vless://uuid-1@a.example.com:443?type=tcp#A\nvless://uuid-2@d.example.com:443?type=tcp#D\n")
	servers, err := ParseBody(body)
	if err != nil {
		t.Fatalf("ParseBody error: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
}

func TestParseBodySkipsUnparseable(t *testing.T) {
	links := "vless://uuid-1@a.example.com:443?type=tcp#A\nssr://garbage\nnot-a-link\n"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(links)))
	servers, err := ParseBody(body)
	if err != nil {
		t.Fatalf("ParseBody error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1 (skip unsupported)", len(servers))
	}
}

func TestParseUserInfo(t *testing.T) {
	ui, ok := ParseUserInfo("upload=455727941; download=6174315083; total=1073741824000; expire=1671815872")
	if !ok {
		t.Fatal("ParseUserInfo returned ok=false")
	}
	if ui.Upload != 455727941 {
		t.Errorf("Upload = %d", ui.Upload)
	}
	if ui.Download != 6174315083 {
		t.Errorf("Download = %d", ui.Download)
	}
	if ui.Total != 1073741824000 {
		t.Errorf("Total = %d", ui.Total)
	}
	if ui.Expire.IsZero() || ui.Expire.Unix() != 1671815872 {
		t.Errorf("Expire = %v (unix %d)", ui.Expire, ui.Expire.Unix())
	}
	if ui.Remaining() != ui.Total-ui.Upload-ui.Download {
		t.Errorf("Remaining = %d", ui.Remaining())
	}
}

func TestParseUserInfoEmpty(t *testing.T) {
	if _, ok := ParseUserInfo(""); ok {
		t.Error("expected ok=false for empty header")
	}
}
