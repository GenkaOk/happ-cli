package xray

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseExclusionsNormalizes(t *testing.T) {
	raw := []string{
		"google.com",
		"full:example.com",
		"keyword:ads",
		`regexp:^ssl\.google\.com$`,
		"10.1.2.3/8",
		"192.0.2.5",
	}
	got, errs := ParseExclusions(raw)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	want := []Exclusion{
		{Domain: "domain:google.com"},
		{Domain: "full:example.com"},
		{Domain: "keyword:ads"},
		{Domain: `regexp:^ssl\.google\.com$`},
		{CIDR: "10.0.0.0/8"},
		{CIDR: "192.0.2.5/32"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseExclusions = %#v, want %#v", got, want)
	}
}

func TestParseExclusionsSkipsCommentsBlankAndCRLF(t *testing.T) {
	raw := []string{
		"",
		"   ",
		"# whole-line comment",
		"\t# indented comment",
		"google.com\r",
		"example.org # trailing note",
		"keyword:#hash", // '#' not preceded by whitespace: part of the pattern
	}
	got, errs := ParseExclusions(raw)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	want := []Exclusion{
		{Domain: "domain:google.com"},
		{Domain: "domain:example.org"},
		{Domain: "keyword:#hash"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseExclusions = %#v, want %#v", got, want)
	}
}

func TestParseExclusionsRejectsIPv6(t *testing.T) {
	for _, entry := range []string{"2001:db8::1", "2001:db8::/32", "::1"} {
		got, errs := ParseExclusions([]string{entry})
		if len(got) != 0 {
			t.Errorf("ParseExclusions(%q) = %#v, want no entries", entry, got)
		}
		if len(errs) != 1 || !strings.Contains(errs[0].Error(), "IPv6") {
			t.Errorf("ParseExclusions(%q) errs = %v, want one IPv6 error", entry, errs)
		}
	}
}

func TestParseExclusionsInvalidEntriesAreReportedWithValue(t *testing.T) {
	for _, entry := range []string{"10.0.0.0/33", "domain:", "full:", "not a domain"} {
		got, errs := ParseExclusions([]string{entry})
		if len(got) != 0 {
			t.Errorf("ParseExclusions(%q) = %#v, want no entries", entry, got)
		}
		if len(errs) != 1 {
			t.Fatalf("ParseExclusions(%q) errs = %v, want exactly one", entry, errs)
		}
		if !strings.Contains(errs[0].Error(), entry) {
			t.Errorf("error %q does not name the offending entry %q", errs[0], entry)
		}
	}
}

func TestExcludedHelpers(t *testing.T) {
	ex := []Exclusion{
		{Domain: "domain:a.example"},
		{CIDR: "10.0.0.0/8"},
		{Domain: "full:b.example"},
		{CIDR: "192.0.2.5/32"},
	}
	if got, want := ExcludedDomains(ex), []string{"domain:a.example", "full:b.example"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ExcludedDomains = %v, want %v", got, want)
	}
	if got, want := ExcludedCIDRs(ex), []string{"10.0.0.0/8", "192.0.2.5/32"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ExcludedCIDRs = %v, want %v", got, want)
	}
	if got := ExcludedDomains(nil); got != nil {
		t.Errorf("ExcludedDomains(nil) = %v, want nil", got)
	}
	if got := ExcludedCIDRs(nil); got != nil {
		t.Errorf("ExcludedCIDRs(nil) = %v, want nil", got)
	}
}
