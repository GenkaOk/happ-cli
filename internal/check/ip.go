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
	return fetchViaSOCKS(socksAddr, "https://cloudflare.com/cdn-cgi/trace", timeout)
}

// FetchURL fetches a URL through the SOCKS5 proxy and returns the response
// body. Use for custom health-check endpoints.
func FetchURL(socksAddr, urlStr string, timeout time.Duration) (string, error) {
	return fetchViaSOCKS(socksAddr, urlStr, timeout)
}

// fetchViaSOCKS fetches a URL through a SOCKS5 proxy and returns the response body.
func fetchViaSOCKS(socksAddr, urlStr string, timeout time.Duration) (string, error) {
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

// WaitIP blocks until the external IP is obtained through the proxy or timeout.
func WaitIP(socksPort int, timeout time.Duration) (string, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	return ExternalIP(addr, timeout)
}

// WaitURL blocks until urlStr is reachable through the SOCKS proxy.
func WaitURL(socksPort int, urlStr string, timeout time.Duration) (string, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", socksPort)
	return FetchURL(addr, urlStr, timeout)
}
