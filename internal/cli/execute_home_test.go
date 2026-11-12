package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aimuzov/happ-cli/internal/store"
)

// ssSubscriptionServer serves a subscription body with one SIP002 ss link.
func ssSubscriptionServer(t *testing.T) *httptest.Server {
	t.Helper()
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pass"))
	body := "ss://" + userinfo + "@1.2.3.4:8388#SS\n"
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
}

// readSubNames returns the subscription names persisted in dir/state.json.
func readSubNames(t *testing.T, dir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("read state.json in %s: %v", dir, err)
	}
	var st struct {
		Subscriptions []struct {
			Name string `json:"name"`
		} `json:"subscriptions"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("decode state.json: %v", err)
	}
	names := make([]string, 0, len(st.Subscriptions))
	for _, s := range st.Subscriptions {
		names = append(names, s.Name)
	}
	return names
}

func TestExecuteHomeFlagIsolatesStore(t *testing.T) {
	// Point the *default* config location at a throwaway HOME so we can prove
	// it was not written to.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

	defaultDir, err := store.DefaultDir()
	if err != nil {
		t.Fatal(err)
	}

	srv := ssSubscriptionServer(t)
	defer srv.Close()

	dir := t.TempDir()
	err = ExecuteArgs(context.Background(), "test", []string{
		"--home", dir, "sub", "add", srv.URL, "--name", "isolated",
	})
	if err != nil {
		t.Fatalf("ExecuteArgs: %v", err)
	}

	names := readSubNames(t, dir)
	if len(names) != 1 || names[0] != "isolated" {
		t.Errorf("subscriptions in --home dir = %v, want [isolated]", names)
	}
	if _, err := os.Stat(filepath.Join(defaultDir, "state.json")); !os.IsNotExist(err) {
		t.Errorf("default dir %s was touched (stat err = %v)", defaultDir, err)
	}
}

func TestExecuteWithoutHomeUsesDefaultDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

	defaultDir, err := store.DefaultDir()
	if err != nil {
		t.Fatal(err)
	}

	srv := ssSubscriptionServer(t)
	defer srv.Close()

	err = ExecuteArgs(context.Background(), "test", []string{
		"sub", "add", srv.URL, "--name", "defaulted",
	})
	if err != nil {
		t.Fatalf("ExecuteArgs: %v", err)
	}

	names := readSubNames(t, defaultDir)
	if len(names) != 1 || names[0] != "defaulted" {
		t.Errorf("subscriptions in default dir = %v, want [defaulted]", names)
	}
}
