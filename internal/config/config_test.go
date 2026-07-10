package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.Connect.Mode != "proxy" {
		t.Errorf("mode = %q", c.Connect.Mode)
	}
	if c.Connect.SocksPort != 10808 {
		t.Errorf("socks-port = %d", c.Connect.SocksPort)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	c := Default()
	c.Connect.Mode = "tun"
	c.Connect.HealthCheck = true

	if err := c.Save(dir); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Connect.Mode != "tun" {
		t.Errorf("mode = %q", loaded.Connect.Mode)
	}
	if !loaded.Connect.HealthCheck {
		t.Error("health-check not preserved")
	}
}

func TestLoadNonExistent(t *testing.T) {
	c, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c.Connect.SocksPort != 10808 {
		t.Errorf("default socks-port = %d", c.Connect.SocksPort)
	}
}

func TestLoadCorrupted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("{bad"), 0o600)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for corrupted yaml")
	}
}
