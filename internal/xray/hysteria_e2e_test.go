package xray

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/net/proxy"

	"github.com/aimuzov/happ-cli/internal/link"
)

// freeUDPPort returns a UDP port that is free at the time of the call, so the
// hysteria2 QUIC inbound does not collide with a sibling test.
func freeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freeUDPPort: %v", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

// writeSelfSignedCert generates a short-lived self-signed certificate for
// localhost and writes it to dir, returning the cert and key paths plus the
// hex sha256 of the leaf certificate (the value xray accepts as
// pinnedPeerCertSha256; allowInsecure no longer exists in xray-core).
func writeSelfSignedCert(t *testing.T, dir string) (certPath, keyPath, pin string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	sum := sha256.Sum256(der)
	return certPath, keyPath, hex.EncodeToString(sum[:])
}

// TestHysteria2EndToEnd stands up a real xray hysteria2 (QUIC) server, a client
// built from a hysteria2 share link via BuildConfig, and verifies an HTTP
// request routed through the client's SOCKS inbound reaches the target through
// the tunnel. This covers the outbound wiring end to end, not just the JSON.
func TestHysteria2EndToEnd(t *testing.T) {
	cases := []struct {
		name         string
		obfs         string
		obfsPassword string
	}{
		{name: "plain"},
		{name: "salamander-obfs", obfs: "salamander", obfsPassword: "obfs-secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testHysteria2(t, tc.obfs, tc.obfsPassword)
		})
	}
}

func testHysteria2(t *testing.T, obfs, obfsPassword string) {
	t.Helper()

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "pong")
	}))
	defer target.Close()

	const auth = "e2e-secret"
	certPath, keyPath, pin := writeSelfSignedCert(t, t.TempDir())
	hyPort := freeUDPPort(t)

	// The server must advertise ALPN "h3": the client's hysteria2 transport is
	// QUIC/HTTP3 and offers h3, and a QUIC handshake with no overlapping ALPN
	// is rejected outright.
	serverMask := ""
	if obfs == "salamander" {
		serverMask = fmt.Sprintf(`,
          "finalmask": {"udp": [{"type": "salamander", "settings": {"password": %q}}]}`, obfsPassword)
	}

	serverCfg := fmt.Sprintf(`{
      "log": {"loglevel": "warning"},
      "inbounds": [{
        "tag": "hy2-in",
        "listen": "127.0.0.1",
        "port": %d,
        "protocol": "hysteria",
        "settings": {"version": 2, "clients": [{"auth": %q}]},
        "streamSettings": {
          "network": "hysteria",
          "security": "tls",
          "tlsSettings": {"alpn": ["h3"], "certificates": [{"certificateFile": %q, "keyFile": %q}]},
          "hysteriaSettings": {"version": 2}%s
        }
      }],
      "outbounds": [{"protocol": "freedom"}]
    }`, hyPort, auth, certPath, keyPath, serverMask)

	server, err := Start([]byte(serverCfg))
	if err != nil {
		t.Fatalf("start hysteria2 server: %v\n%s", err, serverCfg)
	}
	defer server.Close()

	clientSrv := &link.Server{
		Protocol: "hysteria2",
		Address:  "127.0.0.1",
		Port:     hyPort,
		Password: auth,
		Network:  "udp",
		Security: "tls",
		SNI:      "localhost",
		// xray-core removed allowInsecure on 2026-06-01; a self-signed server
		// is trusted by pinning the leaf certificate instead.
		PinSHA256:    pin,
		Obfs:         obfs,
		ObfsPassword: obfsPassword,
	}
	socksPort := freePort(t)
	cfg, err := BuildConfig(clientSrv, Options{SocksPort: socksPort})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}
	raw, err := cfg.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	client, err := Start(raw)
	if err != nil {
		t.Fatalf("start hysteria2 client: %v\n%s", err, raw)
	}
	defer client.Close()

	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), nil, proxy.Direct)
	if err != nil {
		t.Fatalf("socks dialer: %v", err)
	}
	httpClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{Dial: dialer.Dial},
	}

	// The QUIC handshake plus lazy tunnel setup takes longer than a TCP dial,
	// so retry generously while both instances settle.
	var body string
	var lastErr error
	for i := 0; i < 50; i++ {
		resp, err := httpClient.Get(target.URL)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			break
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	if body != "pong" {
		t.Fatalf("response through hysteria2 proxy = %q (last error: %v), want %q\nclient config:\n%s", body, lastErr, "pong", raw)
	}
}
