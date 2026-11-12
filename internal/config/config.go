// Package config manages happ-cli's YAML configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Connect holds default values for connect command flags.
type Connect struct {
	Mode          string `yaml:"mode"`
	SocksPort     int    `yaml:"socks-port"`
	HTTPPort      int    `yaml:"http-port"`
	HealthCheck   bool   `yaml:"health-check"`
	CheckInterval int    `yaml:"check-interval"`
	CheckURL      string `yaml:"check-url"`
	DNSProxy      bool   `yaml:"dns-proxy"`
	NoRouting     bool   `yaml:"no-routing"`
	SkipFirewall  bool   `yaml:"skip-firewall"`
	SystemProxy   bool   `yaml:"system-proxy"`
	// AutoUpdate checks GitHub releases before connecting and installs a newer
	// build in the background (best effort; the current process keeps running).
	AutoUpdate bool `yaml:"auto-update"`
	// Exclude lists domains/IPs that bypass the tunnel; ExcludeFile is a file
	// with one entry per line (default: exclude.txt next to config.yaml).
	Exclude     []string `yaml:"exclude"`
	ExcludeFile string   `yaml:"exclude-file"`
}

// File is the root config structure.
type File struct {
	Connect Connect `yaml:"connect"`
}

// Default returns a File with sensible defaults.
func Default() *File {
	return &File{
		Connect: Connect{
			Mode:          "proxy",
			SocksPort:     10808,
			HTTPPort:      10809,
			CheckInterval: 60,
			CheckURL:      "https://cloudflare.com/cdn-cgi/trace",
			DNSProxy:      true,
			AutoUpdate:    false,
		},
	}
}

// Load reads config from dir/config.yaml. Returns default if the file doesn't exist.
func Load(dir string) (*File, error) {
	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	// Start from defaults so keys absent from a partial file stay at their
	// default rather than reverting to zero values.
	c := Default()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return c, nil
}

// Save writes the config to dir/config.yaml.
func (f *File) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	path := filepath.Join(dir, "config.yaml")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("config: write: %w", err)
	}
	return os.Rename(tmp, path)
}
