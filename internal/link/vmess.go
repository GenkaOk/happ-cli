package link

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// vmessString accepts a JSON value that may be encoded either as a string or a
// bare number/bool, which both occur in vmess:// payloads in the wild.
type vmessString string

func (v *vmessString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) >= 2 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*v = vmessString(s)
		return nil
	}
	*v = vmessString(strings.Trim(string(b), `"`))
	return nil
}

func (v vmessString) String() string { return string(v) }

// vmessConfig mirrors the JSON object embedded in a vmess:// link (v2rayN style).
type vmessConfig struct {
	PS   vmessString `json:"ps"`
	Add  vmessString `json:"add"`
	Port vmessString `json:"port"`
	ID   vmessString `json:"id"`
	Aid  vmessString `json:"aid"`
	Scy  vmessString `json:"scy"`
	Net  vmessString `json:"net"`
	Type vmessString `json:"type"`
	Host vmessString `json:"host"`
	Path vmessString `json:"path"`
	TLS  vmessString `json:"tls"`
	SNI  vmessString `json:"sni"`
	ALPN vmessString `json:"alpn"`
	FP   vmessString `json:"fp"`
}

// parseVMess parses a vmess:// share link (base64-encoded JSON).
func parseVMess(raw string) (*Server, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	decoded, err := decodeBase64(payload)
	if err != nil {
		return nil, fmt.Errorf("vmess: base64 decode: %w", err)
	}

	var c vmessConfig
	if err := json.Unmarshal(decoded, &c); err != nil {
		return nil, fmt.Errorf("vmess: json decode: %w", err)
	}

	port, err := strconv.Atoi(c.Port.String())
	if err != nil {
		return nil, fmt.Errorf("vmess: invalid port %q: %w", c.Port, err)
	}
	aid, _ := strconv.Atoi(c.Aid.String())

	method := c.Scy.String()
	if method == "" {
		method = "auto"
	}
	security := "none"
	if t := c.TLS.String(); t == "tls" || t == "reality" {
		security = t
	}

	return &Server{
		Protocol:    "vmess",
		Tag:         c.PS.String(),
		Address:     c.Add.String(),
		Port:        port,
		UUID:        c.ID.String(),
		AlterID:     aid,
		Method:      method,
		Network:     networkOrDefault(c.Net.String()),
		HeaderType:  c.Type.String(),
		Host:        c.Host.String(),
		Path:        c.Path.String(),
		Security:    security,
		SNI:         c.SNI.String(),
		ALPN:        splitCSV(c.ALPN.String()),
		Fingerprint: c.FP.String(),
		Raw:         raw,
	}, nil
}
