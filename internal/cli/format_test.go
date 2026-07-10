package cli

import (
	"testing"
	"time"

	"github.com/aimuzov/happ-cli/internal/profile"
)

func TestFormatTrafficWithTotal(t *testing.T) {
	ui := &profile.UserInfo{Upload: 1 << 30, Download: 1 << 30, Total: 10 << 30}
	got := formatTraffic(ui)
	want := "2.0 GB / 10.0 GB"
	if got != want {
		t.Errorf("formatTraffic = %q, want %q", got, want)
	}
}

func TestFormatTrafficNoTotal(t *testing.T) {
	ui := &profile.UserInfo{Upload: 1 << 30, Download: 1 << 30, Total: 0}
	got := formatTraffic(ui)
	want := "2.0 GB"
	if got != want {
		t.Errorf("formatTraffic = %q, want %q", got, want)
	}
}

func TestFormatTrafficNil(t *testing.T) {
	if got := formatTraffic(nil); got != "-" {
		t.Errorf("formatTraffic(nil) = %q, want -", got)
	}
}

func TestExpiryString(t *testing.T) {
	if got := expiryString(time.Time{}); got != "∞" {
		t.Errorf("expiryString(zero) = %q, want ∞", got)
	}
	past := time.Now().Add(-24 * time.Hour)
	got := expiryString(past)
	if got == "∞" || got == "-" {
		t.Errorf("expiryString(past) = %q, want a date string", got)
	}
	future := time.Now().Add(9 * 24 * time.Hour)
	got = expiryString(future)
	if got == "-" || got == "∞" {
		t.Errorf("expiryString(future) = %q, want a date string", got)
	}
}

func TestDefaultName(t *testing.T) {
	tests := []struct {
		title, url, want string
	}{
		{"My VPN", "https://example.com/sub", "my-vpn"},
		{"", "https://example.com/sub", "example.com"},
		{"", "", "sub"},
	}
	for _, tt := range tests {
		got := defaultName(tt.title, tt.url)
		if got != tt.want {
			t.Errorf("defaultName(%q, %q) = %q, want %q", tt.title, tt.url, got, tt.want)
		}
	}
}
