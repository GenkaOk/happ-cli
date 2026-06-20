package link

import (
	"testing"
)

func TestParseVLESSRealityVision(t *testing.T) {
	raw := "vless://b831381d-6324-4d53-ad4f-8cda48b30811@example.com:443?" +
		"type=tcp&security=reality&pbk=x_publickey&fp=chrome&sni=www.google.com&" +
		"sid=abcd1234&spx=%2F&flow=xtls-rprx-vision#My%20Reality"

	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if s.Protocol != "vless" {
		t.Errorf("Protocol = %q, want vless", s.Protocol)
	}
	if s.UUID != "b831381d-6324-4d53-ad4f-8cda48b30811" {
		t.Errorf("UUID = %q", s.UUID)
	}
	if s.Address != "example.com" {
		t.Errorf("Address = %q, want example.com", s.Address)
	}
	if s.Port != 443 {
		t.Errorf("Port = %d, want 443", s.Port)
	}
	if s.Network != "tcp" {
		t.Errorf("Network = %q, want tcp", s.Network)
	}
	if s.Security != "reality" {
		t.Errorf("Security = %q, want reality", s.Security)
	}
	if s.PublicKey != "x_publickey" {
		t.Errorf("PublicKey = %q", s.PublicKey)
	}
	if s.Fingerprint != "chrome" {
		t.Errorf("Fingerprint = %q", s.Fingerprint)
	}
	if s.SNI != "www.google.com" {
		t.Errorf("SNI = %q", s.SNI)
	}
	if s.ShortID != "abcd1234" {
		t.Errorf("ShortID = %q", s.ShortID)
	}
	if s.SpiderX != "/" {
		t.Errorf("SpiderX = %q, want /", s.SpiderX)
	}
	if s.Flow != "xtls-rprx-vision" {
		t.Errorf("Flow = %q", s.Flow)
	}
	if s.Tag != "My Reality" {
		t.Errorf("Tag = %q, want 'My Reality'", s.Tag)
	}
}

func TestParseVLESSWebSocketTLS(t *testing.T) {
	raw := "vless://uuid-1@host.net:8443?type=ws&security=tls&path=%2Fwspath&host=cdn.host.net&sni=cdn.host.net&alpn=h2%2Chttp%2F1.1#WS"

	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if s.Network != "ws" {
		t.Errorf("Network = %q, want ws", s.Network)
	}
	if s.Security != "tls" {
		t.Errorf("Security = %q, want tls", s.Security)
	}
	if s.Path != "/wspath" {
		t.Errorf("Path = %q, want /wspath", s.Path)
	}
	if s.Host != "cdn.host.net" {
		t.Errorf("Host = %q", s.Host)
	}
	if len(s.ALPN) != 2 || s.ALPN[0] != "h2" || s.ALPN[1] != "http/1.1" {
		t.Errorf("ALPN = %v, want [h2 http/1.1]", s.ALPN)
	}
}
