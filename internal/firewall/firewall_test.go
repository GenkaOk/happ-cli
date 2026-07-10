package firewall

import "testing"

func TestRulesNoOp(t *testing.T) {
	// On non-Linux, Rules returns a no-op cleanup.
	// On Linux, it tries iptables (best-effort, won't fail test without root).
	cleanup := Rules("test0")
	if cleanup == nil {
		t.Error("Rules returned nil cleanup")
	}
	// Must not panic.
	cleanup()
}
