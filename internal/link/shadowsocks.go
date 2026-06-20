package link

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// parseShadowsocks parses an ss:// share link in either the SIP002 form
//
//	ss://base64(method:password)@host:port[/?plugin=..]#tag
//
// or the legacy fully base64-encoded form
//
//	ss://base64(method:password@host:port)#tag
func parseShadowsocks(raw string) (*Server, error) {
	body := strings.TrimPrefix(raw, "ss://")

	// Split off the #tag fragment.
	var tag string
	if i := strings.IndexByte(body, '#'); i >= 0 {
		tag, _ = url.QueryUnescape(body[i+1:])
		body = body[:i]
	}

	method, password, host, port, err := decodeSS(body)
	if err != nil {
		return nil, err
	}

	return &Server{
		Protocol: "shadowsocks",
		Tag:      tag,
		Address:  host,
		Port:     port,
		Method:   method,
		Password: password,
		Network:  "tcp",
		Raw:      raw,
	}, nil
}

func decodeSS(body string) (method, password, host string, port int, err error) {
	if at := strings.LastIndexByte(body, '@'); at >= 0 {
		// SIP002: userinfo@host:port[/?plugin]
		method, password, ok := decodeSSUserInfo(body[:at])
		if !ok {
			return "", "", "", 0, fmt.Errorf("ss: cannot parse userinfo")
		}
		host, port, err := splitHostPort(stripPathQuery(body[at+1:]))
		return method, password, host, port, err
	}

	// Legacy: whole thing is base64(method:password@host:port).
	decoded, derr := decodeBase64(body)
	if derr != nil {
		return "", "", "", 0, fmt.Errorf("ss: base64 decode: %w", derr)
	}
	at := strings.LastIndexByte(string(decoded), '@')
	if at < 0 {
		return "", "", "", 0, fmt.Errorf("ss: malformed legacy link")
	}
	method, password, found := strings.Cut(string(decoded[:at]), ":")
	if !found {
		return "", "", "", 0, fmt.Errorf("ss: malformed method:password")
	}
	host, port, err = splitHostPort(string(decoded[at+1:]))
	return method, password, host, port, err
}

// decodeSSUserInfo extracts method and password from the userinfo segment,
// which may be base64-encoded or plain "method:password".
func decodeSSUserInfo(userinfo string) (method, password string, ok bool) {
	if dec, err := decodeBase64(userinfo); err == nil {
		if m, p, found := strings.Cut(string(dec), ":"); found {
			return m, p, true
		}
	}
	if m, p, found := strings.Cut(userinfo, ":"); found {
		return m, p, true
	}
	return "", "", false
}

// stripPathQuery removes any trailing path or query (e.g. /?plugin=..).
func stripPathQuery(s string) string {
	if i := strings.IndexAny(s, "/?"); i >= 0 {
		return s[:i]
	}
	return s
}

func splitHostPort(hp string) (host string, port int, err error) {
	h, p, found := strings.Cut(hp, ":")
	if !found {
		return "", 0, fmt.Errorf("ss: missing port in %q", hp)
	}
	port, err = strconv.Atoi(p)
	if err != nil {
		return "", 0, fmt.Errorf("ss: invalid port %q: %w", p, err)
	}
	return h, port, nil
}
