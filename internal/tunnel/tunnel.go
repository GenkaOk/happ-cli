// Package tunnel provides a system-wide TUN tunnel that forwards all traffic to
// a local SOCKS5 proxy (served by the embedded xray-core). On macOS it creates
// a utun device via tun2socks and rewrites the routing table so the whole
// system is tunneled; the encrypted connection to the proxy server itself is
// pinned to the physical gateway to avoid a routing loop.
package tunnel

// Options configures a TUN tunnel.
type Options struct {
	// SocksAddr is the host:port of the local SOCKS5 proxy to forward to.
	SocksAddr string
	// ServerIPs are the resolved IP addresses of the proxy server; routes to
	// these are pinned to the physical gateway so the tunnel does not loop.
	ServerIPs []string
	// TunName is the utun device name (default "utun123").
	TunName string
	// TunIP is the address assigned to the tun device (default "198.18.0.1").
	TunIP string
	// MTU for the tun device (default 1500).
	MTU int
	// LogLevel for tun2socks (default "warning").
	LogLevel string
}

func (o *Options) withDefaults() {
	if o.TunName == "" {
		o.TunName = "utun123"
	}
	if o.TunIP == "" {
		o.TunIP = "198.18.0.1"
	}
	if o.MTU == 0 {
		o.MTU = 1500
	}
	if o.LogLevel == "" {
		// tun2socks parses zapcore levels ("warn"), not xray's "warning".
		o.LogLevel = "warn"
	}
}
