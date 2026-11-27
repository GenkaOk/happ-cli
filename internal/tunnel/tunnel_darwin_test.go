//go:build darwin

package tunnel

import "testing"

// TestUtunNameShape pins the macOS requirement that came from a real failure:
// tun2socks refuses any TUN name that is not utunN, so the default and the
// validation must both stay in that shape.
func TestUtunNameShape(t *testing.T) {
	for _, good := range []string{"utun0", "utun9", "utun42", "utun255"} {
		if !utunNameRe.MatchString(good) {
			t.Errorf("%q should be accepted as a utun name", good)
		}
	}
	for _, bad := range []string{"thapp", "utun", "utunx", "utun9x", "xutun9", ""} {
		if utunNameRe.MatchString(bad) {
			t.Errorf("%q should be rejected as a utun name", bad)
		}
	}
}

func TestDefaultTunNameIsUtun(t *testing.T) {
	got := defaultTunName()
	if !utunNameRe.MatchString(got) {
		t.Fatalf("defaultTunName() = %q, want a name matching utun[0-9]+", got)
	}
	t.Logf("default TUN name on this machine: %s", got)
}
