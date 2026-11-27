package tunnel

import (
	"fmt"
	"testing"
)

func TestPickUtunNameLowestFree(t *testing.T) {
	taken := map[string]bool{"utun0": true, "utun1": true, "utun3": true}
	got := pickUtunName(func(n string) bool { return taken[n] })
	if got != "utun2" {
		t.Errorf("pickUtunName = %q, want utun2 (lowest free after utun0/utun1)", got)
	}
}

func TestPickUtunNameNothingTaken(t *testing.T) {
	if got := pickUtunName(func(string) bool { return false }); got != "utun0" {
		t.Errorf("pickUtunName = %q, want utun0", got)
	}
}

func TestPickUtunNameAllTaken(t *testing.T) {
	if got := pickUtunName(func(string) bool { return true }); got != "" {
		t.Errorf("pickUtunName = %q, want empty when every number is taken", got)
	}
}

func TestPickUtunNameStaysInRange(t *testing.T) {
	// The last candidate must be utun255, not something outside the range that
	// macOS would reject.
	seen := map[string]bool{}
	got := pickUtunName(func(n string) bool {
		seen[n] = true
		return n != fmt.Sprintf("utun%d", utunNameRange-1)
	})
	if want := fmt.Sprintf("utun%d", utunNameRange-1); got != want {
		t.Errorf("pickUtunName = %q, want %q", got, want)
	}
	if len(seen) != utunNameRange {
		t.Errorf("probed %d names, want %d", len(seen), utunNameRange)
	}
}
