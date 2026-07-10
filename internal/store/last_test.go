package store

import (
	"testing"

	"github.com/aimuzov/happ-cli/internal/link"
)

func TestLastUsedNextServer(t *testing.T) {
	dir := t.TempDir()
	l, err := OpenLastUsed(dir)
	if err != nil {
		t.Fatal(err)
	}

	servers := []*link.Server{
		{Protocol: "vless", Address: "a.example.com", Port: 443, Tag: "A"},
		{Protocol: "trojan", Address: "b.example.com", Port: 443, Tag: "B"},
		{Protocol: "vmess", Address: "c.example.com", Port: 443, Tag: "C"},
	}

	// No last-used → first server.
	s, idx := l.NextServer(servers)
	if idx != 0 || s.Tag != "A" {
		t.Errorf("first call: idx=%d tag=%q, want idx=0 tag=A", idx, s.Tag)
	}

	// Mark the first server.
	l.Mark(servers[0])

	// Next should be second.
	s, idx = l.NextServer(servers)
	if idx != 1 || s.Tag != "B" {
		t.Errorf("after mark A: idx=%d tag=%q, want idx=1 tag=B", idx, s.Tag)
	}

	// Mark last server → wraps to first.
	l.Mark(servers[2])
	s, idx = l.NextServer(servers)
	if idx != 0 || s.Tag != "A" {
		t.Errorf("after mark C (wrap): idx=%d tag=%q, want idx=0 tag=A", idx, s.Tag)
	}
}

func TestLastUsedPersists(t *testing.T) {
	dir := t.TempDir()

	l1, _ := OpenLastUsed(dir)
	s := &link.Server{Protocol: "vless", Address: "host", Port: 443, Tag: "X"}
	l1.Mark(s)

	l2, err := OpenLastUsed(dir)
	if err != nil {
		t.Fatal(err)
	}
	servers := []*link.Server{s}
	s2, idx := l2.NextServer(servers)
	if idx != 0 {
		t.Errorf("reopen: idx=%d, want 0", idx)
	}
	_ = s2
}

func TestLastUsedServerDisappeared(t *testing.T) {
	dir := t.TempDir()
	l, _ := OpenLastUsed(dir)

	old := &link.Server{Protocol: "vless", Address: "old", Port: 443, Tag: "Old"}
	l.Mark(old)

	// Server list changed — old server gone.
	servers := []*link.Server{
		{Protocol: "trojan", Address: "new", Port: 443, Tag: "New"},
	}
	s, idx := l.NextServer(servers)
	if idx != 0 || s.Tag != "New" {
		t.Errorf("disappeared: idx=%d tag=%q, want idx=0 tag=New", idx, s.Tag)
	}
}

func TestLastUsedEmptyServers(t *testing.T) {
	dir := t.TempDir()
	l, _ := OpenLastUsed(dir)
	s, idx := l.NextServer(nil)
	if s != nil || idx != 0 {
		t.Errorf("empty: s=%v idx=%d, want nil,0", s, idx)
	}
}
