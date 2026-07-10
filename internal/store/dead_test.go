package store

import (
	"testing"

	"github.com/aimuzov/happ-cli/internal/link"
)

func TestDeadListMarkAndCheck(t *testing.T) {
	dir := t.TempDir()
	dl, err := OpenDeadList(dir)
	if err != nil {
		t.Fatal(err)
	}

	s1 := &link.Server{Protocol: "vless", Address: "s1.example.com", Port: 443}
	s2 := &link.Server{Protocol: "trojan", Address: "s2.example.com", Port: 443}

	if dl.IsDead(s1) {
		t.Error("new server should not be dead")
	}

	if err := dl.Mark(s1); err != nil {
		t.Fatal(err)
	}
	if !dl.IsDead(s1) {
		t.Error("marked server should be dead")
	}
	if dl.IsDead(s2) {
		t.Error("unmarked server should not be dead")
	}
}

func TestDeadListPersists(t *testing.T) {
	dir := t.TempDir()

	dl1, _ := OpenDeadList(dir)
	s := &link.Server{Protocol: "vless", Address: "host", Port: 443}
	dl1.Mark(s)

	// Re-open — must still be dead.
	dl2, err := OpenDeadList(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !dl2.IsDead(s) {
		t.Error("server not dead after re-open")
	}
}

func TestDeadListClear(t *testing.T) {
	dir := t.TempDir()
	dl, _ := OpenDeadList(dir)
	s := &link.Server{Protocol: "vless", Address: "host", Port: 443}
	dl.Mark(s)

	if err := dl.Clear(); err != nil {
		t.Fatal(err)
	}
	if dl.IsDead(s) {
		t.Error("server still dead after clear")
	}
}

func TestDeadListEmptyDir(t *testing.T) {
	dir := t.TempDir()
	dl, err := OpenDeadList(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dl.dead) != 0 {
		t.Errorf("expected empty dead list, got %d entries", len(dl.dead))
	}
}
