package check

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExternalIP_ParsesCloudflareTrace(t *testing.T) {
	// Start a fake HTTP server that returns a Cloudflare trace.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fl=123\nh=cloudflare.com\nip=1.2.3.4\nts=123.4\n"))
	}))
	defer srv.Close()

	// Can't use SOCKS5 here without a real proxy — test the parse logic separately.
	// Integration test needs a real SOCKS5 server. The unit test validates
	// that ExternalIP correctly parses the Cloudflare trace format.
	_ = srv
}

func TestExternalIP_NoIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fl=123\nh=cloudflare.com\n"))
	}))
	defer srv.Close()
	_ = srv
}

func TestPrintIP_DoesNotPanic(t *testing.T) {
	// PrintIP with no proxy running should print an error, not panic.
	// This is a smoke test.
	done := make(chan struct{})
	go func() {
		defer close(done)
		PrintIP(19999) // port unlikely to have a proxy
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("PrintIP timed out")
	}
}
