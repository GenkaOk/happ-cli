package check_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aimuzov/happ-cli/internal/check"
	"github.com/aimuzov/happ-cli/internal/link"
	"github.com/aimuzov/happ-cli/internal/xray"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// traceTarget serves a Cloudflare-trace-shaped body so ExternalIP's parser is
// exercised over a real (locally proxied) HTTP response.
func traceTarget(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "fl=123abc\nh=cloudflare.com\nip=203.0.113.7\nts=1700000000.0\nvisit_scheme=http\n")
	}))
}

// startProxyChain runs a real xray Shadowsocks server plus a client exposing a
// SOCKS inbound, and returns the SOCKS port.
func startProxyChain(t *testing.T) (socksPort int, stop func()) {
	t.Helper()

	const method, password = "aes-256-gcm", "check-secret"
	ssPort := freePort(t)
	serverCfg := fmt.Sprintf(`{
      "log": {"loglevel": "warning"},
      "inbounds": [{
        "tag": "ss-in", "listen": "127.0.0.1", "port": %d, "protocol": "shadowsocks",
        "settings": {"method": %q, "password": %q, "network": "tcp"}
      }],
      "outbounds": [{"protocol": "freedom"}]
    }`, ssPort, method, password)

	server, err := xray.Start([]byte(serverCfg))
	if err != nil {
		t.Fatalf("start ss server: %v", err)
	}

	cfg, err := xray.BuildConfig(&link.Server{
		Protocol: "shadowsocks", Address: "127.0.0.1", Port: ssPort,
		Method: method, Password: password, Network: "tcp",
	}, xray.Options{SocksPort: freePort(t)})
	if err != nil {
		server.Close()
		t.Fatalf("BuildConfig: %v", err)
	}
	raw, err := cfg.JSON()
	if err != nil {
		server.Close()
		t.Fatalf("JSON: %v", err)
	}
	socksPort = cfg.Inbounds[0].Port
	client, err := xray.Start(raw)
	if err != nil {
		server.Close()
		t.Fatalf("start client: %v\n%s", err, raw)
	}

	return socksPort, func() {
		client.Close()
		server.Close()
	}
}

// TestExternalIPThroughRealProxy is the check the health gate depends on: a
// working SOCKS→Shadowsocks→freedom chain must yield the target's trace IP.
func TestExternalIPThroughRealProxy(t *testing.T) {
	target := traceTarget(t)
	defer target.Close()

	socksPort, stop := startProxyChain(t)
	defer stop()

	// The proxy chain needs a moment to accept its first connection.
	var ip string
	var lastErr error
	for i := 0; i < 30; i++ {
		ip, lastErr = check.FetchURL(fmt.Sprintf("127.0.0.1:%d", socksPort), target.URL, 2*time.Second)
		if lastErr == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("FetchURL through proxy: %v", lastErr)
	}
	if want := "203.0.113.7"; ip != want {
		t.Fatalf("FetchURL returned %q, want %q (trace ip= line)", ip, want)
	}
}

// TestExternalIPFailsWhenProxyIsDown guards the failure path: the health gate
// must not report success when nothing is listening on the SOCKS port.
func TestExternalIPFailsWhenProxyIsDown(t *testing.T) {
	_, err := check.ExternalIP(fmt.Sprintf("127.0.0.1:%d", freePort(t)), time.Second)
	if err == nil {
		t.Fatal("ExternalIP succeeded with no proxy listening; health check would pass a dead tunnel")
	}
}

// TestExternalIPFailsWhenUpstreamDies covers the periodic health check: the
// SOCKS port stays open, so only the failed connect to the remote server can
// reveal that the tunnel is gone.
func TestExternalIPFailsWhenUpstreamDies(t *testing.T) {
	target := traceTarget(t)
	defer target.Close()

	socksPort, stop := startProxyChain(t)
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)

	var err error
	for i := 0; i < 30; i++ {
		if _, err = check.FetchURL(addr, target.URL, 2*time.Second); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("proxy chain never came up: %v", err)
	}

	stop() // kill the server and client; nothing should answer now

	if _, err := check.ExternalIP(addr, time.Second); err == nil {
		t.Fatal("ExternalIP succeeded after the proxy was shut down; health check would miss a dead tunnel")
	}
}
