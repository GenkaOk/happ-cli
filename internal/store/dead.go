package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aimuzov/happ-cli/internal/link"
)

// DeadList tracks servers that have failed connectivity checks. Servers are
// identified by protocol:address:port so the blacklist persists across
// subscription updates.
type DeadList struct {
	dir  string
	dead map[string]bool
}

// OpenDeadList loads the dead-server blacklist from dir/dead.json.
func OpenDeadList(dir string) (*DeadList, error) {
	d := &DeadList{dir: dir, dead: map[string]bool{}}
	data, err := os.ReadFile(filepath.Join(dir, "dead.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return d, nil
		}
		return nil, fmt.Errorf("deadlist: read: %w", err)
	}
	if err := json.Unmarshal(data, &d.dead); err != nil {
		// Corrupted — start fresh.
		d.dead = map[string]bool{}
	}
	return d, nil
}

// key returns the identifier for a server: protocol:address:port.
func key(s *link.Server) string {
	return fmt.Sprintf("%s:%s:%d", s.Protocol, s.Address, s.Port)
}

// Mark adds a server to the dead list and persists.
func (d *DeadList) Mark(s *link.Server) error {
	d.dead[key(s)] = true
	return d.save()
}

// IsDead reports whether the server is in the dead list.
func (d *DeadList) IsDead(s *link.Server) bool {
	return d.dead[key(s)]
}

// Clear removes all entries (e.g. after a subscription update).
func (d *DeadList) Clear() error {
	d.dead = map[string]bool{}
	return d.save()
}

func (d *DeadList) save() error {
	if err := os.MkdirAll(d.dir, 0o700); err != nil {
		return fmt.Errorf("deadlist: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(d.dead, "", "  ")
	if err != nil {
		return fmt.Errorf("deadlist: marshal: %w", err)
	}
	tmp := filepath.Join(d.dir, "dead.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("deadlist: write: %w", err)
	}
	return os.Rename(tmp, filepath.Join(d.dir, "dead.json"))
}
