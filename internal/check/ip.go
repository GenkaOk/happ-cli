// Package check provides connectivity verification helpers.
package check

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// dialFunc matches net.Dialer.DialContext and proxy.Dialer-style functions, so
// one fetch path works both through SOCKS and over the system route.
type dialFunc func(context.Context, string, string) (net.Conn, error)

// ExternalIP connects through a local SOCKS5 proxy to Cloudflare's trace
// endpoint and returns the visible external IP address. It makes a single
// attempt; use WaitIP to retry until a deadline.
func ExternalIP(socksAddr string, timeout time.Duration) (string, error) {
	return fetchViaSOCKS(socksAddr, "https://cloudflare.com/cdn-cgi/trace", timeout)
}

// FetchURL fetches a URL through the SOCKS5 proxy and returns the response
// body. Use for custom health-check endpoints. It makes a single attempt; use
// WaitURL to retry until a deadline.
func FetchURL(socksAddr, urlStr string, timeout time.Duration) (string, error) {
	return fetchViaSOCKS(socksAddr, urlStr, timeout)
}

// FetchDirect fetches a URL over the system route (no SOCKS) and returns the
// response body. Used by modes without a SOCKS inbound (tun-direct). It makes a
// single attempt; use WaitDirect to retry until a deadline.
func FetchDirect(urlStr string, timeout time.Duration) (string, error) {
	return fetch(nil, urlStr, timeout)
}

// fetchViaSOCKS fetches a URL through a SOCKS5 proxy and returns the response body.
func fetchViaSOCKS(socksAddr, urlStr string, timeout time.Duration) (string, error) {
	dial, err := socksDial(socksAddr)
	if err != nil {
		return "", err
	}
	return fetch(dial, urlStr, timeout)
}

// socksDial builds a dial function backed by a local SOCKS5 proxy.
func socksDial(socksAddr string) (dialFunc, error) {
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("socks5 dialer: %w", err)
	}
	return func(_ context.Context, network, addr string) (net.Conn, error) {
		return dialer.Dial(network, addr)
	}, nil
}

// fetch performs one HTTP GET using an explicit transport. A nil dial means a
// direct connection over the system route. The transport is built from scratch
// so HTTP(S)_PROXY from the environment can never hijack the check.
func fetch(dial dialFunc, urlStr string, timeout time.Duration) (string, error) {
	if dial == nil {
		d := &net.Dialer{}
		dial = d.DialContext
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	transport := &http.Transport{DialContext: dial}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}

	// Try Cloudflare trace format for IP extraction.
	if strings.Contains(string(body), "ip=") {
		for _, line := range strings.Split(string(body), "\n") {
			k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
			if ok && k == "ip" {
				return v, nil
			}
		}
	}
	return strings.TrimSpace(string(body)), nil
}

// waitVia retries fetch until it succeeds or the timeout elapses. Each attempt
// is capped at 3 seconds (and at the remaining time), with a 250ms pause
// between attempts. On timeout it reports the attempt count and the last error.
func waitVia(dial dialFunc, urlStr string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	attempts := 0
	var lastErr error

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if lastErr == nil {
				lastErr = fmt.Errorf("timeout")
			}
			return "", fmt.Errorf("after %d attempts: %w", attempts, lastErr)
		}

		attemptTimeout := remaining
		if attemptTimeout > 3*time.Second {
			attemptTimeout = 3 * time.Second
		}
		attempts++

		body, err := fetch(dial, urlStr, attemptTimeout)
		if err == nil {
			return body, nil
		}
		lastErr = err

		pause := 250 * time.Millisecond
		if left := time.Until(deadline); left < pause {
			if left <= 0 {
				break
			}
			pause = left
		}
		time.Sleep(pause)
	}

	return "", fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}

// PrintIP fetches and prints the external IP using the local SOCKS5 proxy.
func PrintIP(socksPort int) {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	ip, err := ExternalIP(addr, 5*time.Second)
	if err != nil {
		fmt.Printf("  IP check: %v\n", err)
		return
	}
	fmt.Printf("  Exit IP: %s\n", ip)
}

// WaitIP retries through the local SOCKS5 proxy until the external IP is
// obtained or the timeout elapses.
func WaitIP(socksPort int, timeout time.Duration) (string, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	dial, err := socksDial(addr)
	if err != nil {
		return "", err
	}
	return waitVia(dial, "https://cloudflare.com/cdn-cgi/trace", timeout)
}

// WaitURL retries through the local SOCKS5 proxy until urlStr is reachable or
// the timeout elapses.
func WaitURL(socksPort int, urlStr string, timeout time.Duration) (string, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	dial, err := socksDial(addr)
	if err != nil {
		return "", err
	}
	return waitVia(dial, urlStr, timeout)
}

// WaitDirect retries over the system route (no SOCKS) until urlStr is
// reachable or the timeout elapses.
func WaitDirect(urlStr string, timeout time.Duration) (string, error) {
	return waitVia(nil, urlStr, timeout)
}
