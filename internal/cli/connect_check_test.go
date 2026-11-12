package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartHealthCheckDisabled(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := startHealthCheck(context.Background(), 1, false, func() error {
		calls.Add(1)
		return errors.New("boom")
	})
	defer cancel(nil)

	select {
	case <-ctx.Done():
		t.Fatal("disabled health check cancelled the context")
	case <-time.After(1500 * time.Millisecond):
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("disabled health check called the probe %d times, want 0", n)
	}
}

// TestStartHealthCheckFailingProbeCancels pins the failure tolerance: two
// consecutive failures are not enough, the third one tears the tunnel down.
func TestStartHealthCheckFailingProbeCancels(t *testing.T) {
	ctx, cancel := startHealthCheck(context.Background(), 1, true, func() error {
		return errors.New("boom")
	})
	defer cancel(nil)

	select {
	case <-ctx.Done():
		if !errors.Is(context.Cause(ctx), errHealthCheckFailed) {
			t.Errorf("cause = %v, want errHealthCheckFailed", context.Cause(ctx))
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("3 consecutive failing probes did not cancel the context within 5s (tolerance = %d)", healthCheckMaxFailures)
	}
}

// TestStartHealthCheckToleratesTransientFailures makes sure a couple of
// consecutive failures — the DNS hiccup that used to tear down a working
// tunnel — do not cancel anything, and that success resets the counter.
func TestStartHealthCheckToleratesTransientFailures(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := startHealthCheck(context.Background(), 1, true, func() error {
		if calls.Add(1) <= 2 {
			return errors.New("transient")
		}
		return nil
	})
	defer cancel(nil)

	select {
	case <-ctx.Done():
		t.Fatalf("two transient failures cancelled the context: %v", context.Cause(ctx))
	case <-time.After(3500 * time.Millisecond):
	}
	if n := calls.Load(); n < 3 {
		t.Errorf("probe called %d times, want at least 3 (failures then recovery)", n)
	}
}

// TestRoutedCheckVerdict pins the DNS-vs-tunnel decision: a DNS failure during
// the routed-path check must not roll the routes back, any other failure must.
func TestRoutedCheckVerdict(t *testing.T) {
	dnsFailure := fmt.Errorf("request: %w", &net.DNSError{
		Err: "i/o timeout", Name: "example.com", Server: "127.0.0.1:53", IsTimeout: true,
	})
	if routedCheckVerdict(dnsFailure) {
		t.Error("routedCheckVerdict(DNS failure) = true, want false (leave the tunnel up)")
	}
	if !routedCheckVerdict(errors.New("connection refused")) {
		t.Error("routedCheckVerdict(non-DNS failure) = false, want true (roll back)")
	}
}

func TestStartHealthCheckHealthyProbeKeepsRunning(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := startHealthCheck(context.Background(), 1, true, func() error {
		calls.Add(1)
		return nil
	})
	defer cancel(nil)

	select {
	case <-ctx.Done():
		t.Fatal("healthy probe cancelled the context")
	case <-time.After(2500 * time.Millisecond):
	}
	if calls.Load() == 0 {
		t.Error("healthy probe was never called")
	}
}

func TestCheckExternalHTTPCustomURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fl=123\nh=example\nip=203.0.113.9\nts=1.0\n"))
	}))
	defer srv.Close()

	ip, err := checkExternalHTTP(srv.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("checkExternalHTTP: %v", err)
	}
	if ip != "203.0.113.9" {
		t.Errorf("ip = %q, want 203.0.113.9", ip)
	}
}

func TestConnectFlagsUseConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgYAML := "connect:\n  health-check: true\n  check-url: http://127.0.0.1:9/health\n  mode: tun\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	prevHome := homeDir
	homeDir = dir
	defer func() { homeDir = prevHome }()

	cmd := newConnectCmd(withDeps())

	want := map[string]string{
		"health-check":   "true",
		"check-url":      "http://127.0.0.1:9/health",
		"mode":           "tun",
		"socks":          "10808",
		"http":           "10809",
		"check-interval": "60",
		"dns-proxy":      "true",
	}
	for name, def := range want {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("flag %q is not registered", name)
			continue
		}
		if f.DefValue != def {
			t.Errorf("flag %q default = %q, want %q", name, f.DefValue, def)
		}
	}

	// An explicit flag must still override the config-derived default.
	if err := cmd.Flags().Parse([]string{"--health-check=false"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	got, err := cmd.Flags().GetBool("health-check")
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("explicit --health-check=false did not override the config default")
	}
}

// TestDisconnectReason pins the exit-code contract: a health-check-driven
// teardown must be reported as an error (non-zero exit) so supervisors can
// restart/alert, while a user-initiated stop stays a clean exit.
func TestDisconnectReason(t *testing.T) {
	alive, cancelAlive := context.WithCancelCause(context.Background())
	defer cancelAlive(nil)
	if err := disconnectReason(alive); err != nil {
		t.Errorf("disconnectReason on a live context = %v, want nil", err)
	}

	probeFailed, cancelProbe := context.WithCancelCause(context.Background())
	cancelProbe(errHealthCheckFailed)
	if err := disconnectReason(probeFailed); !errors.Is(err, errTunnelLost) {
		t.Errorf("disconnectReason after a failed probe = %v, want errTunnelLost", err)
	}

	userStopped, cancelUser := context.WithCancelCause(context.Background())
	cancelUser(nil)
	if err := disconnectReason(userStopped); err != nil {
		t.Errorf("disconnectReason after Ctrl+C = %v, want nil", err)
	}
}

// TestDNSTolerant pins the promise that a broken router resolver can never
// tear down a working tunnel, while real network failures still count.
func TestDNSTolerant(t *testing.T) {
	dnsErr := &net.DNSError{Err: "i/o timeout", Name: "example.com", IsTimeout: true}

	if err := dnsTolerant(func() error { return dnsErr })(); err != nil {
		t.Errorf("DNS failure counted as a tunnel failure: %v", err)
	}
	wrapped := fmt.Errorf("request: %w", dnsErr)
	if err := dnsTolerant(func() error { return wrapped })(); err != nil {
		t.Errorf("wrapped DNS failure counted as a tunnel failure: %v", err)
	}

	refused := errors.New("connection refused")
	if err := dnsTolerant(func() error { return refused })(); !errors.Is(err, refused) {
		t.Errorf("real failure swallowed: got %v, want %v", err, refused)
	}
}
