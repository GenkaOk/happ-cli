package xray

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/proxy"

	"github.com/aimuzov/happ-cli/internal/link"
)

// startSSServer runs a real Shadowsocks server and returns it with its port.
func startSSServer(t *testing.T) (*Instance, int) {
	t.Helper()

	port := freePort(t)
	cfg := fmt.Sprintf(`{
      "log": {"loglevel": "warning"},
      "inbounds": [{
        "tag": "ss-in", "listen": "127.0.0.1", "port": %d, "protocol": "shadowsocks",
        "settings": {"method": "aes-256-gcm", "password": "exclude-secret", "network": "tcp"}
      }],
      "outbounds": [{"protocol": "freedom"}]
    }`, port)

	srv, err := Start([]byte(cfg))
	if err != nil {
		t.Fatalf("start ss server: %v", err)
	}
	return srv, port
}

// proxyClient issues requests through a real SOCKS client that may exclude
// destinations. Each call to get opens a fresh connection (idle pooling is
// disabled) so a request can never ride on a connection established before the
// proxy server was killed.
type proxyClient struct {
	get func(*http.Request) (string, error)
}

func startExcludingClient(t *testing.T, ssPort int, exclude []string) *proxyClient {
	t.Helper()

	socksPort := freePort(t)
	cfg, err := BuildConfig(&link.Server{
		Protocol: "shadowsocks", Address: "127.0.0.1", Port: ssPort,
		Method: "aes-256-gcm", Password: "exclude-secret", Network: "tcp",
	}, Options{SocksPort: socksPort, Exclude: exclude})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}
	raw, err := cfg.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	client, err := Start(raw)
	if err != nil {
		t.Fatalf("start client: %v\n%s", err, raw)
	}
	t.Cleanup(func() { client.Close() })

	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), nil, proxy.Direct)
	if err != nil {
		t.Fatalf("socks dialer: %v", err)
	}
	transport := &http.Transport{Dial: dialer.Dial, DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	httpClient := &http.Client{Timeout: 3 * time.Second, Transport: transport}

	get := func(req *http.Request) (string, error) {
		resp, err := httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		return string(b), err
	}
	return &proxyClient{get: get}
}

// waitWorks retries req briefly, because a freshly started client needs a
// moment to accept its first connection.
func (c *proxyClient) waitWorks(t *testing.T, req *http.Request) string {
	t.Helper()
	var lastErr error
	for i := 0; i < 20; i++ {
		body, err := c.get(req)
		if err == nil {
			return body
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("request never succeeded: %v", lastErr)
	return ""
}

func targetServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "pong")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestExcludedNetworkGoesDirect proves the split: an excluded IPv4 network must
// still be reachable after the VPN server dies, which is only possible if the
// traffic never went through it.
func TestExcludedNetworkGoesDirect(t *testing.T) {
	target := targetServer(t)

	ss, ssPort := startSSServer(t)
	client := startExcludingClient(t, ssPort, []string{"127.0.0.1"})

	req, _ := http.NewRequest(http.MethodGet, target.URL, nil)
	if body := client.waitWorks(t, req); body != "pong" {
		t.Fatalf("request before server shutdown = %q, want pong", body)
	}

	ss.Close() // nothing can be tunnelled any more

	req, _ = http.NewRequest(http.MethodGet, target.URL, nil)
	if body, err := client.get(req); err != nil || body != "pong" {
		t.Fatalf("excluded destination failed after the VPN server died: body=%q err=%v", body, err)
	}
}

// TestNonExcludedNetworkNeedsProxy is the control: without an exclusion the
// same request must fail once the server is gone.
func TestNonExcludedNetworkNeedsProxy(t *testing.T) {
	target := targetServer(t)

	ss, ssPort := startSSServer(t)
	client := startExcludingClient(t, ssPort, nil)

	req, _ := http.NewRequest(http.MethodGet, target.URL, nil)
	if body := client.waitWorks(t, req); body != "pong" {
		t.Fatalf("request before server shutdown = %q, want pong", body)
	}

	ss.Close()

	req, _ = http.NewRequest(http.MethodGet, target.URL, nil)
	if _, err := client.get(req); err == nil {
		t.Fatal("request succeeded without a proxy server; the control case is meaningless")
	}
}

// TestExcludedDomainGoesDirect covers the domain path TUN mode relies on: the
// SOCKS destination is an IP and only the sniffed HTTP Host carries the domain,
// which is exactly what xray sees for tunnelled traffic. The request must go
// direct even with the VPN server dead.
func TestExcludedDomainGoesDirect(t *testing.T) {
	target := targetServer(t)

	ss, ssPort := startSSServer(t)
	client := startExcludingClient(t, ssPort, []string{"localhost"})

	req, _ := http.NewRequest(http.MethodGet, target.URL, nil)
	req.Host = "localhost" // sniffed by xray; routes as domain:localhost
	if body := client.waitWorks(t, req); body != "pong" {
		t.Fatalf("request before server shutdown = %q, want pong", body)
	}

	ss.Close()

	req, _ = http.NewRequest(http.MethodGet, target.URL, nil)
	req.Host = "localhost"
	if body, err := client.get(req); err != nil || body != "pong" {
		t.Fatalf("sniffed excluded domain failed after the VPN server died: body=%q err=%v", body, err)
	}
}
