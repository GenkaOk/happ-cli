package check

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DirectTarget probes the routed path (system routing table, no SOCKS) while
// resolving the URL's host exactly once, up front. Probes then dial the
// resolved IP with Host/SNI preserved, so a broken resolver on the router can
// never masquerade as a dead tunnel.
type DirectTarget struct {
	urlStr     string // original URL, with the original host in it
	scheme     string
	host       string // hostname used for Host/SNI
	hostHeader string // original host[:port], for the Host header
	ip         string // resolved IPv4 address to dial
	port       string
}

// NewDirectTarget parses rawURL and resolves its host once. The resolution
// happens when NewDirectTarget is called, which for TUN modes must be before
// routes are installed. An IPv4 literal in the URL skips the lookup entirely.
func NewDirectTarget(rawURL string, timeout time.Duration) (*DirectTarget, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse check URL: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("check URL %q has no host", rawURL)
	}

	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return nil, fmt.Errorf("unsupported check URL scheme %q", u.Scheme)
		}
	}

	host := u.Hostname()
	ip := ""
	if parsed := net.ParseIP(host); parsed != nil && parsed.To4() != nil {
		ip = parsed.To4().String()
	} else {
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", host, err)
		}
		for _, a := range addrs {
			if v4 := a.IP.To4(); v4 != nil {
				ip = v4.String()
				break
			}
		}
		if ip == "" {
			return nil, fmt.Errorf("no IPv4 address found for %q", host)
		}
	}

	return &DirectTarget{
		urlStr:     rawURL,
		scheme:     scheme,
		host:       host,
		hostHeader: u.Host,
		ip:         ip,
		port:       port,
	}, nil
}

// Fetch performs one HTTP GET over the routed path, dialing the pre-resolved
// IPv4 address while keeping the original Host header and TLS SNI.
func (t *DirectTarget) Fetch(timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	dialer := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", net.JoinHostPort(t.ip, t.port))
		},
	}
	if t.scheme == "https" {
		transport.TLSClientConfig = &tls.Config{ServerName: t.host, InsecureSkipVerify: false}
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.urlStr, nil)
	if err != nil {
		return "", err
	}
	if t.scheme == "http" {
		req.Host = t.hostHeader
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
	return parseBody(body), nil
}

// Wait retries Fetch until it succeeds or the timeout elapses, using the same
// policy as waitVia: each attempt is capped at 3 seconds (and at the remaining
// time), with a 250ms pause between attempts. On timeout it reports the attempt
// count and the last error.
func (t *DirectTarget) Wait(timeout time.Duration) (string, error) {
	return retryFetch(t.Fetch, timeout)
}

// retryFetch is the shared retry loop behind waitVia and DirectTarget.Wait.
func retryFetch(fetch func(timeout time.Duration) (string, error), timeout time.Duration) (string, error) {
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

		body, err := fetch(attemptTimeout)
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

// IsDNSError reports whether err (or anything it wraps) is a DNS failure. A
// DNS failure during a routed-path probe says nothing about the tunnel: the
// probe never reached it.
func IsDNSError(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}
