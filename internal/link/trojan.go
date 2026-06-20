package link

import (
	"fmt"
	"net/url"
	"strconv"
)

// parseTrojan parses a trojan:// share link.
//
// Format: trojan://password@host:port?security=..&sni=..&...#tag
func parseTrojan(raw string) (*Server, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("trojan: %w", err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("trojan: invalid port %q: %w", u.Port(), err)
	}

	q := u.Query()
	security := queryString(q, "security")
	if security == "" {
		// Trojan mandates TLS; share links frequently omit security=tls.
		security = "tls"
	}

	password := u.User.Username()
	if password == "" {
		return nil, fmt.Errorf("trojan: missing password")
	}

	return &Server{
		Protocol:      "trojan",
		Tag:           u.Fragment,
		Address:       u.Hostname(),
		Port:          port,
		Password:      password,
		Network:       networkOrDefault(queryString(q, "type")),
		HeaderType:    queryString(q, "headerType"),
		Path:          queryString(q, "path"),
		Host:          queryString(q, "host"),
		ServiceName:   queryString(q, "serviceName"),
		Security:      security,
		SNI:           queryString(q, "sni", "peer"),
		ALPN:          splitCSV(queryString(q, "alpn")),
		Fingerprint:   queryString(q, "fp"),
		Flow:          queryString(q, "flow"),
		AllowInsecure: boolQuery(queryString(q, "allowInsecure", "insecure")),
		Raw:           raw,
	}, nil
}
