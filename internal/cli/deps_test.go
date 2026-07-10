package cli

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/device"
	"github.com/aimuzov/happ-cli/internal/profile"
	"github.com/aimuzov/happ-cli/internal/store"
)

// mockStore implements Store for testing.
type mockStore struct {
	subs   []store.SubEntry
	active string
}

func (m *mockStore) Subscriptions() []store.SubEntry { return m.subs }
func (m *mockStore) Find(name string) (store.SubEntry, bool) {
	for _, s := range m.subs {
		if s.Name == name {
			return s, true
		}
	}
	return store.SubEntry{}, false
}
func (m *mockStore) Upsert(entry store.SubEntry) error {
	for i, s := range m.subs {
		if s.Name == entry.Name {
			m.subs[i] = entry
			return nil
		}
	}
	m.subs = append(m.subs, entry)
	if m.active == "" {
		m.active = entry.Name
	}
	return nil
}
func (m *mockStore) Remove(name string) error {
	out := m.subs[:0]
	for _, s := range m.subs {
		if s.Name == name {
			continue
		}
		out = append(out, s)
	}
	m.subs = out
	if m.active == name {
		m.active = ""
		if len(m.subs) > 0 {
			m.active = m.subs[0].Name
		}
	}
	return nil
}
func (m *mockStore) Active() string { return m.active }
func (m *mockStore) SetActive(name string) error {
	for _, s := range m.subs {
		if s.Name == name {
			m.active = name
			return nil
		}
	}
	return fmt.Errorf("subscription %q not found", name)
}

// mockFetcher implements Fetcher for testing.
type mockFetcher struct {
	fn func(ctx context.Context, url, ua string, headers http.Header) (*profile.Subscription, error)
}

func (m *mockFetcher) Fetch(ctx context.Context, url, ua string, h http.Header) (*profile.Subscription, error) {
	return m.fn(ctx, url, ua, h)
}

// mockDevice implements DeviceProvider for testing.
type mockDevice struct{}

func (mockDevice) Load(dir string) (*device.ID, error) {
	return &device.ID{
		HWID: "test-hwid",
		UUID: "test-uuid",
	}, nil
}

func withDeps() *Deps {
	return &Deps{
		Store: &mockStore{},
		Fetch: &mockFetcher{fn: func(ctx context.Context, url, ua string, h http.Header) (*profile.Subscription, error) {
			return &profile.Subscription{}, nil
		}},
		Device: &mockDevice{},
	}
}

func TestSubListWithMock(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{Name: "vpn1", Title: "VPN One", Links: []string{"vless://uuid@host:443#tag"}},
	}
	deps.Store.(*mockStore).active = "vpn1"

	cmd := subListCmd(deps)
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("sub list failed: %v", err)
	}
}

func TestSubAddWithMock(t *testing.T) {
	deps := withDeps()
	fetcher := &mockFetcher{fn: func(ctx context.Context, url, ua string, h http.Header) (*profile.Subscription, error) {
		if h.Get("x-hwid") != "test-hwid" {
			t.Errorf("expected x-hwid=test-hwid, got %q", h.Get("x-hwid"))
		}
		if h.Get("x-client") != "INCY" {
			t.Errorf("expected x-client=INCY, got %q", h.Get("x-client"))
		}
		return &profile.Subscription{
			Title: "Mock VPN",
		}, nil
	}}
	deps.Fetch = fetcher

	cmd := subAddCmd(deps)
	cmd.SetArgs([]string{"https://example.com/sub"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sub add failed: %v", err)
	}

	subs := deps.Store.Subscriptions()
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].Title != "Mock VPN" {
		t.Errorf("Title = %q, want 'Mock VPN'", subs[0].Title)
	}
	if deps.Store.Active() != subs[0].Name {
		t.Errorf("expected active=%q, got %q", subs[0].Name, deps.Store.Active())
	}
}

func TestSubRemoveWithMock(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{Name: "vpn1"},
		{Name: "vpn2"},
	}
	deps.Store.(*mockStore).active = "vpn1"

	cmd := subRemoveCmd(deps)
	cmd.SetArgs([]string{"vpn1"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sub remove failed: %v", err)
	}

	if len(deps.Store.Subscriptions()) != 1 {
		t.Errorf("expected 1 sub after remove, got %d", len(deps.Store.Subscriptions()))
	}
	if deps.Store.Active() != "vpn2" {
		t.Errorf("expected active=vpn2 after removing active, got %q", deps.Store.Active())
	}
}

func TestConnectValidatesProtocol(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{Name: "vpn", Links: []string{"hysteria2://auth@host:443#tag"}},
	}
	deps.Store.(*mockStore).active = "vpn"

	cmd := newConnectCmd(deps)
	cmd.SetArgs([]string{"1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported protocol, got nil")
	}
}

func TestSubUpdateWithMock(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{Name: "vpn", URL: "https://example.com/sub", UserAgent: "Test/1.0"},
	}
	deps.Store.(*mockStore).active = "vpn"

	callCount := 0
	deps.Fetch = &mockFetcher{fn: func(ctx context.Context, url, ua string, h http.Header) (*profile.Subscription, error) {
		callCount++
		if url != "https://example.com/sub" {
			t.Errorf("url = %q", url)
		}
		if ua != "Test/1.0" {
			t.Errorf("ua = %q", ua)
		}
		return &profile.Subscription{Title: "Updated VPN"}, nil
	}}

	cmd := subUpdateCmd(deps)
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("sub update failed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Fetch called %d times, want 1", callCount)
	}

	sub, ok := deps.Store.Find("vpn")
	if !ok {
		t.Fatal("subscription not found after update")
	}
	if sub.Title != "Updated VPN" {
		t.Errorf("Title = %q, want 'Updated VPN'", sub.Title)
	}
}

func TestSubUpdateNonExistent(t *testing.T) {
	deps := withDeps()

	cmd := subUpdateCmd(deps)
	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for no subscriptions, got nil")
	}
}

func TestSubUseWithMock(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{Name: "vpn1"},
		{Name: "vpn2"},
	}
	deps.Store.(*mockStore).active = "vpn1"

	cmd := subUseCmd(deps)
	cmd.SetArgs([]string{"vpn2"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("sub use failed: %v", err)
	}
	if deps.Store.Active() != "vpn2" {
		t.Errorf("active = %q, want vpn2", deps.Store.Active())
	}
}

func TestSubUseNonExistent(t *testing.T) {
	deps := withDeps()

	cmd := subUseCmd(deps)
	cmd.SetArgs([]string{"nonexistent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent sub, got nil")
	}
}

func TestListServersWithMock(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{
			Name: "vpn",
			Links: []string{
				"vless://uuid1@server1.example.com:443?type=tcp&security=reality&pbk=k#Tag1",
				"trojan://pw@server2.example.com:443#Tag2",
			},
		},
	}
	deps.Store.(*mockStore).active = "vpn"

	cmd := newListCmd(deps)
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
}

func TestListServersEmpty(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{Name: "vpn"},
	}
	deps.Store.(*mockStore).active = "vpn"

	cmd := newListCmd(deps)
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("list empty failed: %v", err)
	}
}

func TestConfigDryRun(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{
			Name:  "vpn",
			Links: []string{"vless://uuid@server.example.com:443?type=tcp&security=reality&pbk=key&sid=sid&fp=chrome&flow=xtls-rprx-vision&sni=sni.example.com#Tag"},
		},
	}
	deps.Store.(*mockStore).active = "vpn"

	cmd := newConfigCmd(deps)
	cmd.SetArgs([]string{"1"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config failed: %v", err)
	}
}

func TestCompleteSubNamesWithMock(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{Name: "home", Title: "Home VPN"},
		{Name: "work"},
	}
	deps.Store.(*mockStore).active = "home"

	fn := completeSubNames(deps)
	completions, directive := fn(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d", directive)
	}
	if len(completions) != 2 {
		t.Fatalf("got %d completions, want 2", len(completions))
	}
}

func TestCompleteServerSelectorWithMock(t *testing.T) {
	deps := withDeps()
	deps.Store.(*mockStore).subs = []store.SubEntry{
		{
			Name:  "vpn",
			Links: []string{"vless://uuid@host:443?security=reality&pbk=key#MyTag"},
		},
	}
	deps.Store.(*mockStore).active = "vpn"

	fn := completeServerSelector(deps)

	// Create a cobra command with the --sub flag so Flags().GetString("sub") works.
	cmd := &cobra.Command{}
	cmd.Flags().String("sub", "", "")

	completions, directive := fn(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %d", directive)
	}
	if len(completions) != 1 {
		t.Fatalf("got %d completions, want 1", len(completions))
	}
}

func TestDefaultDepsWiring(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	deps := DefaultDeps(st)
	if deps.Store == nil {
		t.Error("Store is nil")
	}
	if deps.Fetch == nil {
		t.Error("Fetch is nil")
	}
	if deps.Device == nil {
		t.Error("Device is nil")
	}
}
