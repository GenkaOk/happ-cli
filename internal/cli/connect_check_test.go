package cli

import (
	"context"
	"errors"
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
	case <-time.After(3 * time.Second):
		t.Fatal("failing probe did not cancel the context within 3s")
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
