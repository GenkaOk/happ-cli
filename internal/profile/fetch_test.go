package profile

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetch(t *testing.T) {
	links := strings.Join([]string{
		"vless://uuid-1@a.example.com:443?type=tcp&security=reality&pbk=k#A",
		"trojan://" + "pw" + "@b.example.com:443#B",
	}, "\n")

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("subscription-userinfo", "upload=1; download=2; total=100; expire=0")
		w.Header().Set("profile-title", "base64:"+base64.StdEncoding.EncodeToString([]byte("My VPN")))
		w.Header().Set("profile-update-interval", "24")
		w.Header().Set("support-url", "https://support.example.com")
		_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(links))))
	}))
	defer srv.Close()

	sub, err := Fetch(context.Background(), srv.URL, "Happ/1.0", nil)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if gotUA != "Happ/1.0" {
		t.Errorf("User-Agent sent = %q, want Happ/1.0", gotUA)
	}
	if sub.Title != "My VPN" {
		t.Errorf("Title = %q, want 'My VPN'", sub.Title)
	}
	if sub.UpdateInterval != 24 {
		t.Errorf("UpdateInterval = %d, want 24", sub.UpdateInterval)
	}
	if sub.SupportURL != "https://support.example.com" {
		t.Errorf("SupportURL = %q", sub.SupportURL)
	}
	if sub.UserInfo == nil || sub.UserInfo.Total != 100 {
		t.Errorf("UserInfo = %+v", sub.UserInfo)
	}
	if len(sub.Servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(sub.Servers))
	}
}

func TestFetchExtraHeaders(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("vless://uuid@host:443#test"))
	}))
	defer srv.Close()

	hdrs := http.Header{
		"x-device-os": {"iOS"},
		"x-hwid":      {"deadbeef"},
		"x-client":    {"INCY"},
		"x-uuid":      {"uuid-1234"},
	}
	_, err := Fetch(context.Background(), srv.URL, "", hdrs)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if gotHeaders.Get("x-device-os") != "iOS" {
		t.Errorf("x-device-os = %q", gotHeaders.Get("x-device-os"))
	}
	if gotHeaders.Get("x-hwid") != "deadbeef" {
		t.Errorf("x-hwid = %q", gotHeaders.Get("x-hwid"))
	}
	if gotHeaders.Get("x-client") != "INCY" {
		t.Errorf("x-client = %q", gotHeaders.Get("x-client"))
	}
	if gotHeaders.Get("x-uuid") != "uuid-1234" {
		t.Errorf("x-uuid = %q", gotHeaders.Get("x-uuid"))
	}
	if gotHeaders.Get("User-Agent") != DefaultUserAgent {
		t.Errorf("User-Agent = %q, want %q", gotHeaders.Get("User-Agent"), DefaultUserAgent)
	}
}

func TestFetchDefaultUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("vless://uuid@host:443#test"))
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.URL, "", nil)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if gotUA != DefaultUserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, DefaultUserAgent)
	}
}
