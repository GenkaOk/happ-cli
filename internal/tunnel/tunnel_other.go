//go:build !darwin && !linux

package tunnel

import (
	"fmt"
	"runtime"
)

// defaultTunName is only used by platforms with a real TUN implementation, but
// withDefaults references it on every build.
func defaultTunName() string { return "happ" }

// Tunnel is a no-op on platforms where TUN mode is not yet implemented.
type Tunnel struct{}

// Start reports that TUN mode is unavailable on this platform.
func Start(opts Options) (*Tunnel, error) {
	return nil, fmt.Errorf("TUN mode is not implemented on %s yet; use --mode proxy", runtime.GOOS)
}

// Close is a no-op.
func (t *Tunnel) Close() error { return nil }
