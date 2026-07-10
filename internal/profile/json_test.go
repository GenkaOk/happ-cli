package profile

import (
	"encoding/json"
	"testing"

	"github.com/aimuzov/happ-cli/internal/link"
)

func TestParseJSONBody_ValidArray(t *testing.T) {
	body := []byte(`[
		{
			"remarks": "Finnish server",
			"outbounds": [
				{
					"tag": "proxy",
					"protocol": "vless",
					"settings": {"vnext":[{"address":"fi1.mycloud1.org","port":443,"users":[{"id":"c3aa1821-98e8-4242-8a3f-255a300a91cb","encryption":"none","flow":"xtls-rprx-vision"}]}]},
					"streamSettings": {"network":"tcp","security":"reality","realitySettings":{"serverName":"fi1.mycloud1.org","publicKey":"pk","shortId":"sid","fingerprint":"firefox"}}
				}
			]
		}
	]`)

	servers, err := parseJSONBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	s := servers[0]
	if s.Tag != "Finnish server" {
		t.Errorf("Tag = %q", s.Tag)
	}
	if s.Protocol != "vless" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.Address != "fi1.mycloud1.org" {
		t.Errorf("Address = %q", s.Address)
	}
	if s.Port != 443 {
		t.Errorf("Port = %d", s.Port)
	}
	if s.PublicKey != "pk" {
		t.Errorf("PublicKey = %q", s.PublicKey)
	}
}

func TestParseJSONBody_NonJSON(t *testing.T) {
	servers, err := parseJSONBody([]byte("vless://uuid@host:443#tag"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if servers != nil {
		t.Errorf("expected nil for non-JSON, got %d servers", len(servers))
	}
}

func TestParseJSONBody_Empty(t *testing.T) {
	servers, err := parseJSONBody([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if servers != nil {
		t.Errorf("expected nil for empty, got %d servers", len(servers))
	}
}

func TestParseJSONBody_SkipsNonProxyOutbounds(t *testing.T) {
	body := []byte(`[{
		"remarks": "Test",
		"outbounds": [
			{"tag":"DIRECT","protocol":"freedom","settings":{"domainStrategy":"UseIPv4"}},
			{"tag":"BLOCK","protocol":"blackhole"},
			{"tag":"LOOP","protocol":"loopback","settings":{"inboundTag":"X"}},
			{"tag":"SOCKS","protocol":"socks","settings":{"servers":[{"address":"127.0.0.1","port":10810}]}},
			{"tag":"proxy","protocol":"trojan","settings":{"servers":[{"address":"example.com","port":443,"password":"pass"}]},"streamSettings":{"network":"tcp","security":"tls","tlsSettings":{"serverName":"example.com"}}}
		]
	}]`)
	servers, err := parseJSONBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1 (only trojan)", len(servers))
	}
	if servers[0].Protocol != "trojan" {
		t.Errorf("expected trojan, got %s", servers[0].Protocol)
	}
}

func TestParseJSONBody_MultipleConfigs(t *testing.T) {
	body := []byte(`[
		{"remarks":"DE","outbounds":[{"tag":"p","protocol":"vless","settings":{"vnext":[{"address":"de.example.com","port":443,"users":[{"id":"uuid-1","encryption":"none"}]}]}}]},
		{"remarks":"FI","outbounds":[{"tag":"p","protocol":"trojan","settings":{"servers":[{"address":"fi.example.com","port":443,"password":"pw"}]}}]}
	]`)
	servers, err := parseJSONBody(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	if servers[0].Tag != "DE" || servers[1].Tag != "FI" {
		t.Errorf("tags: %q, %q", servers[0].Tag, servers[1].Tag)
	}
}

// --- Protocol-specific conversion tests ---

func vlessOutbound(extra func(m map[string]interface{})) *xrayOutbound {
	settings, _ := json.Marshal(map[string]interface{}{
		"vnext": []interface{}{map[string]interface{}{
			"address": "server.example.com",
			"port":    443.0,
			"users": []interface{}{map[string]interface{}{
				"id":         "uuid-1234",
				"encryption": "none",
				"flow":       "xtls-rprx-vision",
			}},
		}},
	})
	ob := &xrayOutbound{Tag: "proxy", Protocol: "vless", Settings: settings}
	if extra != nil {
		extra(map[string]interface{}{})
	}
	return ob
}

func TestVLESSReality(t *testing.T) {
	ssJSON, _ := json.Marshal(map[string]interface{}{
		"network":  "tcp",
		"security": "reality",
		"realitySettings": map[string]interface{}{
			"serverName":  "sni.example.com",
			"publicKey":   "pubkey-abc",
			"shortId":     "short-id",
			"fingerprint": "chrome",
		},
	})
	var ss xrayStreamSettings
	json.Unmarshal(ssJSON, &ss)

	ob := vlessOutbound(nil)
	ob.StreamSettings = &ss

	s := vlessToLink(ob, "My Server")
	if s == nil {
		t.Fatal("vlessToLink returned nil")
	}
	if s.Tag != "My Server" {
		t.Errorf("Tag = %q", s.Tag)
	}
	if s.Protocol != "vless" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.UUID != "uuid-1234" {
		t.Errorf("UUID = %q", s.UUID)
	}
	if s.Flow != "xtls-rprx-vision" {
		t.Errorf("Flow = %q", s.Flow)
	}
	if s.Security != "reality" {
		t.Errorf("Security = %q", s.Security)
	}
	if s.SNI != "sni.example.com" {
		t.Errorf("SNI = %q", s.SNI)
	}
	if s.PublicKey != "pubkey-abc" {
		t.Errorf("PublicKey = %q", s.PublicKey)
	}
	if s.ShortID != "short-id" {
		t.Errorf("ShortID = %q", s.ShortID)
	}
}

func TestTrojanWSTLS(t *testing.T) {
	settings, _ := json.Marshal(map[string]interface{}{
		"servers": []interface{}{map[string]interface{}{
			"address":  "tr.example.com",
			"port":     443.0,
			"password": "secret-pw",
		}},
	})
	ssJSON, _ := json.Marshal(map[string]interface{}{
		"network":  "ws",
		"security": "tls",
		"wsSettings": map[string]interface{}{
			"path": "/api/user",
			"host": "cdn.example.com",
		},
		"tlsSettings": map[string]interface{}{
			"serverName":  "cdn.example.com",
			"fingerprint": "firefox",
			"alpn":        []interface{}{"http/1.1"},
		},
	})

	ob := &xrayOutbound{
		Tag:      "proxy",
		Protocol: "trojan",
		Settings: settings,
	}
	var ss xrayStreamSettings
	json.Unmarshal(ssJSON, &ss)
	ob.StreamSettings = &ss

	s := trojanToLink(ob, "Trojan WS")
	if s == nil {
		t.Fatal("trojanToLink returned nil")
	}
	if s.Protocol != "trojan" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.Password != "secret-pw" {
		t.Errorf("Password = %q", s.Password)
	}
	if s.Network != "ws" {
		t.Errorf("Network = %q", s.Network)
	}
	if s.Path != "/api/user" {
		t.Errorf("Path = %q", s.Path)
	}
	if s.Host != "cdn.example.com" {
		t.Errorf("Host = %q", s.Host)
	}
	if s.Fingerprint != "firefox" {
		t.Errorf("Fingerprint = %q", s.Fingerprint)
	}
	if len(s.ALPN) != 1 || s.ALPN[0] != "http/1.1" {
		t.Errorf("ALPN = %v", s.ALPN)
	}
}

func TestShadowsocks(t *testing.T) {
	settings, _ := json.Marshal(map[string]interface{}{
		"servers": []interface{}{map[string]interface{}{
			"address":  "ss.example.com",
			"port":     8388.0,
			"method":   "aes-256-gcm",
			"password": "ss-password",
		}},
	})
	ob := &xrayOutbound{
		Tag:      "proxy",
		Protocol: "shadowsocks",
		Settings: settings,
	}
	s := shadowsocksToLink(ob, "SS Server")
	if s == nil {
		t.Fatal("shadowsocksToLink returned nil")
	}
	if s.Protocol != "shadowsocks" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.Method != "aes-256-gcm" {
		t.Errorf("Method = %q", s.Method)
	}
	if s.Password != "ss-password" {
		t.Errorf("Password = %q", s.Password)
	}
	if s.Address != "ss.example.com" {
		t.Errorf("Address = %q", s.Address)
	}
	if s.Port != 8388 {
		t.Errorf("Port = %d", s.Port)
	}
}

func TestHysteria2(t *testing.T) {
	settings, _ := json.Marshal(map[string]interface{}{
		"address": "hy2.example.com",
		"port":    443.0,
		"version": 2,
	})
	ssJSON, _ := json.Marshal(map[string]interface{}{
		"network":  "hysteria",
		"security": "tls",
		"hysteriaSettings": map[string]interface{}{
			"version": 2,
			"auth":    "hy2-auth-token",
		},
		"tlsSettings": map[string]interface{}{
			"serverName":  "hy2.example.com",
			"fingerprint": "random",
			"alpn":        []interface{}{"h3"},
		},
	})

	ob := &xrayOutbound{
		Tag:      "proxy",
		Protocol: "hysteria2",
		Settings: settings,
	}
	var ss xrayStreamSettings
	json.Unmarshal(ssJSON, &ss)
	ob.StreamSettings = &ss

	s := hysteriaToLink(ob, "HY2 Server")
	if s == nil {
		t.Fatal("hysteriaToLink returned nil")
	}
	if s.Protocol != "hysteria2" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.Password != "hy2-auth-token" {
		t.Errorf("Password = %q", s.Password)
	}
	if s.SNI != "hy2.example.com" {
		t.Errorf("SNI = %q", s.SNI)
	}
}

func TestOutboundToLink_Unsupported(t *testing.T) {
	ob := &xrayOutbound{Protocol: "freedom"}
	if s := outboundToLink(ob, "test"); s != nil {
		t.Errorf("expected nil for freedom, got %+v", s)
	}
	ob.Protocol = "blackhole"
	if s := outboundToLink(ob, "test"); s != nil {
		t.Errorf("expected nil for blackhole, got %+v", s)
	}
	ob.Protocol = "loopback"
	if s := outboundToLink(ob, "test"); s != nil {
		t.Errorf("expected nil for loopback, got %+v", s)
	}
}

func TestVLESSMissingUUID(t *testing.T) {
	settings, _ := json.Marshal(map[string]interface{}{
		"vnext": []interface{}{map[string]interface{}{
			"address": "host",
			"port":    443.0,
			"users":   []interface{}{map[string]interface{}{}},
		}},
	})
	ob := &xrayOutbound{Protocol: "vless", Settings: settings}
	if s := vlessToLink(ob, "test"); s != nil {
		t.Errorf("expected nil for missing UUID, got %+v", s)
	}
}

func TestTrojanMissingPassword(t *testing.T) {
	settings, _ := json.Marshal(map[string]interface{}{
		"servers": []interface{}{map[string]interface{}{
			"address": "host", "port": 443.0,
		}},
	})
	ob := &xrayOutbound{Protocol: "trojan", Settings: settings}
	if s := trojanToLink(ob, "test"); s != nil {
		t.Errorf("expected nil for missing password, got %+v", s)
	}
}

func TestVLESSRawRoundTrip(t *testing.T) {
	ob := vlessOutbound(nil)
	s := vlessToLink(ob, "RoundTrip")
	if s == nil {
		t.Fatal("vlessToLink returned nil")
	}
	// The generated Raw link must parse back successfully.
	parsed, err := link.Parse(s.Raw)
	if err != nil {
		t.Fatalf("link.Parse(%q) failed: %v", s.Raw, err)
	}
	if parsed.UUID != s.UUID {
		t.Errorf("round-trip UUID: %q vs %q", parsed.UUID, s.UUID)
	}
	if parsed.Address != s.Address {
		t.Errorf("round-trip Address: %q vs %q", parsed.Address, s.Address)
	}
}

func TestTrojanRawRoundTrip(t *testing.T) {
	settings, _ := json.Marshal(map[string]interface{}{
		"servers": []interface{}{map[string]interface{}{
			"address": "tr.example.com", "port": 443.0, "password": "pw",
		}},
	})
	ob := &xrayOutbound{Protocol: "trojan", Settings: settings}
	s := trojanToLink(ob, "RT")
	if s == nil {
		t.Fatal("trojanToLink returned nil")
	}
	parsed, err := link.Parse(s.Raw)
	if err != nil {
		t.Fatalf("link.Parse(%q) failed: %v", s.Raw, err)
	}
	if parsed.Password != s.Password {
		t.Errorf("round-trip Password: %q vs %q", parsed.Password, s.Password)
	}
}

func TestParseJSONBody_InvalidJSON(t *testing.T) {
	servers, err := parseJSONBody([]byte("[not json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if servers != nil {
		t.Errorf("expected nil for invalid JSON, got %d servers", len(servers))
	}
}

func TestVMess(t *testing.T) {
	settings, _ := json.Marshal(map[string]interface{}{
		"vnext": []interface{}{map[string]interface{}{
			"address": "vm.example.com",
			"port":    443.0,
			"users": []interface{}{map[string]interface{}{
				"id": "vmess-uuid",
			}},
		}},
	})
	ssJSON, _ := json.Marshal(map[string]interface{}{
		"network":  "ws",
		"security": "tls",
		"wsSettings": map[string]interface{}{
			"path": "/vmess",
		},
		"tlsSettings": map[string]interface{}{
			"serverName": "vm.example.com",
		},
	})

	ob := &xrayOutbound{
		Tag:      "proxy",
		Protocol: "vmess",
		Settings: settings,
	}
	var ss xrayStreamSettings
	json.Unmarshal(ssJSON, &ss)
	ob.StreamSettings = &ss

	s := vmessToLink(ob, "VMess WS")
	if s == nil {
		t.Fatal("vmessToLink returned nil")
	}
	if s.Protocol != "vmess" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.UUID != "vmess-uuid" {
		t.Errorf("UUID = %q", s.UUID)
	}
	if s.Address != "vm.example.com" {
		t.Errorf("Address = %q", s.Address)
	}
	if s.Port != 443 {
		t.Errorf("Port = %d", s.Port)
	}
	if s.Network != "ws" {
		t.Errorf("Network = %q", s.Network)
	}
	if s.Path != "/vmess" {
		t.Errorf("Path = %q", s.Path)
	}
	// Verify round-trip.
	parsed, err := link.Parse(s.Raw)
	if err != nil {
		t.Fatalf("link.Parse(%q) failed: %v", s.Raw, err)
	}
	if parsed.UUID != s.UUID {
		t.Errorf("round-trip UUID: %q vs %q", parsed.UUID, s.UUID)
	}
}
