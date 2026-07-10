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

// ExternalIP connects through a local SOCKS5 proxy to Cloudflare's trace
// endpoint and returns the visible external IP address.
func ExternalIP(socksAddr string, timeout time.Duration) (string, error) {
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return "", fmt.Errorf("socks5 dialer: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	client := &http.Client{Transport: transport, Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://cloudflare.com/cdn-cgi/trace", nil)
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

	// Cloudflare trace: key=value lines.
	for _, line := range strings.Split(string(body), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && k == "ip" {
			return v, nil
		}
	}
	return "", fmt.Errorf("ip not found in response")
}

// PrintIP fetches and prints the external IP using the local SOCKS5 proxy.
// Intended to be called as a goroutine after the proxy is up.
func PrintIP(socksPort int) {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	ip, err := ExternalIP(addr, 5*time.Second)
	if err != nil {
		fmt.Printf("  IP check: %v\n", err)
		return
	}
	fmt.Printf("  Exit IP: %s\n", ip)
}

// WaitIP blocks until the external IP is obtained through the proxy or timeout
// is reached. Returns the IP on success. Use after the proxy is up to verify
// connectivity before proceeding.
func WaitIP(socksPort int, timeout time.Duration) (string, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ip, err := ExternalIP(addr, 3*time.Second)
		if err == nil {
			fmt.Printf("  Exit IP: %s\n", ip)
			return ip, nil
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("IP check failed after %v: %w", timeout, lastErr)
}
