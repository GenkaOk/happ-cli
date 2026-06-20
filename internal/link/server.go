// Package link parses proxy share links (vless://, vmess://, trojan://, ss://,
// hysteria2://) into a normalized Server description that other packages turn
// into xray-core outbound configuration.
package link

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrUnsupportedScheme is returned by Parse for a URI whose scheme is not a
// recognized proxy protocol.
var ErrUnsupportedScheme = errors.New("link: unsupported scheme")

// Server is a protocol-agnostic description of a single proxy endpoint parsed
// from a share link. Fields that do not apply to a given protocol stay at their
// zero value.
type Server struct {
	// Tag is the human-readable name taken from the link fragment (#tag).
	Tag      string
	Protocol string // vless, vmess, trojan, shadowsocks, hysteria2
	Address  string
	Port     int

	// Credentials (protocol-specific).
	UUID     string // vless / vmess id
	Password string // trojan / shadowsocks / hysteria2
	Method   string // shadowsocks cipher
	AlterID  int    // vmess alterId
	Flow     string // vless flow, e.g. xtls-rprx-vision

	// Transport.
	Network     string // tcp, ws, grpc, http, quic, kcp
	Path        string // ws / http path
	Host        string // ws / http Host header
	ServiceName string // grpc service name
	HeaderType  string // tcp header obfuscation type

	// Security / TLS.
	Security      string // none, tls, reality
	SNI           string
	ALPN          []string
	Fingerprint   string // uTLS fingerprint
	AllowInsecure bool

	// REALITY.
	PublicKey string // pbk
	ShortID   string // sid
	SpiderX   string // spx

	// Hysteria2.
	Obfs         string
	ObfsPassword string

	// Raw is the original share link.
	Raw string
}

// Parse turns a single share link into a normalized Server.
func Parse(raw string) (*Server, error) {
	raw = strings.TrimSpace(raw)
	scheme, _, ok := strings.Cut(raw, "://")
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedScheme, raw)
	}

	switch strings.ToLower(scheme) {
	case "vless":
		return parseVLESS(raw)
	case "vmess":
		return parseVMess(raw)
	case "trojan":
		return parseTrojan(raw)
	case "ss":
		return parseShadowsocks(raw)
	case "hysteria2", "hy2":
		return parseHysteria2(raw)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedScheme, scheme)
	}
}

// queryString returns the first value for key, or "".
func queryString(q url.Values, keys ...string) string {
	for _, k := range keys {
		if v := q.Get(k); v != "" {
			return v
		}
	}
	return ""
}

// decodeBase64 decodes a base64 string trying the standard, URL-safe, and
// raw (unpadded) variants used by various share-link generators.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, enc := range encodings {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		} else {
			lastErr = err
		}
	}
	return nil, lastErr
}

// splitCSV splits a comma-separated query value into trimmed, non-empty parts.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
