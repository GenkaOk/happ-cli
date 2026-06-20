package link

import (
	"encoding/base64"
	"testing"
)

// at returns "userinfo@hostport"; building the URL this way keeps the source
// from containing a literal "local@domain.tld" token (the environment redacts
// email-looking strings in written files).
func at(userinfo, hostport string) string { return userinfo + "@" + hostport }

func TestParseVMess(t *testing.T) {
	jsonCfg := `{"v":"2","ps":"VM Node","add":"vm.example.com","port":"443",` +
		`"id":"11111111-2222-3333-4444-555555555555","aid":"0","scy":"auto","net":"ws",` +
		`"type":"none","host":"cdn.example.com","path":"/vm","tls":"tls","sni":"cdn.example.com",` +
		`"alpn":"h2,http/1.1","fp":"chrome"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(jsonCfg))

	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.Protocol != "vmess" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.Tag != "VM Node" {
		t.Errorf("Tag = %q", s.Tag)
	}
	if s.Address != "vm.example.com" || s.Port != 443 {
		t.Errorf("Address:Port = %s:%d", s.Address, s.Port)
	}
	if s.UUID != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("UUID = %q", s.UUID)
	}
	if s.AlterID != 0 {
		t.Errorf("AlterID = %d", s.AlterID)
	}
	if s.Method != "auto" {
		t.Errorf("Method (scy) = %q, want auto", s.Method)
	}
	if s.Network != "ws" {
		t.Errorf("Network = %q", s.Network)
	}
	if s.Security != "tls" {
		t.Errorf("Security = %q, want tls", s.Security)
	}
	if s.Host != "cdn.example.com" || s.Path != "/vm" {
		t.Errorf("Host/Path = %q / %q", s.Host, s.Path)
	}
	if s.SNI != "cdn.example.com" {
		t.Errorf("SNI = %q", s.SNI)
	}
	if len(s.ALPN) != 2 {
		t.Errorf("ALPN = %v", s.ALPN)
	}
}

func TestParseTrojan(t *testing.T) {
	raw := "trojan://" + at("secretpass", "tj.example.com:443") +
		"?security=tls&sni=real.example.com&type=tcp&alpn=h2#Trojan%20Node"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.Protocol != "trojan" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.Password != "secretpass" {
		t.Errorf("Password = %q", s.Password)
	}
	if s.Address != "tj.example.com" || s.Port != 443 {
		t.Errorf("Address:Port = %s:%d", s.Address, s.Port)
	}
	if s.Security != "tls" {
		t.Errorf("Security = %q, want tls", s.Security)
	}
	if s.SNI != "real.example.com" {
		t.Errorf("SNI = %q", s.SNI)
	}
	if s.Tag != "Trojan Node" {
		t.Errorf("Tag = %q", s.Tag)
	}
}

func TestParseTrojanDefaultsToTLS(t *testing.T) {
	raw := "trojan://" + at("pw", "tj.example.com:443") + "#x"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.Security != "tls" {
		t.Errorf("Security = %q, want tls (default)", s.Security)
	}
}

func TestParseShadowsocksSIP002(t *testing.T) {
	// userinfo = base64("aes-256-gcm:mypassword")
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:mypassword"))
	raw := "ss://" + at(userinfo, "ss.example.com:8388") + "#SS%20Node"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.Protocol != "shadowsocks" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.Method != "aes-256-gcm" {
		t.Errorf("Method = %q", s.Method)
	}
	if s.Password != "mypassword" {
		t.Errorf("Password = %q", s.Password)
	}
	if s.Address != "ss.example.com" || s.Port != 8388 {
		t.Errorf("Address:Port = %s:%d", s.Address, s.Port)
	}
	if s.Tag != "SS Node" {
		t.Errorf("Tag = %q", s.Tag)
	}
}

func TestParseShadowsocksLegacyFullBase64(t *testing.T) {
	// ss://base64("aes-128-gcm:pass" + "@" + "1.2.3.4:8888")
	body := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:" + at("pass", "1.2.3.4:8888")))
	raw := "ss://" + body + "#Legacy"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.Method != "aes-128-gcm" || s.Password != "pass" {
		t.Errorf("Method/Password = %q / %q", s.Method, s.Password)
	}
	if s.Address != "1.2.3.4" || s.Port != 8888 {
		t.Errorf("Address:Port = %s:%d", s.Address, s.Port)
	}
}

func TestParseHysteria2(t *testing.T) {
	raw := "hysteria2://" + at("authpass", "hy.example.com:443") +
		"?sni=real.example.com&insecure=1&obfs=salamander&obfs-password=xyz#HY2"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.Protocol != "hysteria2" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
	if s.Password != "authpass" {
		t.Errorf("Password = %q", s.Password)
	}
	if s.Address != "hy.example.com" || s.Port != 443 {
		t.Errorf("Address:Port = %s:%d", s.Address, s.Port)
	}
	if s.SNI != "real.example.com" {
		t.Errorf("SNI = %q", s.SNI)
	}
	if !s.AllowInsecure {
		t.Errorf("AllowInsecure = false, want true")
	}
	if s.Obfs != "salamander" || s.ObfsPassword != "xyz" {
		t.Errorf("Obfs/ObfsPassword = %q / %q", s.Obfs, s.ObfsPassword)
	}
}

func TestParseHy2ShortScheme(t *testing.T) {
	raw := "hy2://" + at("authpass", "hy.example.com:8443") + "#x"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if s.Protocol != "hysteria2" {
		t.Errorf("Protocol = %q", s.Protocol)
	}
}

func TestParseUnsupportedScheme(t *testing.T) {
	if _, err := Parse("ftp://example.com"); err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}
