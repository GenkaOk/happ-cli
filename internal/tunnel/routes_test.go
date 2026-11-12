package tunnel

import (
	"reflect"
	"testing"
)

func TestParseConnectedRoutes(t *testing.T) {
	out := `default via 10.1.1.1 dev eth3
192.168.1.0/24 dev br0 proto kernel scope link src 192.168.1.1
10.1.1.0/24 dev eth3 proto kernel scope link src 10.1.1.2
10.9.9.0/24 dev thapp proto kernel scope link src 10.9.9.1
this line has no device
`
	routes := parseConnectedRoutes(out)

	want := []connectedRoute{
		{Dst: "192.168.1.0/24", Dev: "br0", Src: "192.168.1.1"},
		{Dst: "10.1.1.0/24", Dev: "eth3", Src: "10.1.1.2"},
		{Dst: "10.9.9.0/24", Dev: "thapp", Src: "10.9.9.1"},
	}
	if !reflect.DeepEqual(routes, want) {
		t.Errorf("parseConnectedRoutes = %#v, want %#v", routes, want)
	}
}

func TestParseConnectedRoutesSrcOptional(t *testing.T) {
	routes := parseConnectedRoutes("192.168.1.0/24 dev br0 proto kernel scope link\n")
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	if routes[0].Dst != "192.168.1.0/24" || routes[0].Dev != "br0" || routes[0].Src != "" {
		t.Errorf("route = %#v, want dst/dev set and empty src", routes[0])
	}
}

func TestLanCIDRsExcludesTunAndAddsConstants(t *testing.T) {
	connected := []connectedRoute{
		{Dst: "192.168.1.0/24", Dev: "br0"},
		{Dst: "10.1.1.0/24", Dev: "eth3"},
		{Dst: "10.9.9.0/24", Dev: "thapp"},
	}
	got := lanCIDRs(connected, "thapp")
	want := []string{"192.168.1.0/24", "10.1.1.0/24", "169.254.0.0/16", "224.0.0.0/4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lanCIDRs = %v, want %v", got, want)
	}
}

func TestLanCIDRsStableOrderAndDedupe(t *testing.T) {
	connected := []connectedRoute{
		{Dst: "192.168.1.0/24", Dev: "br0"},
		{Dst: "192.168.1.0/24", Dev: "br0"},
		{Dst: "169.254.0.0/16", Dev: "br0"},
		{Dst: "224.0.0.0/4", Dev: "br0"},
	}
	first := lanCIDRs(connected, "thapp")
	second := lanCIDRs(connected, "thapp")
	want := []string{"192.168.1.0/24", "169.254.0.0/16", "224.0.0.0/4"}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("lanCIDRs = %v, want %v", first, want)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("lanCIDRs is not deterministic: %v then %v", first, second)
	}
}

func TestLanCIDRsEmptyConnected(t *testing.T) {
	got := lanCIDRs(nil, "thapp")
	want := []string{"169.254.0.0/16", "224.0.0.0/4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lanCIDRs = %v, want %v", got, want)
	}
}

func TestLanRuleArgs(t *testing.T) {
	got := lanRuleArgs("192.168.1.0/24", 0)
	want := []string{"to", "192.168.1.0/24", "lookup", "main", "pref", "90"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lanRuleArgs = %v, want %v", got, want)
	}

	got = lanRuleArgs("10.1.1.0/24", 1)
	want = []string{"to", "10.1.1.0/24", "lookup", "main", "pref", "91"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lanRuleArgs = %v, want %v", got, want)
	}
}
