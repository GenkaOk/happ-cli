package cli

import (
	"testing"

	"github.com/aimuzov/happ-cli/internal/link"
)

func servers() []*link.Server {
	return []*link.Server{
		{Tag: "Netherlands #1", Protocol: "vless"},
		{Tag: "Germany Frankfurt", Protocol: "trojan"},
		{Tag: "USA West", Protocol: "vmess"},
	}
}

func TestSelectServerDefaultsToFirst(t *testing.T) {
	s, idx, err := selectServer(servers(), "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 0 || s.Tag != "Netherlands #1" {
		t.Errorf("got idx=%d tag=%q", idx, s.Tag)
	}
}

func TestSelectServerByIndex(t *testing.T) {
	s, idx, err := selectServer(servers(), "2")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if idx != 1 || s.Tag != "Germany Frankfurt" {
		t.Errorf("got idx=%d tag=%q", idx, s.Tag)
	}
}

func TestSelectServerByTagSubstringCaseInsensitive(t *testing.T) {
	s, _, err := selectServer(servers(), "frankfurt")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Tag != "Germany Frankfurt" {
		t.Errorf("got tag=%q", s.Tag)
	}
}

func TestSelectServerIndexOutOfRange(t *testing.T) {
	if _, _, err := selectServer(servers(), "9"); err == nil {
		t.Error("expected error for out-of-range index")
	}
}

func TestSelectServerNoMatch(t *testing.T) {
	if _, _, err := selectServer(servers(), "antarctica"); err == nil {
		t.Error("expected error for no tag match")
	}
}

func TestSelectServerEmptyList(t *testing.T) {
	if _, _, err := selectServer(nil, ""); err == nil {
		t.Error("expected error for empty server list")
	}
}

// mockDeadCheck implements IsDeadChecker for tests.
type mockDeadCheck struct {
	dead map[string]bool
}

func (m *mockDeadCheck) IsDead(s *link.Server) bool {
	return m.dead[s.Protocol+":"+s.Address+":"+s.Tag]
}

func TestFilterAliveSkipsDead(t *testing.T) {
	all := []*link.Server{
		{Protocol: "vless", Address: "good.example.com", Tag: "good"},
		{Protocol: "vless", Address: "dead.example.com", Tag: "dead"},
	}
	dead := &mockDeadCheck{dead: map[string]bool{
		"vless:dead.example.com:dead": true,
	}}
	alive := filterAlive(all, dead, false)
	if len(alive) != 1 {
		t.Fatalf("got %d alive, want 1", len(alive))
	}
	if alive[0].Tag != "good" {
		t.Errorf("alive[0].Tag = %q", alive[0].Tag)
	}
}

func TestFilterAliveIncludeDead(t *testing.T) {
	all := []*link.Server{
		{Protocol: "vless", Address: "a", Tag: "a"},
	}
	dead := &mockDeadCheck{dead: map[string]bool{"vless:a:a": true}}
	alive := filterAlive(all, dead, true)
	if len(alive) != 1 {
		t.Errorf("includeDead should return all, got %d", len(alive))
	}
}

func TestFilterAliveAllDeadReturnsAll(t *testing.T) {
	all := []*link.Server{
		{Protocol: "vless", Address: "a", Tag: "a"},
		{Protocol: "vless", Address: "b", Tag: "b"},
	}
	dead := &mockDeadCheck{dead: map[string]bool{
		"vless:a:a": true,
		"vless:b:b": true,
	}}
	alive := filterAlive(all, dead, false)
	if len(alive) != 2 {
		t.Errorf("all-dead should return all to avoid empty list, got %d", len(alive))
	}
}

func TestFilterAliveNilChecker(t *testing.T) {
	all := []*link.Server{{Protocol: "vless", Address: "a", Tag: "a"}}
	alive := filterAlive(all, nil, false)
	if len(alive) != 1 {
		t.Errorf("nil checker should return all, got %d", len(alive))
	}
}
