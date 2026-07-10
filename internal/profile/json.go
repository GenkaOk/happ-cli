package profile

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aimuzov/happ-cli/internal/link"
)

// xrayEntry is a single config entry from the Incy JSON subscription array.
type xrayEntry struct {
	Remarks   string         `json:"remarks"`
	Outbounds []xrayOutbound `json:"outbounds"`
}

type xrayOutbound struct {
	Tag            string              `json:"tag"`
	Protocol       string              `json:"protocol"`
	Settings       json.RawMessage     `json:"settings"`
	StreamSettings *xrayStreamSettings `json:"streamSettings,omitempty"`
}

type xrayStreamSettings struct {
	Network          string                `json:"network,omitempty"`
	Security         string                `json:"security,omitempty"`
	RealitySettings  *xrayRealitySettings  `json:"realitySettings,omitempty"`
	TLSSettings      *xrayTLSSettings      `json:"tlsSettings,omitempty"`
	WSSettings       *xrayWSSettings       `json:"wsSettings,omitempty"`
	HysteriaSettings *xrayHysteriaSettings `json:"hysteriaSettings,omitempty"`
}

type xrayRealitySettings struct {
	ServerName  string `json:"serverName,omitempty"`
	PublicKey   string `json:"publicKey,omitempty"`
	ShortID     string `json:"shortId,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type xrayTLSSettings struct {
	ServerName  string   `json:"serverName,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	ALPN        []string `json:"alpn,omitempty"`
}

type xrayWSSettings struct {
	Path            string            `json:"path,omitempty"`
	Host            string            `json:"host,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	HeartbeatPeriod int               `json:"heartbeatPeriod,omitempty"`
}

type xrayHysteriaSettings struct {
	Version int    `json:"version,omitempty"`
	Auth    string `json:"auth,omitempty"`
}

// vlessVnext mirrors xray VLESS/VMess settings.vnext[0].
type vlessVnext struct {
	Address string      `json:"address"`
	Port    int         `json:"port"`
	Users   []vlessUser `json:"users"`
}

type vlessUser struct {
	ID         string `json:"id"`
	Encryption string `json:"encryption,omitempty"`
	Flow       string `json:"flow,omitempty"`
}

// trojanSrv mirrors xray Trojan settings.servers[0].
type trojanSrv struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Password string `json:"password"`
}

// parseJSONBody detects Incy-style JSON subscription body (array of xray configs)
// and extracts share links from outbounds. Returns nil if body is not JSON.
func parseJSONBody(body []byte) ([]*link.Server, error) {
	text := strings.TrimSpace(string(body))
	if text == "" || (text[0] != '[' && text[0] != '{') {
		return nil, nil
	}

	var entries []xrayEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, nil // not JSON → fall through
	}

	var servers []*link.Server
	for _, entry := range entries {
		for i := range entry.Outbounds {
			s := outboundToLink(&entry.Outbounds[i], entry.Remarks)
			if s != nil {
				servers = append(servers, s)
			}
		}
	}
	return servers, nil
}

// outboundToLink converts an xray outbound to a share-link *link.Server.
// Returns nil for non-proxy protocols (freedom, blackhole, loopback, socks).
func outboundToLink(ob *xrayOutbound, remarks string) *link.Server {
	switch strings.ToLower(ob.Protocol) {
	case "vless":
		return vlessToLink(ob, remarks)
	case "trojan":
		return trojanToLink(ob, remarks)
	case "vmess":
		return vmessToLink(ob, remarks)
	case "shadowsocks", "ss":
		return shadowsocksToLink(ob, remarks)
	case "hysteria", "hysteria2", "hy2":
		return hysteriaToLink(ob, remarks)
	}
	return nil
}

func vlessToLink(ob *xrayOutbound, remarks string) *link.Server {
	var vnext []vlessVnext
	if err := json.Unmarshal(ob.Settings, &struct {
		Vnext *[]vlessVnext `json:"vnext"`
	}{Vnext: &vnext}); err != nil || len(vnext) == 0 || len(vnext[0].Users) == 0 {
		return nil
	}
	v := vnext[0]
	u := v.Users[0]
	if u.ID == "" {
		return nil
	}

	linkURL := url.URL{
		Scheme:   "vless",
		User:     url.User(u.ID),
		Host:     fmt.Sprintf("%s:%d", v.Address, v.Port),
		Fragment: remarks,
	}
	query := url.Values{}
	if u.Encryption != "" && u.Encryption != "none" {
		query.Set("encryption", u.Encryption)
	}
	if u.Flow != "" {
		query.Set("flow", u.Flow)
	}
	applyStream(ob.StreamSettings, query)
	linkURL.RawQuery = query.Encode()
	raw := linkURL.String()

	s, err := link.Parse(raw)
	if err != nil {
		return nil
	}
	s.Tag = remarks
	s.Raw = raw
	return s
}

func trojanToLink(ob *xrayOutbound, remarks string) *link.Server {
	var servers []trojanSrv
	if err := json.Unmarshal(ob.Settings, &struct {
		Servers *[]trojanSrv `json:"servers"`
	}{Servers: &servers}); err != nil || len(servers) == 0 {
		return nil
	}
	srv := servers[0]
	if srv.Password == "" {
		return nil
	}

	linkURL := url.URL{
		Scheme:   "trojan",
		User:     url.User(srv.Password),
		Host:     fmt.Sprintf("%s:%d", srv.Address, srv.Port),
		Fragment: remarks,
	}
	query := url.Values{}
	applyStream(ob.StreamSettings, query)
	linkURL.RawQuery = query.Encode()
	raw := linkURL.String()

	s, err := link.Parse(raw)
	if err != nil {
		return nil
	}
	s.Tag = remarks
	s.Raw = raw
	return s
}

func vmessToLink(ob *xrayOutbound, remarks string) *link.Server {
	var vnext []vlessVnext
	if err := json.Unmarshal(ob.Settings, &struct {
		Vnext *[]vlessVnext `json:"vnext"`
	}{Vnext: &vnext}); err != nil || len(vnext) == 0 || len(vnext[0].Users) == 0 {
		return nil
	}
	v := vnext[0]
	u := v.Users[0]
	if u.ID == "" {
		return nil
	}

	cfg := map[string]interface{}{
		"v":    "2",
		"ps":   remarks,
		"add":  v.Address,
		"port": v.Port,
		"id":   u.ID,
		"aid":  0,
	}
	if ob.StreamSettings != nil {
		ss := ob.StreamSettings
		if ss.Network != "" {
			cfg["net"] = ss.Network
		}
		if ss.Security != "" {
			cfg["tls"] = ss.Security
		}
		if ss.WSSettings != nil {
			if ss.WSSettings.Path != "" {
				cfg["path"] = ss.WSSettings.Path
			}
			if ss.WSSettings.Host != "" {
				cfg["host"] = ss.WSSettings.Host
			}
		}
		if ss.TLSSettings != nil {
			if ss.TLSSettings.ServerName != "" {
				cfg["sni"] = ss.TLSSettings.ServerName
			}
			if ss.TLSSettings.Fingerprint != "" {
				cfg["fp"] = ss.TLSSettings.Fingerprint
			}
			if len(ss.TLSSettings.ALPN) > 0 {
				cfg["alpn"] = strings.Join(ss.TLSSettings.ALPN, ",")
			}
		}
	}

	b, _ := json.Marshal(cfg)
	raw := "vmess://" + base64.StdEncoding.EncodeToString(b)

	s, err := link.Parse(raw)
	if err != nil {
		return nil
	}
	s.Tag = remarks
	s.Raw = raw
	return s
}

func shadowsocksToLink(ob *xrayOutbound, remarks string) *link.Server {
	type ssSrv struct {
		Address  string `json:"address"`
		Port     int    `json:"port"`
		Method   string `json:"method"`
		Password string `json:"password"`
	}
	var servers []ssSrv
	if err := json.Unmarshal(ob.Settings, &struct {
		Servers *[]ssSrv `json:"servers"`
	}{Servers: &servers}); err != nil || len(servers) == 0 {
		return nil
	}
	srv := servers[0]
	if srv.Method == "" || srv.Password == "" {
		return nil
	}

	userInfo := base64.StdEncoding.EncodeToString([]byte(srv.Method + ":" + srv.Password))
	linkURL := url.URL{
		Scheme:   "ss",
		User:     url.User(userInfo),
		Host:     fmt.Sprintf("%s:%d", srv.Address, srv.Port),
		Fragment: remarks,
	}
	raw := linkURL.String()

	s, err := link.Parse(raw)
	if err != nil {
		return nil
	}
	s.Tag = remarks
	s.Raw = raw
	return s
}

func hysteriaToLink(ob *xrayOutbound, remarks string) *link.Server {
	type hy2Settings struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
		Version int    `json:"version,omitempty"`
		Auth    string `json:"auth,omitempty"`
	}
	var hy2 hy2Settings
	if err := json.Unmarshal(ob.Settings, &hy2); err != nil || hy2.Address == "" {
		return nil
	}

	password := hy2.Auth
	if ob.StreamSettings != nil && ob.StreamSettings.HysteriaSettings != nil {
		if ob.StreamSettings.HysteriaSettings.Auth != "" {
			password = ob.StreamSettings.HysteriaSettings.Auth
		}
	}
	if password == "" {
		return nil
	}

	linkURL := url.URL{
		Scheme:   "hysteria2",
		User:     url.User(password),
		Host:     fmt.Sprintf("%s:%d", hy2.Address, hy2.Port),
		Fragment: remarks,
	}
	query := url.Values{}
	if ob.StreamSettings != nil && ob.StreamSettings.TLSSettings != nil {
		ts := ob.StreamSettings.TLSSettings
		if ts.ServerName != "" {
			query.Set("sni", ts.ServerName)
		}
		if ts.Fingerprint != "" {
			query.Set("fp", ts.Fingerprint)
		}
		if len(ts.ALPN) > 0 {
			query.Set("alpn", strings.Join(ts.ALPN, ","))
		}
	}
	linkURL.RawQuery = query.Encode()
	raw := linkURL.String()

	s, err := link.Parse(raw)
	if err != nil {
		return nil
	}
	s.Tag = remarks
	s.Raw = raw
	return s
}

// applyStream writes transport/tls/reality params from streamSettings into query.
func applyStream(ss *xrayStreamSettings, query url.Values) {
	if ss == nil {
		return
	}
	if ss.Network != "" && ss.Network != "tcp" {
		query.Set("type", ss.Network)
	}
	if ss.Security != "" && ss.Security != "none" {
		query.Set("security", ss.Security)
	}
	if ss.WSSettings != nil {
		if ss.WSSettings.Path != "" {
			query.Set("path", ss.WSSettings.Path)
		}
		if ss.WSSettings.Host != "" {
			query.Set("host", ss.WSSettings.Host)
		}
	}
	if ss.RealitySettings != nil {
		rs := ss.RealitySettings
		if rs.ServerName != "" {
			query.Set("sni", rs.ServerName)
		}
		if rs.PublicKey != "" {
			query.Set("pbk", rs.PublicKey)
		}
		if rs.ShortID != "" {
			query.Set("sid", rs.ShortID)
		}
		if rs.Fingerprint != "" {
			query.Set("fp", rs.Fingerprint)
		}
	}
	if ss.TLSSettings != nil {
		ts := ss.TLSSettings
		if ts.ServerName != "" {
			query.Set("sni", ts.ServerName)
		}
		if ts.Fingerprint != "" {
			query.Set("fp", ts.Fingerprint)
		}
		if len(ts.ALPN) > 0 {
			query.Set("alpn", strings.Join(ts.ALPN, ","))
		}
	}
}
