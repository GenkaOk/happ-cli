package cli

import (
	"context"
	"net/http"

	"github.com/aimuzov/happ-cli/internal/device"
	"github.com/aimuzov/happ-cli/internal/profile"
	"github.com/aimuzov/happ-cli/internal/store"
)

// Store provides subscription storage. Implemented by *store.Store.
type Store interface {
	Subscriptions() []store.SubEntry
	Find(name string) (store.SubEntry, bool)
	Upsert(entry store.SubEntry) error
	Remove(name string) error
	Active() string
	SetActive(name string) error
}

// Fetcher downloads and parses subscription URLs.
type Fetcher interface {
	Fetch(ctx context.Context, subURL, userAgent string, headers http.Header) (*profile.Subscription, error)
}

// DeviceProvider loads or creates the per-machine device identity.
type DeviceProvider interface {
	Load(dir string) (*device.ID, error)
}

// realFetcher delegates to profile.Fetch.
type realFetcher struct{}

func (realFetcher) Fetch(ctx context.Context, url, ua string, h http.Header) (*profile.Subscription, error) {
	return profile.Fetch(ctx, url, ua, h)
}

// realDevice delegates to device.Load.
type realDevice struct{}

func (realDevice) Load(dir string) (*device.ID, error) {
	return device.Load(dir)
}

// Deps holds the injectable dependencies for CLI commands.
type Deps struct {
	Store  Store
	Fetch  Fetcher
	Device DeviceProvider
}

// DefaultDeps returns real implementations using the given store.
func DefaultDeps(st *store.Store) *Deps {
	return &Deps{
		Store:  st,
		Fetch:  realFetcher{},
		Device: realDevice{},
	}
}
