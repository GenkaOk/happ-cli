package link

import (
	"fmt"
	"net/url"
	"strconv"
)

// parseVLESS parses a vless:// share link.
//
// Format: vless://UUID@host:port?type=..&security=..&...#tag
func parseVLESS(raw string) (*Server, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("vless: %w", err)
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("vless: invalid port %q: %w", u.Port(), err)
	}

	q := u.Query()
	s := &Server{
		Protocol: "vless",
		Tag:      u.Fragment,
		Address:  u.Hostname(),
		Port:     port,
		UUID:     u.User.Username(),
		Raw:      raw,

		Flow:     queryString(q, "flow"),
		Network:  networkOrDefault(queryString(q, "type")),
		Security: queryString(q, "security"),

		Path:        queryString(q, "path"),
		Host:        queryString(q, "host"),
		ServiceName: queryString(q, "serviceName"),
		HeaderType:  queryString(q, "headerType"),

		SNI:         queryString(q, "sni", "peer"),
		ALPN:        splitCSV(queryString(q, "alpn")),
		Fingerprint: queryString(q, "fp"),

		PublicKey: queryString(q, "pbk"),
		ShortID:   queryString(q, "sid"),
		SpiderX:   queryString(q, "spx"),
	}
	s.AllowInsecure = boolQuery(queryString(q, "allowInsecure", "insecure"))

	if s.UUID == "" {
		return nil, fmt.Errorf("vless: missing UUID")
	}
	return s, nil
}

// networkOrDefault normalizes the transport network, defaulting to tcp.
func networkOrDefault(network string) string {
	if network == "" {
		return "tcp"
	}
	return network
}

// boolQuery interprets common truthy query values (1, true).
func boolQuery(v string) bool {
	switch v {
	case "1", "true", "True", "TRUE":
		return true
	default:
		return false
	}
}
