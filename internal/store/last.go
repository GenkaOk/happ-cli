package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aimuzov/happ-cli/internal/link"
)

// LastUsed tracks which server was last attempted so the next connect can
// automatically pick the next one (round-robin failover).
type LastUsed struct {
	dir string
	key string // protocol:address:port of the last attempted server
}

// OpenLastUsed loads the last-used server key from dir/last.json.
func OpenLastUsed(dir string) (*LastUsed, error) {
	l := &LastUsed{dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, "last.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return l, nil
		}
		return nil, fmt.Errorf("last: read: %w", err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return l, nil // corrupted → start fresh
	}
	l.key = m["key"]
	return l, nil
}

// Mark stores the last attempted server.
func (l *LastUsed) Mark(s *link.Server) error {
	l.key = fmt.Sprintf("%s:%s:%d", s.Protocol, s.Address, s.Port)
	return l.save()
}

// NextServer returns the server to try next. If servers is empty or no
// previous server is known, returns the first server. Otherwise returns
// the server after the last-used one.
func (l *LastUsed) NextServer(servers []*link.Server) (*link.Server, int) {
	if len(servers) == 0 {
		return nil, 0
	}
	if l.key == "" {
		return servers[0], 0
	}
	for i, s := range servers {
		k := fmt.Sprintf("%s:%s:%d", s.Protocol, s.Address, s.Port)
		if k == l.key {
			next := (i + 1) % len(servers)
			return servers[next], next
		}
	}
	// Last-used server no longer in the list — pick the first.
	return servers[0], 0
}

func (l *LastUsed) save() error {
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return fmt.Errorf("last: mkdir: %w", err)
	}
	data, _ := json.Marshal(map[string]string{"key": l.key})
	tmp := filepath.Join(l.dir, "last.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("last: write: %w", err)
	}
	return os.Rename(tmp, filepath.Join(l.dir, "last.json"))
}
