package tunnel

import (
	"testing"

	t2slog "github.com/xjasonlyu/tun2socks/v2/log"
)

// TestDefaultLogLevelAcceptedByEngine guards against passing an xray-style log
// level (e.g. "warning") to tun2socks, whose engine uses zapcore levels.
func TestDefaultLogLevelAcceptedByEngine(t *testing.T) {
	var o Options
	o.withDefaults()
	if _, err := t2slog.ParseLevel(o.LogLevel); err != nil {
		t.Fatalf("default LogLevel %q rejected by tun2socks: %v", o.LogLevel, err)
	}
}

func TestDefaults(t *testing.T) {
	var o Options
	o.withDefaults()
	if o.TunName == "" || o.TunIP == "" || o.MTU == 0 {
		t.Errorf("withDefaults left zero values: %+v", o)
	}
}
