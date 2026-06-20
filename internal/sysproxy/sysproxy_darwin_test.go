//go:build darwin

package sysproxy

import "testing"

func TestProxyKinds(t *testing.T) {
	kinds := proxyKinds(10808, 10809)
	if len(kinds) != 3 {
		t.Fatalf("got %d kinds, want 3", len(kinds))
	}
	want := []kind{
		{token: "socksfirewallproxy", port: 10808},
		{token: "webproxy", port: 10809},
		{token: "securewebproxy", port: 10809},
	}
	for i, w := range want {
		if kinds[i] != w {
			t.Errorf("kind[%d] = %+v, want %+v", i, kinds[i], w)
		}
	}
}

func TestParseServices(t *testing.T) {
	out := `An asterisk (*) denotes that a network service is disabled.
Thunderbolt Bridge
Wi-Fi
*Old Ethernet
iPhone USB
`
	got := parseServices(out)
	want := []string{"Thunderbolt Bridge", "Wi-Fi", "iPhone USB"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseProxyStateEnabled(t *testing.T) {
	out := `Enabled: Yes
Server: 127.0.0.1
Port: 10808
Authenticated Proxy Enabled: 0
`
	st := parseProxyState(out)
	if !st.Enabled {
		t.Error("Enabled = false, want true")
	}
	if st.Server != "127.0.0.1" {
		t.Errorf("Server = %q", st.Server)
	}
	if st.Port != 10808 {
		t.Errorf("Port = %d", st.Port)
	}
}

func TestParseProxyStateDisabledEmptyServer(t *testing.T) {
	out := "Enabled: No\nServer: \nPort: 0\nAuthenticated Proxy Enabled: 0\n"
	st := parseProxyState(out)
	if st.Enabled {
		t.Error("Enabled = true, want false")
	}
	if st.Server != "" || st.Port != 0 {
		t.Errorf("Server/Port = %q/%d, want empty/0", st.Server, st.Port)
	}
}
