package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// updateAssets mirrors the assets published by the release workflow.
var updateAssets = []string{
	"happ-linux-amd64.tar.gz",
	"happ-linux-arm64.tar.gz",
	"happ-linux-armv5.tar.gz",
	"happ-linux-armv6.tar.gz",
	"happ-linux-armv7.tar.gz",
	"happ-linux-mips.tar.gz",
	"happ-linux-mipsle.tar.gz",
	"happ-darwin-amd64.tar.gz",
	"happ-darwin-arm64.tar.gz",
	"happ-windows-amd64.zip",
	"happ-windows-386.zip",
}

// updateAPIServer fakes the GitHub "latest release" endpoint and counts asset
// downloads (which --check must never trigger).
func updateAPIServer(t *testing.T, tag string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var downloads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			downloads.Add(1)
			http.NotFound(w, r)
			return
		}
		assets := make([]map[string]any, 0, len(updateAssets))
		for _, name := range updateAssets {
			assets = append(assets, map[string]any{
				"name":                 name,
				"browser_download_url": srvURL(r) + "/download/" + name,
				"size":                 1 << 20,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": tag,
			"assets":   assets,
		})
	}))
	return srv, &downloads
}

// srvURL reconstructs the base URL of the request's server.
func srvURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func runUpdateCmdForTest(t *testing.T, version, apiBase string, args ...string) (string, error) {
	t.Helper()
	prevVersion, prevBase := appVersion, updateAPIBase
	appVersion, updateAPIBase = version, apiBase
	defer func() { appVersion, updateAPIBase = prevVersion, prevBase }()

	cmd := newUpdateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestUpdateCheckReportsVersions(t *testing.T) {
	srv, downloads := updateAPIServer(t, "v0.2.0")
	defer srv.Close()

	out, err := runUpdateCmdForTest(t, "v0.1.0", srv.URL, "--check", "--repo", "GenkaOk/happ-cli")
	if err != nil {
		t.Fatalf("update --check: %v", err)
	}
	for _, want := range []string{"current: v0.1.0", "latest:  v0.2.0", "update available: v0.2.0 (happ-"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if n := downloads.Load(); n != 0 {
		t.Errorf("--check downloaded %d asset(s), want 0", n)
	}
}

func TestUpdateCheckAlreadyUpToDate(t *testing.T) {
	srv, downloads := updateAPIServer(t, "v0.2.0")
	defer srv.Close()

	out, err := runUpdateCmdForTest(t, "v0.2.0", srv.URL, "--check", "--repo", "GenkaOk/happ-cli")
	if err != nil {
		t.Fatalf("update --check: %v", err)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("output missing %q:\n%s", "already up to date", out)
	}
	if strings.Contains(out, "update available") {
		t.Errorf("already-current build reported an available update:\n%s", out)
	}
	if n := downloads.Load(); n != 0 {
		t.Errorf("--check downloaded %d asset(s), want 0", n)
	}
}

func TestUpdateCheckCurrentIsNewer(t *testing.T) {
	srv, _ := updateAPIServer(t, "v0.2.0")
	defer srv.Close()

	out, err := runUpdateCmdForTest(t, "v0.3.0", srv.URL, "--check", "--repo", "GenkaOk/happ-cli")
	if err != nil {
		t.Fatalf("update --check: %v", err)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("newer current version should be up to date:\n%s", out)
	}
}

func TestUpdateCheckDevBuild(t *testing.T) {
	srv, downloads := updateAPIServer(t, "v0.2.0")
	defer srv.Close()

	out, err := runUpdateCmdForTest(t, "dev", srv.URL, "--check", "--repo", "GenkaOk/happ-cli")
	if err != nil {
		t.Fatalf("update --check on dev build: %v", err)
	}
	if !strings.Contains(out, "--force") {
		t.Errorf("dev check should mention --force:\n%s", out)
	}
	if !strings.Contains(out, "update available") {
		t.Errorf("dev check should still report the available release:\n%s", out)
	}
	if n := downloads.Load(); n != 0 {
		t.Errorf("--check downloaded %d asset(s), want 0", n)
	}
}

func TestUpdateDevBuildRefusesWithoutForce(t *testing.T) {
	srv, _ := updateAPIServer(t, "v0.2.0")
	defer srv.Close()

	_, err := runUpdateCmdForTest(t, "dev", srv.URL, "--check=false", "--repo", "GenkaOk/happ-cli")
	if err == nil {
		t.Fatal("dev build install = nil error, want a --force hint")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %q, want a hint about --force", err)
	}
}

func TestUpdateNetworkErrorIsActionable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := runUpdateCmdForTest(t, "v0.1.0", srv.URL, "--check", "--repo", "GenkaOk/happ-cli")
	if err == nil {
		t.Fatal("403 from GitHub = nil error, want a rate-limit hint")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error = %q, want it to mention the rate limit", err)
	}
}
