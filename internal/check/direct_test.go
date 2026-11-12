package check

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

// TestDirectTargetFetchByIP covers the IP-literal path: no lookup is needed and
// the Cloudflare trace ip= field is extracted from the body.
func TestDirectTargetFetchByIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fl=123\nh=example.com\nip=203.0.113.7\nts=1.0\n"))
	}))
	defer srv.Close()

	target, err := NewDirectTarget(srv.URL, 2*time.Second)
	if err != nil {
		t.Fatalf("NewDirectTarget: %v", err)
	}
	if target.ip != "127.0.0.1" {
		t.Errorf("resolved IP = %q, want 127.0.0.1", target.ip)
	}

	body, err := target.Fetch(2 * time.Second)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if body != "203.0.113.7" {
		t.Errorf("body = %q, want the trace IP 203.0.113.7", body)
	}
}

// TestDirectTargetResolvesHostOnceAndKeepsHostHeader proves a hostname is
// resolved up front, the probe dials the resulting IP, and the origin still
// sees the original Host header (a virtual-hosted server would otherwise fail).
func TestDirectTargetResolvesHostOnceAndKeepsHostHeader(t *testing.T) {
	const body = "hello-from-origin"
	var gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	rawURL := "http://localhost:" + u.Port() + "/health"

	target, err := NewDirectTarget(rawURL, 2*time.Second)
	if err != nil {
		t.Fatalf("NewDirectTarget: %v", err)
	}
	if target.host != "localhost" {
		t.Errorf("host = %q, want localhost", target.host)
	}
	if target.ip == "" || net.ParseIP(target.ip) == nil || net.ParseIP(target.ip).To4() == nil {
		t.Fatalf("resolved IP = %q, want a concrete IPv4 address", target.ip)
	}

	got, err := target.Fetch(2 * time.Second)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got != body {
		t.Errorf("body = %q, want %q", got, body)
	}
	if want := "localhost:" + u.Port(); gotHost != want {
		t.Errorf("origin saw Host %q, want %q", gotHost, want)
	}
}

func TestIsDNSError(t *testing.T) {
	wrapped := fmt.Errorf("request: %w", &net.DNSError{
		Err: "i/o timeout", Name: "example.com", Server: "127.0.0.1:53", IsTimeout: true,
	})
	if !IsDNSError(wrapped) {
		t.Error("IsDNSError(wrapped *net.DNSError) = false, want true")
	}
	if IsDNSError(errors.New("connection refused")) {
		t.Error("IsDNSError(plain error) = true, want false")
	}
	if IsDNSError(nil) {
		t.Error("IsDNSError(nil) = true, want false")
	}
}

func TestNewDirectTargetUnresolvableHostFailsFast(t *testing.T) {
	start := time.Now()
	_, err := NewDirectTarget("http://host.invalid/health", 2*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("NewDirectTarget succeeded for an unresolvable host")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("NewDirectTarget took %v, want it bounded by the 2s timeout", elapsed)
	}
}
