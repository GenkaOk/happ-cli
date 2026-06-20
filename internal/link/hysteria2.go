package link

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// parseHysteria2 parses a hysteria2:// (or hy2://) share link.
//
// Format: hysteria2://auth@host:port?sni=..&obfs=..&obfs-password=..#tag
//
// Note: hysteria2 is not supported by xray-core outbound; it is parsed so the
// server can be listed, but connecting requires a different core (e.g. sing-box).
func parseHysteria2(raw string) (*Server, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("hysteria2: %w", err)
	}

	host, port, err := hy2HostPort(u)
	if err != nil {
		return nil, err
	}

	auth := u.User.Username()
	if p, ok := u.User.Password(); ok {
		auth = auth + ":" + p
	}

	q := u.Query()
	return &Server{
		Protocol:      "hysteria2",
		Tag:           u.Fragment,
		Address:       host,
		Port:          port,
		Password:      auth,
		Network:       "udp",
		Security:      "tls",
		SNI:           queryString(q, "sni", "peer"),
		ALPN:          splitCSV(queryString(q, "alpn")),
		Fingerprint:   queryString(q, "fp"),
		AllowInsecure: boolQuery(queryString(q, "insecure", "allowInsecure")),
		Obfs:          queryString(q, "obfs"),
		ObfsPassword:  queryString(q, "obfs-password", "obfs_password"),
		Raw:           raw,
	}, nil
}

// hy2HostPort extracts host and the first port, tolerating multi-port / hopping
// authorities such as host:443,5000-6000.
func hy2HostPort(u *url.URL) (string, int, error) {
	if p := u.Port(); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			return "", 0, fmt.Errorf("hysteria2: invalid port %q: %w", p, err)
		}
		return u.Hostname(), port, nil
	}

	// Fallback for multi-port authorities url.Port() rejects.
	host := u.Host
	colon := strings.LastIndexByte(host, ':')
	if colon < 0 {
		return "", 0, fmt.Errorf("hysteria2: missing port in %q", host)
	}
	portPart := host[colon+1:]
	host = host[:colon]
	if i := strings.IndexAny(portPart, ",-"); i >= 0 {
		portPart = portPart[:i]
	}
	port, err := strconv.Atoi(portPart)
	if err != nil {
		return "", 0, fmt.Errorf("hysteria2: invalid port %q: %w", portPart, err)
	}
	return host, port, nil
}
