//go:build !darwin

package sysproxy

import (
	"fmt"
	"runtime"
)

// Restore reverts the system proxy to its previous state.
type Restore func() error

// Enable is unsupported outside macOS in this version.
func Enable(host string, socksPort, httpPort int) (Restore, error) {
	return nil, fmt.Errorf("--system-proxy is not implemented on %s yet", runtime.GOOS)
}

// DisableAll is unsupported outside macOS in this version.
func DisableAll() error {
	return fmt.Errorf("system-proxy is not implemented on %s yet", runtime.GOOS)
}
