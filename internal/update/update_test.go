package update

import (
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v0.3.1", "0.3.2", -1},
		{"0.3.2", "v0.3.2", 0},
		{"v0.3.2", "v0.3.2", 0},
		{"v0.10.0", "v0.9.9", 1},
		{"1.0", "1.0.0", 0},
		{"v0.3", "v0.3.1", -1},
		{"v1.0.0", "v1.0.1", -1},
		{"v2.0.0", "v1.99.99", 1},
		// Build/pre-release suffixes are tolerated (goreleaser-style strings).
		{"v0.4.0-3-gabc123", "v0.4.0", 0},
		{"v0.4.0+build.7", "0.4.0", 0},
		{"0.4.0-rc1", "0.4.0", 0},
		{"V0.2.0", "v0.1.9", 1},
	}
	for _, tt := range tests {
		got, err := CompareVersions(tt.a, tt.b)
		if err != nil {
			t.Errorf("CompareVersions(%q, %q) error: %v", tt.a, tt.b, err)
			continue
		}
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCompareVersionsInvalid(t *testing.T) {
	tests := [][2]string{
		{"", "v1.0.0"},
		{"dev", "v1.0.0"},
		{"v1.x", "v1.0.0"},
		{"v1..0", "v1.0.0"},
		{"v1.0.0", "unknown"},
		{"v-1.0", "v1.0"},
	}
	for _, tt := range tests {
		if _, err := CompareVersions(tt[0], tt[1]); err == nil {
			t.Errorf("CompareVersions(%q, %q) = nil error, want error", tt[0], tt[1])
		}
	}
}

func TestIsDevVersion(t *testing.T) {
	dev := []string{"", "dev", "DEV", "unknown", " unknown "}
	for _, v := range dev {
		if !IsDevVersion(v) {
			t.Errorf("IsDevVersion(%q) = false, want true", v)
		}
	}
	real := []string{"v0.1.0", "0.1.0", "v1.2.3-rc1", "test"}
	for _, v := range real {
		if IsDevVersion(v) {
			t.Errorf("IsDevVersion(%q) = true, want false", v)
		}
	}
}

// allAssets is every asset the release workflow publishes.
var allAssets = []string{
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

func testRelease() *Release {
	rel := &Release{Tag: "v9.9.9"}
	for _, name := range allAssets {
		rel.Assets = append(rel.Assets, Asset{
			Name:        name,
			URL:         "https://example.com/" + name,
			ChecksumURL: "https://example.com/" + name + ".sha256",
		})
	}
	return rel
}

func TestFindAsset(t *testing.T) {
	c := &Client{}
	rel := testRelease()

	tests := []struct {
		goos, goarch, goarm string
		want                string
	}{
		{"linux", "amd64", "", "happ-linux-amd64.tar.gz"},
		{"linux", "arm64", "", "happ-linux-arm64.tar.gz"},
		{"linux", "arm", "", "happ-linux-armv7.tar.gz"}, // empty GOARM means v7
		{"linux", "arm", "5", "happ-linux-armv5.tar.gz"},
		{"linux", "arm", "6", "happ-linux-armv6.tar.gz"},
		{"linux", "arm", "7", "happ-linux-armv7.tar.gz"},
		{"linux", "arm", "v7", "happ-linux-armv7.tar.gz"},
		{"linux", "mips", "", "happ-linux-mips.tar.gz"},
		{"linux", "mipsle", "", "happ-linux-mipsle.tar.gz"},
		{"darwin", "amd64", "", "happ-darwin-amd64.tar.gz"},
		{"darwin", "arm64", "", "happ-darwin-arm64.tar.gz"},
		{"windows", "amd64", "", "happ-windows-amd64.zip"},
		{"windows", "386", "", "happ-windows-386.zip"},
	}
	for _, tt := range tests {
		asset, err := c.FindAsset(rel, tt.goos, tt.goarch, tt.goarm)
		if err != nil {
			t.Errorf("FindAsset(%s/%s/%s): %v", tt.goos, tt.goarch, tt.goarm, err)
			continue
		}
		if asset.Name != tt.want {
			t.Errorf("FindAsset(%s/%s/%s) = %s, want %s", tt.goos, tt.goarch, tt.goarm, asset.Name, tt.want)
		}
		if asset.ChecksumURL != asset.URL+".sha256" {
			t.Errorf("FindAsset(%s/%s/%s) checksum URL = %s", tt.goos, tt.goarch, tt.goarm, asset.ChecksumURL)
		}
	}
}

func TestFindAssetUnsupportedPlatform(t *testing.T) {
	c := &Client{}
	rel := testRelease()

	for _, tt := range []struct{ goos, goarch, goarm string }{
		{"freebsd", "amd64", ""},
		{"linux", "386", ""},
		{"linux", "riscv64", ""},
		{"darwin", "386", ""},
		{"windows", "arm64", ""},
		{"linux", "arm", "8"},
	} {
		_, err := c.FindAsset(rel, tt.goos, tt.goarch, tt.goarm)
		if err == nil {
			t.Errorf("FindAsset(%s/%s/%s) = nil error, want unsupported-platform error", tt.goos, tt.goarch, tt.goarm)
			continue
		}
		if !strings.Contains(err.Error(), tt.goos) {
			t.Errorf("FindAsset(%s/%s/%s) error %q does not name the platform", tt.goos, tt.goarch, tt.goarm, err)
		}
	}
}

func TestFindAssetMissingFromRelease(t *testing.T) {
	c := &Client{}
	rel := &Release{Tag: "v1.0.0"} // no assets at all
	if _, err := c.FindAsset(rel, "linux", "amd64", ""); err == nil {
		t.Fatal("FindAsset on asset-less release = nil error, want error")
	}
}
