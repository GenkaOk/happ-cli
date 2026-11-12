package check_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aimuzov/happ-cli/internal/check"
	"github.com/aimuzov/happ-cli/internal/link"
	"github.com/aimuzov/happ-cli/internal/xray"
)

// startClient starts only the SOCKS→Shadowsocks client on the given ports.
func startClient(t *testing.T, ssPort, socksPort int) *xray.Instance {
	t.Helper()

	cfg, err := xray.BuildConfig(&link.Server{
		Protocol: "shadowsocks", Address: "127.0.0.1", Port: ssPort,
		Method: "aes-256-gcm", Password: "check-secret", Network: "tcp",
	}, xray.Options{SocksPort: socksPort})
	if err != nil {
		t.Fatalf("BuildConfig: %v", err)
	}
	raw, err := cfg.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	client, err := xray.Start(raw)
	if err != nil {
		t.Fatalf("start client: %v\n%s", err, raw)
	}
	return client
}

// TestWaitURLRetriesUntilProxyComesUp pins the "wait" semantics the startup
// gate relies on: a proxy that only appears part-way through the timeout must
// still end in success, not in an immediate abort.
func TestWaitURLRetriesUntilProxyComesUp(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "fl=1\nip=198.51.100.4\n")
	}))
	defer target.Close()

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
	defer server.Close()

	// Reserve a port, then leave it closed for a while: the client starts only
	// after the wait has already begun.
	socksPort := freePort(t)

	type result struct {
		body    string
		err     error
		elapsed time.Duration
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		body, err := check.WaitURL(socksPort, target.URL, 10*time.Second)
		done <- result{body: body, err: err, elapsed: time.Since(start)}
	}()

	time.Sleep(800 * time.Millisecond)
	client := startClient(t, ssPort, socksPort)
	defer client.Close()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("WaitURL failed although the proxy came up within the timeout: %v", r.err)
		}
		if r.body != "198.51.100.4" {
			t.Fatalf("WaitURL returned %q, want the trace IP 198.51.100.4", r.body)
		}
		if r.elapsed < 700*time.Millisecond {
			t.Fatalf("WaitURL returned after %v; it cannot have retried while the proxy was down", r.elapsed)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("WaitURL never returned")
	}
}

// TestWaitIPGivesUpAtDeadline pins the other half: with nothing listening at
// all, the wait must keep retrying until the deadline instead of failing on the
// first attempt.
func TestWaitIPGivesUpAtDeadline(t *testing.T) {
	start := time.Now()
	_, err := check.WaitIP(freePort(t), 600*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("WaitIP succeeded with no proxy listening")
	}
	if elapsed < 500*time.Millisecond {
		t.Fatalf("WaitIP gave up after %v, want it to retry until the 600ms deadline", elapsed)
	}
}
