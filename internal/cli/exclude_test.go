package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aimuzov/happ-cli/internal/xray"
)

// setTestHome points the config dir at dir for the duration of the test.
func setTestHome(t *testing.T, dir string) {
	t.Helper()
	prev := homeDir
	homeDir = dir
	t.Cleanup(func() { homeDir = prev })
}

func TestConnectExcludeFlagsAndFileAreCombined(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "my-excludes.txt")
	if err := os.WriteFile(file, []byte("from-file.example\n10.0.0.0/8\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "connect:\n  exclude:\n    - cfg.example\n  exclude-file: " + file + "\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	setTestHome(t, dir)

	cmd := newConnectCmd(withDeps())

	inline, err := cmd.Flags().GetStringArray("exclude")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"cfg.example"}; !reflect.DeepEqual(inline, want) {
		t.Errorf("exclude flag default = %v, want %v", inline, want)
	}

	fileFlag, err := cmd.Flags().GetString("exclude-file")
	if err != nil {
		t.Fatal(err)
	}
	if fileFlag != file {
		t.Errorf("exclude-file default = %q, want %q", fileFlag, file)
	}

	entries, err := loadExcludeEntries(inline, fileFlag, dir)
	if err != nil {
		t.Fatalf("loadExcludeEntries: %v", err)
	}
	want := []string{"cfg.example", "from-file.example", "10.0.0.0/8"}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}

	exclusions, errs := xray.ParseExclusions(entries)
	if len(errs) != 0 {
		t.Errorf("unexpected parse errors: %v", errs)
	}
	if len(exclusions) != 3 || len(xray.ExcludedDomains(exclusions)) != 2 || len(xray.ExcludedCIDRs(exclusions)) != 1 {
		t.Errorf("exclusions = %#v, want 2 domains + 1 network", exclusions)
	}
}

func TestConnectExcludeFlagIsRepeatable(t *testing.T) {
	setTestHome(t, t.TempDir())
	cmd := newConnectCmd(withDeps())

	if err := cmd.Flags().Parse([]string{"--exclude", "a.example", "--exclude", "10.0.0.0/8"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	got, err := cmd.Flags().GetStringArray("exclude")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.example", "10.0.0.0/8"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("--exclude = %v, want %v", got, want)
	}
}

func TestLoadExcludeEntriesAutoFile(t *testing.T) {
	dir := t.TempDir()

	// Absent implicit file: silently ignored.
	got, err := loadExcludeEntries([]string{"cli.example"}, "", dir)
	if err != nil {
		t.Fatalf("missing auto exclude file must not error: %v", err)
	}
	if want := []string{"cli.example"}; !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}

	// Present implicit file: appended.
	if err := os.WriteFile(filepath.Join(dir, excludeFileName), []byte("# comment\ngoogle.com\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = loadExcludeEntries(nil, "", dir)
	if err != nil {
		t.Fatalf("loadExcludeEntries: %v", err)
	}
	if want := []string{"# comment", "google.com\r"}; !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
	exclusions, errs := xray.ParseExclusions(got)
	if len(errs) != 0 {
		t.Errorf("unexpected parse errors: %v", errs)
	}
	if len(exclusions) != 1 || exclusions[0].Domain != "domain:google.com" {
		t.Errorf("exclusions = %#v, want one domain:google.com", exclusions)
	}
}

func TestLoadExcludeEntriesExplicitFileMissingErrors(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.txt")

	if _, err := loadExcludeEntries(nil, missing, dir); err == nil {
		t.Errorf("explicitly given missing exclude file %q must error", missing)
	}
}
